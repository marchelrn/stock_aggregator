import { useMemo } from 'react'
import { formatCurrency, formatNumber } from '../lib/formatters'
import { buildSectorDistribution, UNCLASSIFIED_SECTOR } from '../lib/sectors'
import type { Holding, TickerSummary } from '../types'

interface SectorAllocationProps {
  holdings?: Holding[]
  tickerSummaries?: TickerSummary[]
}

const SECTOR_COLORS: Record<string, string> = {
  'Financial Services': 'bg-blue-600',
  'Consumer Cyclical': 'bg-amber-500',
  'Consumer Defensive': 'bg-emerald-600',
  'Technology': 'bg-indigo-600',
  'Energy': 'bg-orange-600',
  'Basic Materials': 'bg-yellow-600',
  'Industrials': 'bg-cyan-600',
  'Utilities': 'bg-teal-600',
  'Healthcare': 'bg-rose-600',
  'Real Estate': 'bg-purple-600',
  'Communication Services': 'bg-sky-600',
}

export default function SectorAllocation({ holdings = [], tickerSummaries = [] }: SectorAllocationProps) {
  const sectors = useMemo(
    () => buildSectorDistribution(holdings, tickerSummaries),
    [holdings, tickerSummaries],
  )

  if (!sectors || sectors.length === 0) {
    return <p className="mt-3 text-sm text-slate-500">Belum ada alokasi sektor.</p>
  }

  const getBarColor = (sectorName: string): string => {
    if (sectorName === UNCLASSIFIED_SECTOR) return 'bg-slate-400'
    return SECTOR_COLORS[sectorName] || 'bg-teal-600'
  }

  return (
    <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {sectors.map((sec) => (
        <article
          key={sec.sector}
          className="rounded-xl border border-slate-200 bg-white p-3 shadow-xs transition hover:border-slate-300"
        >
          <div className="flex items-start justify-between gap-2">
            <span className="font-semibold text-sm text-slate-800 break-words">{sec.sector}</span>
            <span className="shrink-0 font-bold text-sm text-teal-800 tabular-nums">
              {formatNumber(sec.weight, 1)}%
            </span>
          </div>
          <p className="mt-1 text-xs text-slate-500 font-mono">{formatCurrency(sec.value)}</p>
          <div
            role="meter"
            aria-label={sec.sector}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={sec.weight}
            aria-valuetext={`${formatNumber(sec.weight, 1)}% — ${formatCurrency(sec.value)}`}
            className="mt-3 h-2 overflow-hidden rounded-full bg-slate-100"
          >
            <div
              className={`h-full rounded-full transition-all duration-300 ${getBarColor(sec.sector)}`}
              style={{ width: `${Math.max(0, Math.min(100, sec.weight))}%` }}
            />
          </div>
        </article>
      ))}
    </div>
  )
}
