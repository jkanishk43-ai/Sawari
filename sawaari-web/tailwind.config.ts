import type { Config } from 'tailwindcss';

const config: Config = {
  content: [
    './pages/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
    './app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        // Sawaari Pine Green Palette
        pine: {
          dark: '#073020',
          mid: '#0B3D29',
          light: '#EFF6F3',
          accent: '#124C38',
        },
        // Surface colors
        paper: {
          white: '#FBFAF7',
          grid: '#E8E5DB',
        },
        // Text colors
        charcoal: '#1A1A1A',
        'ink-dim': '#555E5A',
        // Accent colors
        'accent-green': '#10B981',
        'accent-yellow': '#F59E0B',
        'accent-red': '#EF4444',
        'accent-blue': '#3B82F6',
        'accent-purple': '#8B5CF6',
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        display: ['Space Grotesk', 'Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'Courier New', 'monospace'],
      },
      borderRadius: {
        'xl': '12px',
        '2xl': '16px',
        '3xl': '24px',
      },
      boxShadow: {
        'card': '0 4px 20px rgba(7, 48, 32, 0.08)',
        'card-hover': '0 8px 30px rgba(7, 48, 32, 0.12)',
      },
    },
  },
  plugins: [],
};

export default config;
