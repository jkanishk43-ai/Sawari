# Provider Adapter Matrix

**Source:** PLATE 06 — Provider Adapter Matrix  
**Sawaari System Architecture Blueprint v1.0 — July 2026**

This matrix documents how Sawaari integrates with each transportation provider — where prices come from, how booking works, and the current honest status of each integration.

---

## Summary

| # | Provider | Prices Source | Booking Rail | Status |
|---|----------|--------------|--------------|--------|
| 1 | Uber (Go/Moto/Auto) | Estimate model | Deeplink | Partial |
| 2 | Ola (Mini/Auto/Bike) | Estimate model | Deeplink + Affiliate | Partial |
| 3 | Rapido (bike/auto/cab) | Estimate model | App-open only | Gap |
| 4 | Namma Yatri (auto/cab) | Driver-set meter | ONDC/Beckn | Full |
| 5 | DMRC Metro | Aug-2025 slab law | ONDC QR + WhatsApp | Full |
| 6 | DTC / DIMTS Bus | Slab law + Saheli | One Delhi QR + ONDC | Full |
| 7 | Meter Auto / Kaali-Peeli | Jan-2023 notified tariff | Street hail only | Reference |
| 8 | Redbus (intercity) | Affiliate/B2B API | Affiliate handoff | Partial |
| 9 | AbhiBus (intercity) | Affiliate program | Affiliate handoff | Partial |
| 10 | Indian Railways | NTES timetable + live | Partner license required | Planned |
| 11 | E-rickshaw | Observed hop range | Street hail only | Reference |

---

## Detailed Provider Breakdown

### 1. Uber (Go / Moto / Auto)

| Attribute | Detail |
|-----------|--------|
| **Modes** | Cab (Go), Moto (bike), Auto (auto-rickshaw) |
| **Prices come from** | Sawaari estimate model: base fare + per-km rate + per-minute rate, multiplied by a surge factor. Model is tuned by user fare feedback submissions. |
| **Booking rail** | Deeplink: `https://m.uber.com/ul/?action=setPickup&pickup[latitude]=…&pickup[longitude]=…&dropoff[latitude]=…&dropoff[longitude]=…` with pre-filled coordinates. |
| **Honest status** | **Partial / Partial** — Uber's public fare API is closed. The Terms of Service explicitly bans price-comparison use of their data. Fare estimates are entirely our model, trained by user-reported actual fares. Deeplink prefill has a known intermittent bug that may not populate coordinates correctly — must be tested per app release. |

**Notes:**
- No official partner pricing API available
- Deeplink opens the Uber app with pickup/dropoff pre-filled
- Estimate model accuracy improves as more users report actual fares via `/v1/feedback/quote`

---

### 2. Ola (Mini / Auto / Bike)

| Attribute | Detail |
|-----------|--------|
| **Modes** | Cab (Prime/Mini), Auto, Bike |
| **Prices come from** | Sawaari estimate model with Ola-specific base fare and per-km/per-minute rates. |
| **Booking rail** | Deeplink: `https://book.olacabs.com/?lat=…&drop_lat=…&category=…&utm_source=<affiliate-id>` — the `utm_source` parameter is mandatory and requires affiliate registration. A full Booking API exists but is behind partner approval. |
| **Honest status** | **Partial / Partial** — Affiliate signup is the v1 move. The full partner Booking API is a v2 upgrade that requires going through Ola's partnership approval process. Deeplink requires a registered affiliate ID. |

**Notes:**
- Affiliate program provides revenue share on completed trips
- Partner API gives real-time pricing but requires business approval
- Estimate model serves as the primary v1 integration

---

### 3. Rapido (bike / auto / cab)

| Attribute | Detail |
|-----------|--------|
| **Modes** | Bike, Auto, Cab |
| **Prices come from** | Sawaari estimate model with Rapido-specific rates (base ~₹15, per-km ~₹6). |
| **Booking rail** | **App-open only** — zero public deeplink scheme exists as of the blueprint date. No URL-based pickup prefill is available. |
| **Honest status** | **Gap / Gap** — This is a real gap in v1. The only available path is opening the Rapido app (user must manually enter pickup/dropoff). ONDC onboarding is the long-term answer but is not yet available. |

**Notes:**
- Rapido is the cheapest option for short trips (bike taxi)
- Without deeplink, the UX requires leaving Sawaari and returning
- Users see the price estimate but must book manually

