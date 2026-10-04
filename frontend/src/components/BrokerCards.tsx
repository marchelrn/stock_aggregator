import { useMemo } from 'react'
import { formatCurrency, formatPercent } from '../lib/formatters'
import type { PortfolioSummary } from '../types'

interface BrokerCardsProps {
  summary: PortfolioSummary | null
}

export default function BrokerCards({ summary }: BrokerCardsProps) {
  const brokers = summary?.brokers || []

  const sortedBrokers = useMemo(
    () => [...brokers].sort((a, b) => b.total_value - a.total_value),
    [brokers],
  )

  if (!brokers || brokers.length === 0) {
    return <p className="mt-3 text-sm text-slate-500">Belum ada data broker.</p>
  }

  return (
    <div className="mt-3 grid grid-cols-1 gap-3 lg:grid-cols-2">
      {sortedBrokers.map((broker) => (
        <article key={broker.id} className="rounded-xl border border-slate-300 bg-white p-3">
          <h3 className="font-semibold">{broker.name}</h3>
          <div className="mt-2 grid gap-1 font-mono text-xs text-slate-600">
            <span>Total Value: {formatCurrency(broker.total_value)}</span>
            <span>Stock Value: {formatCurrency(broker.stock_value)}</span>
            <span>Cash: {formatCurrency(broker.cash)}</span>
            <span>Weight: {formatPercent(broker.weight)}</span>
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
