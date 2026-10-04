import { useEffect, useRef } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { toast } from 'react-toastify'
import { clearSession, syncSessionFromServer } from '../lib/session'

export default function AuthCallback() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const handled = useRef(false)

  useEffect(() => {
    if (handled.current) return
    handled.current = true

    const token = searchParams.get('token')

    // Backend hanya redirect ke sini dengan ?token=<JWT> saat login berhasil.
    if (!token) {
      toast.error('Login gagal: token tidak ditemukan.')
      navigate('/login', { replace: true })
      return
    }

    // Simpan JWT dan tandai user sudah terautentikasi.
    localStorage.setItem('token', token)
    localStorage.setItem('isAuthenticated', 'true')

    // Ambil info user dan cek apakah user sudah pernah setup (punya broker).
    // Ini penting agar user yang sudah pernah login tidak diarahkan ke /setup lagi.
    syncSessionFromServer()
      .then(({ setupCompleted }) => {
        navigate(setupCompleted ? '/' : '/setup', { replace: true })
      })
      .catch(() => {
        toast.error('Login gagal: sesi tidak valid.')
        clearSession()
        navigate('/login', { replace: true })
      })
  }, [navigate, searchParams])

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
