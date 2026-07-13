"""
Sawaari WhatsApp Bot - Main Application
FastAPI-based WhatsApp Business Cloud API integration with async handling
"""

import os
import logging
import asyncio
from contextlib import asynccontextmanager
from datetime import datetime, timezone
from typing import Optional

import httpx
from dotenv import load_dotenv
from fastapi import FastAPI, HTTPException, Request, Response
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field, ValidationError

# ---------------------------------------------------------------------------
# Environment
# ---------------------------------------------------------------------------
load_dotenv()

logger = logging.getLogger("sawaari.whatsapp")

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
WHATSAPP_API_BASE = os.getenv("WHATSAPP_API_BASE", "https://graph.facebook.com/v18.0")
PHONE_NUMBER_ID = os.getenv("WHATSAPP_PHONE_NUMBER_ID", "")
ACCESS_TOKEN = os.getenv("WHATSAPP_ACCESS_TOKEN", "")
VERIFY_TOKEN = os.getenv("WHATSAPP_VERIFY_TOKEN", "sawaari-verify-token")
APP_SECRET = os.getenv("WHATSAPP_APP_SECRET", "")
BACKEND_URL = os.getenv("SAWAARI_BACKEND_URL", "http://localhost:8080")

# ---------------------------------------------------------------------------
# Pydantic models for webhook payloads
# ---------------------------------------------------------------------------


class LocationPayload(BaseModel):
    latitude: Optional[float] = None
    longitude: Optional[float] = None
    name: Optional[str] = None
    address: Optional[str] = None


class TextPayload(BaseModel):
    body: str = ""


class IncomingMessage(BaseModel):
    from_number: str = Field(..., alias="from")
    id: str
    timestamp: str
    type: str
    text: Optional[dict] = None
    location: Optional[dict] = None

    class Config:
        populate_by_name = True


class WebhookValue(BaseModel):
    messaging_product: str
    metadata: dict
    messages: list[dict] = []


class WebhookChange(BaseModel):
    value: WebhookValue
    field: str


class WebhookEntry(BaseModel):
    id: str
    changes: list[WebhookChange]


class WebhookBody(BaseModel):
    object: str
    entry: list[WebhookEntry] = []


# ---------------------------------------------------------------------------
# WhatsApp REST client
# ---------------------------------------------------------------------------


class WhatsAppClient:
    """Thin async wrapper around the WhatsApp Business Cloud API."""

    def __init__(self, phone_number_id: str, access_token: str):
        self._phone_number_id = phone_number_id
        self._access_token = access_token
        self._base = WHATSAPP_API_BASE.rstrip("/")
        self._session: Optional[httpx.AsyncClient] = None

    @property
    def _headers(self) -> dict:
        return {
            "Authorization": f"Bearer {self._access_token}",
            "Content-Type": "application/json",
        }

    async def _send(self, method: str, path: str, payload: dict) -> dict:
        """Internal helper — make a single request."""
        url = f"{self._base}/{path}"
        async with httpx.AsyncClient(
            headers=self._headers, timeout=httpx.Timeout(15.0, connect=5.0)
        ) as client:
            try:
                r = await client.request(method, url, json=payload)
                r.raise_for_status()
                return r.json()
            except httpx.HTTPStatusError as exc:
                logger.error(
                    "WhatsApp API %s %s → %s: %s",
                    method,
                    path,
                    exc.response.status_code,
                    exc.response.text[:300],
                )
                raise
            except httpx.TimeoutException:
                logger.error("WhatsApp API timeout: %s %s", method, path)
                raise

    # -- High-level send helpers ------------------------------------------------

    async def send_text(self, to: str, body: str, preview_url: bool = False) -> dict:
        return await self._send(
            "POST",
            f"{self._phone_number_id}/messages",
            {
                "messaging_product": "whatsapp",
                "to": to,
                "type": "text",
                "text": {"body": body, "preview_url": preview_url},
            },
        )

    async def send_interactive(
        self,
        to: str,
        *,
        type_: str,  # "button" | "list"
        header_text: Optional[str] = None,
        body_text: str = "",
        footer_text: str = "",
        buttons: Optional[list[dict]] = None,
        list_rows: Optional[list[dict]] = None,
        list_button_text: str = "Choose",
    ) -> dict:
        """Send an interactive (button or list) template message."""
        interactive: dict = {"type": type_}
        if header_text:
            interactive["header"] = {"type": "text", "text": header_text}
        interactive["body"] = {"text": body_text}
        if footer_text:
            interactive["footer"] = {"text": footer_text}
        action: dict = {}
        if type_ == "button" and buttons:
            action["buttons"] = buttons
        elif type_ == "list" and list_rows:
            action["button"] = list_button_text
            action["sections"] = [{"title": "Options", "rows": list_rows}]
        interactive["action"] = action
        return await self._send(
            "POST",
            f"{self._phone_number_id}/messages",
            {
                "messaging_product": "whatsapp",
                "to": to,
                "type": "interactive",
                "interactive": interactive,
            },
        )

    async def send_template(
        self,
        to: str,
        name: str,
        components: Optional[list] = None,
        lang: str = "en",
    ) -> dict:
        return await self._send(
            "POST",
            f"{self._phone_number_id}/messages",
            {
                "messaging_product": "whatsapp",
                "to": to,
                "type": "template",
                "template": {
                    "name": name,
                    "language": {"code": lang},
                    "components": components or [],
                },
            },
        )


