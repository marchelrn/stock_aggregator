import { useMemo } from 'react'
import { formatCurrency, formatNumber } from '../lib/formatters'
import { buildSectorDistribution, UNCLASSIFIED_SECTOR } from '../lib/sectors'
import type { PortfolioSummary } from '../types'

interface BrokerCardsProps {
  summary: PortfolioSummary | null
}

export default function BrokerCards({ summary }: BrokerCardsProps) {
  const brokers = summary?.brokers || []
  const sortedBrokers = useMemo(
    () => [...brokers].sort((a, b) => b.total_value - a.total_value).map((broker) => ({
      ...broker,
      sectors: buildSectorDistribution(broker.holdings, summary?.by_ticker ?? []),
    })),
    [brokers, summary?.by_ticker],
  )

  if (brokers.length === 0) {
    return <p className="mt-4 text-sm text-slate-600">Belum ada data broker.</p>
  }

  return (
    <div className="mt-4 grid grid-cols-1 items-stretch gap-4 md:auto-rows-fr md:grid-cols-2 xl:grid-cols-3">
      {sortedBrokers.map((broker) => (
        <article key={broker.id} className="flex min-w-0 flex-col rounded-xl border border-slate-300 bg-white p-5">
          <header className="min-h-12">
            <h3 className="break-words text-base font-semibold leading-6 text-slate-800">{broker.name}</h3>
          </header>

          <dl className="grid min-w-0 grid-cols-2 gap-x-4 gap-y-5 pb-5">
            <div className="col-span-2 min-w-0">
              <dt className="text-xs text-slate-600">Total nilai · IDR</dt>
              <dd className="mt-1.5 break-words text-2xl font-semibold leading-tight tracking-tight tabular-nums text-slate-900">{formatCurrency(broker.total_value)}</dd>
            </div>
            <div className="min-w-0">
              <dt className="text-xs text-slate-600">Nilai saham</dt>
              <dd className="mt-1.5 break-words text-sm font-medium tabular-nums text-slate-800">{formatCurrency(broker.stock_value)}</dd>
            </div>
            <div className="min-w-0">
              <dt className="text-xs text-slate-600">Saldo kas</dt>
              <dd className="mt-1.5 break-words text-sm font-medium tabular-nums text-slate-800">{formatCurrency(broker.cash)}</dd>
            </div>
          </dl>

          <section className="flex flex-1 flex-col border-t border-slate-200 pt-5" aria-label={`Distribusi sektor: ${broker.name}`}>
            <h4 className="text-xs font-semibold text-slate-700">Distribusi sektor</h4>
            {broker.sectors.length === 0 ? (
              <p className="py-8 text-sm text-slate-600">Belum ada saham.</p>
            ) : (
              <ul className="mt-4 grid content-start gap-4">
                {broker.sectors.map((sector) => (
                  <li key={sector.sector} title={`${sector.sector}: ${formatCurrency(sector.value)}`}>
                    <div className="mb-1.5 flex items-start justify-between gap-3 text-xs leading-5">
                      <span className="min-w-0 break-words text-slate-600">{sector.sector}</span>
                      <span className="shrink-0 font-medium tabular-nums text-slate-800">{formatNumber(sector.weight, 1)}%</span>
                    </div>
                    <div role="meter" aria-label={sector.sector} aria-valuemin={0} aria-valuemax={100} aria-valuenow={sector.weight} aria-valuetext={`${formatNumber(sector.weight, 1)}%, ${formatCurrency(sector.value)}`} className="h-1.5 overflow-hidden rounded-full bg-slate-100">
                      <div className={`h-full rounded-full ${sector.sector === UNCLASSIFIED_SECTOR ? 'bg-slate-500' : 'bg-teal-700'}`} style={{ width: `${Math.max(0, Math.min(100, sector.weight))}%` }} />
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <footer className="mt-6 border-t border-slate-200 pt-4">
            <div className="mb-2 flex items-baseline justify-between gap-3 text-xs">
              <span className="text-slate-600">Alokasi portofolio</span>
              <span className="font-semibold tabular-nums text-slate-800">{formatNumber(broker.weight, 2)}%</span>
            </div>
            <div role="meter" aria-label={`Alokasi ${broker.name}`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={broker.weight} aria-valuetext={`${formatNumber(broker.weight, 2)}% dari portofolio`} className="h-2 overflow-hidden rounded-full bg-slate-100">
              <div className="h-full rounded-full bg-teal-700" style={{ width: `${Math.max(0, Math.min(100, broker.weight))}%` }} />
            </div>
          </footer>
        </article>
      ))}
    </div>
  )
}
