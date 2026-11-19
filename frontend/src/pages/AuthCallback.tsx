import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { getBaseUrl } from '../lib/api'
import { toast } from 'react-toastify'
import { clearSession, syncSessionFromServer } from '../lib/session'

export default function AuthCallback() {
  const navigate = useNavigate()
  const handled = useRef(false)

  useEffect(() => {
    if (handled.current) return
    handled.current = true

    const params = new URLSearchParams(window.location.hash.slice(1))
    window.history.replaceState(window.history.state, '', window.location.pathname)

    const complete = async () => {
      try {
        const pending = JSON.parse(sessionStorage.getItem('pending-google-login') || 'null')
        if (!pending || pending.state !== params.get('state') || !(pending.expires > Date.now())) {
          throw new Error('Sesi login tidak valid.')
        }
        sessionStorage.removeItem('pending-google-login')
        const code = params.get('code')
        if (!code) throw new Error('Login Google dibatalkan.')
        const response = await fetch(`${getBaseUrl()}/auth/google/complete`, {
          method: 'POST',
          cache: 'no-store',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ code, verifier: pending.verifier }),
        })
        if (!response.ok) throw new Error('Gagal memverifikasi login Google.')
        const { token } = await response.json()
        if (typeof token !== 'string' || !token) throw new Error('Sesi login tidak tersedia.')
        clearSession()
        localStorage.setItem('token', token)
        localStorage.setItem('isAuthenticated', 'true')
        const { setupCompleted } = await syncSessionFromServer()
        navigate(setupCompleted ? '/' : '/setup', { replace: true })
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'Login gagal.')
        navigate('/login', { replace: true })
      }
    }
    void complete()
  }, [navigate])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-white p-4">
      <div className="flex flex-col items-center gap-4 text-slate-600">
        <svg
          className="h-8 w-8 animate-spin text-slate-400"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle
            className="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            strokeWidth="4"
          ></circle>
          <path
            className="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
          ></path>
        </svg>
        <p className="text-[15px] font-medium">Menyelesaikan proses login...</p>
      </div>
    </div>
  )
}
