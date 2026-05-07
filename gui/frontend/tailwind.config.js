/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{svelte,js}'],
  darkMode: 'class',
  theme: {
    extend: {
      fontFamily: {
        display: ['"Bricolage Grotesque"', 'sans-serif'],
        body: ['"Plus Jakarta Sans"', 'sans-serif'],
        mono: ['"DM Mono"', 'monospace']
      },
      colors: {
        surface: {
          0: 'rgb(var(--surface-0) / <alpha-value>)',
          1: 'rgb(var(--surface-1) / <alpha-value>)',
          2: 'rgb(var(--surface-2) / <alpha-value>)',
          3: 'rgb(var(--surface-3) / <alpha-value>)',
        },
        accent: {
          DEFAULT: 'rgb(var(--accent) / <alpha-value>)',
          dim: 'rgb(var(--accent-dim) / <alpha-value>)',
          bright: 'rgb(var(--accent-bright) / <alpha-value>)',
        },
        txt: {
          primary: 'rgb(var(--text-primary) / <alpha-value>)',
          secondary: 'rgb(var(--text-secondary) / <alpha-value>)',
          muted: 'rgb(var(--text-muted) / <alpha-value>)',
        },
        state: {
          ok: 'rgb(var(--state-ok) / <alpha-value>)',
          warn: 'rgb(var(--state-warn) / <alpha-value>)',
          err: 'rgb(var(--state-err) / <alpha-value>)',
          sync: 'rgb(var(--state-sync) / <alpha-value>)',
        },
        bdr: 'rgb(var(--border) / <alpha-value>)',
      },
      borderRadius: {
        'xl2': '12px',
      }
    }
  },
  plugins: []
}
