"""
Sawaari WhatsApp Bot — Message Handlers
compare / book / status / help / location with NLP-lite Indian location parsing
"""

from __future__ import annotations

import logging
import re
from typing import Optional

import httpx

logger = logging.getLogger("sawaari.handlers")

# =============================================================================
# Indian location parsing — NLP-lite patterns
# =============================================================================

# Delhi-NCR metro station names and common landmarks
_KNOWN_PLACES = {
    # Metro stations
    "rajiv chowk": "Rajiv Chowk",
    "connaught place": "Connaught Place",
    "cp": "Connaught Place",
    "new delhi": "New Delhi",
    "ndls": "New Delhi Station",
    "hauz khas": "Hauz Khas",
    "green park": "Green Park",
    "saket": "Saket",
    "qutub minar": "Qutub Minar",
    "dwarka": "Dwarka",
    "janakpuri": "Janakpuri",
    "lajpat nagar": "Lajpat Nagar",
    "nehru place": "Nehru Place",
    "chandni chowk": "Chandni Chowk",
    "kashmere gate": "Kashmere Gate",
    "anand vihar": "Anand Vihar",
    "botanical garden": "Botanical Garden",
    "noida city centre": "Noida City Centre",
    "vaishali": "Vaishali",
    "gurugram": "Gurugram",
    "gurgaon": "Gurugram",
    "huda city centre": "Huda City Centre",
    "iffco chowk": "IFFCO Chowk",
    "mg road": "MG Road",
    "sikanderpur": "Sikanderpur",
    "vishwavidyalaya": "Vishwavidyalaya",
    "vidhan sabha": "Vidhan Sabha",
    "civil lines": "Civil Lines",
    "kirti nagar": "Kirti Nagar",
    "rajouri garden": "Rajouri Garden",
    "tagore garden": "Tagore Garden",
    "subhash nagar": "Subhash Nagar",
    "tilak nagar": "Tilak Nagar",
    "pink line": "Pink Line",
    "yellow line": "Yellow Line",
    "blue line": "Blue Line",
    "red line": "Red Line",
    "green line": "Green Line",
    "violet line": "Violet Line",
    "orange line": "Orange Line",
    "magenta line": "Magenta Line",
    "airport express": "Airport Express",
    # Landmarks / areas
    "aiims": "AIIMS",
    "iit delhi": "IIT Delhi",
    "du": "Delhi University",
    " Delhi university north campus": "Delhi University North Campus",
    "pvr": "PVR Cinemas",
    "select city walk": "Select City Walk",
    "dlf": "DLF",
    "ambience mall": "Ambience Mall",
    "sector 18": "Sector 18, Noida",
    "sector 62": "Sector 62, Noida",
    "sector 29": "Sector 29, Gurugram",
    "sector 14": "Sector 14, Gurugram",
    "karol bagh": "Karol Bagh",
    "paharganj": "Paharganj",
    "sarojini": "Sarojini Nagar",
    "lajpat": "Lajpat Nagar",
    "market": "Local Market",
    "isbt": "ISBT Kashmere Gate",
    "borders": "India Gate",
    "india gate": "India Gate",
    "rashtrapati bhavan": "Rashtrapati Bhavan",
    "lotus temple": "Lotus Temple",
    "akshardham": "Akshardham",
    "jama masjid": "Jama Masjid",
    "red fort": "Red Fort",
    "humayun": "Humayun's Tomb",
    "qutub": "Qutub Minar",
    "lodhi garden": "Lodhi Garden",
    "pragati maidan": "Pragati Maidan",
    "dlf phase 3": "DLF Phase 3",
    "sector 43": "Sector 43, Gurugram",
    "sector 56": "Sector 56, Gurugram",
}

# Abbreviation normalisations
_ABBREV = {
    "ggn": "Gurugram",
    "ggn": "Gurugram",
    "noi": "Noida",
    "fbd": "Faridabad",
    "ghz": "Ghaziabad",
    "cp": "Connaught Place",
    "nd": "New Delhi",
    "sp": "Saket",
    "kb": "Karol Bagh",
    "hk": "Hauz Khas",
    "rn": "Rajiv Chowk",
    "dw": "Dwarka",
    "an": "Anand Vihar",
    "kg": "Kashmere Gate",
    "np": "Nehru Place",
    "ln": "Lajpat Nagar",
}

# Regex:  "from X to Y",  "X to Y",  "X → Y",  "compare X to Y"
_RE_FROM_TO = re.compile(
    r"^(?:from\s+)?(.+?)\s+(?:to|→|->)\s+(.+)$", re.IGNORECASE
)
_RE_COMPARE = re.compile(r"^compare\s+(.+?)\s+to\s+(.+)$", re.IGNORECASE)
_RE_COORDS = re.compile(r"^(-?\d+\.?\d*)\s*,\s*(-?\d+\.?\d*)$")


