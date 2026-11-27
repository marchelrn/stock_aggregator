import { Info } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { usePortfolio } from "../context/PortfolioContext";
import StatusBar from "../components/StatusBar";
import KpiGrid from "../components/KpiGrid";
import BrokerCards from "../components/BrokerCards";
import TickerTable from "../components/TickerTable";
import SectorAllocation from "../components/SectorAllocation";

export default function DashboardPage() {
  const { state, status, isError, loadDashboard, syncStockSectors } =
    usePortfolio();
  const [showPriceInfo, setShowPriceInfo] = useState(false);
  const priceInfoRef = useRef<HTMLDivElement>(null);
  const [syncingSectors, setSyncingSectors] = useState(false);
  const [isEditingTicker, setIsEditingTicker] = useState(false);
  const [hasTickerEdits, setHasTickerEdits] = useState(false);
  const [tickerResetKey, setTickerResetKey] = useState(0);

  const handleResetTicker = () => {
    setTickerResetKey((k) => k + 1);
    setHasTickerEdits(false);
  };

  useEffect(() => {
    loadDashboard();
  }, [loadDashboard]);

  useEffect(() => {
    if (!showPriceInfo) return;
    const dismissOutside = (event: PointerEvent) => {
      if (
        event.target instanceof Node &&
        !priceInfoRef.current?.contains(event.target)
      ) {
        setShowPriceInfo(false);
      }
    };
    const dismissOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setShowPriceInfo(false);
    };
    document.addEventListener("pointerdown", dismissOutside);
    document.addEventListener("keydown", dismissOnEscape);
    return () => {
      document.removeEventListener("pointerdown", dismissOutside);
      document.removeEventListener("keydown", dismissOnEscape);
    };
  }, [showPriceInfo]);

  // const handleSyncSectors = async () => {
  //   setSyncingSectors(true)
  //   await syncStockSectors()
  //   setSyncingSectors(false)
  // }
  //  NEXT IMPLEMENTATION

  const userName =
    state.user?.name ?? localStorage.getItem("userName") ?? undefined;

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-4 px-4">
      <header className="grid gap-4 rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur md:grid-cols-[1.4fr_1fr] md:items-center">
        <div>
          <h1 className="text-2xl font-bold">Portfolio Dashboard</h1>
          <p className="mt-2 text-sm text-slate-500">
            {userName ? (
              <>
                <strong className="font-bold text-slate-800">{userName}</strong>
                's Portfolio
              </>
            ) : (
              "Users Portfolio"
            )}
          </p>
        </div>
      </header>
      <section className="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
        <div className="flex items-center justify-between gap-2">
          <div className="relative flex items-center gap-1">
            <h2 className="text-xl font-semibold">Portfolio Summary</h2>
            <div
              ref={priceInfoRef}
              className="sm:relative"
              onPointerEnter={(event) => {
                if (event.pointerType === "mouse") setShowPriceInfo(true);
              }}
              onPointerLeave={(event) => {
                if (event.pointerType === "mouse") setShowPriceInfo(false);
              }}
            >
              <button
                type="button"
                aria-label="Informasi harga portofolio"
                aria-describedby={
                  showPriceInfo ? "portfolio-price-info" : undefined
                }
                className="grid h-8 w-8 shrink-0 place-items-center rounded-full text-slate-500 focus-visible:ring-2"
                onFocus={(event) => {
                  if (event.currentTarget.matches(":focus-visible"))
                    setShowPriceInfo(true);
                }}
                onBlur={() => setShowPriceInfo(false)}
                onClick={() => setShowPriceInfo(true)}
              >
                <Info size={15} aria-hidden="true" />
              </button>
              {showPriceInfo && (
                <div className="absolute left-0 top-full z-20 w-80 max-w-[calc(100vw-2rem)] pt-2">
                  <div
                    id="portfolio-price-info"
                    role="tooltip"
                    className="rounded-lg border border-slate-300 bg-white p-3.5 text-xs leading-relaxed text-slate-600 shadow-lg dark:border-neutral-700 dark:bg-neutral-900 dark:text-neutral-300"
                  >
                    <p className="mb-1.5 font-semibold text-slate-800 dark:text-neutral-100">
                      Informasi Harga Pasar
                    </p>
                    <ul className="list-disc space-y-1.5 pl-4">
                      <li>
                        Harga dari Yahoo Finance di-cache selama{" "}
                        <strong>5 menit</strong>.
                      </li>
                      <li>
                        Tombol <strong>Refresh Data</strong> dapat digunakan
                        kembali setelah jeda <strong>30 detik</strong>.
                      </li>
                      <li>
                        Jika akses Yahoo dibatasi, tabel akan menampilkan harga
                        terakhir yang tersedia.
                      </li>
                    </ul>
                  </div>
                </div>
              )}
            </div>
          </div>
          <button
            className="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800"
            onClick={() => loadDashboard({ refreshPrices: true })}
          >
            Refresh Data
          </button>
        </div>
        {/*<StatusBar message={status} isError={isError} /> FOR DEBBUG ONLY !*/}
        <KpiGrid summary={state.summary} />
      </section>

      <BrokerCards summary={state.summary} />

      <section className="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
        <div className="flex items-center justify-between gap-2">
          <h2 className="text-xl font-semibold">Sector Allocation</h2>
          {/*<button
            type="button"
            className="rounded-lg border border-teal-600 bg-teal-50 px-3 py-1.5 text-xs font-semibold text-teal-800 hover:bg-teal-100 disabled:opacity-50 transition"
            onClick={handleSyncSectors}
            disabled={syncingSectors}
          >
            {syncingSectors ? 'Menyinkronkan...' : 'Sync Sektor Yahoo'}
          </button>*/}
        </div>
        <SectorAllocation
          holdings={state.holdings}
          tickerSummaries={state.summary?.by_ticker ?? []}
        />
      </section>

      <section className="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
        <div className="flex items-center justify-between gap-2">
          <h2 className="text-xl font-semibold">Ticker Breakdown</h2>
          <div className="flex items-center gap-2">
            {isEditingTicker && hasTickerEdits && (
              <button
                type="button"
                onClick={handleResetTicker}
                className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 transition hover:bg-slate-100 focus-visible:ring-2 focus-visible:ring-slate-400"
              >
                Reset
              </button>
            )}
            <button
              type="button"
              onClick={() => setIsEditingTicker((prev) => !prev)}
              className={`rounded-lg px-3 py-1.5 text-xs font-semibold transition focus-visible:ring-2 focus-visible:ring-teal-500 ${
                isEditingTicker
                  ? "bg-teal-700 text-white hover:bg-teal-800"
                  : "border border-teal-600 bg-teal-50 text-teal-800 hover:bg-teal-100"
              }`}
            >
              {isEditingTicker ? "Selesai" : "Edit"}
            </button>
          </div>
        </div>
        <TickerTable
          prices={state.stockPrices}
          summary={state.summary}
          isEditing={isEditingTicker}
          resetKey={tickerResetKey}
          onHasEditsChange={setHasTickerEdits}
        />
      </section>
    </div>
  );
}
