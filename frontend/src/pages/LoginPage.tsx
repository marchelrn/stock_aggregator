import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { getBaseUrl } from '../lib/api'
import { clearSession, isTokenExpired, syncSessionFromServer } from '../lib/session'

export default function LoginPage() {
  const navigate = useNavigate()
  const [checking, setChecking] = useState(true)

  // Saat halaman login dibuka, cek apakah token yang tersimpan masih valid.
  // Jika valid, langsung masuk tanpa perlu OAuth ulang.
  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token || isTokenExpired(token)) {
      clearSession()
      setChecking(false)
      return
    }

    // Verifikasi token dan sekaligus cek apakah user sudah pernah setup.
    syncSessionFromServer()
      .then(({ setupCompleted }) => {
        localStorage.setItem('isAuthenticated', 'true')
        navigate(setupCompleted ? '/' : '/setup', { replace: true })
      })
      .catch(() => {
        // Token kadaluarsa atau tidak valid — hapus dan tampilkan form login.
        clearSession()
        setChecking(false)
      })
  }, [navigate])

  const handleLogin = () => {
    window.location.href = `${getBaseUrl()}/auth/google/login`
  }

  // Tampilkan spinner sementara mengecek token yang tersimpan.
  if (checking) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-white">
        <div className="flex flex-col items-center gap-4 text-slate-600">
          <svg
            className="h-8 w-8 animate-spin text-slate-400"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
          </svg>
          <p className="text-[15px] font-medium">Memeriksa sesi...</p>
        </div>
      </div>
    )
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-white p-4">
      <div className="w-full max-w-[440px] rounded-[24px] bg-white p-10 shadow-xl relative border border-slate-300 text-left">
        <h2 className="mb-1 text-[22px] font-semibold text-slate-500 tracking-tight">Stock API.</h2>
        <h1 className="mb-8 text-[28px] font-bold text-slate-800 tracking-tight">
          Log in to your account
        </h1>

        <div className="space-y-3">
          <button
            onClick={handleLogin}
            className="flex w-full items-center justify-center gap-3 rounded-xl border border-slate-300 bg-white px-4 py-3.5 text-[15px] font-semibold text-slate-700 transition-all hover:bg-slate-50 active:scale-[0.98] shadow-sm"
          >
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 48 48">
              <path
                fill="#FFC107"
                d="M43.611,20.083H42V20H24v8h11.303c-1.649,4.657-6.08,8-11.303,8c-6.627,0-12-5.373-12-12c0-6.627,5.373-12,12-12c3.059,0,5.842,1.154,7.961,3.039l5.657-5.657C34.046,6.053,29.268,4,24,4C12.955,4,4,12.955,4,24c0,11.045,8.955,20,20,20c11.045,0,20-8.955,20-20C44,22.659,43.862,21.35,43.611,20.083z"
              ></path>
              <path
                fill="#FF3D00"
                d="M6.306,14.691l6.571,4.819C14.655,15.108,18.961,12,24,12c3.059,0,5.842,1.154,7.961,3.039l5.657-5.657C34.046,6.053,29.268,4,24,4C16.318,4,9.656,8.337,6.306,14.691z"
              ></path>
              <path
                fill="#4CAF50"
                d="M24,44c5.166,0,9.86-1.977,13.409-5.192l-6.19-5.238C29.211,35.091,26.715,36,24,36c-5.202,0-9.619-3.317-11.283-7.946l-6.522,5.025C9.505,39.556,16.227,44,24,44z"
              ></path>
              <path
                fill="#1976D2"
                d="M43.611,20.083H42V20H24v8h11.303c-0.792,2.237-2.231,4.166-4.087,5.571c0.001-0.001,0.002-0.001,0.003-0.002l6.19,5.238C36.971,39.205,44,34,44,24C44,22.659,43.862,21.35,43.611,20.083z"
              ></path>
            </svg>
            Continue with Google
          </button>
        </div>

        <div className="mt-8 pt-6 border-t border-slate-300">
          <div className="flex items-start gap-3 text-[16px] text-slate-500 leading-relaxed">
            <small>
              Pastikan Anda menggunakan Gmail utama yang terdaftar dan terhubung dengan akun
              sekuritas/broker Anda.
            </small>
          </div>
        </div>
      </div>
    </div>
  )
}
