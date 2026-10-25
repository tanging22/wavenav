/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        ink: '#0b1020',
      },
      boxShadow: {
        card: '0 8px 30px -12px rgba(0,0,0,.45)',
      },
    },
  },
  plugins: [],
}