# =============================================================================
# Public helpers
# =============================================================================


def normalize(text: str) -> str:
    """Normalise a raw location string: lower, strip, expand abbreviations."""
    t = text.strip().lower()
    # Expand known abbreviations
    for abbr, full in _ABBREV.items():
        if t == abbr:
            return full
    # Expand known place aliases
    for alias, canonical in _KNOWN_PLACES.items():
        if t == alias:
            return canonical
    # Capitalise each word
    return " ".join(w.capitalize() for w in t.split())


def parse_location_pair(text: str) -> Optional[tuple[str, str]]:
    """
    Extract (from, to) from natural language.
    Returns None if it can't confidently parse a pair.
    """
    text = text.strip()

    # "compare X to Y"
    m = _RE_COMPARE.match(text)
    if m:
        return normalize(m.group(1)), normalize(m.group(2))

    # "from X to Y"  or  "X to Y"
    m = _RE_FROM_TO.match(text)
    if m:
        return normalize(m.group(1)), normalize(m.group(2))

    return None


def is_coordinate_pair(text: str) -> Optional[tuple[float, float]]:
    """Return (lat, lng) if text looks like coordinates, else None."""
    m = _RE_COORDS.match(text.strip())
    if m:
        return float(m.group(1)), float(m.group(2))
    return None


# =============================================================================
# Backend caller
# =============================================================================


_BACKEND_BASE = "http://localhost:8080"
_BACKEND_TIMEOUT = httpx.Timeout(12.0, connect=3.0)


async def _backend_compare(from_loc: str, to_loc: str, prefs: Optional[dict] = None) -> dict:
    async with httpx.AsyncClient(timeout=_BACKEND_TIMEOUT) as client:
        r = await client.post(
            f"{_BACKEND_BASE}/v1/compare",
            json={"from": from_loc, "to": to_loc, "prefs": prefs or {}},
        )
        r.raise_for_status()
        return r.json()


async def _backend_book(option_id: str, user_phone: str, rail: str = "deeplink") -> dict:
    async with httpx.AsyncClient(timeout=_BACKEND_TIMEOUT) as client:
        r = await client.post(
            f"{_BACKEND_BASE}/v1/bookings",
            json={"optionId": option_id, "rail": rail, "userPhone": user_phone},
        )
        r.raise_for_status()
        return r.json()


async def _backend_status(booking_id: str) -> dict:
    async with httpx.AsyncClient(timeout=_BACKEND_TIMEOUT) as client:
        r = await client.get(f"{_BACKEND_BASE}/v1/bookings/{booking_id}")
        r.raise_for_status()
        return r.json()


# =============================================================================
# Handler functions (called from app.py)
# =============================================================================


async def compare_handler(from_loc: str, to_loc: str, prefs: Optional[dict] = None) -> str:
    """
    Main compare handler.
    1. Normalise locations
    2. Call backend /v1/compare
    3. Return formatted text message
    """
    from_loc = normalize(from_loc)
    to_loc = normalize(to_loc)

    try:
        data = await _backend_compare(from_loc, to_loc, prefs)
    except httpx.HTTPError as exc:
        logger.error("Backend compare failed: %s", exc)
        return "⚠️ Could not reach the fare engine. Please try again in a moment."
    except Exception as exc:
        logger.exception("Unexpected backend error")
        return "⚠️ Something went wrong. Please retry."

    options = data.get("options", [])
    if not options:
        return f"No transport options found from *{from_loc}* to *{to_loc}*. Try broader location names."

    # Format top 5 options
    lines = [f"*{from_loc} → {to_loc}*\n"]
    for i, opt in enumerate(options[:5], 1):
        provider = opt.get("provider", "?")
        mode = opt.get("mode", "?")
        fare_min = opt.get("fare_min", opt.get("fare", {}).get("min", "?"))
        fare_max = opt.get("fare_max", opt.get("fare", {}).get("max", ""))
        eta = opt.get("eta_minutes", opt.get("eta_seconds", 0))
        badge = opt.get("badge", "")

        if isinstance(eta, int) and eta > 100:
            eta_str = f"{eta // 60}h {eta % 60}m"
        else:
            eta_str = f"{eta} min"

        fare_str = f"₹{fare_min}"
        if fare_max and fare_max != fare_min:
            fare_str += f"-{fare_max}"

        badge_str = f" *[{badge.upper()}]*" if badge else ""
        lines.append(f"{i}. *{provider}* ({mode}){badge_str}\n   💰 {fare_str}  ⏱️ {eta_str}")

    lines.append("\n_Reply with `book <number>` to book — e.g. `book 1`_")
    return "\n".join(lines)


