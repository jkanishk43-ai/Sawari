'use client';

import { useSearchParams } from 'next/navigation';
import { useState, useEffect, Suspense } from 'react';
import Link from 'next/link';
import {
  ArrowLeft,
  Clock,
  Navigation,
  Star,
  MapPin,
  Zap,
  TrendingDown,
  Sparkles,
  ExternalLink,
  WifiOff,
  RefreshCw,
} from 'lucide-react';

// Mock comparison data
const MOCK_RESULTS = [
  {
    id: 'dtc-bus-104',
    provider: 'DTC Bus',
    mode: 'Bus',
    icon: '🚌',
    fare: 15,
    currency: '₹',
    duration: 45,
    eta: 10,
    etaUnit: 'min',
    rating: 3.8,
    badge: 'cheapest',
    badgeColor: 'accent-green',
    badgeIcon: TrendingDown,
    route: 'Route 104 via CP',
    seats: 12,
    stops: 14,
    features: ['AC Available', 'No Saheli'],
    providerLogo: null,
    deeplink: 'https://www.dtcbus.co.in',
    busPosition: { lat: 28.6328, lng: 77.2197 },
  },
  {
    id: 'cluster-bus-534',
    provider: 'Cluster Bus',
    mode: 'Bus',
    icon: '🚌',
    fare: 20,
    currency: '₹',
    duration: 40,
    eta: 8,
    etaUnit: 'min',
    rating: 4.0,
    badge: null,
    badgeColor: null,
    route: 'Route 534 via ITO',
    seats: 5,
    stops: 11,
    features: ['AC', 'Low Floor'],
    providerLogo: null,
    deeplink: 'https://www.dtcbus.co.in',
    busPosition: { lat: 28.6253, lng: 77.2169 },
  },
  {
    id: 'metro-blue',
    provider: 'DMRC Metro',
    mode: 'Metro',
    icon: '🚇',
    fare: 30,
    currency: '₹',
    duration: 35,
    eta: 5,
    etaUnit: 'min',
    rating: 4.5,
    badge: 'fastest',
    badgeColor: 'accent-purple',
    badgeIcon: Zap,
    route: 'Blue Line → Yellow Line',
    seats: null,
    stops: 8,
    features: ['AC', 'WiFi', 'Saheli Coach'],
    providerLogo: null,
    deeplink: 'https://www.delhimetrorail.com',
    trainPosition: { lat: 28.6292, lng: 77.2075 },
  },
  {
    id: 'auto-rickshaw',
    provider: 'Auto Rickshaw',
    mode: 'Auto',
    icon: '🛺',
    fare: 85,
    currency: '₹',
    duration: 30,
    eta: 3,
    etaUnit: 'min',
    rating: 3.5,
    badge: null,
    badgeColor: null,
    route: 'Direct via Mathura Road',
    seats: null,
    stops: 1,
    features: ['Direct', 'No AC'],
    providerLogo: null,
    deeplink: null,
  },
  {
    id: 'uber-go',
    provider: 'Uber Go',
    mode: 'Cab',
    icon: '🚕',
    fare: 150,
    currency: '₹',
    duration: 28,
    eta: 4,
    etaUnit: 'min',
    rating: 4.3,
    badge: 'smart',
    badgeColor: 'accent-blue',
    badgeIcon: Sparkles,
    route: 'Direct route',
    seats: null,
    stops: 1,
    features: ['AC', '4 Seats', 'Contactless'],
    providerLogo: 'Uber',
    deeplink: 'uber://?action=setPickup&pickup=my_location&dropoff=Saket',
  },
  {
    id: 'ola-mini',
    provider: 'Ola Mini',
    mode: 'Cab',
    icon: '🚕',
    fare: 145,
    currency: '₹',
    duration: 29,
    eta: 5,
    etaUnit: 'min',
    rating: 4.1,
    badge: null,
    badgeColor: null,
    route: 'Direct route',
    seats: null,
    stops: 1,
    features: ['AC', '4 Seats'],
    providerLogo: 'Ola',
    deeplink: 'ola://booking/auto/pickup=my_location&drop=Saket',
  },
];

