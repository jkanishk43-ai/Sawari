'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Search, MapPin, Clock, Navigation, Star } from 'lucide-react';

// Mock data for demo
const RECENT_TRIPS = [
  { id: 1, from: 'Rajiv Chowk Metro', to: 'Saket Mall', mode: 'Metro', fare: 30, date: 'Today, 10:30 AM', color: 'accent-purple' },
  { id: 2, from: 'Home', to: 'Cyber Hub, Gurgaon', mode: 'Auto', fare: 85, date: 'Yesterday, 9:15 AM', color: 'accent-yellow' },
  { id: 3, from: 'IGI Airport T3', to: 'Connaught Place', mode: 'Uber Go', fare: 320, date: 'Jul 10, 6:45 PM', color: 'accent-green' },
];

const NEARBY_STOPS = [
  { id: 1, name: 'Rajiv Chowk Metro', distance: 0.3, lines: ['Blue', 'Yellow'], type: 'metro' },
  { id: 2, name: 'Barakhamba Bus Stop', distance: 0.5, lines: ['DTC', 'Cluster'], type: 'bus' },
  { id: 3, name: 'Mandi House Metro', distance: 0.7, lines: ['Blue', 'Violet'], type: 'metro' },
  { id: 4, name: 'ITO Bus Stop', distance: 0.9, lines: ['DTC'], type: 'bus' },
];

const AUTOCOMPLETE_SUGGESTIONS = [
  'Rajiv Chowk Metro Station, Connaught Place',
  'Cyber Hub, DLF Phase 3, Gurgaon',
  'IGI Airport Terminal 3',
  'Saket Select City Walk Mall',
  'Nehru Place Metro Station',
  'Connaught Place, New Delhi',
  'New Delhi Railway Station',
  'India Gate',
  'AIIMS Metro Station',
  'Hauz Khas Village',
];

function SearchSuggestions({
  query,
  onSelect,
}: {
  query: string;
  onSelect: (value: string) => void;
}) {
  if (!query) return null;

  const filtered = AUTOCOMPLETE_SUGGESTIONS.filter((s) =>
    s.toLowerCase().includes(query.toLowerCase())
  );

  if (filtered.length === 0) return null;

  return (
    <div className="autocomplete-list">
      {filtered.map((suggestion) => (
        <div
          key={suggestion}
          className="autocomplete-item"
          onClick={() => onSelect(suggestion)}
          role="option"
        >
          <div className="flex items-center gap-3">
            <MapPin className="w-4 h-4 text-ink-dim flex-shrink-0" />
            <span className="text-sm text-charcoal">{suggestion}</span>
          </div>
        </div>
      ))}
    </div>
  );
}

