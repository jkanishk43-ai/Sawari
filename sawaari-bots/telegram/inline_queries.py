"""
Sawaari Telegram Bot - Inline Queries

Inline query support for quick fare comparisons from any chat.
Format: @SawaariBot from_location to_location
"""

import logging
import os
import re
import time
from typing import Optional

import httpx
from telegram import Update, InlineQueryResultArticle, InputTextMessageContent
from telegram.ext import ContextTypes

# Configure logging
logger = logging.getLogger(__name__)

# Backend API URL
BACKEND_URL = os.getenv("SAWAARI_BACKEND_URL", "http://localhost:8080")

# Inline query cache (simple in-memory cache)
# Format: {(from_query, to_query): (result, timestamp)}
# TTL: 60 seconds
_inline_cache: dict = {}
CACHE_TTL = 60


def _get_cache_key(from_query: str, to_query: str) -> tuple:
    """Generate cache key for inline query."""
    return (from_query.lower().strip(), to_query.lower().strip())


def _get_cached_result(from_query: str, to_query: str) -> Optional[dict]:
    """Get cached result if available and not expired."""
    key = _get_cache_key(from_query, to_query)
    if key in _inline_cache:
        result, timestamp = _inline_cache[key]
        if time.time() - timestamp < CACHE_TTL:
            return result
        else:
            # Cache expired, remove it
            del _inline_cache[key]
    return None


def _set_cached_result(from_query: str, to_query: str, result: dict) -> None:
    """Cache a result."""
    key = _get_cache_key(from_query, to_query)
    _inline_cache[key] = (result, time.time())

    # Clean old cache entries if cache is getting large
    if len(_inline_cache) > 100:
        current_time = time.time()
        expired_keys = [
            k for k, (_, ts) in _inline_cache.items()
            if current_time - ts >= CACHE_TTL
        ]
        for k in expired_keys:
            del _inline_cache[k]


async def _fetch_comparison_inline(from_query: str, to_query: str) -> dict:
    """Fetch comparison from backend with caching."""
    # Check cache first
    cached = _get_cached_result(from_query, to_query)
    if cached:
        logger.info(f"Cache hit for: {from_query} -> {to_query}")
        return cached

    # Fetch from backend
    async with httpx.AsyncClient(timeout=30.0) as client:
        try:
            response = await client.get(
                f"{BACKEND_URL}/api/compare",
                params={"from": from_query, "to": to_query},
            )
            response.raise_for_status()
            result = response.json()
        except httpx.HTTPError as e:
            logger.error(f"Backend API error: {e}")
            return {"error": str(e)}

    # Cache the result
    _set_cached_result(from_query, to_query, result)
    return result


def _parse_inline_query(text: str) -> Optional[tuple]:
    """
    Parse inline query text.

    Supported formats:
    - "from_location to_location" (space separated)
    - "from_location to to_location" (with 'to' keyword)
    - "from_location -> to_location" (with arrow)
    - "from_location : to_location" (with colon)

    Returns (from_query, to_query) or None if invalid.
    """
    if not text:
        return None

    text = text.strip()

    # Try "from to to_location" format
    match = re.match(r'^(.+?)\s+to\s+(.+)$', text, re.IGNORECASE)
    if match:
        return (match.group(1).strip(), match.group(2).strip())

    # Try "from -> to_location" format
    match = re.match(r'^(.+?)\s*->\s*(.+)$', text)
    if match:
        return (match.group(1).strip(), match.group(2).strip())

    # Try "from : to_location" format
    match = re.match(r'^(.+?)\s*:\s*(.+)$', text)
    if match:
        return (match.group(1).strip(), match.group(2).strip())

    # Split on first " to " occurrence (case insensitive)
    parts = re.split(r'\s+to\s+', text, maxsplit=1, flags=re.IGNORECASE)
    if len(parts) == 2:
        return (parts[0].strip(), parts[1].strip())

    # If only two words separated by space, treat as from to
    words = text.split()
    if len(words) >= 2:
        # Assume everything before last word is from, last word is to
        return (' '.join(words[:-1]), words[-1])

    return None


def _format_price_short(price: dict) -> str:
    """Format price for inline result (compact format)."""
    currency = price.get("currency", "INR")
    min_price = price.get("min", 0)
    max_price = price.get("max", 0)

    if min_price == max_price:
        return f"{currency} {min_price:.0f}"

    return f"{currency} {min_price:.0f}-{max_price:.0f}"


def _format_mode_emoji(mode: str) -> str:
    """Get emoji for transport mode."""
    emojis = {
        "bus": "🚌",
        "metro": "🚇",
        "auto": "🛺",
        "cab": "🚗",
        "bike": "🏍️",
        "rickshaw": "🛺",
        "train": "🚆",
        "walk": "🚶",
    }
    return emojis.get(mode.lower(), "🚶")


