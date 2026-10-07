import { useMemo } from 'react'
import { formatCurrency, formatNumber, formatPercent } from '../lib/formatters'
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

  if (!brokers || brokers.length === 0) {
    return <p className="mt-3 text-sm text-slate-500">Belum ada data broker.</p>
  }

  return (
    <div className="mt-3 grid grid-cols-1 gap-3 lg:grid-cols-2">
      {sortedBrokers.map((broker) => (
        <article key={broker.id} className="rounded-xl border border-slate-300 bg-white p-3">
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <h3 className="font-semibold">{broker.name}</h3>
              <div className="mt-2 grid gap-1 font-mono text-xs text-slate-600">
                <span>Total Value: {formatCurrency(broker.total_value)}</span>
                <span>Stock Value: {formatCurrency(broker.stock_value)}</span>
                <span>Cash: {formatCurrency(broker.cash)}</span>
                <span>Weight: {formatPercent(broker.weight)}</span>
              </div>
            </div>

            <section
              className="min-w-0 border-t border-slate-200 pt-3 sm:border-l sm:border-t-0 sm:pl-4 sm:pt-0"
              aria-label={`Sector Distribution — ${broker.name}`}
            >
              <h4 className="text-xs font-semibold text-slate-700">Sector Distribution</h4>
              <p className="mt-1 text-[11px] text-slate-500">Persentase nilai saham, tanpa cash</p>
              {broker.sectors.length === 0 ? (
                <p className="mt-3 text-xs text-slate-500">Belum ada holding bernilai positif.</p>
              ) : (
                <ul className="mt-3 space-y-3">
                  {broker.sectors.map((sector) => (
                    <li key={sector.sector} title={`${sector.sector}: ${formatCurrency(sector.value)}`}>
                      <div className="mb-1 flex items-start justify-between gap-2 text-[11px]">
                        <span className="min-w-0 break-words text-slate-600">{sector.sector}</span>
                        <span className="shrink-0 font-semibold tabular-nums text-slate-700">
                          {formatNumber(sector.weight, 1)}%
                        </span>
                      </div>
                      <div
                        role="meter"
                        aria-label={sector.sector}
                        aria-valuemin={0}
                        aria-valuemax={100}
                        aria-valuenow={sector.weight}
                        aria-valuetext={`${formatNumber(sector.weight, 1)}% — ${formatCurrency(sector.value)}`}
                        className="h-2 overflow-hidden rounded-full bg-slate-100"
                      >
                        <div
                          className={`h-full rounded-full ${sector.sector === UNCLASSIFIED_SECTOR ? 'bg-slate-400' : 'bg-teal-600'}`}
                          style={{ width: `${Math.max(0, Math.min(100, sector.weight))}%` }}
                        />
                      </div>
                    </li>
                  ))}
                </ul>
              )}
              {broker.sectors.some((sector) => sector.sector === UNCLASSIFIED_SECTOR) && (
                <p className="mt-3 text-[11px] text-slate-500">Sebagian data sektor belum tersedia.</p>
              )}
            </section>
          </div>
          <div className="mt-2 h-2 overflow-hidden rounded-full bg-slate-200">
            <span
              className="block h-full bg-gradient-to-r from-teal-700 to-emerald-500"
              style={{ width: `${Math.max(0, Math.min(100, broker.weight))}%` }}
            ></span>
          </div>
        </article>
      ))}
    </div>
  )
}
