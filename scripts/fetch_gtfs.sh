#!/bin/bash
# ===========================================
# GTFS Fetch Script - Nightly GTFS data fetch from OTD Portal
# ===========================================
# This script fetches GTFS static and realtime data from Delhi OTD portal
# and uploads to MinIO for downstream processing.

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="/gtfs"
LOG_FILE="${DATA_DIR}/logs/fetch_gtfs_$(date +%Y%m%d_%H%M%S).log"

# OTD API Configuration
OTD_API_KEY="${OTD_API_KEY:-}"
OTD_STATIC_URL="${OTD_STATIC_URL:-https://otd.delhi.gov.in/api/gtfs/static}"
OTD_RT_URL="${OTD_RT_URL:-https://otd.delhi.gov.in/api/gtfs/realtime}"

# MinIO Configuration
MINIO_ENDPOINT="${MINIO_ENDPOINT:-minio:9000}"
MINIO_ACCESS_KEY="${MINIO_ACCESS_KEY:-minioadmin}"
MINIO_SECRET_KEY="${MINIO_SECRET_KEY:-minioadmin_secret}"
MINIO_BUCKET="${MINIO_BUCKET:-sawaari-gtfs}"

# Timestamps
DATE=$(date +%Y%m%d)
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "${LOG_FILE}"
}

# Error handler
error_exit() {
    log "ERROR: $1"
    exit 1
}

# Check dependencies
check_dependencies() {
    log "Checking dependencies..."

    for cmd in curl jq mc; do
        if ! command -v $cmd &> /dev/null; then
            log "Installing $cmd..."
            apk add --no-cache $cmd 2>/dev/null || apt-get install -y $cmd 2>/dev/null
        fi
    done
}

# Setup directories
setup_directories() {
    log "Setting up directories..."
    mkdir -p "${DATA_DIR}/static"
    mkdir -p "${DATA_DIR}/realtime"
    mkdir -p "${DATA_DIR}/logs"
    mkdir -p "${DATA_DIR}/backup"
}

# Configure MinIO client
configure_minio() {
    log "Configuring MinIO client..."
    mc alias set local ${MINIO_ENDPOINT} ${MINIO_ACCESS_KEY} ${MINIO_SECRET_KEY} --api APIv2 || true
    mc bucket create local/${MINIO_BUCKET} --ignore-existing || true
}

# Fetch static GTFS data
fetch_static_gtfs() {
    log "Fetching static GTFS data from OTD portal..."

    local static_file="${DATA_DIR}/static/gtfs_static_${TIMESTAMP}.zip"
    local latest_file="${DATA_DIR}/static/gtfs_static_latest.zip"

    # Fetch from OTD
    if [ -n "${OTD_API_KEY}" ]; then
        curl -sf -o "${static_file}" \
            -H "X-API-Key: ${OTD_API_KEY}" \
            "${OTD_STATIC_URL}" || {
                log "Warning: Failed to fetch from OTD, trying alternative source..."
                # Alternative: Download from transitfeeds or equivalent
                curl -sf -o "${static_file}" \
                    "https://otp.delhi.gov.in/gtfs/static.zip" || true
            }
    else
        log "Warning: No OTD API key set, skipping static GTFS fetch"
        return 1
    fi

    if [ -f "${static_file}" ] && [ -s "${static_file}" ]; then
        log "Static GTFS downloaded successfully: $(du -h ${static_file} | cut -f1)"

        # Validate ZIP
        if unzip -t "${static_file}" > /dev/null 2>&1; then
            # Backup previous
            [ -f "${latest_file}" ] && mv "${latest_file}" "${DATA_DIR}/backup/gtfs_static_$(date -r ${latest_file} +%Y%m%d).zip"

            # Move to latest
            mv "${static_file}" "${latest_file}"

            # Upload to MinIO
            log "Uploading to MinIO..."
            mc cp "${latest_file}" "local/${MINIO_BUCKET}/static/gtfs_static_${TIMESTAMP}.zip"

            # Keep only last 7 versions
            mc ls "local/${MINIO_BUCKET}/static/" | tail -n +8 | awk '{print $NF}' | xargs -I {} mc rm "local/${MINIO_BUCKET}/static/{}" 2>/dev/null || true

            log "Static GTFS sync completed successfully!"
        else
            log "Warning: Downloaded file is not a valid ZIP"
            rm -f "${static_file}"
        fi
    else
        log "Warning: Failed to download static GTFS"
    fi
}

# Fetch GTFS-RT data (VehiclePositions.pb)
fetch_realtime_gtfs() {
    log "Fetching realtime GTFS data..."

    local rt_file="${DATA_DIR}/realtime/vehicle_positions_${TIMESTAMP}.pb"
    local latest_file="${DATA_DIR}/realtime/vehicle_positions_latest.pb"

    if [ -n "${OTD_API_KEY}" ]; then
        curl -sf -o "${rt_file}" \
            -H "X-API-Key: ${OTD_API_KEY}" \
            "${OTD_RT_URL}/vehiclepositions.pb" || {
                log "Warning: Failed to fetch realtime data"
                return 1
            }
    else
        log "Warning: No OTD API key, skipping realtime fetch"
        return 1
    fi

    if [ -f "${rt_file}" ] && [ -s "${rt_file}" ]; then
        log "Realtime data downloaded: $(du -h ${rt_file} | cut -f1)"

        # Upload to MinIO (for Kafka consumer to pick up)
        mc cp "${rt_file}" "local/${MINIO_BUCKET}/realtime/vehicle_positions_${TIMESTAMP}.pb"

        # Update symlink to latest
        ln -sf "${rt_file}" "${latest_file}"

        log "Realtime GTFS sync completed!"
    fi
}

# Validate GTFS data
validate_gtfs() {
    log "Validating GTFS data..."

    local latest="${DATA_DIR}/static/gtfs_static_latest.zip"

    if [ ! -f "${latest}" ]; then
        log "Warning: No GTFS file to validate"
        return 1
    fi

    # Basic validation
    if unzip -l "${latest}" | grep -q "agency.txt" && \
       unzip -l "${latest}" | grep -q "stops.txt" && \
       unzip -l "${latest}" | grep -q "routes.txt" && \
       unzip -l "${latest}" | grep -q "trips.txt" && \
       unzip -l "${latest}" | grep -q "stop_times.txt"; then
        log "GTFS validation passed: All required files present"
        return 0
    else
        log "Warning: GTFS validation failed - missing required files"
        return 1
    fi
}

# Notify downstream services
notify_downstream() {
    log "Notifying downstream services..."

    # Update the trigger file in MinIO
    echo "{\"timestamp\": \"${TIMESTAMP}\", \"type\": \"gtfs_update\"}" | \
        mc pipe "local/${MINIO_BUCKET}/triggers/gtfs_update_${TIMESTAMP}.json"

    # In production, this would trigger:
    # 1. Kafka message to start OTP2 graph rebuild
    # 2. SNS/SQS notification for other consumers
}

# Main execution
main() {
    log "========================================---"
    log "GTFS Fetch Script Started"
    log "Timestamp: ${TIMESTAMP}"
    log "========================================---"

    check_dependencies
    setup_directories
    configure_minio

    fetch_static_gtfs
    validate_gtfs
    fetch_realtime_gtfs
    notify_downstream

    log "========================================---"
    log "GTFS Fetch Script Completed"
    log "========================================---"
}

# Run
main "$@"
