"""
Sawaari MCP Server

A FastMCP server that exposes ride-hailing and transit comparison tools
for the Sawaari fare comparison ecosystem (Delhi multimodal transport).

Tools wrap calls to the Go backend REST API endpoints.
"""

import os
import httpx
from typing import Optional
from pydantic import BaseModel, Field
from fastmcp import FastMCP

# Initialize FastMCP server
mcp = FastMCP(
    "Sawaari",
    description="Delhi multimodal fare comparison and booking - compare rides across providers, book trips, get real-time ETAs and fare information.",
    dependencies=["httpx", "pydantic", "python-dotenv"]
)

# Load environment variables
from dotenv import load_dotenv
load_dotenv()

# Configuration
BACKEND_URL = os.getenv("SAWAARI_BACKEND_URL", "http://localhost:8080")
API_KEY = os.getenv("SAWAARI_API_KEY", "")

# HTTP client configuration
HTTP_TIMEOUT = 30.0


# =============================================================================
# Pydantic Models for request/response validation
# =============================================================================

class CompareTripPreferences(BaseModel):
    """User preferences for trip comparison."""
    ac: bool = Field(default=False, description="Prefer AC vehicles")
    saheli: bool = Field(default=False, description="Use Saheli women-only service (Aug 2026)")
    night: bool = Field(default=False, description="Night trip (10PM - 6AM)")
    surge_hint: bool = Field(default=False, description="Consider current surge pricing")


class CompareTripResult(BaseModel):
    """Result of a trip comparison."""
    success: bool
    options: list[dict] = Field(default_factory=list)
    cheapest: Optional[dict] = None
    fastest: Optional[dict] = None
    smart_pick: Optional[dict] = None


class BookRideResult(BaseModel):
    """Result of a booking attempt."""
    success: bool
    booking_id: Optional[str] = None
    status: str
    provider: str
    deeplink: Optional[str] = None
    message: str


class EtaResult(BaseModel):
    """ETA information for a route."""
    route_id: str
    eta_seconds: int
    eta_formatted: str
    distance_km: float
    live: bool
    last_updated: Optional[str] = None


class FareInfo(BaseModel):
    """Fare information for a transport mode."""
    mode: str
    provider: str
    base_fare: float
    per_km: float
    per_minute: Optional[float] = None
    currency: str = "INR"
    notes: Optional[str] = None


# =============================================================================
# Helper Functions
# =============================================================================

async def make_backend_request(
    method: str,
    endpoint: str,
    json_data: Optional[dict] = None,
    params: Optional[dict] = None
) -> dict:
    """
    Make an authenticated request to the Sawaari backend.

    Args:
        method: HTTP method (GET, POST, etc.)
        endpoint: API endpoint path (e.g., "/v1/compare")
        json_data: JSON body for POST requests
        params: Query parameters

    Returns:
        JSON response from backend

    Raises:
        httpx.HTTPStatusError: On HTTP errors
        httpx.RequestError: On network errors
    """
    url = f"{BACKEND_URL}{endpoint}"
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"

    async with httpx.AsyncClient(timeout=HTTP_TIMEOUT) as client:
        response = await client.request(
            method=method,
            url=url,
            json=json_data,
            params=params,
            headers=headers
        )
        response.raise_for_status()
        return response.json()


def format_error(error: Exception) -> str:
    """Format error messages for user-friendly output."""
    if isinstance(error, httpx.HTTPStatusError):
        status = error.response.status_code
        try:
            detail = error.response.json().get("error", error.response.text)
        except Exception:
            detail = error.response.text
        return f"Backend error ({status}): {detail}"
    elif isinstance(error, httpx.RequestError):
        return f"Network error: Unable to reach backend at {BACKEND_URL}. Is the server running?"
    else:
        return f"Error: {str(error)}"


# =============================================================================
# MCP Tools
# =============================================================================

