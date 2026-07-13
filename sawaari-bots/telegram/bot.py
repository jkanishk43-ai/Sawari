"""
Sawaari Telegram Bot - Main Entry Point

A fare comparison and booking bot for Delhi's multimodal transport system.
"""

import logging
import os
import asyncio
from typing import Any

from dotenv import load_dotenv
from telegram import Update, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import (
    Application,
    CommandHandler,
    MessageHandler,
    CallbackQueryHandler,
    InlineQueryHandler,
    ConversationHandler,
    filters,
    ContextTypes,
)

from handlers import (
    start_command,
    help_command,
    compare_command,
    compare_from,
    compare_to,
    location_handler,
    status_command,
    nearby_command,
    book_callback,
    cancel_handler,
    error_handler,
)
from inline_queries import inline_query_handler

# Load environment variables
load_dotenv()

# Configure logging
logging.basicConfig(
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    level=logging.INFO,
)
logger = logging.getLogger(__name__)


async def post_init(application: Application) -> None:
    """Initialize application after startup."""
    bot = application.bot
    bot_info = await bot.get_me()
    logger.info(f"Bot {bot_info.name} (@{bot_info.username}) started successfully")

    # Set bot commands for menu
    await application.bot.set_my_commands([
        ("start", "Get started with Sawaari"),
        ("compare", "Compare fares between two locations"),
        ("status", "Check your booking status"),
        ("nearby", "Find nearby stops"),
        ("help", "Get help and see all commands"),
    ])


def main() -> None:
    """Start the bot."""
    # Get bot token from environment
    bot_token = os.getenv("TELEGRAM_BOT_TOKEN")
    if not bot_token:
        logger.error("TELEGRAM_BOT_TOKEN not set in environment")
        logger.error("Please set TELEGRAM_BOT_TOKEN in your .env file")
        return

    # Backend API URL (default to localhost for development)
    backend_url = os.getenv("SAWAARI_BACKEND_URL", "http://localhost:8080")
    logger.info(f"Backend URL: {backend_url}")

    # Build application
    application = (
        Application.builder()
        .token(bot_token)
        .post_init(post_init)
        .build()
    )

    # Conversation states for multi-step flows
    CONVERSATION_STATES = {
        "compare": {
            "from_location": 1,
            "to_location": 2,
            "confirm": 3,
        }
    }

    # ===================
    # Command Handlers
    # ===================

    # Start command - entry point
    application.add_handler(CommandHandler("start", start_command))

    # Help command
    application.add_handler(CommandHandler("help", help_command))

    # Compare command - starts conversation flow
    compare_conv_handler = ConversationHandler(
        entry_points=[CommandHandler("compare", compare_command)],
        states={
            1: [  # Waiting for FROM location
                MessageHandler(
                    filters.LOCATION,
                    lambda u, c: compare_from(u, c, is_from=True)
                ),
                MessageHandler(
                    filters.TEXT & ~filters.COMMAND,
                    compare_from
                ),
            ],
            2: [  # Waiting for TO location
                MessageHandler(
                    filters.LOCATION,
                    lambda u, c: compare_to(u, c, is_from=False)
                ),
                MessageHandler(
                    filters.TEXT & ~filters.COMMAND,
                    compare_to
                ),
            ],
        },
        fallbacks=[
            CommandHandler("cancel", cancel_handler),
            CommandHandler("start", start_command),
        ],
        per_user=True,
        per_chat=False,
    )
    application.add_handler(compare_conv_handler)

    # Status command
    application.add_handler(CommandHandler("status", status_command))

    # Nearby command
    application.add_handler(CommandHandler("nearby", nearby_command))

    # ===================
    # Callback Query Handlers (inline button clicks)
    # ===================
    application.add_handler(CallbackQueryHandler(book_callback, pattern="^book_"))

    # ===================
    # Inline Query Handler (for @SawaariBot query text)
    # ===================
    application.add_handler(InlineQueryHandler(inline_query_handler))

    # ===================
    # Error Handler
    # ===================
    application.add_error_handler(error_handler)

    # ===================
    # Run the bot
    # ===================
    logger.info("Starting Sawaari Telegram Bot...")
    application.run_polling(
        allowed_updates=Update.ALL_TYPES,
        drop_pending_updates=True,
    )


if __name__ == "__main__":
    main()
