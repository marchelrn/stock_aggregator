import { useEffect, useRef, useState, type ReactNode } from "react";
import {
  Navigate,
  NavLink,
  Outlet,
  Route,
  Routes,
  useLocation,
  useNavigate,
} from "react-router-dom";
import { Menu, SquareArrowRightExit, X } from "lucide-react";
import DashboardPage from "./pages/DashboardPage";
import BrokersPage from "./pages/BrokersPage";
import MarketPage from "./pages/MarketPage";
import LoginPage from "./pages/LoginPage";
import BrokerListPage from "./pages/BrokerListPage";
import ThemeToggle from "./components/ThemeToggle";
import NotFoundPage from "./pages/NotFoundPage";
import AuthCallback from "./pages/AuthCallback";
import GmailCallback from "./pages/GmailCallback";
import SetupPage from "./pages/SetupPage";
import { UNAUTHORIZED_EVENT } from "./lib/api";
import { clearSession, hasValidSession } from "./lib/session";

const isAuthenticated = () => hasValidSession();
const isSetupCompleted = () =>
  localStorage.getItem("isSetupCompleted") === "true";

function RequireAuth({
  requireSetup = false,
  children,
}: {
  requireSetup?: boolean;
  children: ReactNode;
}) {
  if (!isAuthenticated()) return <Navigate to="/login" replace />;
  if (requireSetup && !isSetupCompleted())
    return <Navigate to="/setup" replace />;
  return <>{children}</>;
}

function LoginRoute() {
  if (isAuthenticated()) {
    return <Navigate to={isSetupCompleted() ? "/" : "/setup"} replace />;
  }
  return <LoginPage />;
}

function Nav() {
  const location = useLocation();
  const navigate = useNavigate();

  const [open, setOpen] = useState(false);
  const navRef = useRef<HTMLElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    setOpen(false);
  }, [location.key]);
  useEffect(() => {
    if (!open) return;
    const closeOutside = (event: PointerEvent) => {
      if (
        event.target instanceof Node &&
        !navRef.current?.contains(event.target)
      )
        setOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
        trigger.current?.focus();
      }
    };
    const desktop = window.matchMedia("(min-width: 768px)");
    const closeOnResize = () => setOpen(false);
    desktop.addEventListener("change", closeOnResize);
    document.addEventListener("pointerdown", closeOutside);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      desktop.removeEventListener("change", closeOnResize);
      document.removeEventListener("pointerdown", closeOutside);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [open]);

  if (
    !["/", "/manage", "/market"].includes(
      location.pathname.replace(/\/$/, "") || "/",
    ) ||
    !isAuthenticated()
  )
    return null;

  const logout = () => {
    clearSession();
    navigate("/login");
  };

  const linkClass = (active: boolean) =>
    `rounded-lg px-3 py-1.5 text-sm font-semibold transition ${
      active ? "bg-teal-700 text-white" : "text-slate-600 hover:bg-slate-100"
    }`;

  return (
    <nav
      ref={navRef}
      aria-label="Menu utama"
      className="relative z-30 border-b border-slate-300 bg-white/80 backdrop-blur"
      onBlur={(event) => {
        if (
          event.relatedTarget &&
          !event.currentTarget.contains(event.relatedTarget)
        )
          setOpen(false);
      }}
    >
      <div className="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-2 md:py-3">
        <span className="text-sm font-bold uppercase tracking-widest text-teal-700">
          Stock API
        </span>
        <button
          ref={trigger}
          type="button"
          aria-label={open ? "Tutup menu" : "Buka menu"}
          aria-controls="main-navigation"
          aria-expanded={open}
          onClick={() => setOpen(!open)}
          className="grid size-11 place-items-center rounded-lg text-slate-700 hover:bg-slate-100 md:hidden"
        >
          {open ? (
            <X size={22} aria-hidden="true" />
          ) : (
            <Menu size={22} aria-hidden="true" />
          )}
        </button>
        <div
          id="main-navigation"
          className={`absolute inset-x-0 top-full flex max-h-[calc(100dvh-4rem)] flex-col gap-1 overflow-y-auto border-b border-slate-300 bg-white px-4 py-3 shadow-md transition-[opacity,transform] motion-reduce:transition-none md:static md:visible md:flex-1 md:translate-y-0 md:flex-row md:items-center md:gap-4 md:overflow-visible md:border-0 md:bg-transparent md:p-0 md:opacity-100 md:shadow-none ${open ? "visible translate-y-0 opacity-100" : "invisible -translate-y-2 opacity-0"}`}
        >
          {[
            ["/", "Dashboard"],
            ["/manage", "Broker"],
            ["/market", "Market"],
          ].map(([path, label]) => (
            <NavLink
              key={path}
              to={path}
              end={path === "/"}
              onClick={() => setOpen(false)}
              className={({ isActive }) =>
                `${linkClass(isActive)} flex min-h-11 items-center md:min-h-0`
              }
            >
              {label}
            </NavLink>
          ))}
          <div className="mt-2 flex items-center justify-between border-t border-slate-200 pt-2 md:ml-auto md:mt-0 md:gap-4 md:border-0 md:pt-0">
            <ThemeToggle mobileLabel />
            <button
              type="button"
              aria-label="Logout"
              onClick={logout}
              className="inline-flex min-h-11 items-center gap-2 rounded-lg px-3 text-sm font-semibold text-rose-600 transition hover:bg-rose-50"
            >
              <SquareArrowRightExit size={18} aria-hidden="true" />
              <span className="md:hidden">Logout</span>
            </button>
          </div>
        </div>
      </div>
    </nav>
  );
}

export default function App() {
  const navigate = useNavigate();

  // Token ditolak server (expired/tidak valid) → bersihkan sesi dan minta login ulang.
  useEffect(() => {
    const onUnauthorized = () => {
      clearSession();
      navigate("/login", { replace: true });
    };
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized);
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized);
  }, [navigate]);

  return (
    <div className="min-h-screen bg-white text-slate-800">
      <Routes>
        <Route
          element={
            <>
              <Nav />
              <main className="w-full py-7">
                <Outlet />
              </main>
            </>
          }
        >
          <Route path="/login" element={<LoginRoute />} />
          <Route path="/brokerlist" element={<BrokerListPage />} />
          <Route path="/auth/callback" element={<AuthCallback />} />
          <Route
            path="/auth/gmail/callback"
            element={
              <RequireAuth>
                <GmailCallback />
              </RequireAuth>
            }
          />
          <Route
            path="/setup/review?"
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
        </Route>
        <Route
          path="*"
          element={
            <NotFoundPage
              authenticated={isAuthenticated()}
              setupCompleted={isSetupCompleted()}
            />
          }
        />
      </Routes>
    </div>
  );
}
