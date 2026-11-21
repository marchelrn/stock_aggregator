import { Info } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { usePortfolio } from '../context/PortfolioContext'
import StatusBar from '../components/StatusBar'
import KpiGrid from '../components/KpiGrid'
import BrokerCards from '../components/BrokerCards'
import TickerTable from '../components/TickerTable'
import SectorAllocation from '../components/SectorAllocation'

export default function DashboardPage() {
  const { state, status, isError, loadDashboard, syncStockSectors } = usePortfolio()
  const [showPriceInfo, setShowPriceInfo] = useState(false)
  const priceInfoRef = useRef<HTMLDivElement>(null)
  const [syncingSectors, setSyncingSectors] = useState(false)

  useEffect(() => {
    loadDashboard()
  }, [loadDashboard])

  useEffect(() => {
    if (!showPriceInfo) return
    const dismissOutside = (event: PointerEvent) => {
      if (event.target instanceof Node && !priceInfoRef.current?.contains(event.target)) {
        setShowPriceInfo(false)
      }
    }
    const dismissOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setShowPriceInfo(false)
    }
    document.addEventListener('pointerdown', dismissOutside)
    document.addEventListener('keydown', dismissOnEscape)
    return () => {
      document.removeEventListener('pointerdown', dismissOutside)
      document.removeEventListener('keydown', dismissOnEscape)
    }
  }, [showPriceInfo])

  // const handleSyncSectors = async () => {
  //   setSyncingSectors(true)
  //   await syncStockSectors()
  //   setSyncingSectors(false)
  // }
  //  NEXT IMPLEMENTATION

  const userName = state.user?.name ?? localStorage.getItem('userName') ?? undefined

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-4 px-4">
      <header className="grid gap-4 rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur md:grid-cols-[1.4fr_1fr] md:items-center">
        <div>
          <h1 className="text-2xl font-bold">Portfolio Dashboard</h1>
          <p className="mt-2 text-sm text-slate-500">
                      {userName ? (
                        <>
                          <strong className="font-bold text-slate-800">{userName}</strong>'s Portfolio
                        </>
                      ) : (
                        'Users Portfolio'
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
                if (event.pointerType === 'mouse') setShowPriceInfo(true)
              }}
              onPointerLeave={(event) => {
                if (event.pointerType === 'mouse') setShowPriceInfo(false)
              }}
            >
              <button
                type="button"
                aria-label="Informasi harga portofolio"
                aria-describedby={showPriceInfo ? 'portfolio-price-info' : undefined}
                className="grid h-8 w-8 shrink-0 place-items-center rounded-full text-slate-500 focus-visible:ring-2"
                onFocus={(event) => {
                  if (event.currentTarget.matches(':focus-visible')) setShowPriceInfo(true)
                }}
                onBlur={() => setShowPriceInfo(false)}
                onClick={() => setShowPriceInfo(true)}
              >
                <Info size={15} aria-hidden="true" />
              </button>
              {showPriceInfo && (
                <div className="absolute left-0 top-full z-20 w-64 max-w-[calc(100vw-4rem)] pt-1">
                  <p id="portfolio-price-info" role="tooltip" className="rounded-lg border border-slate-300 bg-white p-3 text-sm text-slate-600 shadow-md">
                    Harga dari Yahoo Finance disimpan selama 5 menit. Refresh Data bisa memeriksa harga
                    lagi setelah 30 detik. Saat Yahoo membatasi akses, tabel menampilkan harga terakhir yang tersedia.
                  </p>
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

      <section className="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
        <div className="flex items-center justify-between gap-2">
          <h2 className="text-xl font-semibold">Broker Allocation</h2>
        </div>
        <BrokerCards summary={state.summary} />
      </section>

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
        <SectorAllocation holdings={state.holdings} tickerSummaries={state.summary?.by_ticker ?? []} />
      </section>

      <section className="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
        <div className="flex items-center justify-between gap-2">
          <h2 className="text-xl font-semibold">Ticker Breakdown</h2>
        </div>
        <TickerTable prices={state.stockPrices} summary={state.summary} />
      </section>
    </div>
  )
}
