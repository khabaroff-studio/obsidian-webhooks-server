module.exports = {
  content: ['./src/templates/**/*.html'],
  theme: {
    extend: {
      fontFamily: {
        display: ['Oswald', 'sans-serif'],
        body: ['Inter', '-apple-system', 'BlinkMacSystemFont', 'sans-serif'],
      },
      colors: {
        ink: '#0a0a0a',
        'ink-soft': '#444444',
        'ink-muted': '#666666',
        'ink-dark': '#1a1a1a',
        accent: '#7C3AED',
        line: '#e5e5e5',
        paper: '#FFFFFF',
        'paper-warm': '#f5f5f5',
        'paper-cta': '#f7f7f7',
        github: '#24292e',
        status: '#22c55e',
      },
    },
  },
}