async def compare_handler_list(from_loc: str, to_loc: str, prefs: Optional[dict] = None) -> tuple[str, list[dict]]:
    """
    Same as compare_handler but also returns rows for an interactive List message.
    Returns (text_body, rows_for_list)
    """
    from_loc = normalize(from_loc)
    to_loc = normalize(to_loc)

    try:
        data = await _backend_compare(from_loc, to_loc, prefs)
    except httpx.HTTPError:
        raise
    except Exception:
        raise

    options = data.get("options", [])
    if not options:
        return f"No options from *{from_loc}* to *{to_loc}*.", []

    lines = [f"*{from_loc} → {to_loc}*\n"]
    rows: list[dict] = []
    for i, opt in enumerate(options[:10]):
        provider = opt.get("provider", "?")
        mode = opt.get("mode", "?")
        fare_min = opt.get("fare_min", opt.get("fare", {}).get("min", "?"))
        fare_max = opt.get("fare_max", opt.get("fare", {}).get("max", ""))
        eta = opt.get("eta_minutes", opt.get("eta_seconds", 0))
        badge = opt.get("badge", "")
        option_id = opt.get("id", f"opt_{i}")

        if isinstance(eta, int) and eta > 100:
            eta_str = f"{eta // 60}h {eta % 60}m"
        else:
            eta_str = f"{eta} min"

        fare_str = f"₹{fare_min}"
        if fare_max and fare_max != fare_min:
            fare_str += f"-{fare_max}"

        badge_str = f" [{badge.upper()}]" if badge else ""
        lines.append(f"{i+1}. *{provider}* ({mode}){badge_str}\n   💰 {fare_str}  ⏱️ {eta_str}")
        rows.append(
            {
                "id": f"book_{option_id}",
                "title": f"{provider} ({mode}){badge_str}",
                "description": f"{fare_str}  •  {eta_str}",
            }
        )

    lines.append("\n_Select an option below:_")
    return "\n".join(lines), rows


async def book_handler(option_id: str, user_phone: str, rail: str = "deeplink") -> str:
    """Book a specific option via backend."""
    try:
        result = await _backend_book(option_id, user_phone, rail)
    except httpx.HTTPStatusError as exc:
        logger.error("Booking failed HTTP %s", exc.response.status_code)
        return "⚠️ Booking service unavailable. Please try again later."
    except Exception:
        logger.exception("Booking error")
        return "⚠️ Could not complete booking. Please retry."

    lines = [
        "✅ *Booking Initiated*\n",
        f"Booking ID: `{result.get('booking_id', '?')}`",
        f"Provider: {result.get('provider', 'Unknown')}",
    ]
    if result.get("deeplink"):
        lines.append(f"\n🔗 {result['deeplink']}")
    lines.append(f"\nTrack with: `status {result.get('booking_id', '')}`")
    return "\n".join(lines)


async def status_handler(booking_id: str) -> str:
    """Check booking status via backend."""
    if not booking_id:
        return "Usage: `status <booking_id>` — e.g. `status bk_12345`"
    try:
        data = await _backend_status(booking_id)
    except httpx.HTTPStatusError:
        return f"⚠️ Booking `{booking_id}` not found. Check the ID."
    except Exception:
        return "⚠️ Could not reach booking service. Try again later."

    status = data.get("status", "unknown").upper()
    provider = data.get("provider", "?")
    lines = [
        "📋 *Booking Status*\n",
        f"ID: `{booking_id}`",
        f"Provider: {provider}",
        f"Status: *{status}*",
    ]
    eta = data.get("eta_minutes")
    if eta:
        lines.append(f"ETA: {eta} min")
    return "\n".join(lines)


def help_handler() -> str:
    """Return the help message."""
    return (
        "*Sawaari — Your Commands*\n\n"
        "🚖 compare  —  Compare fares across all modes\n"
        "   `compare Karol Bagh to Saket`\n\n"
        "📍 from / to  —  Set pickup or dropoff\n"
        "   `from Connaught Place`\n\n"
        "🎫 book  —  Book a selected option\n"
        "   `book 1`\n\n"
        "📋 status  —  Check booking status\n"
        "   `status <booking_id>`\n\n"
        "⚙️ prefs  —  Toggle preferences\n"
        "   `prefs ac` | `prefs saheli` | `prefs night`\n\n"
        "🔄 reset  —  Start over\n"
        "❓ help  —  This menu\n\n"
        "_Tip: Share your GPS location for instant pickup!_"
    )


async def location_handler(phone: str, latitude: float, longitude: float, name: str = "") -> str:
    """Acknowledge location share and prompt next step."""
    label = name or f"{latitude:.4f}, {longitude:.4f}"
    return f"📍 Got your location: *{label}*\n\nNow share your *dropoff* location."
