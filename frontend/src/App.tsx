import { useEffect, type ReactNode } from 'react'
import { Navigate, NavLink, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import DashboardPage from './pages/DashboardPage'
import BrokersPage from './pages/BrokersPage'
import MarketPage from './pages/MarketPage'
import LoginPage from './pages/LoginPage'
import AuthCallback from './pages/AuthCallback'
import SetupPage from './pages/SetupPage'
import { UNAUTHORIZED_EVENT } from './lib/api'
import { clearSession, hasValidSession } from './lib/session'

const isAuthenticated = () => hasValidSession()
const isSetupCompleted = () => localStorage.getItem('isSetupCompleted') === 'true'

function RequireAuth({ requireSetup = false, children }: { requireSetup?: boolean; children: ReactNode }) {
  if (!isAuthenticated()) return <Navigate to="/login" replace />
  if (requireSetup && !isSetupCompleted()) return <Navigate to="/setup" replace />
  return <>{children}</>
}

function LoginRoute() {
  if (isAuthenticated()) {
    return <Navigate to={isSetupCompleted() ? '/' : '/setup'} replace />
  }
  return <LoginPage />
}

function Nav() {
  const location = useLocation()
  const navigate = useNavigate()

  const isAuthRoute = location.pathname === '/login' || location.pathname === '/setup'
  if (isAuthRoute) return null

  const userName = localStorage.getItem('userName')

  const logout = () => {
    clearSession()
    navigate('/login')
  }

  const linkClass = (active: boolean) =>
    `rounded-lg px-3 py-1.5 text-sm font-semibold transition ${
      active ? 'bg-teal-700 text-white' : 'text-slate-600 hover:bg-slate-100'
    }`

  return (
    <nav className="border-b border-slate-300 bg-white/80 backdrop-blur">
      <div className="mx-auto flex max-w-5xl flex-wrap items-center gap-2 px-4 py-3 sm:gap-4">
        <span className="text-sm font-bold uppercase tracking-widest text-teal-700">Stock API</span>
        <NavLink to="/" className={({ isActive }) => linkClass(isActive)} end>
          Dashboard
        </NavLink>
        <NavLink to="/manage" className={({ isActive }) => linkClass(isActive)}>
          Broker
        </NavLink>
        <NavLink to="/market" className={({ isActive }) => linkClass(isActive)}>
          Market
        </NavLink>
        <div className="flex-1"></div>
        <button
          onClick={logout}
          className="rounded-lg px-3 py-1.5 text-sm font-semibold text-rose-600 hover:bg-rose-50 transition"
        >
          Logout
        </button>
      </div>
    </nav>
  )
}

export default function App() {
  const navigate = useNavigate()

  // Token ditolak server (expired/tidak valid) → bersihkan sesi dan minta login ulang.
  useEffect(() => {
    const onUnauthorized = () => {
      clearSession()
      navigate('/login', { replace: true })
    }
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
  }, [navigate])

  return (
    <div className="min-h-screen bg-white text-slate-800">
      <Nav />
      <main className="w-full py-7">
        <Routes>
          <Route path="/login" element={<LoginRoute />} />
          <Route path="/auth/callback" element={<AuthCallback />} />
          <Route
            path="/setup"
            element={
              <RequireAuth>
                <SetupPage />
              </RequireAuth>
            }
          />
          <Route
            path="/"
            element={
              <RequireAuth requireSetup>
                <DashboardPage />
              </RequireAuth>
            }
          />
          <Route
            path="/market"
            element={
              <RequireAuth requireSetup>
                <MarketPage />
              </RequireAuth>
            }
          />
          <Route
            path="/manage"
            element={
              <RequireAuth requireSetup>
                <BrokersPage />
              </RequireAuth>
            }
          />
        </Routes>
      </main>
    </div>
  )
}