function CompareResults() {
  const searchParams = useSearchParams();
  const from = searchParams.get('from') || '';
  const to = searchParams.get('to') || '';
  const [loading, setLoading] = useState(true);
  const [results, setResults] = useState<typeof MOCK_RESULTS>([]);
  const [offline, setOffline] = useState(false);

  useEffect(() => {
    // Simulate API fetch
    const timer = setTimeout(() => {
      setResults(MOCK_RESULTS);
      setLoading(false);
    }, 800);
    return () => clearTimeout(timer);
  }, [from, to]);

  const handleBook = (deeplink: string | null, id: string) => {
    if (!deeplink) {
      // Queue booking for later sync
      console.log('Queuing booking:', id);
      return;
    }
    if (deeplink.startsWith('uber://') || deeplink.startsWith('ola://')) {
      window.location.href = deeplink;
    } else {
      window.open(deeplink, '_blank', 'noopener,noreferrer');
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <RefreshCw className="w-8 h-8 text-accent-green animate-spin mx-auto mb-4" />
          <p className="text-ink-dim font-medium">Comparing fares across all providers...</p>
        </div>
      </div>
    );
  }

  return (
    <main className="min-h-screen pb-24">
      {/* Header */}
      <header className="sticky top-0 z-40 glass border-b border-pine-dark/5">
        <div className="max-w-2xl mx-auto px-4 sm:px-6">
          <div className="flex items-center gap-4 py-4">
            <Link
              href="/"
              className="w-10 h-10 rounded-xl bg-pine-light flex items-center justify-center hover:bg-pine-dark/10 transition-colors"
            >
              <ArrowLeft className="w-5 h-5 text-pine-dark" />
            </Link>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 text-sm">
                <span className="text-charcoal font-medium truncate">{from || 'Pickup'}</span>
                <Navigation className="w-3 h-3 text-ink-dim flex-shrink-0" />
                <span className="text-charcoal font-medium truncate">{to || 'Destination'}</span>
              </div>
              <p className="text-xs text-ink-dim mt-0.5">
                {results.length} options found
              </p>
            </div>
            <button
              onClick={() => setOffline(!offline)}
              className="text-xs text-ink-dim flex items-center gap-1"
            >
              {offline ? <WifiOff className="w-4 h-4" /> : null}
            </button>
          </div>
        </div>
      </header>

      {/* Route Map (Mini) */}
      <section className="max-w-2xl mx-auto px-4 sm:px-6 mt-4">
        <div className="card p-0 overflow-hidden">
          <div className="relative h-40 bg-pine-light flex items-center justify-center">
            {/* Simplified route visualization */}
            <div className="relative w-full h-full">
              <svg className="w-full h-full" viewBox="0 0 400 160">
                {/* Route line */}
                <path
                  d="M 60 80 Q 200 20 340 80"
                  fill="none"
                  stroke="#073020"
                  strokeWidth="2"
                  strokeDasharray="6,4"
                  opacity="0.3"
                />

                {/* From dot */}
                <circle cx="60" cy="80" r="8" fill="#073020" />
                <circle cx="60" cy="80" r="4" fill="#10B981" />

                {/* To dot */}
                <circle cx="340" cy="80" r="8" fill="#073020" />
                <circle cx="340" cy="80" r="4" fill="#EF4444" />

                {/* Live bus position */}
                {results[0]?.busPosition && (
                  <circle
                    cx={((results[0].busPosition.lng - 77.2) / 0.03) * 340}
                    cy={160 - ((results[0].busPosition.lat - 28.62) / 0.02) * 140}
                    r="6"
                    fill="#10B981"
                    className="animate-pulse-green"
                  />
                )}
              </svg>
              <div className="absolute top-3 left-3 bg-white/80 backdrop-blur rounded-lg px-3 py-1.5">
                <p className="text-xs text-ink-dim font-mono uppercase tracking-wider">
                  Live Route
                </p>
              </div>
              <div className="absolute bottom-3 right-3 bg-pine-dark text-paper-white rounded-lg px-3 py-1.5 flex items-center gap-1.5">
                <div className="w-2 h-2 rounded-full bg-accent-green animate-pulse" />
                <span className="text-xs font-medium">1 bus tracking</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Results List */}
      <section className="max-w-2xl mx-auto px-4 sm:px-6 mt-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="font-display font-semibold text-lg text-charcoal">
            {results.length} Options
          </h2>
          <div className="flex gap-2">
            <SortButton label="Recommended" active />
            <SortButton label="Cheapest" />
            <SortButton label="Fastest" />
          </div>
        </div>

        <div className="space-y-3">
          {results.map((result, index) => (
            <FareCard key={result.id} result={result} index={index} onBook={handleBook} />
          ))}
        </div>
      </section>

      {/* Saved for Later CTA */}
      <section className="max-w-2xl mx-auto px-4 sm:px-6 mt-8">
        <div className="card p-6 bg-gradient-to-br from-pine-light to-white">
          <h3 className="font-display font-semibold text-charcoal mb-2">
            Save this trip for later?
          </h3>
          <p className="text-sm text-ink-dim mb-4">
            Get price alerts and track live positions.
          </p>
          <button className="btn-primary bg-pine-dark text-paper-white">
            Save Trip
          </button>
        </div>
      </section>
    </main>
  );
}

