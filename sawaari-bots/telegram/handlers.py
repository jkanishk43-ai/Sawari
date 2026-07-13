"""
Sawaari Telegram Bot - Handlers

Command and message handlers for the Sawaari fare comparison bot.
"""

import logging
import os
import httpx
from typing import Optional, Tuple

from telegram import Update, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import ContextTypes, ConversationHandler

# Configure logging
logger = logging.getLogger(__name__)

# Backend API URL
BACKEND_URL = os.getenv("SAWAARI_BACKEND_URL", "http://localhost:8080")

# Conversation states
WAITING_FROM = 1
WAITING_TO = 2


# ===================
# Helper Functions
# ===================

async def fetch_comparison(from_loc: str, to_loc: str) -> dict:
    """Fetch comparison results from the backend API."""
    async with httpx.AsyncClient(timeout=30.0) as client:
        try:
            response = await client.get(
                f"{BACKEND_URL}/api/compare",
                params={"from": from_loc, "to": to_loc},
            )
            response.raise_for_status()
            return response.json()
        except httpx.HTTPError as e:
            logger.error(f"Backend API error: {e}")
            return {"error": str(e)}


async def fetch_nearby_stops(lat: float, lng: float, radius: int = 1000) -> dict:
    """Fetch nearby stops from the backend API."""
    async with httpx.AsyncClient(timeout=30.0) as client:
        try:
            response = await client.get(
                f"{BACKEND_URL}/api/stops/nearby",
                params={"lat": lat, "lng": lng, "r": radius},
            )
            response.raise_for_status()
            return response.json()
        except httpx.HTTPError as e:
            logger.error(f"Backend API error: {e}")
            return {"error": str(e)}


def format_price(price: dict) -> str:
    """Format price object for display."""
    currency = price.get("currency", "INR")
    min_price = price.get("min", 0)
    max_price = price.get("max", 0)

    if min_price == max_price:
        return f"{currency} {min_price:.0f}"

    return f"{currency} {min_price:.0f} - {max_price:.0f}"


def format_ride_option(option: dict, index: int) -> str:
    """Format a single ride option for display."""
    provider = option.get("provider", "Unknown")
    mode = option.get("mode", "ride").upper()
    display_name = option.get("display_name", f"{provider} {mode}")
    price_info = format_price(option.get("price", {}))
    eta = option.get("eta_minutes", 0)
    badges = option.get("badges", [])

    badge_str = ""
    if badges:
        badge_str = " " + " ".join([f"[{b}]" for b in badges])

    reliability = option.get("reliability", 1.0)
    reliability_str = f"({reliability*100:.0f}% reliable)"

    return (
        f"{index}. *{display_name}*\n"
        f"   Price: {price_str} | ETA: {eta} min\n"
        f"   {reliability_str}{badge_str}"
    )


# ===================
# Main Menu Keyboard
# ===================

def get_main_menu_keyboard() -> InlineKeyboardMarkup:
    """Create the main menu keyboard."""
    keyboard = [
        [
            InlineKeyboardButton("Compare Fares", callback_data="menu_compare"),
            InlineKeyboardButton("My Bookings", callback_data="menu_bookings"),
        ],
        [
            InlineKeyboardButton("Nearby Stops", callback_data="menu_nearby"),
            InlineKeyboardButton("Help", callback_data="menu_help"),
        ],
        [
            InlineKeyboardButton("Share Location", callback_data="menu_location", request_location=True),
        ],
    ]
    return InlineKeyboardMarkup(keyboard)


def get_ride_options_keyboard(options: list) -> InlineKeyboardMarkup:
    """Create keyboard with ride options for booking."""
    keyboard = []
    for i, option in enumerate(options[:5]):  # Limit to 5 options
        display_name = option.get("display_name", "Option")
        option_id = option.get("id", "")
        keyboard.append([
            InlineKeyboardButton(
                f"Book {display_name[:30]}",
                callback_data=f"book_{option_id}"
            )
        ])

    keyboard.append([
        InlineKeyboardButton("Back to Menu", callback_data="menu_main"),
    ])
    return InlineKeyboardMarkup(keyboard)


# ===================
# Command Handlers
# ===================

