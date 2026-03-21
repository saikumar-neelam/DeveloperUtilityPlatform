import type { Config } from 'tailwindcss';

const config: Config = {
  content: [
    './app/**/*.{ts,tsx}',
    './components/**/*.{ts,tsx}',
    './lib/**/*.{ts,tsx}',
  ],
  theme: {
    extend: {
      fontFamily: {
        mono: ['ui-monospace', 'JetBrains Mono', 'Fira Code', 'monospace'],
      },
      colors: {
        zinc: {
          925: '#111113',
        },
      },
    },
  },
  plugins: [],
};

export default config;
