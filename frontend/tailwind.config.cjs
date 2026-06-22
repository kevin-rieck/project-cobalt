module.exports = {
  content: ['./index.html', './src/**/*.{svelte,ts,js}'],
  theme: {
    extend: {
      colors: {
        'primary-fixed-dim': '#7bd0ff',
        'inverse-on-surface': '#283044',
        'surface-container-low': '#131b2e',
        'inverse-surface': '#dae2fd',
        'surface-container-high': '#222a3d',
        primary: '#8ed5ff',
        'error-container': '#93000a',
        'surface-bright': '#31394d',
        secondary: '#b9c8de',
        'surface-container-highest': '#2d3449',
        'on-surface': '#dae2fd',
        'surface-tint': '#7bd0ff',
        'surface-variant': '#2d3449',
        error: '#ffb4ab',
        'surface-dim': '#0b1326',
        'surface-container-lowest': '#060e20',
        'on-surface-variant': '#bdc8d1',
        'secondary-container': '#39485a',
        'on-secondary-container': '#a7b6cc',
        'on-secondary': '#233143',
        'on-primary': '#00354a',
        outline: '#87929a',
        background: '#0b1326',
        surface: '#0b1326',
        'surface-container': '#171f33',
        'on-error-container': '#ffdad6',
        'primary-container': '#38bdf8',
        tertiary: '#ffc176',
        'tertiary-container': '#f1a02b',
        'outline-variant': '#3e484f',
        'on-background': '#dae2fd'
      },
      borderRadius: {
        DEFAULT: '0.125rem',
        lg: '0.25rem',
        xl: '0.5rem'
      },
      spacing: {
        xs: '4px',
        sm: '8px',
        md: '16px',
        lg: '24px',
        xl: '32px',
        'margin-desktop': '40px',
        'sidebar-width': '280px'
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace']
      }
    }
  },
  plugins: []
}
