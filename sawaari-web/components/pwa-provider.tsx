'use client';

import { useState, useEffect } from 'react';

export function PWAProvider({ children }: { children: React.ReactNode }) {
  const [deferredPrompt, setDeferredPrompt] = useState<any>(null);
  const [showInstallBanner, setShowInstallBanner] = useState(false);
  const [isOffline, setIsOffline] = useState(false);

  useEffect(() => {
    // Handle PWA install prompt
    const handleBeforeInstall = (e: Event) => {
      e.preventDefault();
      setDeferredPrompt(e);
      setShowInstallBanner(true);
    };

    window.addEventListener('beforeinstallprompt', handleBeforeInstall);

    // Handle online/offline status
    const handleOnline = () => setIsOffline(false);
    const handleOffline = () => setIsOffline(true);

    window.addEventListener('online', handleOnline);
    window.addEventListener('offline', handleOffline);

    // Set initial state
    setIsOffline(!navigator.onLine);

    return () => {
      window.removeEventListener('beforeinstallprompt', handleBeforeInstall);
      window.removeEventListener('online', handleOnline);
      window.removeEventListener('offline', handleOffline);
    };
  }, []);

  const handleInstall = async () => {
    if (!deferredPrompt) return;

    deferredPrompt.prompt();
    const { outcome } = await deferredPrompt.userChoice;

    if (outcome === 'accepted') {
      setShowInstallBanner(false);
    }

    setDeferredPrompt(null);
  };

  const dismissBanner = () => {
    setShowInstallBanner(false);
  };

  return (
    <>
      {children}

      {/* Offline Banner */}
      {isOffline && (
        <div className="fixed top-0 left-0 right-0 z-50 bg-accent-yellow text-pine-dark px-4 py-2 flex items-center justify-center gap-2 text-sm font-medium">
          <span className="w-2 h-2 rounded-full bg-accent-yellow animate-pulse" />
          You&apos;re offline — cached results are still available
        </div>
      )}

      {/* PWA Install Banner */}
      {showInstallBanner && (
        <div className="pwa-banner">
          <div className="max-w-2xl mx-auto flex items-center justify-between gap-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 bg-pine-dark rounded-xl flex items-center justify-center text-lg">
                🚌
              </div>
              <div>
                <p className="text-sm font-semibold text-charcoal">
                  Install Sawaari
                </p>
                <p className="text-xs text-ink-dim">
                  Add to home screen for quick access
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={dismissBanner}
                className="text-xs text-ink-dim px-3 py-2"
              >
                Not now
              </button>
              <button
                onClick={handleInstall}
                className="text-sm font-semibold bg-pine-dark text-paper-white px-4 py-2 rounded-xl"
              >
                Install
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