def _build_inline_result(
    option: dict,
    index: int,
    from_query: str,
    to_query: str,
) -> InlineQueryResultArticle:
    """Build a single inline query result."""
    provider = option.get("provider", "Unknown")
    mode = option.get("mode", "ride")
    display_name = option.get("display_name", f"{provider} {mode}")
    price_info = _format_price_short(option.get("price", {}))
    eta = option.get("eta_minutes", 0)
    badges = option.get("badges", [])

    mode_emoji = _format_mode_emoji(mode)

    # Build badge string
    badge_str = ""
    if badges:
        badge_labels = {
            "CHEAPEST": "💚 Cheapest",
            "FASTEST": "⚡ Fastest",
            "SMART_PICK": "⭐ Smart Pick",
            "SAHELI": "👩 Saheli",
        }
        badge_str = " " + " ".join([
            badge_labels.get(b, b) for b in badges
        ])

    # Build message content
    message = (
        f"{mode_emoji} *{display_name}*\n"
        f"💰 {price_info} | ⏱️ {eta} min\n"
        f"{badge_str}\n\n"
        f"/book_{option.get('id', '')} to book this ride"
    )

    # Build title (limited to 64 chars for Telegram)
    title_parts = [mode_emoji, display_name[:20], f"{price_info}", f"{eta}min"]
    title = " | ".join(title_parts)
    if len(title) > 64:
        title = title[:61] + "..."

    return InlineQueryResultArticle(
        id=f"option_{index}_{option.get('id', index)}",
        title=title,
        description=f"{price_info} | {eta} min to destination",
        input_message_content=InputTextMessageContent(
            message_text=message,
            parse_mode="Markdown",
        ),
    )


async def inline_query_handler(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """
    Handle inline queries.

    Format: @SawaariBot from_location to_location
    Example: @SawaariBot Connaught Place to Rajiv Chowk
    """
    query = update.inline_query
    query_text = query.query.strip()

    logger.info(f"Inline query from {query.from_user.id}: '{query_text}'")

    # Parse the query
    parsed = _parse_inline_query(query_text)

    if not parsed:
        # Invalid query format
        results = [
            InlineQueryResultArticle(
                id="help",
                title="How to use Sawaari Bot",
                description="Type: @SawaariBot pickup to destination",
                input_message_content=InputTextMessageContent(
                    message_text=(
                        "🔍 *Sawaari Inline Query*\n\n"
                        "To compare fares inline, type:\n"
                        "`@SawaariBot pickup to destination`\n\n"
                        "*Examples:*\n"
                        "• `@SawaariBot Connaught Place to Rajiv Chowk`\n"
                        "• `@SawaariBot AIIMS Metro to Nehru Place`\n"
                        "• `@SawaariBot Dwaraka to Gurgaon`"
                    ),
                    parse_mode="Markdown",
                ),
            )
        ]
        await query.answer(results, cache_time=300)
        return

    from_query, to_query = parsed

    if not from_query or not to_query:
        results = [
            InlineQueryResultArticle(
                id="invalid",
                title="Please provide both pickup and destination",
                description='Example: "Connaught Place to Rajiv Chowk"',
                input_message_content=InputTextMessageContent(
                    message_text="❌ Please provide both pickup and destination locations.",
                    parse_mode="Markdown",
                ),
            )
        ]
        await query.answer(results, cache_time=60)
        return

    # Fetch comparison
    result = await _fetch_comparison_inline(from_query, to_query)

    if "error" in result:
        results = [
            InlineQueryResultArticle(
                id="error",
                title="Unable to fetch results",
                description=str(result["error"])[:50],
                input_message_content=InputTextMessageContent(
                    message_text=(
                        f"❌ *Error*\n\n"
                        f"Unable to fetch fare comparison: {result['error']}\n\n"
                        "Please try again later."
                    ),
                    parse_mode="Markdown",
                ),
            )
        ]
        await query.answer(results, cache_time=30)
        return

    options = result.get("options", [])

    if not options:
        results = [
            InlineQueryResultArticle(
                id="no_results",
                title="No routes found",
                description=f"No routes from {from_query} to {to_query}",
                input_message_content=InputTextMessageContent(
                    message_text=(
                        f"😕 *No Results*\n\n"
                        f"No routes found from *{from_query}* to *{to_query}*.\n\n"
                        "Try different locations or check spelling."
                    ),
                    parse_mode="Markdown",
                ),
            )
        ]
        await query.answer(results, cache_time=300)
        return

    # Build inline results (limit to top 3)
    inline_results = []
    top_options = options[:3]

    for i, option in enumerate(top_options):
        inline_results.append(_build_inline_result(option, i, from_query, to_query))

    # Answer with results
    await query.answer(
        inline_results,
        cache_time=60,  # Cache for 60 seconds
        is_personal=False,  # Allow caching across users
    )