# ---------------------------------------------------------------------------
# Sawaari Backend client
# ---------------------------------------------------------------------------


class SawaariBackend:
    """Async client for the Go backend."""

    def __init__(self, base_url: str):
        self._base = base_url.rstrip("/")

    async def compare(self, from_loc: str, to_loc: str, prefs: Optional[dict] = None) -> dict:
        async with httpx.AsyncClient(
            timeout=httpx.Timeout(12.0, connect=3.0)
        ) as client:
            resp = await client.post(
                f"{self._base}/v1/compare",
                json={"from": from_loc, "to": to_loc, "prefs": prefs or {}},
            )
            resp.raise_for_status()
            return resp.json()

    async def booking_status(self, booking_id: str) -> dict:
        async with httpx.AsyncClient(
            timeout=httpx.Timeout(8.0, connect=3.0)
        ) as client:
            resp = await client.get(f"{self._base}/v1/bookings/{booking_id}")
            resp.raise_for_status()
            return resp.json()

    async def create_booking(self, option_id: str, user_phone: str, rail: str = "deeplink") -> dict:
        async with httpx.AsyncClient(
            timeout=httpx.Timeout(10.0, connect=3.0)
        ) as client:
            resp = await client.post(
                f"{self._base}/v1/bookings",
                json={"optionId": option_id, "rail": rail, "userPhone": user_phone},
            )
            resp.raise_for_status()
            return resp.json()


# ---------------------------------------------------------------------------
# User session store (in-memory — swap for Redis in prod)
# ---------------------------------------------------------------------------


class SessionStore:
    """Per-user conversation state."""

    def __init__(self) -> None:
        self._sessions: dict[str, dict] = {}

    def get(self, phone: str) -> dict:
        if phone not in self._sessions:
            self._sessions[phone] = {
                "state": "idle",
                "from_location": None,
                "to_location": None,
                "prefs": {},
                "last_result": None,
            }
        return self._sessions[phone]

    def set_state(self, phone: str, state: str) -> None:
        self.get(phone)["state"] = state

    def reset(self, phone: str) -> None:
        self._sessions[phone] = {
            "state": "idle",
            "from_location": None,
            "to_location": None,
            "prefs": {},
            "last_result": None,
        }


# ---------------------------------------------------------------------------
# Application lifecycle
# ---------------------------------------------------------------------------


