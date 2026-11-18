import { Link } from 'react-router-dom'

export default function NotFoundPage({ authenticated, setupCompleted }: {
  authenticated: boolean
  setupCompleted: boolean
}) {
  const destination = !authenticated ? '/login' : setupCompleted ? '/' : '/setup'
  const label = !authenticated ? 'Login' : setupCompleted ? 'Kembali ke dashboard' : 'Lanjutkan setup'

  return (
    <main className="mx-auto flex min-h-dvh max-w-xl flex-col items-center justify-center px-5 py-8 text-center" aria-labelledby="not-found-title">
      <p className="mb-2 text-lg font-semibold text-teal-700">404</p>
      <h1 id="not-found-title" className="text-3xl font-semibold tracking-tight">Halaman tidak ditemukan</h1>
      <p className="mt-3 text-slate-600">Periksa kembali alamat yang Anda masukkan.</p>
      <Link to={destination} replace className="mt-6 inline-flex min-h-11 items-center rounded-lg bg-teal-700 px-4 text-sm font-semibold text-white hover:bg-teal-800 active:bg-teal-900 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700">
        {label}
      </Link>
    </main>
  )
}