---

### 4. Namma Yatri / Yatri (auto / cab)

| Attribute | Detail |
|-----------|--------|
| **Modes** | Auto, Cab |
| **Prices come from** | Driver-set fares (near meter price) with ₹0 commission — the driver sets the price in real time. |
| **Booking rail** | **ONDC/Beckn — natively bookable.** Live in Delhi since January 2024. Sawaari acts as a Beckn protocol client (BAP) to search, select, and book rides. |
| **Honest status** | **Full / Full** — This is the open rail. AGPL reference code exists. Namma Yatri is the most transparent integration: real-time quotes from drivers, direct booking through ONDC, no middleman commission. |

**Notes:**
- Best option for women passengers (Saheli flag feature)
- ₹0 commission means driver keeps full fare
- ONDC Beckn protocol handles the entire booking flow
- Can serve as the template for other ONDC integrations

---

### 5. DMRC Metro

| Attribute | Detail |
|-----------|--------|
| **Modes** | Metro |
| **Prices come from** | **Aug-2025 notified fare slab law** (₹11–₹64 depending on distance). Computed locally — no API call needed. |
| **Booking rail** | **ONDC QR tickets** (10+ buyer apps already sell them, including Uber since May 2025). Also available via WhatsApp at +91 96508 55800. |
| **Honest status** | **Full / Full** — Proven rail. The fare slab is public knowledge (government notification). ONDC QR ticket purchase is operational and verified. DMRC's own app also sells QR codes. |

**Notes:**
- Fare computation is entirely local — no external dependency
- QR tickets are the digital replacement for smart cards
- Offline-first: tariffs are bundled with the app
- Gates open on QR scan — no internet required at the gate

---

### 6. DTC / DIMTS Bus

| Attribute | Detail |
|-----------|--------|
| **Modes** | Bus (DTC cluster buses, DIMTS AC bus) |
| **Prices come from** | **Slab law** (DTC: ₹5–₹15, DIMTS: ₹10–₹25) + Saheli gate (women discount, effective Aug 1, 2026). Computed locally. |
| **Booking rail** | **One Delhi QR** (in-app discount of ~10%) · Chartr · ONDC bus ticketing (rolling out since September 2024). |
| **Honest status** | **Full / Full** — GTFS gives schedule inventory. ONDC gives the ticketing rail. Both are operational. |

**Notes:**
- GTFS data from Delhi's Open Transit Data portal (otd.delhi.gov.in)
- Saheli scheme provides free or discounted rides for women on DTC buses
- Bus ETAs come from live GPS positions (GTFS-RT) + our speed history, not static timetables

---

### 7. Meter Auto / Kaali-Peeli

| Attribute | Detail |
|-----------|--------|
| **Modes** | Auto-rickshaw |
| **Prices come from** | **Jan-2023 notified tariff** from the Delhi government: base fare ₹25 (first 1.5 km), then ₹18/km. Computed locally. |
| **Booking rail** | **Street hail only** — no digital booking rail exists. No app, no deeplink, no ONDC option. |
| **Honest status** | **Reference only / Street hail** — Sawaari presents the meter-based fare as a price anchor/negotiation reference. Users see what the legal fare should be before haggling. A meter-refusal disclaimer is always shown. |

**Notes:**
- In practice, many auto drivers refuse meter and quote 2-3x the government rate
- Sawaari shows the correct meter fare and a realistic negotiated range
- Serves as the "price floor" — no option should cost less
- No booking possible; purely informational

---

### 8. Redbus (Intercity)

| Attribute | Detail |
|-----------|--------|
| **Modes** | Intercity Bus |
| **Prices come from** | **Redbus affiliate/B2B inventory API** (partner program). Real-time seat availability and pricing from Redbus's inventory system. |
| **Booking rail** | **Affiliate handoff** → redirects to Redbus for completed booking. Redbus handles the entire transaction. |
| **Honest status** | **Partial / Partial** — Partnership required. Standard affiliate onboarding process. Redbus has a formal affiliate program for third-party platforms. |

**Notes:**
- Redbus is the dominant intercity bus booking platform in India
- Affiliate model gives Sawaari a commission on referred bookings
- Users see live seat availability and prices
- Redbus handles all payment, cancellation, and support

---

### 9. AbhiBus (Intercity)

