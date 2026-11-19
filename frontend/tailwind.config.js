import colors from 'tailwindcss/colors'

// Keep opacity modifiers (including overlays) in both color schemes.
const lightPalette = {}
const darkPalette = {}
function paired(light, dark) {
  const rgb = (hex) => hex.match(/[a-f0-9]{2}/gi).map((part) => parseInt(part, 16)).join(' ')
  const token = `--palette-${light.slice(1)}-${dark.slice(1)}`
  lightPalette[token] = rgb(light)
  darkPalette[token] = rgb(dark)
  return `rgb(var(${token}) / <alpha-value>)`
}

function tones(name, dark) {
  return Object.fromEntries(Object.entries(dark).map(([shade, color]) =>
    [shade, paired(colors[name][shade], color)]))
}

/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './index.html',
    './src/**/*.{js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      backgroundColor: {
        white: paired('#ffffff', '#101010'),
        slate: tones('slate', {50:'#0a0a0a',100:'#191919',200:'#242424',300:'#333333',400:'#454545',900:'#000000'}),
        teal: tones('teal', {50:'#191919',100:'#242424',700:'#414141',800:'#505050',900:'#595959'}),
        rose: tones('rose', {50:'#30151d'}),
        amber: tones('amber', {50:'#2e2414'}),
      },
      textColor: {
        slate: tones('slate', {400:'#a0a0a0',500:'#aaaaaa',600:'#b5b5b5',700:'#cecece',800:'#ededed',900:'#fafafa'}),
        teal: tones('teal', {600:'#d4d4d4',700:'#ededed',800:'#dedede'}),
        emerald: tones('emerald', {600:'#6ee7b7',700:'#86efac'}),
        rose: tones('rose', {500:'#fda4af',600:'#fda4af',700:'#fda4af',800:'#fecdd3'}),
        amber: tones('amber', {800:'#fcd34d',900:'#fde68a'}),
      },
      borderColor: {
        slate: tones('slate', {100:'#202020',200:'#2b2b2b',300:'#383838',500:'#737373'}),
        teal: tones('teal', {200:'#494949',600:'#888888',700:'#888888'}),
        rose: tones('rose', {200:'#65303b',700:'#df7289'}),
        amber: tones('amber', {200:'#675032'}),
      },
      outlineColor: {teal: tones('teal', {700:'#ededed'})},
      fontFamily: {
        sans: ['Matter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
      },
    },
  },
  plugins: [({ addBase }) => addBase({ ':root': lightPalette, ':root.dark': darkPalette })],
}