@asynccontextmanager
async def lifespan(_app: FastAPI):
    wa_client = WhatsAppClient(PHONE_NUMBER_ID, ACCESS_TOKEN) if PHONE_NUMBER_ID and ACCESS_TOKEN else None
    backend = SawaariBackend(BACKEND_URL)
    sessions = SessionStore()
    _app.state.wa = wa_client
    _app.state.backend = backend
    _app.state.sessions = sessions
    logger.info(
        "WhatsApp bot initialised  backend=%s  wa=%s",
        BACKEND_URL,
        "yes" if wa_client else "NO CREDENTIALS",
    )
    yield
    logger.info("WhatsApp bot shutting down")


app = FastAPI(
    title="Sawaari WhatsApp Bot",
    version="1.0.0",
    lifespan=lifespan,
)


# ---------------------------------------------------------------------------
# Webhook verification (GET)
# ---------------------------------------------------------------------------


@app.get("/webhook")
async def verify_webhook(request: Request):
    mode = request.query_params.get("hub.mode")
    token = request.query_params.get("hub.verify_token")
    challenge = request.query_params.get("hub.challenge", "")
    if mode == "subscribe" and token == VERIFY_TOKEN:
        logger.info("Webhook verified")
        return Response(content=challenge, media_type="text/plain")
    logger.warning("Webhook verification failed")
    return Response(content="Forbidden", status_code=403)


# ---------------------------------------------------------------------------
# Webhook handler (POST)
# ---------------------------------------------------------------------------


@app.post("/webhook")
async def handle_webhook(request: Request):
    try:
        body = await request.json()
    except Exception:
        return JSONResponse({"status": "error", "reason": "bad_json"}, 400)

    parsed = WebhookBody.model_validate(body)
    wa: Optional[WhatsAppClient] = request.app.state.wa
    sessions: SessionStore = request.app.state.sessions

    for entry in parsed.entry:
        for change in entry.changes:
            if change.field != "messages":
                continue
            value = change.value
            for raw_msg in value.messages:
                try:
                    msg = IncomingMessage.model_validate(raw_msg)
                except ValidationError as exc:
                    logger.warning("Malformed message: %s", exc)
                    continue

                # Fire-and-forget — do not block webhook ACK
                asyncio.ensure_future(route_message(request.app, msg, wa, sessions))

    return JSONResponse({"status": "ok"})


# ---------------------------------------------------------------------------
# Router
# ---------------------------------------------------------------------------


async def route_message(app: FastAPI, msg: IncomingMessage, wa: Optional[WhatsAppClient], sessions: SessionStore):
    if wa is None:
        logger.error("WhatsApp client not configured — cannot respond to %s", msg.from_number)
        return

    phone = msg.from_number
    sess = sessions.get(phone)
    backend: SawaariBackend = app.state.backend

    try:
        if msg.type == "text":
            await handle_text(wa, backend, sessions, phone, sess, msg)
        elif msg.type == "location":
            await handle_location(wa, backend, sessions, phone, sess, msg)
        elif msg.type == "interactive":
            await handle_interactive(wa, backend, sessions, phone, sess, msg)
        else:
            await wa.send_text(phone, "I only understand text, locations, and button taps. Type *help*.")
    except Exception as exc:
        logger.exception("Error routing message from %s: %s", phone, exc)
        try:
            await wa.send_text(phone, "Something went wrong. Please try again in a moment.")
        except Exception:
            pass


# ---------------------------------------------------------------------------
# Text handler
# ---------------------------------------------------------------------------


