#!/bin/bash
# ===========================================
# OTP2 Graph Build Script
# ===========================================
# This script builds the OpenTripPlanner graph for Delhi NCR
# using GTFS data and OSM extract.

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OTP_HOME="/otp"
GRAPHS_DIR="${OTP_HOME}/graphs"
GRAPH_DIR="${GRAPHS_DIR}/delhi"
DATA_DIR="${OTP_HOME}/data"

# OTP2 Configuration
OTP_VERSION="${OTP_VERSION:-2.5.0}"
JAVA_OPTS="${JAVA_OPTS:--Xmx6g -Xms2g -server -XX:+UseG1GC -XX:MaxGCPauseMillis=500}"
OTP_PORT="${OTP_PORT:-8080}"

# Timezone
TZ="${TZ:-Asia/Kolkata}"
export TZ

# URLs for data
DELHI_OSM_URL="${DELHI_OSM_URL:-https://download.geofabrik.de/asia/india/delhi-ncr-latest.osm.pbf}"
DTC_GTFS_URL="${DTC_GTFS_URL:-https://otd.delhi.gov.in/api/gtfs/static/dtc.zip}"
DMRC_GTFS_URL="${DMRC_GTFS_URL:-https://otd.delhi.gov.in/api/gtfs/static/dmrc.zip}"
DIMTS_GTFS_URL:-"${DIMTS_GTFS_URL:-https://otd.delhi.gov.in/api/gtfs/static/dimts.zip}"

# Logging
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [OTP2-BUILD] $1"
}

# Error handler
error_exit() {
    log "ERROR: $1"
    exit 1
}

# Check if running in Docker
IN_DOCKER=false
if [ -f /.dockerenv ] || grep -q docker /proc/1/cgroup 2>/dev/null; then
    IN_DOCKER=true
    log "Running inside Docker container"
fi

# Setup directories
setup_directories() {
    log "Setting up directories..."
    mkdir -p "${GRAPHS_DIR}"
    mkdir -p "${DATA_DIR}/osm"
    mkdir -p "${DATA_DIR}/gtfs"
    mkdir -p "${OTP_HOME}/logs"
}

# Download Delhi OSM extract
download_osm() {
    local osm_file="${DATA_DIR}/osm/delhi.osm.pbf"
    local osm_latest="${DATA_DIR}/osm/delhi-latest.osm.pbf"

    if [ -f "${osm_latest}" ]; then
        local age=$(($(date +%s) - $(stat -c %Y "${osm_latest}" 2>/dev/null || echo $(date +%s))))
        if [ $age -lt 86400 ]; then
            log "OSM data is recent (< 24h), skipping download"
            return 0
        fi
    fi

    log "Downloading Delhi OSM extract..."
    curl -sfL "${DELHI_OSM_URL}" -o "${osm_file}" || error_exit "Failed to download OSM data"

    if [ -f "${osm_file}" ]; then
        mv "${osm_file}" "${osm_latest}"
        log "OSM downloaded: $(du -h ${osm_latest} | cut -f1)"
    fi
}

