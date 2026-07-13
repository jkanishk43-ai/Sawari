-- Sawaari initial schema (PLATE 07)
-- Apply with: psql -U sawaari -d sawaari -f 001_initial.sql

BEGIN;

CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS h3;

-- ============================================================
-- USERS
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone           VARCHAR(15) UNIQUE NOT NULL,
    name            VARCHAR(120),
    saheli_flag     BOOLEAN NOT NULL DEFAULT false,
    prefs           JSONB NOT NULL DEFAULT '{}'::jsonb,
    home_stop_id    VARCHAR(64),
    work_stop_id    VARCHAR(64),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);
CREATE INDEX IF NOT EXISTS idx_users_saheli ON users(saheli_flag) WHERE saheli_flag = true;

-- ============================================================
-- TARIFFS (versioned)
-- ============================================================
CREATE TABLE IF NOT EXISTS tariffs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mode            VARCHAR(32) NOT NULL,             -- auto, cab, bus, metro, bike
    provider        VARCHAR(64),                     -- nullable for city-wide tariffs
    region          VARCHAR(64) NOT NULL DEFAULT 'delhi',
    version         INT NOT NULL,
    effective_from  DATE NOT NULL,
    effective_to    DATE,
    rules_json      JSONB NOT NULL,
    source_url      TEXT,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(mode, provider, region, version)
);

CREATE INDEX IF NOT EXISTS idx_tariffs_mode_effective
    ON tariffs(mode, region, effective_from DESC);

