/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./index.html', './src/**/*.{svelte,ts,js}'],
  theme: {
    extend: {
      colors: {
        // Industrial night-shift palette: high contrast surfaces, precise cyan actions.
        background: '#030817',
        surface: '#060d1b',
        'surface-dim': '#030817',
        'surface-container-lowest': '#050b18',
        'surface-container-low': '#081121',
        'surface-container': '#0b1526',
        'surface-container-high': '#101c30',
        'surface-container-highest': '#16233a',
        'surface-bright': '#1d2b43',
        'surface-variant': '#16233a',
        'on-background': '#f4f7ff',
        'on-surface': '#eef3fc',
        'on-surface-variant': '#9caac0',
        'inverse-on-surface': '#111a2c',
        'inverse-surface': '#eef3fc',
        primary: '#39c1f4',
        'primary-fixed-dim': '#71d5fb',
        'primary-container': '#3ab8eb',
        'on-primary': '#03101a',
        'surface-tint': '#39c1f4',
        secondary: '#d8e1ef',
        'secondary-container': '#173149',
        'on-secondary': '#101b2a',
        'on-secondary-container': '#d9ebf6',
        tertiary: '#f7bb62',
        'tertiary-container': '#e99a28',
        error: '#ff9b9b',
        'error-container': '#7f2633',
        'on-error-container': '#ffd9dc',
        outline: '#8996aa',
        'outline-variant': '#44536a'
      },
      borderRadius: {
        DEFAULT: '0.125rem',
        lg: '0.1875rem',
        xl: '0.25rem'
      },
      spacing: {
        xs: '4px',
        sm: '8px',
        md: '16px',
        lg: '24px',
        xl: '32px',
        'margin-desktop': '32px',
        'sidebar-width': '260px'
      },
      fontFamily: {
        sans: ['Poppins', 'Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'monospace']
      },
      boxShadow: {
        panel: '0 14px 32px rgba(0, 0, 0, 0.18)'
      }
    }
  },
  plugins: []
}