async def start_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /start command - Welcome message with main menu."""
    welcome_message = (
        "🚌 *Welcome to Sawaari!* 🚖\n\n"
        "Your Delhi multimodal fare comparison and booking assistant.\n\n"
        "I can help you:\n"
        "• Compare fares across bus, metro, auto, and cabs\n"
        "• Find nearby stops and transit options\n"
        "• Book rides through partner apps\n\n"
        "*Quick Actions:*"
    )

    await update.message.reply_text(
        welcome_message,
        parse_mode="Markdown",
        reply_markup=get_main_menu_keyboard(),
    )


async def help_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /help command - Show help and available commands."""
    help_text = (
        "*Sawaari Bot - Help*\n\n"
        "*Available Commands:*\n"
        "/start - Get started with Sawaari\n"
        "/compare - Compare fares between two locations\n"
        "/status - Check your booking status\n"
        "/nearby - Find nearby transit stops\n"
        "/help - Show this help message\n\n"
        "*How to Use:*\n\n"
        "*Compare Fares:*\n"
        "1. Type /compare\n"
        "2. Enter your pickup location\n"
        "3. Enter your destination\n"
        "4. View and compare all options\n\n"
        "*Inline Queries:*\n"
        "Type @SawaariBot in any chat:\n"
        "@SawaariBot Connaught Place to Rajiv Chowk\n\n"
        "*Share Location:*\n"
        "You can share your location to find nearby stops.\n\n"
        "*Supported Modes:*\n"
        "🚌 Bus (DTC/DIMTS)\n"
        "🚇 Metro (DMRC)\n"
        "🛺 Auto / E-rickshaw\n"
        "🚗 Cab / Taxi\n"
        "🏍️ Bike Taxi\n\n"
        "Need more help? Contact support."
    )

    await update.message.reply_text(help_text, parse_mode="Markdown")


async def compare_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    """Handle /compare command - Start comparison flow."""
    # Clear any previous comparison data
    context.user_data.clear()

    message = (
        "🔍 *Compare Fares*\n\n"
        "Let's find the best ride for you!\n\n"
        "*Step 1:* Where are you starting from?\n\n"
        "Send me:\n"
        "• Your location (tap the paper clip icon → Location)\n"
        "• Or type an address (e.g., 'Connaught Place')\n"
        "• Or a landmark (e.g., 'Rajiv Chowk Metro')"
    )

    await update.message.reply_text(message, parse_mode="Markdown")
    return WAITING_FROM


async def compare_from(update: Update, context: ContextTypes.DEFAULT_TYPE, is_from: bool = True) -> int:
    """Handle from location input in compare flow."""
    user_data = context.user_data

    # Handle location message
    if update.message.location:
        lat = update.message.location.latitude
        lng = update.message.location.longitude
        user_data["from_location"] = {"lat": lat, "lng": lng}
        from_text = f"📍 Location ({lat:.4f}, {lng:.4f})"
    else:
        # Handle text input
        from_text = update.message.text.strip()
        user_data["from_location"] = from_text

    # Store for logging
    user_data["from_text"] = from_text
    logger.info(f"User {update.effective_user.id} - From location: {from_text}")

    # Ask for destination
    message = (
        f"✅ *From:* {from_text}\n\n"
        "*Step 2:* Where are you going?\n\n"
        "Send me:\n"
        "• Your destination (tap the paper clip icon → Location)\n"
        "• Or type an address (e.g., 'Nehru Place')\n"
        "• Or a landmark (e.g., 'AIIMS Metro')"
    )

    await update.message.reply_text(message, parse_mode="Markdown")
    return WAITING_TO


