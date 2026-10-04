import { useMemo } from 'react'
import { formatCurrency, formatPercent, plClass } from '../lib/formatters'
import type { PortfolioSummary } from '../types'

interface KpiGridProps {
  summary: PortfolioSummary | null
}

export default function KpiGrid({ summary }: KpiGridProps) {
  const kpiItems = useMemo(() => {
    if (!summary) return []
    return [
      { label: 'Total Portfolio', value: formatCurrency(summary.total_portfolio_value), class: '' },
      { label: 'Total Stock Value', value: formatCurrency(summary.total_stock_value), class: '' },
      { label: 'Total Cash', value: formatCurrency(summary.total_cash), class: '' },
      { label: 'Total Invested', value: formatCurrency(summary.total_invested_value), class: '' },
      {
        label: 'Total P/L',
        value: formatCurrency(summary.total_profit_loss),
        class: plClass(summary.total_profit_loss),
      },
      {
        label: 'Total P/L %',
        value: formatPercent(summary.total_profit_pct),
        class: plClass(summary.total_profit_pct),
      },
    ]
  }, [summary])

  if (!summary) return null

  return (
    <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {kpiItems.map((item) => (
        <article key={item.label} className="rounded-xl border border-slate-300 bg-white p-3">
          <label className="block text-xs text-slate-500">{item.label}</label>
          <strong className={`mt-1 block text-base ${item.class}`}>{item.value}</strong>
        </article>
      ))}
    </div>
  )
}
