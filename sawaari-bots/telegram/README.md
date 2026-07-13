# Sawaari Telegram Bot

A Telegram bot for the Sawaari fare comparison and booking system for Delhi's multimodal transport.

## Features

- **Compare Fares**: Compare prices across bus, metro, auto, and cabs
- **Multi-step Conversations**: Intuitive conversation flow for entering pickup/dropoff locations
- **Location Support**: Share your location directly or type addresses
- **Inline Queries**: Quick fare comparisons from any chat with `@SawaariBot`
- **Booking Integration**: Book rides via deeplinks to partner apps
- **Nearby Stops**: Find nearby transit stops

## Installation

1. **Create a virtual environment**:
   ```bash
   python -m venv venv
   source venv/bin/activate  # Linux/Mac
   venv\Scripts\activate     # Windows
   ```

2. **Install dependencies**:
   ```bash
   pip install -r requirements.txt
   ```

3. **Configure environment**:
   ```bash
   cp .env.example .env
   # Edit .env and add your TELEGRAM_BOT_TOKEN
   ```

4. **Get a Telegram Bot Token**:
   - Message [@BotFather](https://t.me/BotFather) on Telegram
   - Send `/newbot` and follow the instructions
   - Copy the token into your `.env` file

5. **Configure Backend URL** (optional):
   - Default: `http://localhost:8080`
   - Set `SAWAARI_BACKEND_URL` in `.env` for production

## Running the Bot

```bash
python bot.py
```

## Commands

| Command | Description |
|---------|-------------|
| `/start` | Get started with Sawaari |
| `/compare` | Compare fares between two locations |
| `/status` | Check booking status |
| `/nearby` | Find nearby transit stops |
| `/help` | Show help and commands |

## Inline Queries

Type `@SawaariBot` in any chat followed by your route:

```
@SawaariBot Connaught Place to Rajiv Chowk
@SawaariBot AIIMS Metro to Nehru Place
@SawaariBot Dwarka to Gurgaon
```

## Supported Transport Modes

- 🚌 Bus (DTC/DIMTS)
- 🚇 Metro (DMRC)
- 🛺 Auto / E-rickshaw
- 🚗 Cab / Taxi
- 🏍️ Bike Taxi

## Architecture

```
bot.py           # Main entry point, Application setup
handlers.py      # Command and message handlers
inline_queries.py # Inline query support
```

### Conversation Flow

```
User sends /compare
       ↓
Bot asks for FROM location
       ↓
User provides location (text or GPS)
       ↓
Bot asks for TO location
       ↓
User provides destination
       ↓
Bot fetches comparison from backend
       ↓
Bot displays ranked results with booking buttons
```

## API Integration

The bot communicates with the Sawaari Go backend:

- `GET /api/compare?from=<loc>&to=<loc>` - Fetch fare comparison
- `GET /api/stops/nearby?lat=<lat>&lng=<lng>&r=<radius>` - Find nearby stops

## Development

### Project Structure

```
sawaari-bots/
├── telegram/
│   ├── bot.py           # Main bot file
│   ├── handlers.py      # Command handlers
│   ├── inline_queries.py # Inline query handler
│   ├── requirements.txt  # Python dependencies
│   ├── .env.example     # Environment template
│   └── README.md        # This file
└── whatsapp/
    └── ...
```

### Adding New Commands

1. Add the command handler in `bot.py`
2. Create the handler function in `handlers.py`
3. Follow the existing patterns for consistency

## License

Part of the Sawaari project.