@mcp.tool(
    name="compare_trip",
    description="""Compare fares across all available transport providers for a trip.

    Analyzes routes and pricing for multiple transport modes including:
    - App cabs: Uber, Ola, Rapido
    - Public transit: DTC buses, DMRC Metro
    - Auto rickshaws: Metered and app-based
    - Intercity: Redbus, Indian Railways

    Returns ranked options with CHEAPEST, FASTEST, and SMART PICK badges.
    The SMART PICK score combines fare and ETA: 0.55*fare_normalized + 0.45*eta_normalized.

    Example:
        compare_trip(
            from="Connaught Place, Delhi",
            to="IGI Airport Terminal 3",
            preferences={"ac": true, "saheli": false, "night": false}
        )""",
    arguments={
        "from": "Origin location - can be address, landmark, or coordinates (lat,lng)",
        "to": "Destination location - can be address, landmark, or coordinates (lat,lng)",
        "preferences": {
            "type": "object",
            "properties": {
                "ac": {"type": "boolean", "description": "Prefer AC vehicles"},
                "saheli": {"type": "boolean", "description": "Use Saheli women-only service (Aug 2026)"},
                "night": {"type": "boolean", "description": "Night trip surcharge applies (10PM-6AM)"},
                "surge_hint": {"type": "boolean", "description": "Include surge pricing estimates"}
            }
        }
    }
)
async def compare_trip(
    from_location: str,
    to: str,
    preferences: Optional[dict] = None
) -> dict:
    """
    Compare fares across all providers for a given trip.

    Args:
        from_location: Origin point (address, landmark, or "lat,lng")
        to: Destination point (address, landmark, or "lat,lng")
        preferences: Optional user preferences dict

    Returns:
        Comparison results with ranked options
    """
    preferences = preferences or {}

    try:
        payload = {
            "from": from_location,
            "to": to,
            "prefs": {
                "ac": preferences.get("ac", False),
                "saheli": preferences.get("saheli", False),
                "night": preferences.get("night", False),
                "surgeHint": preferences.get("surge_hint", False)
            }
        }

        data = await make_backend_request("POST", "/v1/compare", json_data=payload)

        # Extract badges
        options = data.get("options", [])
        cheapest = data.get("cheapest")
        fastest = data.get("fastest")
        smart_pick = data.get("smart_pick")

        return {
            "success": True,
            "options": options,
            "cheapest": cheapest,
            "fastest": fastest,
            "smart_pick": smart_pick,
            "total_options": len(options),
            "message": f"Found {len(options)} options. Cheapest: {cheapest.get('provider', 'N/A') if cheapest else 'N/A'} at {cheapest.get('fare', 'N/A') if cheapest else 'N/A'}"
        }

    except Exception as e:
        return {
            "success": False,
            "error": format_error(e),
            "options": [],
            "message": "Trip comparison failed"
        }


@mcp.tool(
    name="book_ride",
    description="""Book a ride with the selected provider option.

    Supports two booking rails:
    - DEEPLINK: Opens provider app with pre-filled pickup/dropoff
    - ONDC: Native booking via ONDC/Beckn network (for Yatri, Metro QR)

    After booking, returns a tracking link or deeplink to the provider's app.

    Example:
        book_ride(
            option_id="uber_sedan_delhi_2024",
            provider="uber",
            user_phone="+919876543210"
        )""",
    arguments={
        "option_id": "Unique identifier for the selected fare option from compare_trip",
        "provider": "Provider name: uber, ola, rapido, yatri, namma_yatri, metro, dtc_bus",
        "user_phone": "User phone number for booking confirmation (E.164 format)"
    }
)
async def book_ride(
    option_id: str,
    provider: str,
    user_phone: Optional[str] = None
) -> dict:
    """
    Book a ride with the selected provider.

    Args:
        option_id: The option ID from compare_trip response
        provider: Provider name (uber, ola, rapido, etc.)
        user_phone: Optional phone number for the booking

    Returns:
        Booking confirmation with status and deeplink
    """
    try:
        payload = {
            "option_id": option_id,
            "provider": provider,
            "rail": "deeplink" if provider in ["uber", "ola", "rapido"] else "ondc"
        }
        if user_phone:
            payload["user_phone"] = user_phone

        data = await make_backend_request("POST", "/v1/bookings", json_data=payload)

        return {
            "success": True,
            "booking_id": data.get("booking_id"),
            "status": data.get("status", "pending"),
            "provider": provider,
            "deeplink": data.get("deeplink"),
            "message": f"Booking initiated with {provider}. Status: {data.get('status', 'pending')}"
        }

    except Exception as e:
        return {
            "success": False,
            "booking_id": None,
            "status": "failed",
            "provider": provider,
            "deeplink": None,
            "error": format_error(e),
            "message": f"Booking failed with {provider}"
        }


@mcp.tool(
    name="get_eta",
    description="""Get real-time ETA for a route.

    Returns estimated time of arrival based on:
    - Live traffic conditions (where GTFS-RT is available)
    - Historical speed profiles for the corridor
    - Current time of day

    For buses: Uses OTD (Open Transit Data) live positions updated every 10 seconds.
    For app cabs: Estimates based on driver availability and traffic.

    Example:
        get_eta(route_id="500_down")  # DTC bus route 500 going downtown""",
    arguments={
        "route_id": "Route identifier (e.g., '500_down', 'metro_blue_line', 'uber_auto_delhi')"
    }
)
async def get_eta(route_id: str) -> dict:
    """
    Get real-time ETA for a route.

    Args:
        route_id: The route identifier

    Returns:
        ETA information including distance and live status
    """
    try:
        data = await make_backend_request("GET", f"/v1/routes/{route_id}")

        return {
            "route_id": route_id,
            "eta_seconds": data.get("eta_seconds", 0),
            "eta_formatted": data.get("eta_formatted", "Calculating..."),
            "distance_km": data.get("distance_km", 0.0),
            "live": data.get("live", False),
            "last_updated": data.get("last_updated"),
            "next_departure": data.get("next_departure"),
            "message": f"ETA for {route_id}: {data.get('eta_formatted', 'N/A')}"
        }

    except httpx.HTTPStatusError as e:
        if e.response.status_code == 404:
            return {
                "route_id": route_id,
                "error": f"Route '{route_id}' not found",
                "message": "Route ID not recognized. Check available routes with get_fares."
            }
        return {"route_id": route_id, "error": format_error(e), "message": "ETA lookup failed"}
    except Exception as e:
        return {"route_id": route_id, "error": format_error(e), "message": "ETA lookup failed"}