async def handle_text(wa: WhatsAppClient, backend: SawaariBackend, sessions: SessionStore, phone: str, sess: dict, msg: IncomingMessage):
    raw = msg.text.body.strip() if msg.text else ""
    tokens = raw.lower().split()

    # ---- Command dispatch ---------------------------------------------------
    if tokens and tokens[0] in ("help", "/help", "menu", "commands"):
        sessions.reset(phone)
        await wa.send_text(
            phone,
            (
                "*Sawaari — Your Commands*\n\n"
                "🚖 compare  —  Compare fares across all modes\n"
                "   Usage: `compare Karol Bagh to Saket`\n\n"
                "📍 from / to  —  Set pickup or dropoff\n"
                "   Usage: `from Connaught Place`\n\n"
                "🎫 book  —  Book a selected option\n"
                "   Usage: `book 1`\n\n"
                "📋 status  —  Check booking status\n"
                "   Usage: `status <booking_id>`\n\n"
                "⚙️ prefs  —  Set preferences (AC, Saheli, night)\n"
                "   Usage: `prefs ac` or `prefs saheli`\n\n"
                "🔄 reset  —  Start over\n"
                "❓ help  —  Show this menu\n\n"
                "_Tip: Share your GPS location for instant pickup!_"
            ),
        )
        return

    if tokens and tokens[0] == "reset":
        sessions.reset(phone)
        await wa.send_text(phone, "🔄 Session cleared. Type *help* or send a location to get started.")
        return

    if tokens and tokens[0] in ("prefs", "settings", "preferences"):
        await handle_prefs(wa, sessions, phone, sess, tokens[1:] if len(tokens) > 1 else [])
        return

    if tokens and tokens[0] == "book":
        await handle_book(wa, backend, phone, sess, tokens[1] if len(tokens) > 1 else "")
        return

    if tokens and tokens[0] == "status":
        await handle_status(wa, backend, phone, tokens[1] if len(tokens) > 1 else "")
        return

    # ---- Conversation flow --------------------------------------------------
    if sess["state"] == "awaiting_from":
        sess["from_location"] = raw
        sessions.set_state(phone, "awaiting_to")
        await wa.send_text(phone, f"✅ Pickup set: *{raw}*\n\nNow send your *dropoff* location:")
        return

    if sess["state"] == "awaiting_to":
        sess["to_location"] = raw
        sessions.set_state(phone, "comparing")
        await run_compare(wa, backend, sessions, phone, sess)
        return

    # ---- Compare shortcut: "from X to Y" / "compare X to Y" ---------------
    import re
    m = re.match(r"(?:compare\s+)?from\s+(.+?)\s+to\s+(.+)", raw, re.IGNORECASE)
    if m:
        sess["from_location"] = m.group(1).strip()
        sess["to_location"] = m.group(2).strip()
        await run_compare(wa, backend, sessions, phone, sess)
        return

    # Default: treat whole message as a "compare" intent
    sess["from_location"] = raw
    sessions.set_state(phone, "awaiting_to")
    await wa.send_text(
        phone,
        f"📍 Pickup set to: *{raw}*\n\nNow send your *dropoff* location (or type `compare {raw} to <dest>`).",
    )


# ---------------------------------------------------------------------------
# Location handler
# ---------------------------------------------------------------------------


async def handle_location(wa: WhatsAppClient, backend: SawaariBackend, sessions: SessionStore, phone: str, sess: dict, msg: IncomingMessage):
    loc = LocationPayload.model_validate(msg.location) if msg.location else LocationPayload()
    lat = loc.latitude
    lng = loc.longitude
    label = loc.name or loc.address or "your location"

    coord_str = f"{lat:.5f},{lng:.5f}"

    if sess["state"] == "awaiting_from":
        sess["from_location"] = coord_str
        sessions.set_state(phone, "awaiting_to")
        await wa.send_text(phone, f"📍 Pickup set to: *{label}*\n\nNow share your *dropoff* location.")
    elif sess["state"] == "awaiting_to":
        sess["to_location"] = coord_str
        await run_compare(wa, backend, sessions, phone, sess)
    else:
        # Fresh start
        sess["from_location"] = coord_str
        sessions.set_state(phone, "awaiting_to")
        await wa.send_text(phone, f"📍 Pickup set to: *{label}*\n\nNow share your *dropoff* location.")


# ---------------------------------------------------------------------------
# Interactive (button / list) handler
# ---------------------------------------------------------------------------


