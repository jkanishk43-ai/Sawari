"""
Sawaari WhatsApp Bot — Message Templates
Interactive WhatsApp templates for fare comparison, booking, and status.
"""

from __future__ import annotations

from typing import Any, Optional

# ---------------------------------------------------------------------------
# Emoji constants
# ---------------------------------------------------------------------------

E = {
    "car": "🚗",
    "auto": "🛺",
    "bus": "🚌",
    "metro": "🚇",
    "bike": "🏍️",
    "train": "🚂",
    "walk": "🚶",
    "ok": "✅",
    "err": "❌",
    "star": "⭐",
    "crown": "👑",
    "savings": "💰",
    "clock": "⏱️",
    "pin": "📍",
    "warn": "⚠️",
    "info": "ℹ️",
    "ticket": "🎫",
    "arrow": "➡️",
    "fast": "⚡",
    "eco": "🌿",
    "saheli": "♀️",
    "night": "🌙",
    "ac": "❄️",
    "search": "🔍",
}

MODE_ICON = {
    "car": E["car"],
    "auto": E["auto"],
    "bus": E["bus"],
    "metro": E["metro"],
    "bike": E["bike"],
    "train": E["train"],
    "walk": E["walk"],
    "cab": E["car"],
    "bike_taxi": E["bike"],
    "auto_rickshaw": E["auto"],
    "e_rickshaw": "🛺",
    "default": E["car"],
}


# ---------------------------------------------------------------------------
# Duration / currency helpers
# ---------------------------------------------------------------------------


def fmt_duration(seconds: int) -> str:
    if seconds < 60:
        return f"{seconds}s"
    if seconds < 3600:
        return f"{seconds // 60} min"
    return f"{seconds // 3600}h {(seconds % 3600) // 60}m"


def fmt_fare(min_fare: Any, max_fare: Any = None) -> str:
    try:
        mn = f"₹{float(min_fare):.0f}"
        if max_fare and float(max_fare) != float(min_fare):
            return f"{mn}-₹{float(max_fare):.0f}"
        return mn
    except (TypeError, ValueError):
        return "₹?"


# ---------------------------------------------------------------------------
# Comparison results text
# ---------------------------------------------------------------------------


def comparison_text(
    from_loc: str,
    to_loc: str,
    options: list[dict[str, Any]],
    max_options: int = 5,
) -> str:
    """
    Build the plain-text body for a compare result.
    Used as the body of an interactive template.
    """
    lines = [f"*{from_loc} {E['arrow']} {to_loc}*\n"]
    for i, opt in enumerate(options[:max_options], 1):
        provider = opt.get("provider", "?")
        mode = opt.get("mode", "car")
        icon = MODE_ICON.get(mode, MODE_ICON["default"])
        fare = fmt_fare(opt.get("fare_min", 0), opt.get("fare_max"))
        eta = fmt_duration(opt.get("eta_minutes", opt.get("eta_seconds", 0)))
        badge = opt.get("badge", "")
        badge_str = f" *[{badge.upper()}]*" if badge else ""
        lines.append(f"{i}. {icon} *{provider}* ({mode}){badge_str}\n   💰 {fare}  ⏱️ {eta}")
    lines.append("\n_Select an option below to book:_")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Interactive list rows (for List template)
# ---------------------------------------------------------------------------


def comparison_list_rows(
    options: list[dict[str, Any]],
    max_rows: int = 10,
) -> list[dict[str, str]]:
    rows = []
    for i, opt in enumerate(options[:max_rows]):
        option_id = opt.get("id", f"opt_{i}")
        provider = opt.get("provider", "?")
        mode = opt.get("mode", "car")
        fare = fmt_fare(opt.get("fare_min", 0), opt.get("fare_max"))
        eta = fmt_duration(opt.get("eta_minutes", opt.get("eta_seconds", 0)))
        badge = opt.get("badge", "")
        suffix = f" [{badge.upper()}]" if badge else ""
        rows.append(
            {
                "id": f"book_{option_id}",
                "title": f"{provider} ({mode}){suffix}",
                "description": f"{fare}  •  {eta}",
            }
        )
    return rows


