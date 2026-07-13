# Sawaari MCP Server

A FastMCP server that exposes ride-hailing and transit comparison tools for the Sawaari fare comparison ecosystem - Delhi's multimodal transport platform.

## Overview

The Sawaari MCP Server provides AI agents (including Claude Code) with tools to:
- Compare fares across multiple transport providers (Uber, Ola, Rapido, Metro, DTC buses, etc.)
- Book rides through deeplinks or ONDC
- Get real-time ETAs for routes
- Query current fare tariffs

## Installation

### Prerequisites

- Python 3.10 or higher
- Sawaari Go backend running (or configured endpoint)

### Setup

```bash
# Navigate to the MCP server directory
cd sawaari-mcp

# Create a virtual environment (recommended)
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt

# Configure environment
cp .env.example .env
# Edit .env with your backend URL and API keys
```

### Environment Configuration

Create a `.env` file with:

```env
# Backend Configuration
SAWAARI_BACKEND_URL=http://localhost:8080

# API Key (if required by backend)
SAWAARI_API_KEY=your_api_key_here
```

## Available Tools

### compare_trip

Compare fares across all available transport providers for a trip.

**Arguments:**
- `from` (string): Origin location - address, landmark, or "lat,lng"
- `to` (string): Destination location - address, landmark, or "lat,lng"
- `preferences` (object, optional): User preferences
  - `ac`: Prefer AC vehicles
  - `saheli`: Use Saheli women-only service (Aug 2026)
  - `night`: Night trip (10PM - 6AM surcharge)
  - `surge_hint`: Include surge pricing estimates

**Example:**
```python
# Compare trip from Connaught Place to IGI Airport
result = await compare_trip(
    from_location="Connaught Place, Delhi",
    to="IGI Airport Terminal 3",
    preferences={"ac": True, "saheli": False}
)
```

**Returns:**
```json
{
    "success": true,
    "options": [...],
    "cheapest": {"provider": "metro", "fare": 60, "eta_minutes": 55},
    "fastest": {"provider": "uber", "fare": 450, "eta_minutes": 25},
    "smart_pick": {"provider": "ola_auto", "fare": 180, "eta_minutes": 35}
}
```

---

### book_ride

Book a ride with the selected provider option.

**Arguments:**
- `option_id` (string): Unique identifier from compare_trip
- `provider` (string): Provider name - `uber`, `ola`, `rapido`, `yatri`, `namma_yatri`, `metro`, `dtc_bus`
- `user_phone` (string, optional): Phone number for confirmation (E.164 format)

**Example:**
```python
# Book an Uber from the comparison result
result = await book_ride(
    option_id="uber_sedan_delhi_2024",
    provider="uber",
    user_phone="+919876543210"
)
```

**Returns:**
```json
{
    "success": true,
    "booking_id": "BK123456",
    "status": "initiated",
    "provider": "uber",
    "deeplink": "https://m.uber.com/ul/?action=setPickup&..."
}
```

---

### get_eta

Get real-time ETA for a route.

**Arguments:**
- `route_id` (string): Route identifier (e.g., `500_down`, `metro_blue_line`)

**Example:**
```python
# Get ETA for DTC bus route 500
result = await get_eta(route_id="500_down")
```

**Returns:**
```json
{
    "route_id": "500_down",
    "eta_seconds": 420,
    "eta_formatted": "7 minutes",
    "distance_km": 2.3,
    "live": true,
    "last_updated": "2026-07-14T10:30:00Z"
}
```

---

### get_fares

Get current fare information for a transport mode.

**Arguments:**
- `mode` (string): Transport mode - `uber`, `ola`, `rapido`, `yatri`, `namma_yatri`, `metro`, `dtc_bus`, `auto`, `e_rickshaw`, `redbus`, `trains`

**Example:**
```python
# Get DMRC Metro fare slabs
result = await get_fares(mode="metro")

# Get Auto rickshaw tariff
result = await get_fares(mode="auto")
```

**Returns:**
```json
{
    "success": true,
    "mode": "metro",
    "fares": [
        {
            "category": "Standard",
            "base_fare": 11.0,
            "max_fare": 64.0,
            "currency": "INR",
            "notes": "DMRC Aug-2025 slabs"
        }
    ]
}
```

## Connection to Claude Code

### Method 1: Using Claude Code's MCP Support

Add to your Claude Code settings (`settings.json`):

```json
{
    "mcpServers": {
        "sawaari": {
            "command": "python",
            "args": ["path/to/sawaari-mcp/server.py"],
            "env": {
                "SAWAARI_BACKEND_URL": "http://localhost:8080"
            }
        }
    }
}
```

### Method 2: Direct Execution

Run the server and interact via stdio:

```bash
cd sawaari-mcp
python server.py
```

### Method 3: With uvx (fastmcp CLI)

```bash
fastmcp run sawaari-mcp/server.py
```

## Backend API Endpoints

The MCP server wraps these Go backend endpoints:

| MCP Tool | Backend Endpoint | Method |
|----------|------------------|--------|
| `compare_trip` | `/v1/compare` | POST |
| `book_ride` | `/v1/bookings` | POST |
| `get_eta` | `/v1/routes/{id}` | GET |
| `get_fares` | `/v1/fares?mode=X` | GET |

## Transport Providers

The system supports these providers and booking rails:

| Provider | Pricing | Booking Rail | Status |
|----------|---------|--------------|--------|
| Uber | Estimate model | Universal deeplink | Active |
| Ola | Estimate model | Affiliate deeplink | Active |
| Rapido | Estimate model | App-open only | Limited |
| Namma Yatri | Near-meter (₹0 commission) | ONDC native | Active |
| Yatri | Near-meter (₹0 commission) | ONDC native | Active |
| DMRC Metro | Aug-2025 slab law | ONDC QR, WhatsApp | Active |
| DTC Bus | Slab law | ONDC (rolling) | Active |
| Meter Auto | Jan-2023 tariff | Street hail | Active |

## Error Handling

The MCP server provides meaningful error messages:

```python
# Network error (backend unreachable)
{"success": false, "error": "Network error: Unable to reach backend at http://localhost:8080. Is the server running?"}

# Invalid route
{"success": false, "error": "Route 'XYZ' not found", "message": "Route ID not recognized."}

# Booking failure
{"success": false, "booking_id": null, "status": "failed", "error": "Backend error (400): Invalid option_id"}
```

## Development

### Running Tests

```bash
# Run with mock backend
SAWAARI_BACKEND_URL=http://localhost:8080 python -m pytest tests/
```

### Local Development

```bash
# Start backend locally
cd ../backend && go run cmd/server/main.go

# Start MCP server
cd ../sawaari-mcp && python server.py
```

## License

Proprietary - Sawaari Project