-- ============================================================
-- TRIPS
-- ============================================================
CREATE TABLE IF NOT EXISTS trips (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    from_text   VARCHAR(255),
    to_text     VARCHAR(255),
    from_geog   GEOGRAPHY(POINT, 4326),
    to_geog     GEOGRAPHY(POINT, 4326),
    distance_km NUMERIC(7,2),
    duration_s  INT,
    ts          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_trips_user_ts ON trips(user_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_trips_from_geog ON trips USING GIST(from_geog);
CREATE INDEX IF NOT EXISTS idx_trips_to_geog ON trips USING GIST(to_geog);

-- ============================================================
-- QUOTES
-- ============================================================
CREATE TABLE IF NOT EXISTS quotes (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id       UUID REFERENCES trips(id) ON DELETE CASCADE,
    provider      VARCHAR(64) NOT NULL,
    mode          VARCHAR(32) NOT NULL,
    fare_min      NUMERIC(10,2) NOT NULL,
    fare_max      NUMERIC(10,2) NOT NULL,
    eta_s         INT NOT NULL,
    surge         NUMERIC(4,2) NOT NULL DEFAULT 1.00,
    badge         VARCHAR(32),                       -- CHEAPEST, FASTEST, SMART_PICK
    reliability   NUMERIC(4,3),                      -- 0.000–1.000
    breakdown     JSONB NOT NULL DEFAULT '[]'::jsonb,
    deep_link     TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_quotes_trip ON quotes(trip_id);
CREATE INDEX IF NOT EXISTS idx_quotes_provider_mode ON quotes(provider, mode);

-- ============================================================
-- QUOTE FEEDBACK (the moat)
-- ============================================================
CREATE TABLE IF NOT EXISTS quote_feedback (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quote_id      UUID REFERENCES quotes(id) ON DELETE CASCADE,
    user_id       UUID REFERENCES users(id) ON DELETE SET NULL,
    actual_fare   NUMERIC(10,2) NOT NULL,
    delta         NUMERIC(10,2) GENERATED ALWAYS AS (actual_fare - (SELECT fare_min FROM quotes WHERE quotes.id = quote_id)) STORED,
    submitted_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_feedback_quote ON quote_feedback(quote_id);
CREATE INDEX IF NOT EXISTS idx_feedback_user ON quote_feedback(user_id);

-- ============================================================
-- BOOKINGS
-- ============================================================
CREATE TABLE IF NOT EXISTS bookings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quote_id        UUID REFERENCES quotes(id) ON DELETE SET NULL,
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    rail            VARCHAR(32) NOT NULL,           -- ondc, deeplink, direct
    ondc_txn_id     VARCHAR(128),
    ondc_order_id   VARCHAR(128),
    status          VARCHAR(32) NOT NULL DEFAULT 'pending',
    failure_reason  TEXT,
    deeplink        TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bookings_user_status ON bookings(user_id, status);
CREATE INDEX IF NOT EXISTS idx_bookings_quote ON bookings(quote_id);

-- ============================================================
-- TICKETS
-- ============================================================
CREATE TABLE IF NOT EXISTS tickets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id      UUID REFERENCES bookings(id) ON DELETE CASCADE,
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
    qr_payload      TEXT NOT NULL,                  -- base64 QR
    pdf_key         VARCHAR(255),                   -- S3/MinIO object key
    valid_from      TIMESTAMPTZ NOT NULL,
    valid_to        TIMESTAMPTZ NOT NULL,
    state           VARCHAR(32) NOT NULL DEFAULT 'active',  -- active, used, refunded, expired
    fare            NUMERIC(10,2) NOT NULL,
    provider        VARCHAR(64) NOT NULL,
    mode            VARCHAR(32) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_tickets_user_state ON tickets(user_id, state);
CREATE INDEX IF NOT EXISTS idx_tickets_booking ON tickets(booking_id);
CREATE INDEX IF NOT EXISTS idx_tickets_validity ON tickets(valid_to) WHERE state = 'active';

-- ============================================================
-- PROVIDERS (health, cancel rates)
-- ============================================================
CREATE TABLE IF NOT EXISTS providers (
    id              VARCHAR(64) PRIMARY KEY,        -- uber, ola, rapido, yatri, redbus, abhibus
    kind            VARCHAR(32) NOT NULL,            -- app_cab, bus, metro, train, auto, bike
    display_name    VARCHAR(120) NOT NULL,
    health_status   VARCHAR(32) NOT NULL DEFAULT 'healthy',  -- healthy, degraded, down
    cancel_rate     NUMERIC(4,3) NOT NULL DEFAULT 0.000,
    reliability     NUMERIC(4,3) NOT NULL DEFAULT 0.950,
    surge_baseline  NUMERIC(4,2) NOT NULL DEFAULT 1.00,
    booking_rail    VARCHAR(32) NOT NULL,            -- ondc, deeplink, partner_api, none
    config          JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- ALERTS
-- ============================================================
CREATE TABLE IF NOT EXISTS alerts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
    kind            VARCHAR(32) NOT NULL,            -- fare_watcher, disruption_watcher
    config          JSONB NOT NULL DEFAULT '{}'::jsonb,
    active          BOOLEAN NOT NULL DEFAULT true,
    last_fired_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alerts_user_active ON alerts(user_id, active) WHERE active = true;

-- ============================================================
-- GTFS (imported)
-- ============================================================
CREATE TABLE IF NOT EXISTS gtfs_agency (
    agency_id        VARCHAR(64) PRIMARY KEY,
    agency_name      VARCHAR(255) NOT NULL,
    agency_url       TEXT,
    agency_timezone  VARCHAR(64)
);

CREATE TABLE IF NOT EXISTS gtfs_routes (
    route_id         VARCHAR(64) PRIMARY KEY,
    agency_id        VARCHAR(64) REFERENCES gtfs_agency(agency_id),
    route_short_name VARCHAR(64),
    route_long_name  VARCHAR(255),
    route_type       INT NOT NULL,                   -- 0 tram, 1 subway, 2 rail, 3 bus, ...
    route_color      VARCHAR(7)
);

CREATE INDEX IF NOT EXISTS idx_gtfs_routes_type ON gtfs_routes(route_type);

CREATE TABLE IF NOT EXISTS gtfs_stops (
    stop_id        VARCHAR(64) PRIMARY KEY,
    stop_name      VARCHAR(255) NOT NULL,
    stop_lat       NUMERIC(10,7) NOT NULL,
    stop_lon       NUMERIC(10,7) NOT NULL,
    location       GEOGRAPHY(POINT, 4326) GENERATED ALWAYS AS (
        ST_SetSRID(ST_MakePoint(stop_lon, stop_lat), 4326)::geography
    ) STORED,
    zone_id        VARCHAR(64)
);

CREATE INDEX IF NOT EXISTS idx_gtfs_stops_loc ON gtfs_stops USING GIST(location);
CREATE INDEX IF NOT EXISTS idx_gtfs_stops_name ON gtfs_stops(stop_name);

CREATE TABLE IF NOT EXISTS gtfs_trips (
    trip_id        VARCHAR(64) PRIMARY KEY,
    route_id       VARCHAR(64) REFERENCES gtfs_routes(route_id),
    service_id     VARCHAR(64) NOT NULL,
    trip_headsign  VARCHAR(255),
    direction_id   INT
);

CREATE INDEX IF NOT EXISTS idx_gtfs_trips_route ON gtfs_trips(route_id);

CREATE TABLE IF NOT EXISTS gtfs_stop_times (
    id              BIGSERIAL PRIMARY KEY,
    trip_id         VARCHAR(64) REFERENCES gtfs_trips(trip_id) ON DELETE CASCADE,
    arrival_time    INTERVAL NOT NULL,
    departure_time  INTERVAL NOT NULL,
    stop_id         VARCHAR(64) REFERENCES gtfs_stops(stop_id),
    stop_sequence   INT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_gtfs_stop_times_trip ON gtfs_stop_times(trip_id);
CREATE INDEX IF NOT EXISTS idx_gtfs_stop_times_stop ON gtfs_stop_times(stop_id);

CREATE TABLE IF NOT EXISTS gtfs_calendar (
    service_id     VARCHAR(64) PRIMARY KEY,
    monday         BOOLEAN NOT NULL DEFAULT false,
    tuesday        BOOLEAN NOT NULL DEFAULT false,
    wednesday      BOOLEAN NOT NULL DEFAULT false,
    thursday       BOOLEAN NOT NULL DEFAULT false,
    friday         BOOLEAN NOT NULL DEFAULT false,
    saturday       BOOLEAN NOT NULL DEFAULT false,
    sunday         BOOLEAN NOT NULL DEFAULT false,
    start_date     DATE NOT NULL,
    end_date       DATE NOT NULL
);

CREATE TABLE IF NOT EXISTS gtfs_fare_attributes (
    fare_id           VARCHAR(64) PRIMARY KEY,
    price             NUMERIC(10,2) NOT NULL,
    currency_type     VARCHAR(3) NOT NULL DEFAULT 'INR',
    payment_method    INT NOT NULL DEFAULT 0,
    transfers         INT NOT NULL DEFAULT 0,
    transfer_duration INT
);

CREATE TABLE IF NOT EXISTS gtfs_fare_rules (
    id          BIGSERIAL PRIMARY KEY,
    fare_id     VARCHAR(64) REFERENCES gtfs_fare_attributes(fare_id),
    route_id    VARCHAR(64) REFERENCES gtfs_routes(route_id),
    origin_id   VARCHAR(64),
    destination_id VARCHAR(64),
    contains_id VARCHAR(64)
);

-- ============================================================
-- POSITION HISTORY (ClickHouse mirror table for joins)
-- ============================================================
CREATE TABLE IF NOT EXISTS position_history (
    vehicle_id   VARCHAR(64) NOT NULL,
    trip_id      VARCHAR(64),
    route_id     VARCHAR(64) NOT NULL,
    lat          NUMERIC(10,7) NOT NULL,
    lng          NUMERIC(10,7) NOT NULL,
    bearing      NUMERIC(5,2),
    speed_kmh    NUMERIC(5,2),
    h3_cell      VARCHAR(16),
    stop_status  VARCHAR(32),
    observed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pos_route_time ON position_history(route_id, observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_pos_h3 ON position_history(h3_cell);
-- Partition by month when volume grows (pg_partman or native partitioning)

-- ============================================================
-- SEED: providers from PLATE 06
-- ============================================================
INSERT INTO providers (id, kind, display_name, booking_rail) VALUES
    ('uber', 'app_cab', 'Uber', 'deeplink'),
    ('ola', 'app_cab', 'Ola', 'deeplink'),
    ('rapido', 'app_cab', 'Rapido', 'deeplink'),
    ('yatri', 'app_cab', 'Namma Yatri', 'ondc'),
    ('namma_yatri', 'app_cab', 'Namma Yatri', 'ondc'),
    ('metro', 'metro', 'Delhi Metro', 'ondc'),
    ('dmrc', 'metro', 'DMRC', 'ondc'),
    ('dtc_bus', 'bus', 'DTC Bus', 'ondc'),
    ('dimts_bus', 'bus', 'DIMTS Cluster Bus', 'ondc'),
    ('cluster_bus', 'bus', 'Cluster Bus', 'ondc'),
    ('meter_auto', 'auto', 'Meter Auto (kaali-peeli)', 'none'),
    ('redbus', 'bus', 'Redbus (intercity)', 'deeplink'),
    ('abhibus', 'bus', 'Abhibus (intercity)', 'deeplink'),
    ('irctc', 'train', 'Indian Railways', 'partner_api'),
    ('erickshaw', 'auto', 'E-Rickshaw', 'none')
ON CONFLICT (id) DO NOTHING;

-- SEED: Aug-2025 metro slabs (DMRC)
INSERT INTO tariffs (mode, provider, region, version, effective_from, rules_json, source_url, notes) VALUES
    ('metro', 'dmrc', 'delhi', 1, '2025-08-01', '{
        "slabs": [
            {"max_km": 2,   "fare": 11},
            {"max_km": 5,   "fare": 21},
            {"max_km": 12,  "fare": 32},
            {"max_km": 21,  "fare": 43},
            {"max_km": 32,  "fare": 54},
            {"max_km": 999, "fare": 64}
        ],
        "saheli": false
    }'::jsonb, 'https://www.delhimetrorail.com', 'DMRC Aug-2025 fare slabs'),
    ('auto', NULL, 'delhi', 1, '2023-01-01', '{
        "min_fare": 25,
        "per_km": 11.50,
        "waiting_per_min": 0.75,
        "night_multiplier": 1.25,
        "luggage_per_kg": 0,
        "notes": "Jan-2023 notified tariff; first 1.5km ₹25, then ₹11.50/km"
    }'::jsonb, 'https://transport.delhi.gov.in', 'Notified Jan-2023 auto-rickshaw tariff'),
    ('bus', 'dtc_bus', 'delhi', 1, '2024-01-01', '{
        "slabs": [
            {"type": "ordinary", "min_fare": 5, "max_fare": 15},
            {"type": "ac",       "min_fare": 10, "max_fare": 25}
        ],
        "saheli_free": true
    }'::jsonb, 'https://dtc.delhi.gov.in', 'DTC ordinary/AC slabs; Saheli free rides'),
    ('bus', 'dimts_bus', 'delhi', 1, '2024-01-01', '{
        "slabs": [
            {"type": "ordinary", "min_fare": 5, "max_fare": 15},
            {"type": "ac",       "min_fare": 10, "max_fare": 25}
        ],
        "saheli_free": true
    }'::jsonb, 'https://dimts.in', 'Cluster bus ordinary/AC slabs; Saheli free rides')
ON CONFLICT DO NOTHING;

-- updated_at trigger for users
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_users_updated ON users;
CREATE TRIGGER trg_users_updated BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_bookings_updated ON bookings;
CREATE TRIGGER trg_bookings_updated BEFORE UPDATE ON bookings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_providers_updated ON providers;
CREATE TRIGGER trg_providers_updated BEFORE UPDATE ON providers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;

-- Verification queries (run after applying):
-- SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename;
-- SELECT COUNT(*) FROM providers;
-- SELECT mode, version, effective_from FROM tariffs ORDER BY mode, version DESC;