| Attribute | Detail |
|-----------|--------|
| **Modes** | Intercity Bus |
| **Prices come from** | **AbhiBus/ixigo affiliate program**. Real-time inventory through their affiliate API. |
| **Booking rail** | **Affiliate handoff** → redirects to AbhiBus for completed booking. |
| **Honest status** | **Partial / Partial** — Partnership required. Similar affiliate model to Redbus. |

**Notes:**
- AbhiBus is the second-largest intercity bus platform
- ixigo owns AbhiBus; integration may be coordinated through ixigo's developer portal
- Same affiliate model as Redbus — commission on referrals

---

### 10. Indian Railways (Trains)

| Attribute | Detail |
|-----------|--------|
| **Modes** | Train |
| **Prices come from** | **Open timetable data** (IRCTC public schedule) + **NTES live status** (National Train Enquiry System). Fare computed from IRCTC's published fare tables based on distance and class. |
| **Booking rail** | **IRCTC booking requires an authorized partner license** (the ixigo/ConfirmTkt path). Unreserved UTS (Unreserved Ticketing System) has no public API. |
| **Honest status** | **Planned / Unavailable** — IRCTC does not offer a public booking API to general developers. This is a Phase-3 roadmap item. Sawaari should state plainly to users that direct booking is not yet available — show timetables, live status, and estimated fares, with a link to IRCTC for actual booking. |

**Notes:**
- Train fares are among the cheapest options for long distances
- IRCTC booking APIs are restricted to licensed travel partners
- NTES provides live train running status (delays, platform)
- ConfirmTkt and ixigo Trains are examples of licensed partners
- Phase-3 item: pursue IRCTC partner license or build a UTS unreserved flow

---

### 11. E-rickshaw

| Attribute | Detail |
|-----------|--------|
| **Modes** | E-rickshaw (battery-powered three-wheeler) |
| **Prices come from** | **Observed hop range** of ₹10–₹30 per short trip (1–2 km). No regulated tariff exists. |
| **Booking rail** | **Street hail only** — no digital booking option. E-rickshaws operate informally, primarily in Delhi NCR residential areas and near metro stations. |
| **Honest status** | **Reference only / Street hail** — E-rickshaws appear as a "last mile" card in comparison results. The price is a rough estimate. No formal integration possible in v1. |

**Notes:**
- Used primarily for 0.5–2 km trips (metro station to home/office)
- Prices are unregulated and vary by operator
- Appears as an option alongside meter auto for very short distances
- No booking, no tracking — informational only

---

## Booking Rail Reference

| Rail | Description | Available For | Status |
|------|-------------|---------------|--------|
| **ONDC/Beckn** | Open network for digital commerce. Sawaari acts as BAP (Buyer App Platform). | Namma Yatri (auto/cab), DMRC Metro (QR), DTC Bus | Operational |
| **Deeplink** | URL scheme that opens the provider app with pre-filled pickup/dropoff. | Uber, Ola | Operational (with caveats) |
| **Affiliate** | Redirect to partner booking page with tracking ID for commission. | Redbus, AbhiBus | Requires partnership |
| **IRCTC Partner** | Authorized partner license for IRCTC booking API. | Indian Railways | Not yet available |
| **Street Hail** | No digital rail — user must physically hail the vehicle. | Meter Auto, E-rickshaw | N/A |

## Estimate Model Reference

| Provider | Base Fare (₹) | Per-km (₹) | Per-min (₹) | Notes |
|----------|--------------|------------|-------------|-------|
| Uber | 40 | 8.0 | 1.0 | Surge multiplier applied |
| Ola | 35 | 9.0 | 1.0 | Affiliate utm_source required |
| Rapido | 15 | 6.0 | 0.5 | Cheapest for short trips |
| Namma Yatri | 0 | meter rate | 0 | ₹0 commission, driver-set price |

## Fare Computation Reference

| Mode | Fare Basis | Source | Computed |
|------|-----------|--------|----------|
| DMRC Metro | Aug-2025 slab law | Government notification | Locally |
| DTC Bus | Slab law (₹5–15) | Government notification | Locally |
| DIMTS Bus | Slab law (₹10–25) | Government notification | Locally |
| Meter Auto | Jan-2023 tariff | Government notification | Locally |
| Saheli (women) | Discount/free gate | Aug 1, 2026 cutover | Locally |
| App-cabs | Estimate model | User feedback training | Server-side |
| Intercity buses | Affiliate API | Redbus/AbhiBus | External |