async def handle_interactive(wa: WhatsAppClient, backend: SawaariBackend, sessions: SessionStore, phone: str, sess: dict, msg: IncomingMessage):
    raw = msg.text if hasattr(msg, "text") and msg.text else {}
    # The interactive reply payload isn't in the validated model; re-parse
    body = await Request(scope={"type": "http"}).body()
    import json
    full = json.loads(body) if body else {}

    for entry in full.get("entry", []):
        for change in entry.get("changes", []):
            for m in change.get("value", {}).get("messages", []):
                if m.get("id") == msg.id:
                    inter = m.get("interactive", {})
                    if inter.get("type") == "button_reply":
                        btn_id = inter["button_reply"]["id"]
                        await handle_button_action(wa, backend, sessions, phone, sess, btn_id)
                        return
                    if inter.get("type") == "list_reply":
                        row_id = inter["list_reply"]["id"]
                        await handle_button_action(wa, backend, sessions, phone, sess, row_id)


async def handle_button_action(wa: WhatsAppClient, backend: SawaariBackend, sessions: SessionStore, phone: str, sess: dict, action_id: str):
    if action_id == "share_current_location":
        await wa.send_text(phone, "📍 Please use the attachment button to share your GPS location.")
    elif action_id == "enter_location_name":
        sessions.set_state(phone, "awaiting_from")
        await wa.send_text(phone, "✏️ Type your pickup location:")
    elif action_id.startswith("book_"):
        option_id = action_id[5:]
        await handle_book(wa, backend, phone, sess, option_id)
    elif action_id.startswith("compare_route_"):
        await wa.send_text(phone, "🔁 Comparing fares…")
        await run_compare(wa, backend, sessions, phone, sess)
    elif action_id.startswith("toggle_"):
        pref = action_id[7:]
        current = sess["prefs"]
        current[pref] = not current.get(pref, False)
        state_str = "ON" if current[pref] else "OFF"
        await wa.send_text(phone, f"⚙️ Preference *{pref}* is now {state_str}.\n\nType *compare* to see updated fares.")
    else:
        await wa.send_text(phone, "Got it! Type *help* for commands.")


# ---------------------------------------------------------------------------
# Compare logic
# ---------------------------------------------------------------------------


async def run_compare(wa: WhatsAppClient, backend: SawaariBackend, sessions: SessionStore, phone: str, sess: dict):
    from_loc = sess.get("from_location", "")
    to_loc = sess.get("to_location", "")
    if not from_loc or not to_loc:
        await wa.send_text(phone, "Please provide both pickup and dropoff locations.")
        sessions.set_state(phone, "awaiting_from")
        return

    await wa.send_text(phone, "🔍 Comparing fares across all transport modes…")

    try:
        data = await backend.compare(from_loc, to_loc, sess.get("prefs"))
    except httpx.HTTPStatusError as exc:
        logger.error("Backend error: %s", exc.response.status_code)
        await wa.send_text(phone, "⚠️ Backend is temporarily unavailable. Please try again in a moment.")
        return
    except Exception as exc:
        logger.exception("Backend call failed")
        await wa.send_text(phone, "⚠️ Something went wrong. Please retry.")
        return

    options = data.get("options", [])
    if not options:
        await wa.send_text(phone, f"No options found from *{from_loc}* to *{to_loc}*. Try broader location names.")
        sessions.reset(phone)
        return

    sess["last_result"] = data
    sessions.set_state(phone, "results_ready")

    # Build button/list rows
    rows = []
    buttons_short = []
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
        rows.append(
            {
                "id": f"book_{option_id}",
                "title": f"{provider} ({mode}){badge_str}",
                "description": f"{fare_str}  •  {eta_str}",
            }
        )
        if i < 3:
            buttons_short.append(
                {
                    "type": "reply",
                    "reply": {
                        "id": f"book_{option_id}",
                        "title": f"{provider} — {fare_str}",
                    },
                }
            )

    # Send primary message with text
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

    lines.append("\n_Select an option below to book:_")
    text_body = "\n".join(lines)

    if len(rows) > 3:
        await wa.send_interactive(
            phone,
            type_="list",
            header_text="Sawaari Fare Compare",
            body_text=text_body,
            footer_text="Select a fare to book",
            list_rows=rows,
            list_button_text=f"Book ({len(rows)} options)",
        )
    else:
        await wa.send_interactive(
            phone,
            type_="button",
            header_text="Sawaari Fare Compare",
            body_text=text_body,
            footer_text="Tap to book",
            buttons=buttons_short,
        )