async def compare_to(update: Update, context: ContextTypes.DEFAULT_TYPE, is_from: bool = False) -> int:
    """Handle to location input in compare flow."""
    user_data = context.user_data

    # Handle location message
    if update.message.location:
        lat = update.message.location.latitude
        lng = update.message.location.longitude
        user_data["to_location"] = {"lat": lat, "lng": lng}
        to_text = f"📍 Location ({lat:.4f}, {lng:.4f})"
    else:
        # Handle text input
        to_text = update.message.text.strip()
        user_data["to_location"] = to_text

    # Store for logging
    user_data["to_text"] = to_text
    logger.info(f"User {update.effective_user.id} - To location: {to_text}")

    # Show loading message
    loading_msg = await update.message.reply_text(
        "🔄 Fetching comparison results...\n\n"
        "This may take a few seconds."
    )

    # Fetch comparison from backend
    from_loc = user_data.get("from_location")
    to_loc = user_data.get("to_location")

    if isinstance(from_loc, str):
        from_query = from_loc
    elif isinstance(from_loc, dict):
        from_query = f"{from_loc['lat']},{from_loc['lng']}"
    else:
        from_query = str(from_loc)

    if isinstance(to_loc, str):
        to_query = to_loc
    elif isinstance(to_loc, dict):
        to_query = f"{to_loc['lat']},{to_loc['lng']}"
    else:
        to_query = str(to_loc)

    result = await fetch_comparison(from_query, to_query)

    # Edit the loading message with results
    if "error" in result:
        await loading_msg.edit_text(
            f"❌ *Error fetching results*\n\n"
            f"{result['error']}\n\n"
            "Please try again with /compare",
            parse_mode="Markdown"
        )
        return ConversationHandler.END

    options = result.get("options", [])

    if not options:
        await loading_msg.edit_text(
            "😕 *No results found*\n\n"
            "We couldn't find any routes between these locations.\n"
            "Please try different locations with /compare",
            parse_mode="Markdown"
        )
        return ConversationHandler.END

    # Format results
    header = (
        f"📊 *Fare Comparison*\n\n"
        f"*From:* {user_data.get('from_text', from_query)}\n"
        f"*To:* {user_data.get('to_text', to_query)}\n\n"
        f"Found {len(options)} options:\n\n"
    )

    # Build results message
    results = []
    for i, option in enumerate(options, 1):
        results.append(format_ride_option(option, i))

    results_text = "\n\n".join(results)

    # Add footer
    footer = (
        f"\n\n*Prices are estimates and may vary.*\n"
        f"Results expire at: {result.get('expires_at', 'N/A')}"
    )

    full_message = header + results_text + footer

    # Split message if too long (Telegram limit is ~4096 chars)
    if len(full_message) > 4000:
        await loading_msg.edit_text(header, parse_mode="Markdown")
        for i in range(0, len(results), 3):
            chunk = "\n\n".join(results[i:i+3])
            await update.message.reply_text(chunk, parse_mode="Markdown")
        await update.message.reply_text(footer, parse_mode="Markdown")
    else:
        await loading_msg.edit_text(full_message, parse_mode="Markdown")

    # Send booking buttons
    await update.message.reply_text(
        "Select an option to book:",
        reply_markup=get_ride_options_keyboard(options),
    )

    # Store results for potential booking
    context.user_data["comparison_results"] = options
    context.user_data["from_query"] = from_query
    context.user_data["to_query"] = to_query

    return ConversationHandler.END


async def status_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /status command - Check booking status."""
    # For now, show a placeholder since we don't have booking state management
    status_text = (
        "📋 *Booking Status*\n\n"
        "You don't have any active bookings.\n\n"
        "To book a ride:\n"
        "1. Use /compare to find options\n"
        "2. Select a ride and tap 'Book'\n\n"
        "Your booked rides will appear here."
    )

    await update.message.reply_text(status_text, parse_mode="Markdown")


async def nearby_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /nearby command - Find nearby stops."""
    message = (
        "📍 *Find Nearby Stops*\n\n"
        "Share your location to find nearby transit stops:\n\n"
        "• Tap the paper clip icon\n"
        "• Select 'Location'\n"
        "• Share your current position\n\n"
        "I'll show you the nearest:\n"
        "🚌 Bus stops\n"
        "🚇 Metro stations\n"
        "🛺 Auto stands"
    )

    keyboard = [
        [InlineKeyboardButton("📍 Share My Location", callback_data="nearby_share", request_location=True)],
    ]
    reply_markup = InlineKeyboardMarkup(keyboard)

    await update.message.reply_text(message, parse_mode="Markdown", reply_markup=reply_markup)


