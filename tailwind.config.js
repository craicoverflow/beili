/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'media',
  content: [
    './internal/templates/**/*.templ',
    './internal/templates/**/*_templ.go',
  ],
  // Class names built dynamically by inline <script> (chip builder, toasts, etc.)
  // never appear as literal strings in .templ source, so they must be safelisted.
  safelist: [
    'bg-orange-100', 'dark:bg-orange-950/30', 'border-orange-300/50',
    'bg-red-900', 'border-red-700', 'text-red-100',
    'bg-red-950/40', 'border-red-800', 'text-red-400', 'text-red-300', 'border-red-900/50',
    'text-green-500',
  ],
  theme: {
    extend: {
      colors: {
        surface: 'var(--color-surface)',
        'surface-2': 'var(--color-surface-2)',
        'surface-3': 'var(--color-surface-3)',
        'ui-text': 'var(--ui-text)',
        'ui-subtext': 'var(--ui-subtext)',
        'ui-muted': 'var(--ui-muted)',
        'ui-dim': 'var(--ui-dim)',
        'ui-border': 'var(--ui-border)',
        'ui-border-medium': 'var(--ui-border-medium)',
        'ui-border-strong': 'var(--ui-border-strong)',
        accent: '#f97316',
        'accent-hover': '#ea6c0a',
      },
    },
  },
  plugins: [],
}
