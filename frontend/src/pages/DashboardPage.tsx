import { useEffect, useState } from 'react'
import { usePortfolio } from '../context/PortfolioContext'
import StatusBar from '../components/StatusBar'
import KpiGrid from '../components/KpiGrid'
import BrokerCards from '../components/BrokerCards'
import TickerTable from '../components/TickerTable'
import SectorAllocation from '../components/SectorAllocation'

export default function DashboardPage() {
  const { state, status, isError, loadDashboard, syncStockSectors } = usePortfolio()
  const [syncingSectors, setSyncingSectors] = useState(false)

  useEffect(() => {
    loadDashboard()
  }, [loadDashboard])

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
          <h2 className="text-xl font-semibold">Portfolio Summary</h2>
          <button
            className="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800"
            onClick={() => loadDashboard()}
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
        <TickerTable summary={state.summary} />
      </section>
    </div>
  )
}