# ---------------------------------------------------------------------------
# Reply-button set (for Button template)
# ---------------------------------------------------------------------------


def comparison_reply_buttons(
    options: list[dict[str, Any]],
    max_buttons: int = 3,
) -> list[dict[str, Any]]:
    """Up to 3 reply buttons for quick booking."""
    buttons = []
    for opt in options[:max_buttons]:
        option_id = opt.get("id", "?")
        provider = opt.get("provider", "?")
        fare = fmt_fare(opt.get("fare_min", 0), opt.get("fare_max"))
        buttons.append(
            {
                "type": "reply",
                "reply": {
                    "id": f"book_{option_id}",
                    "title": f"{provider} — {fare}",
                },
            }
        )
    return buttons


# ---------------------------------------------------------------------------
# Booking confirmation
# ---------------------------------------------------------------------------


def booking_confirmation_text(result: dict[str, Any]) -> str:
    lines = [
        f"{E['ok']} *Booking Initiated*\n",
        f"Booking ID: `{result.get('booking_id', '?')}`",
        f"Provider: {result.get('provider', 'Unknown')}",
    ]
    if result.get("deeplink"):
        lines.append(f"\n🔗 {result['deeplink']}")
    lines.append(f"\nTrack with: `status {result.get('booking_id', '')}`")
    return "\n".join(lines)


def booking_confirmation_buttons(result: dict[str, Any]) -> list[dict[str, Any]]:
    booking_id = result.get("booking_id", "")
    buttons = [
        {
            "type": "reply",
            "reply": {
                "id": f"status_{booking_id}",
                "title": "Check Status",
            },
        }
    ]
    if result.get("deeplink"):
        buttons.append(
            {
                "type": "reply",
                "reply": {
                    "id": f"open_{booking_id}",
                    "title": "Open App",
                },
            }
        )
    return buttons


# ---------------------------------------------------------------------------
# Status response
# ---------------------------------------------------------------------------


def status_text(data: dict[str, Any], booking_id: str) -> str:
    status = data.get("status", "unknown").upper()
    provider = data.get("provider", "?")
    eta = data.get("eta_minutes")
    lines = [
        "📋 *Booking Status*\n",
        f"ID: `{booking_id}`",
        f"Provider: {provider}",
        f"Status: *{status}*",
    ]
    if eta:
        lines.append(f"ETA: {eta} min")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Help text
# ---------------------------------------------------------------------------


HELP_TEXT = (
    "*Sawaari — Your Commands*\n\n"
    "🚖 compare  Compare fares across all modes\n"
    "   `compare Karol Bagh to Saket`\n\n"
    "📍 from / to  Set pickup or dropoff\n"
    "   `from Connaught Place`\n\n"
    "🎫 book  Book a selected option\n"
    "   `book 1`\n\n"
    "📋 status  Check booking status\n"
    "   `status <booking_id>`\n\n"
    "⚙️ prefs  Toggle preferences\n"
    "   `prefs ac` | `prefs saheli` | `prefs night`\n\n"
    "🔄 reset  Start over\n"
    "❓ help  This menu\n\n"
    "_Tip: Share your GPS location for instant pickup!_"
)


def help_buttons() -> list[dict[str, Any]]:
    return [
        {
            "type": "reply",
            "reply": {"id": "help_compare", "title": "Compare Fares"},
        },
        {
            "type": "reply",
            "reply": {"id": "help_book", "title": "Book a Ride"},
        },
        {
            "type": "reply",
            "reply": {"id": "help_settings", "title": "Settings"},
        },
    ]


# ---------------------------------------------------------------------------
# Prefs prompt
# ---------------------------------------------------------------------------


