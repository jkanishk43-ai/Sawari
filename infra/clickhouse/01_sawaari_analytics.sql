-- Sawaari ClickHouse analytics schema
-- Apply with: clickhouse-client < 01_sawaari_analytics.sql

CREATE DATABASE IF NOT EXISTS sawaari;

-- ============================================================
-- BUS POSITIONS — raw vehicle position stream from GTFS-RT
-- ============================================================
CREATE TABLE IF NOT EXISTS sawaari.bus_positions
(
    vehicle_id    String,
    trip_id       String,
    route_id      String,
    lat           Float64,
    lng           Float64,
    bearing       Nullable(Float32),
    speed_kmh     Nullable(Float32),
    h3_cell       String,
    stop_status   LowCardinality(String),
    observed_at   DateTime64(3) CODEC(DoubleDelta, ZSTD(1))
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(observed_at)
ORDER BY (route_id, observed_at)
TTL observed_at + INTERVAL 90 DAY;

CREATE INDEX idx_bus_h3 ON sawaari.bus_positions (h3_cell) TYPE bloom_filter(0.01) GRANULARITY 4;

-- ============================================================
-- QUOTES ISSUED — every quote the orchestrator returns
-- ============================================================
CREATE TABLE IF NOT EXISTS sawaari.quotes_issued
(
    request_id     UUID,
    user_id        Nullable(UUID),
    trip_id        Nullable(UUID),
    provider       LowCardinality(String),
    mode           LowCardinality(String),
    fare_min       Decimal(10, 2),
    fare_max       Decimal(10, 2),
    eta_s          UInt16,
    surge          Decimal(4, 2),
    badge          LowCardinality(String),
    reliability    Nullable(Decimal(4, 3)),
    rank           UInt8,
    score          Float32,
    from_lat       Float64,
    from_lng       Float64,
    to_lat         Float64,
    to_lng         Float64,
    prefs          String,
    issued_at      DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(issued_at)
ORDER BY (provider, issued_at);

-- ============================================================
-- QUOTE FEEDBACK — actual fares paid
-- ============================================================
CREATE TABLE IF NOT EXISTS sawaari.quote_feedback
(
    request_id     UUID,
    quote_id       UUID,
    user_id        Nullable(UUID),
    provider       LowCardinality(String),
    mode           LowCardinality(String),
    quoted_min     Decimal(10, 2),
    quoted_max     Decimal(10, 2),
    actual_fare    Decimal(10, 2),
    delta          Decimal(10, 2),
    submitted_at   DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(submitted_at)
ORDER BY (provider, submitted_at);

-- ============================================================
-- CORRIDOR RELIABILITY — per-corridor cancel rates, ETAs
-- ============================================================
CREATE TABLE IF NOT EXISTS sawaari.corridor_reliability
(
    provider       LowCardinality(String),
    mode           LowCardinality(String),
    corridor       String,  -- hex of midpoint, h3 res 6
    hour_of_day    UInt8,
    sample_count   UInt32,
    avg_eta_s      UInt16,
    cancel_rate    Decimal(4, 3),
    avg_surge      Decimal(4, 2),
    computed_at    DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(computed_at)
ORDER BY (provider, mode, corridor, hour_of_day);

-- ============================================================
-- BOOKINGS
-- ============================================================
CREATE TABLE IF NOT EXISTS sawaari.bookings
(
    booking_id     UUID,
    user_id        UUID,
    provider       LowCardinality(String),
    rail           LowCardinality(String),
    ondc_txn_id    Nullable(String),
    ondc_order_id  Nullable(String),
    status         LowCardinality(String),
    fare           Decimal(10, 2),
    from_lat       Float64,
    from_lng       Float64,
    to_lat         Float64,
    to_lng         Float64,
    created_at     DateTime DEFAULT now(),
    updated_at     DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(created_at)
ORDER BY (user_id, created_at);

-- ============================================================
-- ALERTS FIRED
-- ============================================================
CREATE TABLE IF NOT EXISTS sawaari.alerts_fired
(
    alert_id       UUID,
    user_id        UUID,
    kind           LowCardinality(String),
    channels       Array(LowCardinality(String)),
    fired_at       DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(fired_at)
ORDER BY (user_id, fired_at);

-- ============================================================
-- TARIFF CHANGES — track when notified tariffs change
-- ============================================================
CREATE TABLE IF NOT EXISTS sawaari.tariff_changes
(
    mode           LowCardinality(String),
    provider       LowCardinality(String),
    region         LowCardinality(String),
    version        UInt32,
    effective_from Date,
    diff_json      String,
    source_url     String,
    detected_at    DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(detected_at)
ORDER BY (mode, provider, region, version);

-- ============================================================
-- MATERIALIZED VIEW — quote accuracy by provider/hour
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS sawaari.quote_accuracy_hourly
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(hour)
ORDER BY (provider, mode, hour)
AS
SELECT
    toStartOfHour(qf.submitted_at)        AS hour,
    qf.provider                           AS provider,
    qf.mode                               AS mode,
    count()                               AS samples,
    avg(qf.delta)                         AS avg_delta,
    quantile(0.5)(qf.delta)               AS median_delta,
    quantile(0.95)(abs(qf.delta))         AS p95_abs_error,
    sum(if(qf.actual_fare > qf.quoted_max, 1, 0)) AS overrun_count
FROM sawaari.quote_feedback qf
GROUP BY hour, provider, mode;

-- ============================================================
-- MATERIALIZED VIEW — corridor demand heatmap
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS sawaari.corridor_demand_hourly
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(hour)
ORDER BY (provider, hour, corridor)
AS
SELECT
    toStartOfHour(issued_at)              AS hour,
    provider,
    mode,
    geoToH3(toFloat64(toFloat32(from_lat)), toFloat64(toFloat32(from_lng)), 6) AS from_h3,
    geoToH3(toFloat64(toFloat32(to_lat)),   toFloat64(toFloat32(to_lng)),   6) AS to_h3,
    cityHash64(concat(geoToH3(toFloat64(toFloat32(from_lat)), toFloat64(toFloat32(from_lng)), 6), geoToH3(toFloat64(toFloat32(to_lat)), toFloat64(toFloat32(to_lng)), 6))) AS corridor,
    count()                                AS quote_count
FROM sawaari.quotes_issued
GROUP BY hour, provider, mode, from_h3, to_h3, corridor;