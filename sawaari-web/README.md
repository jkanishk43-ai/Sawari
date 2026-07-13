# Sawaari Web

A Next.js 14 PWA for Sawaari - Compare fares across all transport modes.

## Features

- PWA support with offline capability
- Real-time fare comparison across bus, metro, auto, and cab services
- Location-based nearby stops finder
- Dark pine green theme with paper white surfaces
- Responsive design for mobile-first experience

## Tech Stack

- Next.js 14 (App Router)
- React 18
- Tailwind CSS
- TypeScript
- PWA (Service Worker)

## Getting Started

1. Install dependencies:
   ```bash
   npm install
   ```

2. Run the development server:
   ```bash
   npm run dev
   ```

3. Open [http://localhost:3000](http://localhost:3000)

## Project Structure

```
sawaari-web/
├── app/
│   ├── api/
│   │   ├── compare/     # Fare comparison API proxy
│   │   ├── stops/       # Nearby stops API proxy
│   │   └── bookings/    # Bookings API proxy
│   ├── compare/          # Comparison results page
│   ├── layout.tsx       # Root layout with PWA setup
│   ├── page.tsx         # Home page
│   └── globals.css      # Tailwind + brand styles
├── components/
│   └── pwa-provider.tsx # PWA install prompt & offline detection
├── public/
│   ├── manifest.json    # PWA manifest
│   ├── sw.js           # Service worker
│   └── offline.html    # Offline fallback page
└── package.json
```

## Environment Variables

- `BACKEND_URL` - Go backend URL (default: http://localhost:8080)

## Design System

### Colors
- Pine Dark: `#073020`
- Pine Mid: `#0B3D29`
- Pine Light: `#EFF6F3`
- Paper White: `#FBFAF7`
- Accent Green: `#10B981`

### Typography
- Display: Space Grotesk
- Body: Inter
- Mono: JetBrains Mono

## License

MIT