def prefs_text(current_prefs: dict[str, bool]) -> str:
    lines = ["⚙️ *Your Preferences:*\n"]
    for key in ("ac", "saheli", "night"):
        state = "ON" if current_prefs.get(key) else "OFF"
        lines.append(f"  • {key}: {state}")
    lines.append("\n_To toggle: send `prefs <name>`_")
    return "\n".join(lines)


def prefs_buttons(current_prefs: dict[str, bool]) -> list[dict[str, Any]]:
    return [
        {
            "type": "reply",
            "reply": {
                "id": f"toggle_ac",
                "title": f"❄️ AC: {'ON' if current_prefs.get('ac') else 'OFF'}",
            },
        },
        {
            "type": "reply",
            "reply": {
                "id": f"toggle_saheli",
                "title": f"♀️ Saheli: {'ON' if current_prefs.get('saheli') else 'OFF'}",
            },
        },
        {
            "type": "reply",
            "reply": {
                "id": f"toggle_night",
                "title": f"🌙 Night: {'ON' if current_prefs.get('night') else 'OFF'}",
            },
        },
    ]


# ---------------------------------------------------------------------------
# Location prompt (interactive)
# ---------------------------------------------------------------------------


def location_request_rows() -> list[dict[str, str]]:
    return [
        {
            "id": "share_current_location",
            "title": "Share Current Location",
            "description": "Use GPS to share your pickup location",
        },
        {
            "id": "enter_location_name",
            "title": "Enter Location Name",
            "description": "Type a place name manually",
        },
    ]


def location_acknowledgement_text(latitude: float, longitude: float, name: str = "") -> str:
    label = name or f"{latitude:.5f}, {longitude:.5f}"
    return (
        f"{E['ok']} Location received: *{label}*\n\n"
        "Now share your *dropoff* location to compare fares."
    )


# ---------------------------------------------------------------------------
# Error
# ---------------------------------------------------------------------------


def error_text(message: str) -> str:
    return f"{E['warn']} *Error*\n\n{message}\n\nType *help* for available commands."


# ---------------------------------------------------------------------------
# Fare card detail (breakdown)
# ---------------------------------------------------------------------------


def fare_card_detail(option: dict[str, Any]) -> str:
    provider = option.get("provider", "?")
    mode = option.get("mode", "?")
    icon = MODE_ICON.get(mode, MODE_ICON["default"])
    lines = [
        f"{icon} *{provider}* ({mode})\n",
        f"  Base fare: {fmt_fare(option.get('fare_min', 0))}",
    ]
    if option.get("fare_max"):
        lines.append(f"  Max fare: {fmt_fare(option.get('fare_max'))}")
    eta = option.get("eta_minutes", option.get("eta_seconds", 0))
    lines.append(f"  ETA: {fmt_duration(eta) if isinstance(eta, int) else f'{eta} min'}")
    if option.get("badge"):
        lines.append(f"  Badge: {option['badge'].upper()}")
    if option.get("surge_factor"):
        lines.append(f"  Surge: {option['surge_factor']}x")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Template-name constants (pre-approved HSM templates in Meta dashboard)
# ---------------------------------------------------------------------------


HSM_TEMPLATES = {
    "booking_confirmed": {
        "name": "booking_confirmed",
        "language": "en",
        "params": ["booking_id", "provider", "deeplink"],
    },
    "fare_alert": {
        "name": "fare_alert",
        "language": "en",
        "params": ["route", "old_price", "new_price"],
    },
}


def build_hsm(template_key: str, param_values: list[str]) -> Optional[dict[str, Any]]:
    """Build a pre-approved template payload for outbound notifications."""
    tpl = HSM_TEMPLATES.get(template_key)
    if not tpl:
        return None
    components = [
        {
            "type": "body",
            "parameters": [
                {"type": "text", "text": v} for v in param_values[: len(tpl["params"])]
            ],
        }
    ]
    return {
        "messaging_product": "whatsapp",
        "to": "",  # filled by caller
        "type": "template",
        "template": {
            "name": tpl["name"],
            "language": {"code": tpl["language"]},
            "components": components,
        },
    }