@mcp.tool(
    name="get_fares",
    description="""Get current fare information for a transport mode.

    Returns tariff information including:
    - Base fare
    - Per-kilometer rate
    - Per-minute rate (for metered rides)
    - Night surcharge rules
    - Saheli discount info

    Fare data is sourced from:
    - Notified government tariffs (Auto Jan-2023, Metro Aug-2025, Bus slabs)
    - Estimated models for app cabs (tuned by user feedback)
    - ONDC partner rates for open-network providers

    Example:
        get_fares(mode="metro")    # DMRC Metro fare slabs
        get_fares(mode="auto")      # Meter auto Jan-2023 tariff
        get_fares(mode="uber")      # Uber estimate model""",
    arguments={
        "mode": "Transport mode: uber, ola, rapido, yatri, namma_yatri, metro, dtc_bus, auto, e_rickshaw, redbus, trains"
    }
)
async def get_fares(mode: str) -> dict:
    """
    Get current fare information for a transport mode.

    Args:
        mode: Transport mode identifier

    Returns:
        Fare tariff information for the mode
    """
    valid_modes = [
        "uber", "ola", "rapido", "yatri", "namma_yatri",
        "metro", "dtc_bus", "auto", "e_rickshaw",
        "redbus", "trains"
    ]

    if mode not in valid_modes:
        return {
            "success": False,
            "error": f"Invalid mode '{mode}'. Valid modes: {', '.join(valid_modes)}",
            "fares": []
        }

    try:
        # Try to get fares from backend
        data = await make_backend_request("GET", "/v1/fares", params={"mode": mode})

        return {
            "success": True,
            "mode": mode,
            "fares": data.get("fares", []),
            "effective_from": data.get("effective_from"),
            "notes": data.get("notes"),
            "message": f"Found {len(data.get('fares', []))} fare options for {mode}"
        }

    except httpx.HTTPStatusError as e:
        if e.response.status_code == 404:
            # Return known tariff data as fallback
            fallback = get_fallback_fares(mode)
            if fallback:
                return {
                    "success": True,
                    "mode": mode,
                    "fares": fallback,
                    "source": "fallback_tariff",
                    "message": f"Returning notified tariff data for {mode}"
                }
        return {"success": False, "error": format_error(e), "fares": []}
    except Exception as e:
        # Try fallback for offline mode
        fallback = get_fallback_fares(mode)
        if fallback:
            return {
                "success": True,
                "mode": mode,
                "fares": fallback,
                "source": "fallback_tariff",
                "offline": True,
                "message": f"Returning cached tariff data for {mode}"
            }
        return {"success": False, "error": format_error(e), "fares": []}


def get_fallback_fares(mode: str) -> list[dict]:
    """
    Return notified tariff data when backend is unavailable.

    These are official government tariffs and known rates.
    """
    tariffs = {
        "metro": [
            {"category": "Standard", "base_fare": 11.0, "max_fare": 64.0, "currency": "INR",
             "notes": "DMRC Aug-2025 slabs: Rs 11-20 (0-5km), 21-35 (5-12km), 36-50 (12-21km), 51-64 (21-32km+)"}
        ],
        "auto": [
            {"category": "Meter Rate", "base_fare": 25.0, "per_km": 10.0, "per_minute": 1.0,
             "currency": "INR", "notes": "Delhi Auto Jan-2023 notified tariff"}
        ],
        "dtc_bus": [
            {"category": "Ordinary", "base_fare": 5.0, "max_fare": 15.0, "currency": "INR",
             "notes": "DTC bus slab: Rs 5 (0-4km), 10 (4-10km), 15 (10km+)"},
            {"category": "AC/Volvo", "base_fare": 10.0, "max_fare": 50.0, "currency": "INR",
             "notes": "AC Bus fares vary by route"}
        ],
        "e_rickshaw": [
            {"category": "Standard", "base_fare": 10.0, "per_km": 5.0, "currency": "INR",
             "notes": "Last-mile e-rickshaw: observed range Rs 10-30"}
        ]
    }
    return tariffs.get(mode, [])


# =============================================================================
# Server Entry Point
# =============================================================================

if __name__ == "__main__":
    # Run with: python server.py
    # Or use fastmcp CLI: fastmcp run server.py

    print(f"Starting Sawaari MCP Server...")
    print(f"Backend URL: {BACKEND_URL}")
    print(f"API Key configured: {'Yes' if API_KEY else 'No'}")

    # Run the server
    mcp.run()
