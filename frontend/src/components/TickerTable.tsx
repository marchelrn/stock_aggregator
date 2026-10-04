import { useMemo } from 'react'
import { formatCurrency, formatNumber, formatPercent, plClass } from '../lib/formatters'
import type { PortfolioSummary, TickerSummary } from '../types'

interface TickerTableProps {
  summary: PortfolioSummary | null
}

const toNumber = (value: unknown): number => Number(value || 0)
const getShares = (row: TickerSummary): number => toNumber(row.total_lot) * 100

const getCurrentPrice = (row: TickerSummary): number => {
  if (row.current_price != null) return toNumber(row.current_price)
  const shares = getShares(row)
  if (shares > 0 && row.market_value != null) return toNumber(row.market_value) / shares
  return toNumber(row.avg_price)
}

const getInvestedValue = (row: TickerSummary): number => {
  if (row.invested_value != null) return toNumber(row.invested_value)
  return getShares(row) * toNumber(row.avg_price)
}

const getMarketValue = (row: TickerSummary): number => {
  if (row.current_price != null) return getShares(row) * getCurrentPrice(row)
  if (row.market_value != null) return toNumber(row.market_value)
  return getInvestedValue(row)
}

const getProfitLoss = (row: TickerSummary): number => getMarketValue(row) - getInvestedValue(row)

const getProfitPct = (row: TickerSummary): number => {
  const invested = getInvestedValue(row)
  if (invested <= 0) return 0
  return (getProfitLoss(row) / invested) * 100
}

export default function TickerTable({ summary }: TickerTableProps) {
  const rows = summary?.by_ticker || []

  const sortedRows = useMemo(
    () => [...rows].sort((a, b) => getMarketValue(b) - getMarketValue(a)),
    [rows],
  )

  const getWeight = (row: TickerSummary): number => {
    const totalStockValue = toNumber(summary?.total_stock_value)
    if (totalStockValue <= 0) return 0
    return (getMarketValue(row) / totalStockValue) * 100
  }

  return (
    <div className="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
      <table className="w-full min-w-[760px] border-collapse">
        <thead>
          <tr className="bg-slate-50 text-slate-500">
            <th className="border-b border-slate-300 p-2 text-left text-xs">Ticker</th>
            <th className="border-b border-slate-300 p-2 text-left text-xs">Total Lot</th>
            <th className="border-b border-slate-300 p-2 text-left text-xs">Avg Price</th>
            <th className="border-b border-slate-300 p-2 text-left text-xs">Current Price</th>
            <th className="border-b border-slate-300 p-2 text-left text-xs">Market Value</th>
            <th className="border-b border-slate-300 p-2 text-left text-xs">P/L</th>
            <th className="border-b border-slate-300 p-2 text-left text-xs">P/L %</th>
            <th className="border-b border-slate-300 p-2 text-left text-xs">Weight</th>
          </tr>
        </thead>
        <tbody>
          {!rows || rows.length === 0 ? (
            <tr>
              <td colSpan={12} className="p-3 text-center text-sm text-slate-500">
                Tidak ada data ticker.
              </td>
            </tr>
          ) : (
            sortedRows.map((row) => (
              <tr key={row.ticker}>
                <td className="border-b border-slate-100 p-2 text-sm font-semibold">{row.ticker}</td>
                <td className="border-b border-slate-100 p-2 text-sm">
                  {formatNumber(row.total_lot, 0)}
                </td>
                <td className="border-b border-slate-100 p-2 text-sm">
                  {formatCurrency(row.avg_price)}
                </td>
                <td className="border-b border-slate-100 p-2 text-sm">
                  {formatCurrency(getCurrentPrice(row))}
                </td>
                <td className="border-b border-slate-100 p-2 text-sm">
                  {formatCurrency(getMarketValue(row))}
                </td>
                <td className={`border-b border-slate-100 text-sm ${plClass(getProfitLoss(row))}`}>
                  {formatCurrency(getProfitLoss(row))}
                </td>
                <td className={`border-b border-slate-100 p-2 text-sm ${plClass(getProfitPct(row))}`}>
                  {formatPercent(getProfitPct(row))}
                </td>
                <td className="border-b border-slate-100 p-2 text-sm">
                  {formatPercent(getWeight(row))}
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}
