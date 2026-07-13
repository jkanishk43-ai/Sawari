import type { Metadata } from 'next';
import { Inter, Space_Grotesk, JetBrains_Mono } from 'next/font/google';
import { PWAProvider } from '@/components/pwa-provider';
import './globals.css';

const inter = Inter({
  subsets: ['latin'],
  variable: '--font-sans',
  display: 'swap',
});

const spaceGrotesk = Space_Grotesk({
  subsets: ['latin'],
  variable: '--font-display',
  display: 'swap',
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ['latin'],
  variable: '--font-mono',
  display: 'swap',
});

export const metadata: Metadata = {
  title: {
    default: 'Sawaari - Compare Fares Across All Transport',
    template: '%s | Sawaari',
  },
  description: 'Compare fares across bus, metro, auto-rickshaw, and cab services. Find the cheapest, fastest, and smartest way to get anywhere.',
  keywords: ['transport', 'fare comparison', 'bus', 'metro', 'auto', 'cab', 'travel', 'delhi', 'sawaari'],
  authors: [{ name: 'Sawaari' }],
  creator: 'Sawaari',
  openGraph: {
    type: 'website',
    locale: 'en_IN',
    url: 'https://sawaari.app',
    siteName: 'Sawaari',
    title: 'Sawaari - Compare Fares Across All Transport',
    description: 'Find the best way to travel. Compare fares in real-time across all transport modes.',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Sawaari - Compare Fares Across All Transport',
    description: 'Find the best way to travel. Compare fares in real-time across all transport modes.',
  },
  manifest: '/manifest.json',
  themeColor: '#073020',
  appleWebApp: {
    capable: true,
    statusBarStyle: 'default',
    title: 'Sawaari',
  },
  formatDetection: {
    telephone: false,
  },
  viewport: {
    width: 'device-width',
    initialScale: 1,
    maximumScale: 5,
    userScalable: true,
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className={`${inter.variable} ${spaceGrotesk.variable} ${jetbrainsMono.variable}`}>
      <head>
        <link rel="icon" href="/favicon.ico" sizes="any" />
        <link rel="apple-touch-icon" href="/icons/icon-192x192.png" />
        <meta name="theme-color" content="#073020" />
        <meta name="apple-mobile-web-app-capable" content="yes" />
        <meta name="apple-mobile-web-app-status-bar-style" content="default" />
        <meta name="apple-mobile-web-app-title" content="Sawaari" />
      </head>
      <body className="min-h-screen bg-paper-white font-sans antialiased">
        <PWAProvider>
          {children}
        </PWAProvider>
      </body>
    </html>
  );
}