# ---------------------------------------------------------------------------
# Sub-command handlers
# ---------------------------------------------------------------------------


async def handle_prefs(wa: WhatsAppClient, sessions: SessionStore, phone: str, sess: dict, args: list[str]):
    if not args:
        cur = sess.get("prefs", {})
        lines = ["⚙️ *Your Preferences:*\n"]
        for key in ("ac", "saheli", "night"):
            state = "ON" if cur.get(key) else "OFF"
            lines.append(f"  • {key}: {state}")
        lines.append("\n_To toggle: send `prefs <name>` — e.g. `prefs ac`_")
        await wa.send_text(phone, "\n".join(lines))
        return
    pref = args[0].lower()
    prefs = sess.setdefault("prefs", {})
    prefs[pref] = not prefs.get(pref, False)
    state_str = "ON" if prefs[pref] else "OFF"
    await wa.send_text(phone, f"⚙️ *{pref}* is now {state_str}.")


async def handle_book(wa: WhatsAppClient, backend: SawaariBackend, phone: str, sess: dict, raw_id: str):
    if not raw_id:
        await wa.send_text(phone, "Usage: `book <option_number>` — e.g. `book 1`")
        return
    option_id = raw_id.lstrip("#")
    await wa.send_text(phone, f"🎫 Booking option *{option_id}* …")
    try:
        result = await backend.create_booking(option_id, phone)
    except Exception as exc:
        logger.exception("Booking failed")
        await wa.send_text(phone, "⚠️ Booking failed. Please try again.")
        return
    booking_id = result.get("booking_id", "unknown")
    deeplink = result.get("deeplink", "")
    lines = [
        "✅ *Booking Initiated*\n",
        f"Booking ID: `{booking_id}`",
        f"Provider: {result.get('provider', 'Unknown')}",
    ]
    if deeplink:
        lines.append(f"\n🔗 {deeplink}")
    lines.append("\nTrack with: `status " + booking_id + "`")
    await wa.send_text(phone, "\n".join(lines))


async def handle_status(wa: WhatsAppClient, backend: SawaariBackend, phone: str, booking_id: str):
    if not booking_id:
        await wa.send_text(phone, "Usage: `status <booking_id>` — e.g. `status bk_12345`")
        return
    try:
        data = await backend.booking_status(booking_id)
    except Exception:
        await wa.send_text(phone, f"⚠️ Could not find booking `{booking_id}`. Check the ID and try again.")
        return
    status = data.get("status", "unknown").upper()
    provider = data.get("provider", "?")
    lines = [
        f"📋 *Booking Status*\n",
        f"ID: `{booking_id}`",
        f"Provider: {provider}",
        f"Status: *{status}*",
    ]
    eta = data.get("eta_minutes")
    if eta:
        lines.append(f"ETA: {eta} min")
    await wa.send_text(phone, "\n".join(lines))


# ---------------------------------------------------------------------------
# Health & misc
# ---------------------------------------------------------------------------


@app.get("/health")
async def health():
    return {
        "status": "ok",
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "backend": BACKEND_URL,
        "whatsapp": "configured" if PHONE_NUMBER_ID and ACCESS_TOKEN else "NOT_CONFIGURED",
    }


@app.get("/")
async def root():
    return {"service": "sawaari-whatsapp-bot", "version": "1.0.0"}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "app:app",
        host=os.getenv("HOST", "0.0.0.0"),
        port=int(os.getenv("PORT", 8000)),
        reload=os.getenv("DEBUG", "false").lower() == "true",
    )
