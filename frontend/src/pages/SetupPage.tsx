import { useCallback, useEffect, useRef, useState } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { toast } from "react-toastify";
import { usePortfolio } from "../context/PortfolioContext";
import SetupReview, { type ManualHolding } from "../components/SetupReview";
import { startGmailConnection } from "../components/GmailConnection";
import type { GmailSyncStatusResponse } from "../types";
import ThemeToggle from "../components/ThemeToggle";
import { formatCurrency } from "../lib/formatters";
import { api } from "../lib/api";
import { ChevronRightIcon } from "lucide-react";

import { BROKERS } from "../lib/brokerOptions";

export default function SetupPage() {
  const navigate = useNavigate();
  const {
    state,
    addBroker,
    loadDashboard,
    syncSetupHistorical,
    createTransaction,
  } = usePortfolio();
  const location = useLocation();
  const [brokerName, setBrokerName] = useState(
    () =>
      BROKERS.find(
        (broker) =>
          broker.name === location.state?.brokerName && !broker.disabled,
      )?.name ?? BROKERS[0].name,
  );
  const [syncing, setSyncing] = useState(false);
  const [cash, setCash] = useState("");
  const [pin, setPin] = useState("");
  const [showForm, setShowForm] = useState(true);
  const [running, setRunning] = useState(false);
  const [initialLoading, setInitialLoading] = useState(true);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [elapsed, setElapsed] = useState(0);
  const review = location.pathname === "/setup/review";
  const [manualHoldings, setManualHoldings] = useState<ManualHolding[]>([]);
  const [finishing, setFinishing] = useState(false);
  const [finishError, setFinishError] = useState("");
  const busy = useRef(false);
  const selectRef = useRef<HTMLSelectElement>(null);
  const option = BROKERS.find((broker) => broker.name === brokerName)!;
  const savedBroker = state.brokers.find(
    (broker) => broker.name === brokerName,
  );
  const existing = Boolean(savedBroker);
  const cost = state.holdings.reduce(
    (sum, holding) => sum + holding.lot * 100 * holding.avg_price,
    0,
  );
  const totalCash = state.brokers.reduce((sum, broker) => sum + broker.cash, 0);

  useEffect(() => {
    let active = true;
    loadDashboard({ silent: true }).finally(() => {
      if (active) setInitialLoading(false);
    });
    return () => {
      active = false;
    };
  }, [loadDashboard]);

  useEffect(() => {
    if (!running) return;
    const timer = window.setInterval(
      () => setElapsed((value) => value + 1),
      1000,
    );
    return () => window.clearInterval(timer);
  }, [running]);

  const syncSavedBroker = useCallback(
    async (name: string) => {
      setSyncing(true);
      try {
        const result = await syncSetupHistorical();
        await loadDashboard({ silent: true });
        if (!result)
          throw new Error(
            "Sinkronisasi gagal. Broker tetap tersimpan. Coba lagi atau lengkapi saham secara manual.",
          );
        if (result.trade_error || result.cash_error) {
          throw new Error(
            [result.trade_error, result.cash_error].filter(Boolean).join(" "),
          );
        }
        setShowForm(false);
      } catch (err) {
        setError(
          err instanceof Error
            ? err.message
            : "Sinkronisasi gagal. Broker tetap tersimpan.",
        );
        setMessage("");
      } finally {
        setSyncing(false);
      }
    },
    [syncSetupHistorical, loadDashboard],
  );

  useEffect(() => {
    if (!location.state?.syncAfterConsent || initialLoading || busy.current)
      return;
    busy.current = true;
    navigate("/setup", { replace: true, state: null });
    setRunning(true);
    void syncSavedBroker(brokerName).finally(() => {
      busy.current = false;
      setRunning(false);
    });
  }, [location.state, initialLoading, brokerName, navigate, syncSavedBroker]);

  const connectBroker = async (event: React.FormEvent) => {
    event.preventDefault();
    if (busy.current || initialLoading) return;
    busy.current = true;
    setRunning(true);
    setError("");
    setElapsed(0);
    setMessage(`Menyiapkan ${brokerName}…`);
    try {
      if (savedBroker && pin) {
        await api(`/broker/${savedBroker.id}`, {
          method: "PUT",
          body: JSON.stringify({ pin }),
        });
        setPin("");
      }
      if (!existing && !(await addBroker(brokerName, Number(cash) || 0, pin))) {
        setError(
          `Broker ${brokerName} belum berhasil disimpan. Silakan coba lagi.`,
        );
        setMessage("");
        return;
      }
      setCash("");
      setPin("");
      const status = await api<GmailSyncStatusResponse>("/api/gmail/status");
      if (!status.gmail_sync_enabled) {
        await startGmailConnection("/setup", brokerName);
        return;
      }
      await syncSavedBroker(brokerName);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Gagal menyiapkan sinkronisasi Gmail. Coba lagi.",
      );
      setMessage("");
    } finally {
      busy.current = false;
      setRunning(false);
    }
  };

  const finishSetup = async () => {
    if (busy.current) return;
    busy.current = true;
    setFinishing(true);
    setFinishError("");
    const failed: ManualHolding[] = [];
    try {
      for (const holding of manualHoldings) {
        const ok = await createTransaction({
          type: "BUY",
          ticker: holding.ticker,
          broker_id: holding.brokerId,
          broker_name: holding.brokerName,
          lot: holding.lot,
          price: holding.price,
        });
        if (!ok) failed.push(holding);
      }
      setManualHoldings(failed);
      if (failed.length) {
        setFinishError(
          `Saham ${failed.map((holding) => holding.ticker).join(", ")} gagal disimpan. Periksa saldo kas broker, lalu coba lagi.`,
        );
        return;
      }
      localStorage.setItem("isSetupCompleted", "true");
      toast.success("Setup awal selesai. Portofolio siap dipakai.");
      navigate("/manage");
    } finally {
      busy.current = false;
      setFinishing(false);
    }
  };

  const openForm = () => {
    const available = BROKERS.find(
      (broker) =>
        !broker.disabled &&
        !state.brokers.some((saved) => saved.name === broker.name),
    );
    if (available) setBrokerName(available.name);
    setShowForm(true);
    setCash("");
    setPin("");
    window.setTimeout(() => selectRef.current?.focus(), 0);
  };

  const continueButton = (
    <button
      type="button"
      className="cursor-pointer rounded-lg border px-4 py-3 font-semibold transition-colors motion-reduce:transition-none disabled:cursor-not-allowed disabled:border-slate-300 disabled:bg-slate-200 disabled:text-slate-600 border-teal-700 bg-teal-700 text-white enabled:hover:bg-teal-800 enabled:active:bg-teal-900"
      disabled={running || initialLoading || state.brokers.length === 0}
      onClick={() => navigate("/setup/review")}
    >
      Continue
    </button>
  );

  return (
    <div className="-mt-7 min-h-[100dvh] bg-white text-[0.9375rem] text-slate-800 [&_:is(button,select,input):focus-visible]:outline [&_:is(button,select,input):focus-visible]:outline-teal-700 [&_:is(button,select,input):focus-visible]:outline-offset-2">
      <header className="grid grid-cols-[1fr_auto] items-center gap-y-2 border-b border-slate-300 px-5 py-4 sm:grid-cols-[1fr_auto_1fr] sm:px-10">
        <span className="sm:ml-8 text-lg font-[550] text-teal-700">
          Stock API
        </span>
        <nav
          aria-label="Progres konfigurasi"
          className="col-span-full row-start-2 justify-self-center sm:col-span-1 sm:col-start-2 sm:row-start-1"
        >
          <ol className="m-0 flex list-none items-center gap-3 p-0 text-base text-slate-600 [&_[aria-current=step]]:font-bold [&_[aria-current=step]]:text-teal-700">
            <li aria-current={review ? undefined : "step"}>Connect</li>
            <ChevronRightIcon size={16} aria-hidden="true" />

            <li aria-current={review ? "step" : undefined}>Review</li>
          </ol>
        </nav>
        <div className="col-start-2 row-start-1 flex justify-self-end sm:col-start-3">
          <ThemeToggle />
        </div>
      </header>
      {!review && (
        <div className="grid min-w-0 min-[1000px]:min-h-[calc(100dvh_-_5rem)] min-[1000px]:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)]">
          <section
            className="flex min-w-0 flex-col gap-8 px-5 py-8 min-[600px]:p-10 min-[1000px]:p-[clamp(2.5rem,5vw,5rem)]"
            aria-labelledby="setup-title"
          >
            <div>
              <h1
                className="mb-3 text-[clamp(1.75rem,3vw,2.5rem)] font-semibold leading-tight tracking-tight"
                id="setup-title"
              >
                Hubungkan broker
              </h1>
              <p className="mb-0 max-w-[38rem] leading-7 text-slate-600">
                Simpan broker untuk mengimpor transaksi dan mutasi kas.
              </p>
            </div>

            {(initialLoading || state.brokers.length > 0) && (
              <div aria-busy={initialLoading}>
                <h2 className="mb-3 flex items-baseline gap-3 text-[1.0625rem] font-[650]">
                  Broker terhubung{" "}
                  <span className="text-sm font-normal text-slate-600">
                    ({state.brokers.length})
                  </span>
                </h2>
                {initialLoading ? (
                  <p
                    className="mb-0 leading-relaxed text-slate-600"
                    role="status"
                  >
                    Memuat broker Anda…
                  </p>
                ) : state.brokers.length === 0 ? (
                  <p className="mb-0 leading-relaxed text-slate-600">
                    Belum ada broker terhubung. Pilih broker pertama di bawah.
                  </p>
                ) : (
                  <ul className="m-0 list-none divide-y divide-slate-300 p-0">
                    {state.brokers.map((broker) => (
                      <li
                        className="flex flex-wrap items-center justify-between gap-3 py-4 [&>div]:grid [&>div]:min-w-0 [&>div]:gap-1.5 [&_span]:text-[0.8125rem] [&_span]:text-slate-600 [&_span]:[overflow-wrap:anywhere]"
                        key={broker.id}
                      >
                        <div>
                          <strong>{broker.name}</strong>
                          <span>Saldo kas</span>
                        </div>
                        <span className="text-right tabular-nums">
                          {formatCurrency(broker.cash)}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )}

            {showForm && (
              <form
                onSubmit={connectBroker}
                className="grid gap-2 [&>label]:text-sm [&>label]:font-semibold"
              >
                <label htmlFor="setup-broker">Sekuritas</label>
                <select
                  className="w-full min-w-0 rounded-md border border-slate-500 bg-white px-3 py-2.5 text-slate-800 disabled:cursor-not-allowed disabled:bg-slate-100 cursor-pointer"
                  ref={selectRef}
                  id="setup-broker"
                  value={brokerName}
                  disabled={running || initialLoading}
                  onChange={(event) => {
                    setBrokerName(event.target.value);
                    setCash("");
                    setPin("");
                    setError("");
                  }}
                >
                  {BROKERS.map((broker) => (
                    <option
                      key={broker.name}
                      value={broker.name}
                      disabled={broker.disabled}
                    >
                      {broker.name}
                      {broker.disabled ? " (belum tersedia)" : ""}
                    </option>
                  ))}
                </select>
                {!existing && (
                  <>
                    <label htmlFor="setup-cash">Saldo kas awal (IDR)</label>
                    <input
                      className="w-full min-w-0 rounded-md border border-slate-500 bg-white px-3 py-2.5 text-slate-800 disabled:cursor-not-allowed disabled:bg-slate-100"
                      id="setup-cash"
                      type="number"
                      min="0"
                      step="0.01"
                      placeholder="0"
                      value={cash}
                      disabled={running}
                      onChange={(event) => setCash(event.target.value)}
                    />
                  </>
                )}
                {option.needsPin && (
                  <>
                    <label htmlFor="setup-pin">PIN trade confirmation</label>
                    <input
                      className="w-full min-w-0 rounded-md border border-slate-500 bg-white px-3 py-2.5 text-slate-800 disabled:cursor-not-allowed disabled:bg-slate-100"
                      id="setup-pin"
                      type="password"
                      required={!existing}
                      autoComplete="off"
                      placeholder={
                        existing ? "Isi untuk memperbarui PIN" : undefined
                      }
                      value={pin}
                      disabled={running}
                      onChange={(event) => setPin(event.target.value)}
                    />
                  </>
                )}
                <button
                  className="cursor-pointer rounded-lg border px-4 py-3 font-semibold transition-colors motion-reduce:transition-none disabled:cursor-not-allowed disabled:border-slate-300 disabled:bg-slate-200 disabled:text-slate-600 border-slate-500 bg-white text-slate-700 mt-4"
                  type="submit"
                  disabled={running || initialLoading}
                >
                  {running
                    ? syncing
                      ? "Menyinkronkan…"
                      : "Menyiapkan broker…"
                    : existing
                      ? "Sync Again"
                      : "Add Broker"}
                </button>
              </form>
            )}

            {/*<div className="text-sm leading-relaxed text-teal-800 [overflow-wrap:anywhere] empty:hidden [&>p]:mb-0" aria-live="polite" aria-atomic="true">
            {message && !running && <p role="status">{message}</p>}
          </div>*/}
            {error && (
              <p
                className="mb-0 rounded-md border border-rose-700 bg-rose-50 p-4 text-rose-800 [overflow-wrap:anywhere]"
                role="alert"
              >
                {error}
              </p>
            )}
            <div
              className={showForm ? "hidden min-[1000px]:grid" : "grid gap-3"}
            >
              {!showForm && (
                <button
                  type="button"
                  className="min-h-[2.875rem] cursor-pointer rounded-lg border px-4 py-3 font-semibold transition-colors motion-reduce:transition-none disabled:cursor-not-allowed disabled:border-slate-300 disabled:bg-slate-200 disabled:text-slate-600 border-slate-500 bg-white text-slate-700 enabled:hover:bg-slate-100 enabled:active:bg-slate-200"
                  disabled={running}
                  onClick={openForm}
                >
                  Tambah broker lain
                </button>
              )}
              <div className="hidden min-[1000px]:grid">{continueButton}</div>
            </div>
          </section>

          <aside
            className="min-w-0 border-t border-slate-300 bg-slate-50 px-5 py-8 min-[600px]:p-10 min-[1000px]:border-l min-[1000px]:border-t-0 min-[1000px]:px-[clamp(2rem,4vw,4rem)] min-[1000px]:py-[clamp(2.5rem,5vw,5rem)]"
            aria-labelledby="portfolio-title"
          >
            <h2
              className="mb-3 text-[clamp(1.25rem,2vw,1.5rem)] font-semibold tracking-tight"
              id="portfolio-title"
            >
              Portofolio Anda
            </h2>
            <dl className="mb-6 grid gap-6 border-b border-slate-300 py-7 min-[600px]:grid-cols-2 [&_dt]:text-[0.8125rem] [&_dt]:text-slate-600 [&_dd]:mb-0 [&_dd]:mt-2 [&_dd]:text-[clamp(1.25rem,2vw,1.875rem)] [&_dd]:font-[650] [&_dd]:tabular-nums [&_dd]:[overflow-wrap:anywhere]">
              <div>
                <dt>Nilai pembelian · IDR</dt>
                <dd>{formatCurrency(cost)}</dd>
              </div>
              <div>
                <dt>Saldo kas · IDR</dt>
                <dd>{formatCurrency(totalCash)}</dd>
              </div>
            </dl>
            <div className="flex flex-wrap items-baseline justify-between gap-2 [&>h3]:m-0 [&>h3]:text-base [&>h3]:font-[650] [&>span]:text-sm [&>span]:text-slate-600">
              <h3>Daftar Ticker</h3>
              <span>
                {state.holdings.length} Emiten dari {state.brokers.length}{" "}
                Broker
              </span>
            </div>
            {syncing && (
              <div
                className="mt-5 grid gap-1.5 rounded-md border border-teal-700 bg-teal-50 p-4 text-sm leading-relaxed text-teal-800 [&>span]:text-[0.8125rem]"
                role="status"
              >
                <strong>Membaca saham dari email…</strong>
                <span>{elapsed} detik · Dapat memerlukan beberapa menit.</span>
              </div>
            )}
            {initialLoading ? (
              <p
                role="status"
                className="mb-0 mt-4 py-6 leading-relaxed text-slate-600 [&>p]:mb-0 [&>p]:mt-2 [&>strong]:font-semibold [&>strong]:text-slate-700"
              >
                Memuat portofolio…
              </p>
            ) : state.holdings.length === 0 ? (
              <div className="mb-0 mt-10 py-6 text-center leading-relaxed text-slate-600 [&>p]:mb-0 [&>p]:mt-2 [&>strong]:font-semibold [&>strong]:text-slate-700">
                <strong>Belum ada saham terbaca</strong>
                <p>
                  {state.brokers.length
                    ? "Sinkronkan ulang atau lengkapi saham pada langkah berikutnya."
                    : "Saham akan muncul setelah broker terhubung."}
                </p>
              </div>
            ) : (
              <ul className="m-0 list-none p-0">
                {state.holdings.map((holding, index) => (
                  <li
                    className="flex flex-wrap items-center justify-between gap-3 py-4 [&>div]:grid [&>div]:min-w-0 [&>div]:gap-1.5 [&_span]:text-[0.8125rem] [&_span]:text-slate-600 [&_span]:[overflow-wrap:anywhere] border-b border-slate-300"
                    key={`${holding.broker_id}-${holding.ticker}-${index}`}
                  >
                    <div>
                      <strong>{holding.ticker}</strong>
                      <span>{holding.broker_name}</span>
                    </div>
                    <div className="text-right tabular-nums">
                      <strong>{holding.lot} lot</strong>
                      <span>Rata-rata {formatCurrency(holding.avg_price)}</span>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </aside>
          <div className="grid px-5 py-6 min-[600px]:px-10 min-[1000px]:hidden">
            {continueButton}
          </div>
        </div>
      )}
      {review &&
        (initialLoading ? (
          <p role="status" className="p-5 text-center">
            Memuat portofolio…
          </p>
        ) : state.brokers.length === 0 ? (
          <Navigate to="/setup" replace />
        ) : (
          <SetupReview
            syncError={error}
            holdings={state.holdings}
            brokers={state.brokers}
            manualHoldings={manualHoldings}
            finishing={finishing}
            finishError={finishError}
            onFinish={finishSetup}
            onBack={() => navigate("/setup")}
            onAddManual={(holding) => {
              setFinishError("");
              setManualHoldings((prev) => [...prev, holding]);
            }}
            onRemoveManual={(index) => {
              setFinishError("");
              setManualHoldings((prev) =>
                prev.filter((_, position) => position !== index),
              );
            }}
          />
        ))}
    </div>
  );
}