export default function HomePage() {
  const router = useRouter();
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [fromSuggestions, setFromSuggestions] = useState(false);
  const [toSuggestions, setToSuggestions] = useState(false);
  const [preferences, setPreferences] = useState({
    ac: true,
    saheli: false,
    night: false,
  });
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const togglePreference = (key: keyof typeof preferences) => {
    setPreferences((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  const handleCompare = () => {
    if (!from || !to) return;
    const params = new URLSearchParams({
      from,
      to,
      ac: preferences.ac.toString(),
      saheli: preferences.saheli.toString(),
      night: preferences.night.toString(),
    });
    router.push(`/compare?${params.toString()}`);
  };

  return (
    <main className="min-h-screen">
      {/* Hero Section */}
      <section className="relative overflow-hidden bg-pine-dark text-paper-white">
        <div className="absolute inset-0 opacity-10">
          <div
            className="absolute inset-0"
            style={{
              backgroundImage:
                'linear-gradient(rgba(255,255,255,0.05) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,0.05) 1px, transparent 1px)',
              backgroundSize: '24px 24px',
            }}
          />
        </div>
        <div className="relative max-w-2xl mx-auto px-4 sm:px-6 pt-12 pb-16">
          <div className="flex items-center gap-2 mb-6">
            <div className="w-8 h-8 bg-accent-green rounded-lg flex items-center justify-center">
              <Navigation className="w-5 h-5 text-pine-dark" />
            </div>
            <span className="font-mono text-xs text-paper-white/60 uppercase tracking-wider">
              Sawaari
            </span>
          </div>

          <h1 className="font-display text-4xl sm:text-5xl font-bold leading-tight mb-4">
            Where to?
          </h1>
          <p className="text-paper-white/70 text-lg mb-8">
            Compare fares across bus, metro, auto &amp; cab. Find the best way to go.
          </p>

          {/* Search Form */}
          <div className="space-y-3">
            {/* From Input */}
            <div className="relative">
              <div className="flex items-center gap-3 bg-white/10 backdrop-blur rounded-2xl px-4 py-1">
                <div className="flex flex-col items-center gap-1 pt-2">
                  <div className="w-2.5 h-2.5 rounded-full bg-accent-green border-2 border-white" />
                  <div className="w-0.5 h-8 bg-white/30" />
                  <div className="w-2.5 h-2.5 rounded-full bg-accent-red border-2 border-white" />
                </div>
                <div className="flex-1 space-y-2 py-2">
                  <div className="relative">
                    <MapPin className="absolute left-0 top-1/2 -translate-y-1/2 w-4 h-4 text-paper-white/50" />
                    <input
                      type="text"
                      placeholder="Pickup location"
                      value={from}
                      onChange={(e) => {
                        setFrom(e.target.value);
                        setFromSuggestions(true);
                      }}
                      onFocus={() => setFromSuggestions(true)}
                      onBlur={() => setTimeout(() => setFromSuggestions(false), 200)}
                      className="w-full bg-transparent text-paper-white placeholder:text-paper-white/40 pl-6 pr-4 py-2 text-base focus:outline-none"
                    />
                  </div>
                  <div className="relative">
                    <MapPin className="absolute left-0 top-1/2 -translate-y-1/2 w-4 h-4 text-accent-red" />
                    <input
                      type="text"
                      placeholder="Where to?"
                      value={to}
                      onChange={(e) => {
                        setTo(e.target.value);
                        setToSuggestions(true);
                      }}
                      onFocus={() => setToSuggestions(true)}
                      onBlur={() => setTimeout(() => setToSuggestions(false), 200)}
                      className="w-full bg-transparent text-paper-white placeholder:text-paper-white/40 pl-6 pr-4 py-2 text-base focus:outline-none"
                    />
                  </div>
                </div>
              </div>

              {/* Autocomplete overlays */}
              {fromSuggestions && (
                <SearchSuggestions
                  query={from}
                  onSelect={(val) => {
                    setFrom(val);
                    setFromSuggestions(false);
                  }}
                />
              )}
              {toSuggestions && (
                <SearchSuggestions
                  query={to}
                  onSelect={(val) => {
                    setTo(val);
                    setToSuggestions(false);
                  }}
                />
              )}
            </div>

            {/* Date & Time */}
            <div className="flex gap-3">
              <div className="flex-1 bg-white/10 backdrop-blur rounded-xl px-4 py-3 flex items-center gap-3">
                <Clock className="w-5 h-5 text-paper-white/60" />
                <div>
                  <p className="text-xs text-paper-white/50 font-mono uppercase tracking-wider">
                    Departure
                  </p>
                  <input
                    type="datetime-local"
                    className="bg-transparent text-paper-white text-sm focus:outline-none w-full"
                  />
                </div>
              </div>
            </div>

            {/* Preference Toggles */}
            <div className="flex flex-wrap gap-2">
              <button
                onClick={() => togglePreference('ac')}
                className={`flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-medium transition-all ${
                  preferences.ac
                    ? 'bg-accent-green/20 text-accent-green border border-accent-green/30'
                    : 'bg-white/5 text-paper-white/50 border border-white/10'
                }`}
              >
                <span>AC</span>
              </button>
              <button
                onClick={() => togglePreference('saheli')}
                className={`flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-medium transition-all ${
                  preferences.saheli
                    ? 'bg-pink-500/20 text-pink-300 border border-pink-500/30'
                    : 'bg-white/5 text-paper-white/50 border border-white/10'
                }`}
              >
                <span>Saheli</span>
              </button>
              <button
                onClick={() => togglePreference('night')}
                className={`flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-medium transition-all ${
                  preferences.night
                    ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/30'
                    : 'bg-white/5 text-paper-white/50 border border-white/10'
                }`}
              >
                <span>Night</span>
              </button>
            </div>

            {/* Compare Button */}
            <button
              onClick={handleCompare}
              disabled={!from || !to || !mounted}
              className="btn-primary bg-accent-green text-pine-dark font-bold text-lg py-4 rounded-2xl disabled:opacity-40 disabled:cursor-not-allowed"
            >
              Compare Fares
            </button>
          </div>
        </div>
      </section>

      {/* Quick Actions */}
      <section className="max-w-2xl mx-auto px-4 sm:px-6 -mt-6 relative z-10">
        <div className="bg-white rounded-2xl shadow-card p-4">
          <div className="grid grid-cols-4 gap-3">
            {[
              { label: 'Bus', icon: '🚌', color: 'accent-blue', bg: 'bg-accent-blue/10' },
              { label: 'Metro', icon: '🚇', color: 'accent-purple', bg: 'bg-accent-purple/10' },
              { label: 'Auto', icon: '🛺', color: 'accent-yellow', bg: 'bg-accent-yellow/10' },
              { label: 'Cab', icon: '🚕', color: 'accent-green', bg: 'bg-accent-green/10' },
            ].map((action) => (
              <button
                key={action.label}
                className={`flex flex-col items-center gap-2 py-4 rounded-xl ${action.bg} transition-transform hover:scale-105`}
              >
                <span className="text-2xl">{action.icon}</span>
                <span className={`text-sm font-semibold text-${action.color}`}>
                  {action.label}
                </span>
              </button>
            ))}
          </div>
        </div>
      </section>

      {/* Nearby Stops */}
      <section className="max-w-2xl mx-auto px-4 sm:px-6 mt-8">
        <div className="section-header">
          <h2 className="section-title">Nearby Stops</h2>
          <Link href="/stops" className="text-sm text-accent-green font-medium hover:underline">
            See all
          </Link>
        </div>
        <div className="flex gap-3 overflow-x-auto scrollbar-hide -mx-4 px-4 sm:mx-0 sm:px-0">
          {NEARBY_STOPS.map((stop) => (
            <div
              key={stop.id}
              className="flex-shrink-0 w-44 bg-white rounded-2xl p-3 shadow-card card-hover cursor-pointer"
            >
              <div className="flex items-center gap-2 mb-2">
                <div
                  className={`w-8 h-8 rounded-lg flex items-center justify-center ${
                    stop.type === 'metro' ? 'bg-accent-purple/10' : 'bg-accent-blue/10'
                  }`}
                >
                  <span className="text-sm">
                    {stop.type === 'metro' ? '🚇' : '🚌'}
                  </span>
                </div>
                <span className="text-xs text-ink-dim font-mono">{stop.distance} km</span>
              </div>
              <p className="text-sm font-semibold text-charcoal truncate">{stop.name}</p>
              <div className="flex gap-1 mt-2 flex-wrap">
                {stop.lines.map((line) => (
                  <span
                    key={line}
                    className="text-xs px-2 py-0.5 bg-pine-light text-pine-dark rounded-md font-medium"
                  >
                    {line}
                  </span>
                ))}
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* Recent Trips */}
      <section className="max-w-2xl mx-auto px-4 sm:px-6 mt-8 mb-20">
        <div className="section-header">
          <h2 className="section-title">Recent Trips</h2>
        </div>
        <div className="space-y-3">
          {RECENT_TRIPS.map((trip) => (
            <Link
              key={trip.id}
              href={`/compare?from=${encodeURIComponent(trip.from)}&to=${encodeURIComponent(trip.to)}`}
              className="card card-hover p-4 flex items-center gap-4 block no-underline"
            >
              <div
                className={`w-12 h-12 rounded-xl flex items-center justify-center bg-${trip.color}/10 flex-shrink-0`}
              >
                <Navigation className="w-5 h-5 text-${trip.color}" />
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between">
                  <span className={`text-sm font-semibold text-${trip.color}`}>
                    {trip.mode}
                  </span>
                  <span className="text-lg font-bold font-display text-charcoal">
                    ₹{trip.fare}
                  </span>
                </div>
                <p className="text-sm text-charcoal truncate mt-0.5">
                  {trip.from} → {trip.to}
                </p>
                <p className="text-xs text-ink-dim mt-0.5">{trip.date}</p>
              </div>
              <Star className="w-4 h-4 text-ink-dim/40 flex-shrink-0" />
            </Link>
          ))}
        </div>
      </section>
    </main>
  );
}