async def location_handler(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle location messages for nearby stops search."""
    if not update.message.location:
        return

    lat = update.message.location.latitude
    lng = update.message.location.longitude

    # Fetch nearby stops
    result = await fetch_nearby_stops(lat, lng)

    if "error" in result:
        await update.message.reply_text(
            f"❌ Error finding nearby stops: {result['error']}",
            parse_mode="Markdown"
        )
        return

    stops = result.get("stops", [])

    if not stops:
        await update.message.reply_text(
            f"📍 Location: ({lat:.4f}, {lng:.4f})\n\n"
            "No transit stops found nearby.\n"
            "This area might not be covered yet.",
            parse_mode="Markdown"
        )
        return

    # Format stops
    header = f"📍 *Nearby Stops*\n*Your Location:* ({lat:.4f}, {lng:.4f})\n\n"
    stops_text = "*Found {count} stops:*\n\n".format(count=len(stops))

    for stop in stops[:10]:  # Limit to 10 stops
        name = stop.get("name", "Unknown Stop")
        mode = stop.get("mode", "transit")
        distance = stop.get("distance_m", 0)
        mode_emoji = {
            "bus": "🚌",
            "metro": "🚇",
            "auto": "🛺",
            "train": "🚆",
        }.get(mode, "🚌")

        stops_text += f"{mode_emoji} *{name}*\n   {distance}m away\n\n"

    await update.message.reply_text(header + stops_text, parse_mode="Markdown")


async def book_callback(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle booking callback from inline buttons."""
    query = update.callback_query
    await query.answer()  # Acknowledge the callback

    # Parse callback data
    data = query.data
    if not data.startswith("book_"):
        return

    option_id = data[5:]  # Remove "book_" prefix

    # Find the option in stored results
    options = context.user_data.get("comparison_results", [])
    option = None
    for opt in options:
        if opt.get("id") == option_id:
            option = opt
            break

    if not option:
        await query.edit_message_text(
            "❌ *Booking Error*\n\n"
            "This option is no longer available.\n"
            "Please search again with /compare",
            parse_mode="Markdown"
        )
        return

    # Get deeplink or booking info
    deeplink = option.get("deeplink")
    provider = option.get("provider", "Unknown")
    mode = option.get("mode", "ride")
    display_name = option.get("display_name", f"{provider} {mode}")

    if deeplink:
        # Send booking deeplink
        keyboard = [
            [InlineKeyboardButton(f"Book on {provider}", url=deeplink)],
            [InlineKeyboardButton("Back to Results", callback_data="back_results")],
        ]
        reply_markup = InlineKeyboardMarkup(keyboard)

        await query.edit_message_text(
            f"✅ *Ready to Book*\n\n"
            f"*Option:* {display_name}\n"
            f"*Provider:* {provider}\n\n"
            f"Tap the button below to complete your booking on {provider}'s app.",
            parse_mode="Markdown",
            reply_markup=reply_markup,
        )
    else:
        # No direct booking available
        keyboard = [
            [InlineKeyboardButton("Back to Results", callback_data="back_results")],
        ]
        reply_markup = InlineKeyboardMarkup(keyboard)

        await query.edit_message_text(
            f"ℹ️ *Booking via {provider}*\n\n"
            f"*Option:* {display_name}\n\n"
            f"This provider doesn't support in-app booking yet.\n"
            f"You'll need to open the {provider} app separately.\n\n"
            f"We recommend:\n"
            f"1. Open {provider}\n"
            f"2. Enter the same route\n"
            f"3. The fare shown should be similar",
            parse_mode="Markdown",
            reply_markup=reply_markup,
        )


async def cancel_handler(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    """Handle /cancel command to exit conversation."""
    context.user_data.clear()
    await update.message.reply_text(
        "❌ *Cancelled*\n\n"
        "Your action has been cancelled.\n"
        "Use /start to return to the main menu.",
        parse_mode="Markdown"
    )
    return ConversationHandler.END


async def error_handler(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle errors."""
    logger.error(f"Error: {context.error}")

    error_message = (
        "❌ *An error occurred*\n\n"
        "Something went wrong. Please try again.\n"
        "If the problem persists, contact support."
    )

    if update and update.effective_message:
        await update.effective_message.reply_text(error_message, parse_mode="Markdown")
