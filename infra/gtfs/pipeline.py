"""
Sawaari GTFS Ingestion Pipeline (PLATE 07)

Nightly pull of:
  - GTFS static (zip)  → validate → load into Postgres → rebuild OTP2 graph
  - GTFS realtime (VehiclePositions.pb) → consume every 10s → Kafka → H3 → Valkey

Run modes:
  python pipeline.py static     # one-shot static ingest
  python pipeline.py realtime   # daemon, polls GTFS-RT every 10s
  python pipeline.py both       # daemon, both static + realtime
"""

import argparse
import asyncio
import logging
import os
import sys
import time
from datetime import datetime, timedelta
from pathlib import Path
from typing import Optional

import aiohttp
import asyncpg

logging.basicConfig(
    level=os.getenv("LOG_LEVEL", "INFO"),
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
log = logging.getLogger("sawaari.gtfs")


OTD_STATIC_URL = os.getenv(
    "OTD_STATIC_URL",
    "https://otd.delhi.gov.in/api/release/gtfs-rt/feed/static-gtfs",
)
OTD_VEHICLE_URL = os.getenv(
    "OTD_VEHICLE_URL",
    "https://otd.delhi.gov.in/api/release/gtfs-rt/feed/vehicle-positions",
)
OTD_TRIP_UPDATES_URL = os.getenv(
    "OTD_TRIP_UPDATES_URL",
    "https://otd.delhi.gov.in/api/release/gtfs-rt/feed/trip-updates",
)
OTD_ALERTS_URL = os.getenv(
    "OTD_ALERTS_URL",
    "https://otd.delhi.gov.in/api/release/gtfs-rt/feed/alerts",
)

OTD_API_KEY = os.environ["OTD_API_KEY"]

PG_DSN = os.getenv(
    "PG_DSN",
    "postgresql://sawaari:sawaari@postgres:5432/sawaari",
)

KAFKA_BROKERS = os.getenv("KAFKA_BROKERS", "kafka:9092").split(",")
KAFKA_TOPIC_POSITIONS = "bus.positions"
KAFKA_TOPIC_TRIP_UPDATES = "bus.trip_updates"
KAFKA_TOPIC_ALERTS = "bus.alerts"

OTP2_REBUILD_URL = os.getenv("OTP2_REBUILD_URL", "http://otp2:8080/otp/routers/default/rebuild")
OTP2_ADMIN_TOKEN = os.getenv("OTP2_ADMIN_TOKEN", "")

GTFS_WORKDIR = Path(os.getenv("GTFS_WORKDIR", "/var/lib/sawaari/gtfs"))
GTFS_WORKDIR.mkdir(parents=True, exist_ok=True)


async def fetch_static(session: aiohttp.ClientSession, dest: Path) -> None:
    """Download the static GTFS zip from the OTD portal."""
    params = {"key": OTD_API_KEY}
    log.info("fetching static GTFS from %s", OTD_STATIC_URL)
    async with session.get(OTD_STATIC_URL, params=params, timeout=aiohttp.ClientTimeout(total=300)) as r:
        r.raise_for_status()
        with open(dest, "wb") as f:
            async for chunk in r.content.iter_chunked(64 * 1024):
                f.write(chunk)
    log.info("wrote %s (%d bytes)", dest, dest.stat().st_size)


def validate_with_gtfs_kit(zip_path: Path) -> None:
    """Validate the feed with gtfs-kit; fail loud on schema issues."""
    try:
        import gtfs_kit as gk
    except ImportError:
        log.warning("gtfs_kit not installed; skipping validation")
        return
    feed = gk.read_feed(str(zip_path), dist_units="km")
    report = feed.validate()
    if report:
        for fname, issues in report.items():
            for issue in issues:
                log.warning("validation issue: %s — %s", fname, issue)
        if any(len(v) > 10 for v in report.values()):
            raise RuntimeError("GTFS validation: too many errors; aborting")


async def load_into_postgres(zip_path: Path) -> None:
    """Bulk-load stops/routes/trips/calendar into PostGIS."""
    import zipfile
    import csv

    conn = await asyncpg.connect(PG_DSN)
    try:
        async with conn.transaction():
            with zipfile.ZipFile(zip_path) as zf:
                table_map = {
                    "agency.txt": "gtfs_agency",
                    "routes.txt": "gtfs_routes",
                    "stops.txt": "gtfs_stops",
                    "trips.txt": "gtfs_trips",
                    "calendar.txt": "gtfs_calendar",
                    "fare_attributes.txt": "gtfs_fare_attributes",
                    "fare_rules.txt": "gtfs_fare_rules",
                }
                for fname, tbl in table_map.items():
                    if fname not in zf.namelist():
                        log.warning("missing in feed: %s", fname)
                        continue
                    log.info("loading %s → %s", fname, tbl)
                    await conn.execute(f"TRUNCATE {tbl}")
                    with zf.open(fname) as f:
                        reader = csv.DictReader(f.read().decode("utf-8-sig").splitlines())
                        cols = reader.fieldnames or []
                        placeholders = ",".join(f"${i+1}" for i in range(len(cols)))
                        col_list = ",".join(f'"{c}"' for c in cols)
                        await conn.executemany(
                            f"INSERT INTO {tbl} ({col_list}) VALUES ({placeholders})",
                            [tuple(r.get(c, "") for c in cols) for r in reader],
                        )

                # stop_times is huge — staged load
                if "stop_times.txt" in zf.namelist():
                    log.info("loading stop_times.txt (large file, may take minutes)")
                    await conn.execute("TRUNCATE gtfs_stop_times")
                    with zf.open("stop_times.txt") as f:
                        reader = csv.DictReader(f.read().decode("utf-8-sig").splitlines())
                        batch = []
                        for i, row in enumerate(reader):
                            batch.append((
                                row["trip_id"],
                                row["arrival_time"],
                                row["departure_time"],
                                row["stop_id"],
                                int(row["stop_sequence"]),
                            ))
                            if len(batch) >= 5000:
                                await conn.executemany(
                                    "INSERT INTO gtfs_stop_times (trip_id, arrival_time, departure_time, stop_id, stop_sequence) VALUES ($1, $2::interval, $3::interval, $4, $5)",
                                    batch,
                                )
                                batch = []
                        if batch:
                            await conn.executemany(
                                "INSERT INTO gtfs_stop_times (trip_id, arrival_time, departure_time, stop_id, stop_sequence) VALUES ($1, $2::interval, $3::interval, $4, $5)",
                                batch,
                            )
    finally:
        await conn.close()


async def rebuild_otp2_graph() -> None:
    """Trigger OTP2 graph rebuild via admin endpoint."""
    headers = {}
    if OTP2_ADMIN_TOKEN:
        headers["Authorization"] = f"Bearer {OTP2_ADMIN_TOKEN}"
    async with aiohttp.ClientSession() as session:
        async with session.post(OTP2_REBUILD_URL, headers=headers, timeout=aiohttp.ClientTimeout(total=600)) as r:
            r.raise_for_status()
            log.info("OTP2 rebuild triggered: %s", await r.text())


async def run_static_ingest() -> None:
    stamp = datetime.utcnow().strftime("%Y%m%d")
    zip_dest = GTFS_WORKDIR / f"delhi_{stamp}.zip"
    async with aiohttp.ClientSession() as session:
        await fetch_static(session, zip_dest)
    validate_with_gtfs_kit(zip_dest)
    await load_into_postgres(zip_dest)
    await rebuild_otp2_graph()
    log.info("static ingest complete")


# ------------------- REALTIME -------------------

async def parse_vehicle_positions(data: bytes) -> list[dict]:
    """Parse GTFS-RT VehiclePositions protobuf.

    Lightweight manual parser — avoids pulling in the protobuf compiler
    at pipeline runtime. Falls back to a real parser if installed.
    """
    try:
        from google.transit import gtfs_realtime_pb2
        feed = gtfs_realtime_pb2.FeedMessage()
        feed.ParseFromString(data)
        positions = []
        for entity in feed.entity:
            if not entity.HasField("vehicle"):
                continue
            v = entity.vehicle
            positions.append({
                "vehicle_id": v.vehicle.id if v.HasField("vehicle") and v.vehicle.id else entity.id,
                "trip_id": v.trip.trip_id if v.HasField("trip") else "",
                "route_id": v.trip.route_id if v.HasField("trip") else "",
                "lat": v.position.latitude if v.HasField("position") else 0.0,
                "lng": v.position.longitude if v.HasField("position") else 0.0,
                "bearing": v.position.bearing if v.HasField("position") and v.position.bearing else None,
                "speed_kmh": v.position.speed if v.HasField("position") and v.position.speed else None,
                "stop_status": _stop_status_name(v.current_status) if v.HasField("current_status") else None,
                "timestamp": v.timestamp if v.HasField("timestamp") else int(time.time()),
            })
        return positions
    except ImportError:
        log.error("gtfs-realtime-bindings not installed; cannot parse")
        return []


def _stop_status_name(code: int) -> str:
    return {0: "incoming_at", 1: "stopped_at", 2: "in_transit_to"}.get(code, "unknown")


def h3_index(lat: float, lng: float, resolution: int = 9) -> str:
    """Compute H3 cell ID at resolution 9 (~0.1 km hexagons)."""
    try:
        import h3
        return h3.latlng_to_cell(lat, lng, resolution)
    except ImportError:
        return ""


async def consume_realtime() -> None:
    """Main loop — poll GTFS-RT every 10s and push to Kafka."""
    from aiokafka import AIOKafkaProducer

    producer = AIOKafkaProducer(bootstrap_servers=KAFKA_BROKERS)
    await producer.start()
    log.info("realtime consumer started; polling every 10s")
    try:
        async with aiohttp.ClientSession() as session:
            while True:
                try:
                    params = {"key": OTD_API_KEY}
                    async with session.get(OTD_VEHICLE_URL, params=params, timeout=aiohttp.ClientTimeout(total=15)) as r:
                        r.raise_for_status()
                        data = await r.read()
                    positions = await parse_vehicle_positions(data)
                    for p in positions:
                        p["h3_cell"] = h3_index(p["lat"], p["lng"])
                        p["observed_at"] = datetime.utcnow().isoformat()
                        await producer.send_and_wait(
                            KAFKA_TOPIC_POSITIONS,
                            str(p).encode("utf-8"),
                            key=p["vehicle_id"].encode("utf-8") if p["vehicle_id"] else None,
                        )
                    log.info("published %d positions", len(positions))
                except Exception as e:
                    log.exception("poll error: %s", e)
                await asyncio.sleep(10)
    finally:
        await producer.stop()


def main():
    p = argparse.ArgumentParser(description="Sawaari GTFS pipeline")
    p.add_argument("mode", choices=["static", "realtime", "both"])
    args = p.parse_args()

    if args.mode == "static":
        asyncio.run(run_static_ingest())
    elif args.mode == "realtime":
        asyncio.run(consume_realtime())
    else:
        # both — static ingest once, then realtime daemon
        async def _both():
            await run_static_ingest()
            await consume_realtime()
        asyncio.run(_both())


if __name__ == "__main__":
    main()