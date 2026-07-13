# Sawaari Database Migration Strategy

## Overview

This directory contains database migrations for the Sawaari platform. We use a **versioned migration approach** with **golang-migrate** for the Go backend.

## Migration Tool

We use [golang-migrate](https://github.com/golang-migrate/migrate) for database migrations:

```bash
# Install migrate CLI
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations
migrate -path ./infra/migrations -database $DATABASE_URL up

# Rollback one migration
migrate -path ./infra/migrations -database $DATABASE_URL down 1

# Check current version
migrate -path ./infra/migrations -database $DATABASE_URL version
```

## Migration Naming Convention

Migrations follow the format: `{version}_{name}.up.sql` and `{version}_{name}.down.sql`

Examples:
- `001_initial_schema.up.sql` / `001_initial_schema.down.sql`
- `002_add_gtfs_tables.up.sql` / `002_add_gtfs_tables.down.sql`

## Migration Order

### Phase 1: Core Schema
1. `001_initial_schema.up.sql` - Users, trips, quotes, bookings
2. `002_add_tariffs.up.sql` - Fare tariff tables with versioning
3. `003_add_providers.up.sql` - Provider registry

### Phase 2: GTFS Data
4. `004_add_gtfs_schema.up.sql` - GTFS static tables (routes, stops, trips)
5. `005_add_gtfs_realtime.up.sql` - GTFS-RT tables (vehicle positions)

### Phase 3: Features
6. `006_add_wallet.up.sql` - Wallet and tickets
7. `007_add_alerts.up.sql` - Alert configurations
8. `008_add_geocoding.up.sql` - Gazetteer and geocoding cache

### Phase 4: Analytics
9. `009_add_analytics.up.sql` - ClickHouse materialized views setup

## Core Tables

### Users
```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone VARCHAR(15) UNIQUE NOT NULL,
    email VARCHAR(255),
    saheli_flag BOOLEAN DEFAULT FALSE,
    prefs JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Trips
```sql
CREATE TABLE trips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    from_lat DOUBLE PRECISION NOT NULL,
    from_lng DOUBLE PRECISION NOT NULL,
    from_name VARCHAR(255),
    to_lat DOUBLE PRECISION NOT NULL,
    to_lng DOUBLE PRECISION NOT NULL,
    to_name VARCHAR(255),
    scheduled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Quotes
```sql
CREATE TABLE quotes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID REFERENCES trips(id),
    provider VARCHAR(50) NOT NULL,
    mode VARCHAR(50) NOT NULL,
    fare_min DECIMAL(10,2),
    fare_max DECIMAL(10,2),
    eta_seconds INTEGER,
    surge_multiplier DECIMAL(3,2) DEFAULT 1.0,
    breakdown JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Tariffs
```sql
CREATE TABLE tariffs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mode VARCHAR(50) NOT NULL,
    version INTEGER NOT NULL,
    effective_from DATE NOT NULL,
    effective_to DATE,
    rules JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(mode, version)
);
```

### Bookings
```sql
CREATE TABLE bookings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quote_id UUID REFERENCES quotes(id),
    rail VARCHAR(20) NOT NULL, -- 'deeplink', 'ondc', 'qr'
    external_id VARCHAR(255),
    ondc_txn_id VARCHAR(255),
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

## PostGIS Extensions

The migrations automatically enable PostGIS:
```sql
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS postgis_topology;
```

Geographic queries use PostGIS functions:
```sql
-- Find stops within 500m of a point
SELECT * FROM gtfs_stops
WHERE ST_DWithin(
    stop_location::geography,
    ST_SetSRID(ST_MakePoint(lng, lat), 4326)::geography,
    500
);
```

## Migration in Docker Compose

Migrations run automatically on container startup via the backend service init script:

```yaml
# In docker-compose.yml
sawaari-backend:
  volumes:
    - ./infra/migrations:/migrations:ro
  command: >
    sh -c "migrate -path /migrations -database $DATABASE_URL up && ./sawaari-backend serve"
```

## Rollback Policy

- Never delete migration files after they've been applied to production
- Always create a corresponding `.down.sql` file
- Test rollbacks in staging before applying to production
- For critical schema changes, create a backup before migrating:

```bash
pg_dump -h localhost -U sawaari sawaari > backup_$(date +%Y%m%d_%H%M%S).sql
```

## Adding New Migrations

1. Create new migration files in this directory
2. Use the next sequential version number
3. Test both up and down migrations
4. Commit with descriptive message

## Environment Variables

Migrations require:
- `DATABASE_URL` - PostgreSQL connection string