# Download GTFS feeds
download_gtfs() {
    local gtfs_dir="${DATA_DIR}/gtfs"
    local timestamp=$(date +%Y%m%d)

    # DTC (Delhi Transport Corporation)
    if [ -n "${DTC_GTFS_URL}" ]; then
        log "Downloading DTC GTFS..."
        curl -sfL "${DTC_GTFS_URL}" -o "${gtfs_dir}/dtc_gtfs.zip" || log "Warning: Failed to download DTC GTFS"
    fi

    # DMRC (Delhi Metro)
    if [ -n "${DMRC_GTFS_URL}" ]; then
        log "Downloading DMRC GTFS..."
        curl -sfL "${DMRC_GTFS_URL}" -o "${gtfs_dir}/dmrc_gtfs.zip" || log "Warning: Failed to download DMRC GTFS"
    fi

    # DIMTS (Delhi Integrated Multi-Modal Transit System)
    if [ -n "${DIMTS_GTFS_URL}" ]; then
        log "Downloading DIMTS GTFS..."
        curl -sfL "${DIMTS_GTFS_URL}" -o "${gtfs_dir}/dimts_gtfs.zip" || log "Warning: Failed to download DIMTS GTFS"
    fi

    # Also check local GTFS directory
    if [ -d "/gtfs" ] && [ "$(ls -A /gtfs/*.zip 2>/dev/null)" ]; then
        log "Using local GTFS files from /gtfs"
        cp /gtfs/*.zip "${gtfs_dir}/" 2>/dev/null || true
    fi

    # List downloaded files
    log "GTFS files:"
    ls -lh "${gtfs_dir}"/*.zip 2>/dev/null || log "Warning: No GTFS files found"
}

# Download OTP2 JAR
download_otp() {
    local otp_jar="${OTP_HOME}/bin/otp-${OTP_VERSION}.jar"

    if [ -f "${otp_jar}" ]; then
        log "OTP2 JAR already exists"
        return 0
    fi

    log "Downloading OTP2 v${OTP_VERSION}..."
    mkdir -p "${OTP_HOME}/bin"

    curl -sfL "https://repo1.maven.org/maven2/org/opentripplanner/otp/${OTP_VERSION}/otp-${OTP_VERSION}.jar" \
        -o "${otp_jar}" || error_exit "Failed to download OTP2"

    log "OTP2 downloaded: $(du -h ${otp_jar} | cut -f1)"
}

# Build the OTP graph
build_graph() {
    local graph_version=$(date +%Y%m%d_%H%M%S)
    local build_log="${OTP_HOME}/logs/build_${graph_version}.log"

    log "Starting OTP2 graph build..."
    log "Graph version: ${graph_version}"
    log "Java options: ${JAVA_OPTS}"

    cd "${GRAPHS_DIR}"

    # Build command
    local build_cmd="java ${JAVA_OPTS} -jar ${OTP_HOME}/bin/otp-${OTP_VERSION}.jar"
    build_cmd="${build_cmd} --build ${GRAPH_DIR}"
    build_cmd="${build_cmd} --osm ${DATA_DIR}/osm/delhi-latest.osm.pbf"

    # Add GTFS feeds
    for gtfs_file in ${DATA_DIR}/gtfs/*.zip; do
        if [ -f "${gtfs_file}" ]; then
            build_cmd="${build_cmd} --gtfs ${gtfs_file}"
        fi
    done

    # Add OTP2 specific options
    build_cmd="${build_cmd} --transit"
    build_cmd="${build_cmd} --timezone ${TZ}"
    build_cmd="${build_cmd} --maxStopDistanceMultiplier 1.0"

    log "Build command prepared. Executing..."

    # Execute build
    ${build_cmd} 2>&1 | tee "${build_log}"

    if [ -f "${GRAPH_DIR}/Graph.obj" ]; then
        log "Graph build successful!"

        # Create version marker
        echo "${graph_version}" > "${GRAPH_DIR}/graph_version.txt"

        # Calculate graph size
        local graph_size=$(du -sh "${GRAPH_DIR}" | cut -f1)
        log "Graph size: ${graph_size}"

        # Create backup of previous graph if exists
        if [ -d "${GRAPHS_DIR}/delhi_previous" ]; then
            rm -rf "${GRAPHS_DIR}/delhi_previous"
        fi
        if [ -d "${GRAPH_DIR}" ]; then
            mv "${GRAPH_DIR}" "${GRAPHS_DIR}/delhi_previous"
        fi

        # Promote new graph
        mkdir -p "${GRAPH_DIR}"
        mv "${GRAPHS_DIR}/delhi_previous/"* "${GRAPH_DIR}/" 2>/dev/null || true

        return 0
    else
        log "Graph build failed - no Graph.obj found"
        return 1
    fi
}

# Verify graph
verify_graph() {
    log "Verifying graph..."

    if [ ! -f "${GRAPH_DIR}/Graph.obj" ]; then
        error_exit "Graph.obj not found"
    fi

    # Check file sizes
    local graph_size=$(stat -c%s "${GRAPH_DIR}/Graph.obj" 2>/dev/null || stat -f%z "${GRAPH_DIR}/Graph.obj")
    local min_size=$((100 * 1024 * 1024))  # 100MB minimum

    if [ "${graph_size}" -lt "${min_size}" ]; then
        log "Warning: Graph file seems too small (${graph_size} bytes)"
    fi

    log "Graph verification passed"
}

# Clean up old graphs
cleanup_old_graphs() {
    log "Cleaning up old graphs..."

    # Keep only last 3 graphs
    cd "${GRAPHS_DIR}"
    ls -dt delhi_* 2>/dev/null | tail -n +4 | xargs -r rm -rf

    # Clean up old logs
    find "${OTP_HOME}/logs" -name "*.log" -mtime +7 -delete 2>/dev/null || true

    log "Cleanup completed"
}

# Main execution
main() {
    log "=========================================="
    log "OTP2 Graph Build Script"
    log "Date: $(date)"
    log "=========================================="

    setup_directories

    if [ "${IN_DOCKER}" = "true" ]; then
        # In Docker, assume data is mounted
        log "Running in Docker mode"
    else
        # Download required data
        download_otp
        download_osm
        download_gtfs
    fi

    build_graph
    verify_graph
    cleanup_old_graphs

    log "=========================================="
    log "OTP2 Graph Build Completed Successfully"
    log "=========================================="

    if [ "${IN_DOCKER}" = "true" ]; then
        log "Starting OTP2 server..."
        exec java ${JAVA_OPTS} \
            -jar ${OTP_HOME}/bin/otp-${OTP_VERSION}.jar \
            --load ${GRAPH_DIR} \
            --port ${OTP_PORT} \
            --serveVehicles
    fi
}

# Handle arguments
case "${1:-}" in
    build)
        main
        ;;
    verify)
        verify_graph
        ;;
    cleanup)
        cleanup_old_graphs
        ;;
    download)
        download_otp
        download_osm
        download_gtfs
        ;;
    *)
        main
        ;;
esac