function SortButton({ label, active }: { label: string; active?: boolean }) {
  return (
    <button
      className={`text-xs font-medium px-3 py-1.5 rounded-lg transition-colors ${
        active
          ? 'bg-pine-dark text-paper-white'
          : 'bg-pine-light text-pine-dark hover:bg-pine-dark/10'
      }`}
    >
      {label}
    </button>
  );
}

interface FareCardProps {
  result: typeof MOCK_RESULTS[0];
  index: number;
  onBook: (deeplink: string | null, id: string) => void;
}

function FareCard({ result, index }: FareCardProps) {
  const BadgeIcon = result.badgeIcon;
  const isBus = result.mode === 'Bus';
  const isMetro = result.mode === 'Metro';

  return (
    <div
      className="card card-hover p-4 animate-fade-in"
      style={{ animationDelay: `${index * 60}ms` }}
    >
      <div className="flex gap-4">
        {/* Provider Icon */}
        <div className="flex flex-col items-center">
          <div className="provider-icon w-12 h-12 bg-pine-light">
            <span className="text-xl">{result.icon}</span>
          </div>
        </div>

        {/* Main Content */}
        <div className="flex-1 min-w-0">
          <div className="flex items-start justify-between gap-2">
            <div>
              <div className="flex items-center gap-2 flex-wrap">
                <span className="font-semibold text-charcoal">{result.provider}</span>
                {result.badge && (
                  <span
                    className={`badge badge-${result.badgeColor} flex items-center gap-1`}
                  >
                    {BadgeIcon && <BadgeIcon className="w-3 h-3" />}
                    {result.badge}
                  </span>
                )}
              </div>
              <p className="text-xs text-ink-dim mt-1 font-mono">{result.route}</p>
            </div>
            <div className="text-right flex-shrink-0">
              <p className="font-display font-bold text-xl text-charcoal">
                {result.currency}{result.fare}
              </p>
              <div className="flex items-center justify-end gap-1 text-xs text-ink-dim">
                <Clock className="w-3 h-3" />
                <span>{result.duration} min</span>
              </div>
            </div>
          </div>

          {/* ETA & Rating Row */}
          <div className="flex items-center gap-4 mt-3 flex-wrap">
            <span className="text-xs text-accent-green font-medium flex items-center gap-1">
              <div className="w-1.5 h-1.5 rounded-full bg-accent-green" />
              {result.eta}{result.etaUnit} away
            </span>
            <span className="text-xs text-ink-dim flex items-center gap-1">
              <Star className="w-3 h-3 text-accent-yellow fill-accent-yellow" />
              {result.rating}
            </span>
            <span className="text-xs text-ink-dim">
              {result.seats !== null && `${result.seats} seats left`}
              {result.stops} stops
            </span>
          </div>

          {/* Features */}
          <div className="flex gap-2 mt-3 flex-wrap">
            {result.features.map((feature) => (
              <span
                key={feature}
                className="text-xs px-2 py-1 bg-pine-light text-pine-dark rounded-md font-medium"
              >
                {feature}
              </span>
            ))}
          </div>

          {/* Action Buttons */}
          <div className="flex gap-2 mt-4">
            {result.deeplink ? (
              <button
                onClick={() => onBook(result.deeplink, result.id)}
                className="btn-primary bg-pine-dark text-paper-white text-sm py-2.5 flex-1"
              >
                Book with {result.provider.split(' ')[0]}
              </button>
            ) : (
              <button className="btn-outline border-pine-dark/20 text-pine-dark text-sm py-2.5 flex-1">
                Call to Book
              </button>
            )}
            {isBus || isMetro ? (
              <button className="btn-outline border-pine-dark/20 text-pine-dark text-sm py-2.5 px-4">
                Track
              </button>
            ) : null}
            {result.deeplink && (
              <button
                onClick={() => onBook(result.deeplink, result.id)}
                className="btn-outline border-pine-dark/20 text-pine-dark px-3"
              >
                <ExternalLink className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

export default function ComparePage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen flex items-center justify-center">
          <RefreshCw className="w-8 h-8 text-accent-green animate-spin" />
        </div>
      }
    >
      <CompareResults />
    </Suspense>
  );
}
