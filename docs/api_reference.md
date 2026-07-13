# Sawaari API Reference

**Version:** 1.0.0  
**Base URL:** `https://api.sawaari.in/v1`  
**Last Updated:** July 2026

---

## Table of Contents

1. [Overview](#overview)
2. [Authentication](#authentication)
3. [Rate Limits](#rate-limits)
4. [API Endpoints](#api-endpoints)
   - [Compare Fares](#post-v1compare)
   - [Nearby Stops](#get-v1stopsnearby)
   - [Route Details](#get-v1routesno)
   - [Live Route Positions](#get-v1liveroutenoo)
   - [Create Booking](#post-v1bookings)
   - [List Bookings](#get-v1bookings)
   - [Ticket Vault](#get-v1wallettickets)
   - [Download Ticket PDF](#get-v1walletticketsidpdf)
   - [Create Alert](#post-v1alerts)
   - [List Alerts](#get-v1alerts)
   - [Submit Fare Feedback](#post-v1feedbackquote)
5. [Error Codes](#error-codes)
6. [Common Parameters](#common-parameters)
7. [Code Examples](#code-examples)

---

## Overview

The Sawaari API is a RESTful API for Delhi multimodal fare comparison and booking. It provides:

- **Fare Comparison**: Compare prices across 10+ transportation modes
- **Transit Information**: Real-time schedules, stops, and route details
- **Live Tracking**: SSE-based vehicle position streaming
- **Bookings**: ONDC-native and deeplink booking management
- **Wallet**: Digital ticket vault with QR codes
- **Alerts**: Fare drop and disruption notifications
- **Feedback**: Actual fare reporting to improve estimates

### Supported Providers

| Provider | Modes |
|----------|-------|
| DMRC Metro | Metro |
| DTC / DIMTS | Bus |
| Meter Auto | Auto-rickshaw |
| Uber | Cab, Auto, Moto |
| Ola | Cab, Auto, Bike |
| Rapido | Bike, Auto, Cab |
| Namma Yatri | Auto, Cab (ONDC) |
| Redbus | Intercity Bus |
| AbhiBus | Intercity Bus |
| Indian Railways | Trains |
| E-rickshaw | Last-mile |

---

## Authentication

All endpoints require JWT Bearer token authentication.

### Obtaining a Token

1. Send a phone number to the OTP endpoint with MSG91 or Firebase Auth
2. Verify the OTP code received via SMS
3. Receive a JWT token (valid for 24 hours) and a refresh token

### Using the Token

Include the JWT token in the `Authorization` header of every request:

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Token Refresh

When the access token expires, use the refresh token at `/auth/refresh` to get a new access token without re-authenticating.

---

## Rate Limits

| Endpoint Category | Limit | Window |
|-------------------|-------|--------|
| Standard (compare, stops, routes) | 100 requests | per minute per user |
| Booking endpoints | 10 requests | per minute per user |
| Live stream | 1 concurrent connection | per route |

### Rate Limit Headers

Every response includes rate limit headers:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1710445800
```

### Handling 429 Responses

When rate-limited, the API returns HTTP 429. Implement exponential backoff:

```python
import time

def api_call_with_retry(func, max_retries=3):
    for attempt in range(max_retries):
        response = func()
        if response.status_code != 429:
            return response
        wait = 2 ** attempt  # 1s, 2s, 4s
        time.sleep(wait)
    raise RateLimitExceeded("Max retries exceeded")
```

---

## API Endpoints

---

### POST /v1/compare

Compare fares for a trip between two locations. Returns ranked options from all available providers.

**Purpose:** Returns ranked fare options including app-cabs, metro, bus, auto-rickshaws, and more with ETA, distance, and badges.

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `from` | string | Yes* | Origin address or place name |
| `to` | string | Yes* | Destination address or place name |
| `from_loc` | Location | Yes* | Origin coordinates (alternative to `from`) |
| `to_loc` | Location | Yes* | Destination coordinates (alternative to `to`) |
| `prefs` | UserPrefs | No | User preferences filter |

*Either address strings or coordinates must be provided.

**UserPrefs Fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ac` | boolean | false | Prefer AC vehicles |
| `saheli` | boolean | false | Women-only transport (Saheli scheme) |
| `night` | boolean | false | Night travel preference |
| `surge` | boolean | true | Allow surge-priced options |
| `wheelchair` | boolean | false | Wheelchair accessibility required |

#### Response

| Field | Type | Description |
|-------|------|-------------|
| `options` | RideOption[] | Ranked list of ride options |
| `expires_at` | string (ISO8601) | When these quotes expire (5 minutes) |

**RideOption Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique option identifier |
| `provider` | string | Provider name (e.g., "Uber", "DMRC") |
| `mode` | string | Transport mode: `metro`, `bus`, `auto`, `cab`, `bike`, `train`, `e-rickshaw` |
| `display_name` | string | Human-readable name (e.g., "Uber Go") |
| `price` | Price | Fare information |
| `eta_minutes` | integer | Estimated travel time |
| `distance_km` | number | Distance in kilometers |
| `badges` | string[] | Achievement badges: `CHEAPEST`, `FASTEST`, `SMART_PICK` |
| `reliability` | number (0-1) | Provider reliability score (cancellation rate inverse) |
| `deeplink` | string | Direct booking URL |
| `bookable` | boolean | Whether direct booking is available |

**Badge Assignment:**
- **SMART_PICK**: Best overall score (55% price + 45% time weighted)
- **CHEAPEST**: Lowest price option
- **FASTEST**: Lowest ETA option

#### Example Requests

**cURL:**
```bash
curl -X POST https://api.sawaari.in/v1/compare \
  -H "Authorization: Bearer $SAWAARI_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "from": "Rajiv Chowk Metro Station",
    "to": "Connaught Place"
  }'
```

**Python:**
```python
import requests

token = "eyJhbG..."
headers = {"Authorization": f"Bearer {token}"}
response = requests.post(
    "https://api.sawaari.in/v1/compare",
    headers=headers,
    json={
        "from": "Karol Bagh",
        "to": "Aerocity Metro Station",
        "prefs": {
            "ac": True,
            "saheli": False,
            "surge": True
        }
    }
)
options = response.json()["options"]

for opt in options:
    badges = ", ".join(opt["badges"]) if opt["badges"] else "—"
    print(f"{opt['display_name']:20s} ₹{opt['price']['min']:6.0f}  {opt['eta_minutes']:3d}min  [{badges}]")
```

**JavaScript:**
```javascript
async function compareFares(from, to, preferences = {}) {
  const response = await fetch('https://api.sawaari.in/v1/compare', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${getJwtToken()}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      from,
      to,
      prefs: {
        ac: false,
        saheli: false,
        night: false,
        surge: true,
        wheelchair: false,
        ...preferences
      }
    })
  });

  if (!response.ok) {
    throw new Error(`API error: ${response.status}`);
  }

  const { options, expires_at } = await response.json();

  // Find best options
  const cheapest = options.reduce((a, b) =>
    a.price.min < b.price.min ? a : b
  );
  const fastest = options.reduce((a, b) =>
    a.eta_minutes < b.eta_minutes ? a : b
  );
  const smartPick = options.find(o =>
    o.badges.includes('SMART_PICK')
  );

  return { options, cheapest, fastest, smartPick, expires_at };
}
```

#### Example Response

```json
{
  "options": [
    {
      "id": "metro_standard",
      "provider": "DMRC",
      "mode": "metro",
      "display_name": "DMRC Metro",
      "price": {
        "min": 50,
        "max": 50,
        "currency": "INR",
        "breakdown": [
          { "name": "Metro Fare", "amount": 50 }
        ]
      },
      "eta_minutes": 25,
      "distance_km": 8.5,
      "badges": ["CHEAPEST"],
      "bookable": true
    },
    {
      "id": "uber_cab",
      "provider": "Uber",
      "mode": "cab",
      "display_name": "Uber Go",
      "price": {
        "min": 180,
        "max": 250,
        "currency": "INR"
      },
      "eta_minutes": 5,
      "distance_km": 8.5,
      "badges": ["FASTEST", "SMART_PICK"],
      "reliability": 0.92,
      "deeplink": "https://m.uber.com/ul/?action=setPickup&...",
      "bookable": true
    },
    {
      "id": "auto_meter",
      "provider": "meter",
      "mode": "auto",
      "display_name": "Meter Auto",
      "price": {
        "min": 80,
        "max": 120,
        "currency": "INR"
      },
      "eta_minutes": 3,
      "distance_km": 8.5,
      "badges": [],
      "bookable": false
    }
  ],
  "expires_at": "2026-07-14T12:35:00Z"
}
```

---

### GET /v1/stops/nearby

Returns transit stops near a location with next departure information.

#### Query Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `lat` | number | Yes | — | Latitude |
| `lng` | number | Yes | — | Longitude |
| `r` | integer | No | 500 | Search radius in meters (100-5000) |
| `modes` | string | No | all | Comma-separated mode filter: `metro,bus` |

#### Response

```json
{
  "stops": [
    {
      "id": "metro_rajiv_chowk",
      "name": "Rajiv Chowk Metro Station",
      "mode": "metro",
      "distance_m": 120,
      "location": { "lat": 28.6329, "lng": 77.2197 },
      "next_departures": [
        {
          "route_no": "Blue Line",
          "direction": "Dwarka",
          "eta_seconds": 180,
          "platform": "Platform 2"
        }
      ]
    }
  ],
  "searched_at": "2026-07-14T12:30:00Z"
}
```

#### Example Requests

**cURL:**
```bash
curl "https://api.sawaari.in/v1/stops/nearby?lat=28.6329&lng=77.2197&r=500" \
  -H "Authorization: Bearer $SAWAARI_TOKEN"
```

**Python:**
```python
import requests

response = requests.get(
    "https://api.sawaari.in/v1/stops/nearby",
    headers={"Authorization": f"Bearer {token}"},
    params={"lat": 28.6329, "lng": 77.2197, "r": 500, "modes": "metro,bus"}
)

for stop in response.json()["stops"]:
    print(f"{stop['name']} ({stop['mode']}) - {stop['distance_m']}m away")
    for dep in stop.get("next_departures", []):
        print(f"  → {dep['route_no']} towards {dep['direction']}: {dep['eta_seconds']}s")
```

---

### GET /v1/routes/{no}

Returns detailed information about a specific transit route.

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `no` | string | Route number or identifier (e.g., "BL-9", "427") |

#### Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `mode` | string | Optional mode filter: `metro`, `bus`, `auto`, `cab` |

#### Response

```json
{
  "route_no": "BL-9",
  "mode": "metro",
  "name": "Blue Line - Dwarka Sector 21",
  "operator": "DMRC",
  "direction": "Dwarka Sector 21",
  "stops": [
    {
      "id": "blue_rajiv_chowk",
      "name": "Rajiv Chowk",
      "sequence": 9,
      "distance_from_start_km": 4.2,
      "fare_from_first": 30
    }
  ],
  "first_service": "05:30",
  "last_service": "23:30",
  "headway_minutes": 4,
  "fare_slab": {
    "min": 11,
    "max": 64,
    "description": "Aug-2025 DMRC fare slab"
  }
}
```

#### Example Requests

**cURL:**
```bash
curl "https://api.sawaari.in/v1/routes/BL-9" \
  -H "Authorization: Bearer $SAWAARI_TOKEN"
```

**JavaScript:**
```javascript
const response = await fetch(
  'https://api.sawaari.in/v1/routes/BL-9',
  { headers: { 'Authorization': `Bearer ${token}` } }
);
const route = await response.json();

console.log(`${route.name}`);
console.log(`First: ${route.first_service}  Last: ${route.last_service}`);
console.log(`Headway: ${route.headway_minutes} min`);
console.log(`Fare: ₹${route.fare_slab.min}–₹${route.fare_slab.max}`);
```

---

### GET /v1/live/route/{no}

Streams real-time vehicle positions for a route via Server-Sent Events (SSE).

**Purpose:** Receive live vehicle location updates every 10 seconds. Use this to show moving bus/metro positions on the map.

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `no` | string | Route number |

#### Response Format

Content-Type: `text/event-stream`

**Event Types:**

**1. `position` event** — Vehicle position update:

```
event: position
data: {
  "vehicle_id": "DL-1PC-1234",
  "lat": 28.6329,
  "lng": 77.2197,
  "bearing": 45,
  "speed_kmh": 25,
  "timestamp": "2026-07-14T12:30:00Z"
}
```

**2. `eta` event** — Arrival prediction at a stop:

```
event: eta
data: {
  "stop_id": "rajiv_chowk",
  "vehicle_id": "DL-1PC-1234",
  "eta_seconds": 180,
  "load": "medium"
}
```

**Event Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `vehicle_id` | string | Unique vehicle identifier |
| `lat`, `lng` | number | Current position |
| `bearing` | number | Direction in degrees (0-360) |
| `speed_kmh` | number | Current speed |
| `timestamp` | string | ISO8601 timestamp |
| `stop_id` | string | Target stop ID |
| `eta_seconds` | integer | Seconds until arrival |
| `load` | string | Passenger load: `low`, `medium`, `high` |

#### Example: Python SSE Client

```python
import requests

def stream_live_positions(route_no, token):
    url = f"https://api.sawaari.in/v1/live/route/{route_no}"
    headers = {"Authorization": f"Bearer {token}"}

    with requests.get(url, headers=headers, stream=True) as response:
        for line in response.iter_lines(decode_unicode=True):
            if line.startswith("data:"):
                event_data = line[5:].strip()
                if event_data:
                    print(event_data)

stream_live_positions("427", token)
```

#### Example: JavaScript SSE Client

```javascript
function streamLivePositions(routeNo, onPosition, onEta) {
  const eventSource = new EventSource(
    `https://api.sawaari.in/v1/live/route/${routeNo}`,
    {
      headers: {
        'Authorization': `Bearer ${getJwtToken()}`
      }
    }
  );

  eventSource.addEventListener('position', (e) => {
    const data = JSON.parse(e.data);
    onPosition(data);
    // Update marker on map
    map.getSource('vehicle').setData({
      type: 'Point',
      coordinates: [data.lng, data.lat]
    });
  });

  eventSource.addEventListener('eta', (e) => {
    const data = JSON.parse(e.data);
    onEta(data);
  });

  eventSource.onerror = () => {
    console.error('SSE connection error');
    eventSource.close();
  };

  return eventSource;
}
```

---

### POST /v1/bookings

Creates a new booking via the specified rail (ONDC or deeplink).

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `option_id` | string | Yes | ID from a compare response (e.g., `uber_cab`) |
| `rail` | string | Yes | Booking rail: `ondc` or `deeplink` |

#### Response

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | `pending`, `completed`, `failed` |
| `booking_id` | string | Unique booking identifier |
| `deeplink` | string | Direct booking URL (for deeplink rail) |
| `message` | string | Human-readable message |

#### Booking Rails

**ONDC (v2 native booking):**
- Books directly through ONDC Beckn protocol
- Supports: Namma Yatri, DMRC Metro QR, DTC Bus
- Processed natively with UPI payment integration

**Deeplink (v1):**
- Opens the provider app with pre-filled pickup/dropoff
- Supports: Uber, Ola, Rapido
- User completes booking in the external app

#### Example Requests

**cURL:**
```bash
curl -X POST https://api.sawaari.in/v1/bookings \
  -H "Authorization: Bearer $SAWAARI_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "option_id": "uber_cab",
    "rail": "deeplink"
  }'
```

**Python:**
```python
import requests

response = requests.post(
    "https://api.sawaari.in/v1/bookings",
    headers={"Authorization": f"Bearer {token}"},
    json={
        "option_id": "metro_standard",
        "rail": "ondc"
    }
)

booking = response.json()
print(f"Booking ID: {booking['booking_id']}")
print(f"Status: {booking['status']}")
if 'deeplink' in booking:
    # Open deeplink in browser/app
    webbrowser.open(booking['deeplink'])
```

**JavaScript:**
```javascript
async function bookRide(optionId, rail = 'deeplink') {
  const response = await fetch('https://api.sawaari.in/v1/bookings', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${getJwtToken()}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ option_id: optionId, rail })
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error.message);
  }

  const booking = await response.json();

  if (booking.deeplink) {
    // Open in-app browser
    window.open(booking.deeplink, '_blank');
  }

  return booking;
}
```

#### Example Response (ONDC)

```json
{
  "status": "pending",
  "booking_id": "bkg_abc123xyz",
  "message": "UPI payment required"
}
```

#### Example Response (Deeplink)

```json
{
  "status": "pending",
  "booking_id": "bkg_abc123xyz",
  "deeplink": "https://m.uber.com/ul/?action=setPickup&pickup[latitude]=28.6329&pickup[longitude]=77.2197&dropoff[latitude]=28.5494&dropoff[longitude]=77.2000",
  "message": "Complete booking in the provider app"
}
```

---

### GET /v1/bookings

Returns a paginated list of the user's bookings.

#### Query Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `status` | string | No | — | Filter: `pending`, `completed`, `cancelled`, `expired` |
| `limit` | integer | No | 20 | Results per page (max 100) |
| `offset` | integer | No | 0 | Results to skip |

#### Response

```json
{
  "bookings": [
    {
      "id": "bkg_abc123xyz",
      "option_id": "uber_cab",
      "provider": "Uber",
      "mode": "cab",
      "status": "completed",
      "created_at": "2026-07-14T10:30:00Z",
      "fare": 245.00,
      "currency": "INR"
    }
  ],
  "total": 45,
  "limit": 20,
  "offset": 0
}
```

---

### GET /v1/wallet/tickets

Returns the user's digital ticket vault.

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `status` | string | No | Filter: `active`, `used`, `expired` |
| `type` | string | No | Filter: `metro`, `bus`, `movie`, `event`, `parking` |

#### Response

```json
{
  "tickets": [
    {
      "id": "tkt_xyz789",
      "type": "metro",
      "provider": "DMRC",
      "route": "Blue Line",
      "from": "Rajiv Chowk",
      "to": "Dwarka Sector 21",
      "valid_from": "2026-07-14T08:00:00Z",
      "valid_to": "2026-07-14T23:59:00Z",
      "status": "active",
      "qr_payload": "DMRC|2026-07-14|...",
      "price": 60,
      "currency": "INR"
    }
  ],
  "total": 12
}
```

#### Example Requests

**cURL:**
```bash
curl "https://api.sawaari.in/v1/wallet/tickets?status=active" \
  -H "Authorization: Bearer $SAWAARI_TOKEN"
```

**Python:**
```python
response = requests.get(
    "https://api.sawaari.in/v1/wallet/tickets",
    headers={"Authorization": f"Bearer {token}"},
    params={"status": "active"}
)

for ticket in response.json()["tickets"]:
    if ticket["status"] == "active":
        print(f"Ticket: {ticket['provider']} {ticket['route']}")
        print(f"Valid until: {ticket['valid_to']}")
        print(f"QR: {ticket['qr_payload']}")
```

---

### GET /v1/wallet/tickets/{id}.pdf

Downloads a ticket as a PDF file.

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Ticket ID |

#### Response

- **Content-Type:** `application/pdf`
- **Body:** Binary PDF data

#### Example Requests

**cURL:**
```bash
curl "https://api.sawaari.in/v1/wallet/tickets/tkt_xyz789.pdf" \
  -H "Authorization: Bearer $SAWAARI_TOKEN" \
  -o ticket.pdf
```

**Python:**
```python
response = requests.get(
    "https://api.sawaari.in/v1/wallet/tickets/tkt_xyz789.pdf",
    headers={"Authorization": f"Bearer {token}"}
)

with open("ticket.pdf", "wb") as f:
    f.write(response.content)
```

---

### POST /v1/alerts

Creates a fare or disruption alert. Notifications are sent via push, WhatsApp, or Telegram.

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `kind` | string | Yes | Alert type: `fare_drop` or `disruption` |
| `route` | object | No | Origin/destination (for `fare_drop`) |
| `route` | string | No | Route number (for `disruption`) |
| `route.from` | string | Yes (if route) | Origin |
| `route.to` | string | Yes (if route) | Destination |
| `modes` | string[] | No | Modes to watch: `metro`, `bus`, `auto`, `cab`, `bike` |
| `threshold` | number | Yes (for fare_drop) | Price threshold to trigger alert |
| `notify_via` | string[] | Yes | Notification channels: `push`, `whatsapp`, `telegram` |

#### Response

```json
{
  "id": "alt_def456",
  "kind": "fare_drop",
  "status": "active",
  "created_at": "2026-07-14T12:30:00Z"
}
```

#### Example Requests

**cURL (fare drop alert):**
```bash
curl -X POST https://api.sawaari.in/v1/alerts \
  -H "Authorization: Bearer $SAWAARI_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "fare_drop",
    "route": {
      "from": "Rajiv Chowk",
      "to": "Dwarka Sector 21"
    },
    "modes": ["metro", "cab"],
    "threshold": 40,
    "notify_via": ["push", "whatsapp"]
  }'
```

**cURL (disruption alert):**
```bash
curl -X POST https://api.sawaari.in/v1/alerts \
  -H "Authorization: Bearer $SAWAARI_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "disruption",
    "route_no": "BL-9",
    "notify_via": ["push"]
  }'
```

---

### GET /v1/alerts

Returns the user's active and past alerts.

#### Response

```json
{
  "alerts": [
    {
      "id": "alt_def456",
      "kind": "fare_drop",
      "status": "active",
      "created_at": "2026-07-14T12:30:00Z"
    }
  ]
}
```

---

### POST /v1/feedback/quote

Reports the actual fare paid. This data trains and improves estimate models.

**Purpose:** Every submission improves fare estimates for all users. Nightly jobs re-fit estimate models per corridor and time-of-day.

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `quote_id` | string | Yes | ID of the quote being reviewed |
| `provider` | string | No | Provider name |
| `mode` | string | No | Transport mode |
| `actual_fare` | number | Yes | Actual fare paid (in currency) |
| `currency` | string | No | Currency code (default: `INR`) |
| `notes` | string | No | Optional trip notes |

#### Response

```json
{
  "id": "fbk_ghi789",
  "status": "recorded",
  "thank_you_message": "Thanks for helping improve Sawaari estimates!"
}
```

#### Example Requests

**cURL:**
```bash
curl -X POST https://api.sawaari.in/v1/feedback/quote \
  -H "Authorization: Bearer $SAWAARI_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "quote_id": "qte_abc123",
    "provider": "Uber",
    "mode": "cab",
    "actual_fare": 235.00,
    "notes": "Normal traffic, no surge"
  }'
```

**Python:**
```python
response = requests.post(
    "https://api.sawaari.in/v1/feedback/quote",
    headers={"Authorization": f"Bearer {token}"},
    json={
        "quote_id": "qte_abc123",
        "provider": "Uber",
        "mode": "cab",
        "actual_fare": 235.00,
        "notes": "Normal traffic, no surge"
    }
)

print(response.json()["thank_you_message"])
```

**JavaScript:**
```javascript
async function reportActualFare(quoteId, actualFare, details = {}) {
  const response = await fetch(
    'https://api.sawaari.in/v1/feedback/quote',
    {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${getJwtToken()}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        quote_id: quoteId,
        ...details,
        actual_fare: actualFare
      })
    }
  );

  if (!response.ok) {
    throw new Error(`Feedback failed: ${response.status}`);
  }

  return await response.json();
}
```

---

## Error Codes

All errors follow a consistent format:

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Missing required parameter: from",
    "details": {}
  }
}
```

### Standard Error Codes

| HTTP Status | Error Code | Description |
|-------------|------------|-------------|
| 400 | `INVALID_REQUEST` | Malformed request body or missing required fields |
| 400 | `INVALID_COORDINATES` | Latitude/longitude values out of range |
| 400 | `UNKNOWN_LOCATION` | Address could not be geocoded |
| 401 | `UNAUTHENTICATED` | Missing or invalid JWT token |
| 401 | `TOKEN_EXPIRED` | JWT token has expired, use refresh token |
| 403 | `FORBIDDEN` | Valid token but insufficient permissions |
| 404 | `NOT_FOUND` | Resource not found (route, stop, ticket) |
| 409 | `OPTION_EXPIRED` | Selected option expired (quotes valid for 5 min) |
| 409 | `BOOKING_CONFLICT` | Option no longer bookable |
| 422 | `PROVIDER_UNAVAILABLE` | Provider service is down |
| 429 | `RATE_LIMIT_EXCEEDED` | Too many requests, retry after `Retry-After` header |
| 500 | `INTERNAL_ERROR` | Server error, retry with backoff |
| 503 | `SERVICE_UNAVAILABLE` | Degraded service, try again later |

### Retry-After Header

For 429 responses, check the `Retry-After` header:

```
Retry-After: 60
```

This indicates how many seconds to wait before retrying.

---

## Common Parameters

### Location Object

```json
{
  "lat": 28.6329,
  "lng": 77.2197
}
```

### Currency

All prices use INR (Indian Rupees). The `currency` field defaults to `INR` and can be overridden for international use.

### Pagination

List endpoints support `limit` and `offset` for pagination:

```
GET /v1/bookings?limit=20&offset=40
```

### Time Formats

All timestamps use ISO 8601 format:

```
2026-07-14T12:30:00+05:30
2026-07-14T12:30:00Z
```

---

## Code Examples

### Full Trip Flow: Compare → Book → Wallet

**Python:**

```python
import requests

BASE_URL = "https://api.sawaari.in/v1"
token = "your_jwt_token"
headers = {"Authorization": f"Bearer {token}"}

# Step 1: Compare fares
compare_resp = requests.post(
    f"{BASE_URL}/compare",
    headers={**headers, "Content-Type": "application/json"},
    json={"from": "Rajiv Chowk", "to": "Dwarka Sector 21"}
)
compare_resp.raise_for_status()
options = compare_resp.json()["options"]

# Find best metro option
metro = next(o for o in options if o["mode"] == "metro")
print(f"Metro fare: ₹{metro['price']['min']}  ETA: {metro['eta_minutes']}min")

# Step 2: Book the ticket
booking_resp = requests.post(
    f"{BASE_URL}/bookings",
    headers={**headers, "Content-Type": "application/json"},
    json={"option_id": metro["id"], "rail": "ondc"}
)
booking = booking_resp.json()
print(f"Booked: {booking['booking_id']}")

# Step 3: Check wallet
wallet_resp = requests.get(
    f"{BASE_URL}/wallet/tickets?type=metro",
    headers=headers
)
tickets = wallet_resp.json()["tickets"]
print(f"Active metro tickets: {len(tickets)}")
```

**JavaScript:**

```javascript
async function planTrip(from, to) {
  const token = getJwtToken();
  const headers = {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  };

  // Compare
  const compare = await fetch('https://api.sawaari.in/v1/compare', {
    method: 'POST',
    headers,
    body: JSON.stringify({ from, to })
  });
  const { options } = await compare.json();

  // Filter by mode
  const metro = options.filter(o => o.mode === 'metro');
  const cabs = options.filter(o => o.mode === 'cab');

  // Display results
  console.log(`Found ${options.length} options`);
  console.log(`Cheapest: ₹${Math.min(...options.map(o => o.price.min))}`);
  console.log(`Fastest: ${Math.min(...options.map(o => o.eta_minutes))} min`);

  // Book cheapest metro option
  if (metro.length > 0) {
    const best = metro.find(o => o.badges.includes('SMART_PICK')) || metro[0];
    const booking = await fetch('https://api.sawaari.in/v1/bookings', {
      method: 'POST',
      headers,
      body: JSON.stringify({
        option_id: best.id,
        rail: 'ondc'
      })
    });
    const result = await booking.json();
    console.log('Booked:', result.booking_id);
  }
}
```

### Live Position Tracking with Map

**JavaScript + MapLibre:**

```javascript
let vehicleMarkers = {};

function startLiveTracking(routeNo, map) {
  const es = new EventSource(
    `https://api.sawaari.in/v1/live/route/${routeNo}`,
    { headers: { 'Authorization': `Bearer ${getJwtToken()}` }}
  );

  es.addEventListener('position', (e) => {
    const v = JSON.parse(e.data);
    updateVehicleMarker(v);
  });

  es.addEventListener('eta', (e) => {
    const data = JSON.parse(e.data);
    updateEtaDisplay(data);
  });

  return es;
}

function updateVehicleMarker(vehicle) {
  if (!vehicleMarkers[vehicle.vehicle_id]) {
    // Create new marker
    vehicleMarkers[vehicle.vehicle_id] = new maplibregl.Marker({
      element: createBusElement()
    }).setLngLat([vehicle.lng, vehicle.lat]).addTo(map);
  } else {
    // Smoothly update position
    vehicleMarkers[vehicle.vehicle_id]
      .setLngLat([vehicle.lng, vehicle.lat])
      .setRotation(vehicle.bearing);
  }
}
```

### Setting Up Alerts

**cURL:**

```bash
# Create fare drop alert
curl -X POST https://api.sawaari.in/v1/alerts \
  -H "Authorization: Bearer $SAWAARI_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "fare_drop",
    "route": {
      "from": "Aerocity",
      "to": "Karol Bagh"
    },
    "modes": ["cab", "metro"],
    "threshold": 80,
    "notify_via": ["push", "telegram"]
  }'

# List active alerts
curl "https://api.sawaari.in/v1/alerts" \
  -H "Authorization: Bearer $SAWAARI_TOKEN"
```

**JavaScript:**

```javascript
// Fare drop alert
const alert = await fetch('https://api.sawaari.in/v1/alerts', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    kind: 'fare_drop',
    route: { from: 'Aerocity', to: 'Karol Bagh' },
    modes: ['cab', 'metro'],
    threshold: 80,
    notify_via: ['push', 'telegram']
  })
});

const result = await alert.json();
console.log(`Alert ${result.id} is ${result.status}`);
```
