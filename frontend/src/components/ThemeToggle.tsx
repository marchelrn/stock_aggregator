import { useEffect, useState } from 'react'
import { Moon, Sun } from 'lucide-react'

function isDark() {
  const scheme = document.documentElement.style.colorScheme
  return scheme ? scheme === 'dark' : window.matchMedia('(prefers-color-scheme: dark)').matches
}

export default function ThemeToggle() {
  const [dark, setDark] = useState(isDark)
  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const update = () => setDark(isDark())
    media.addEventListener('change', update)
    return () => media.removeEventListener('change', update)
  }, [])
  const toggle = () => {
    const scheme = dark ? 'light' : 'dark'
    document.documentElement.style.colorScheme = scheme
    document.documentElement.classList.toggle('dark', !dark)
    setDark(!dark)
    try { localStorage.setItem('portfolio-theme', scheme) } catch {}
  }
  const Icon = dark ? Sun : Moon
  return (
    <button type="button" onClick={toggle} aria-label="Mode gelap" aria-pressed={dark}
      title={dark ? 'Gunakan tema terang' : 'Gunakan tema gelap'}
      className="inline-flex size-11 cursor-pointer items-center justify-center rounded-lg text-slate-600 hover:bg-slate-100 active:bg-slate-200 focus-visible:outline focus-visible:outline-2 focus-visible:outline-teal-700">
      <Icon size={18} aria-hidden="true" />
    </button>
  )
}
