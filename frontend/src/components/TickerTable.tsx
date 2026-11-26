import { useEffect, useMemo, useState } from 'react'
import {
  formatCurrency,
  formatLot,
  formatPercent,
  formatPrice,
  parseNumberInput,
  plClass,
} from '../lib/formatters'
import type { PortfolioSummary, StockPriceMap, TickerSummary } from '../types'

interface TickerTableProps {
  prices: StockPriceMap
  summary: PortfolioSummary | null
  isEditing?: boolean
  resetKey?: number
  onHasEditsChange?: (hasEdits: boolean) => void
}

interface TickerDraft {
  lotStr: string
  priceStr: string
}

interface CalculatedRow {
  ticker: string
  original: TickerSummary
  lot: number
  avgPrice: number
  currentPrice: number
  shares: number
  investedValue: number
  marketValue: number
  profitLoss: number
  profitPct: number
  weight: number
}

const toNumber = (value: unknown): number => Number(value || 0)

export default function TickerTable({
  summary,
  prices,
  isEditing = false,
  resetKey = 0,
  onHasEditsChange,
}: TickerTableProps) {
  const rows = summary?.by_ticker || []

  // Simpan draft teks untuk inline input per ticker
  const [drafts, setDrafts] = useState<Record<string, TickerDraft>>({})

  // Reset saat tombol reset ditekan atau data summary berubah
  useEffect(() => {
    setDrafts({})
    onHasEditsChange?.(false)
  }, [resetKey, summary, onHasEditsChange])

  // Dapatkan harga pasar saat ini (statis, tidak dipengaruhi perubahan lot/avg price)
  const getBaseCurrentPrice = (row: TickerSummary): number => {
    const priceObj = prices[row.ticker]
    if (priceObj && Number.isFinite(Number(priceObj.price)) && Number(priceObj.price) > 0) {
      return Number(priceObj.price)
    }
    if (row.current_price != null && Number(row.current_price) > 0) {
      return toNumber(row.current_price)
    }
    const shares = toNumber(row.total_lot) * 100
    if (shares > 0 && row.market_value != null) {
      return toNumber(row.market_value) / shares
    }
    return toNumber(row.avg_price)
  }

  // Hitung ulang semua baris berdasarkan perubahan inline
  const calculatedRows: CalculatedRow[] = useMemo(() => {
    const tempRows = rows.map((row) => {
      const baseLot = toNumber(row.total_lot)
      const baseAvgPrice = toNumber(row.avg_price)
      const draft = drafts[row.ticker]

      const lot = draft !== undefined ? parseNumberInput(draft.lotStr) : baseLot
      const avgPrice = draft !== undefined ? parseNumberInput(draft.priceStr) : baseAvgPrice

      // Perhitungan lot (bisa lot setengah e.g. 0.5 lot = 50 lembar saham)
      const shares = lot * 100
      const currentPrice = getBaseCurrentPrice(row)
      const investedValue = shares * avgPrice
      const marketValue = shares * currentPrice
      const profitLoss = marketValue - investedValue
      const profitPct = investedValue > 0 ? (profitLoss / investedValue) * 100 : 0

      return {
        ticker: row.ticker,
        original: row,
        lot,
        avgPrice,
        currentPrice,
        shares,
        investedValue,
        marketValue,
        profitLoss,
        profitPct,
        weight: 0,
      }
    })

    // Hitung total nilai pasar untuk bobot (Weight)
    const totalMarketVal = tempRows.reduce((sum, r) => sum + r.marketValue, 0)

    return tempRows.map((r) => ({
      ...r,
      weight: totalMarketVal > 0 ? (r.marketValue / totalMarketVal) * 100 : 0,
    }))
  }, [rows, drafts, prices])

  // Supaya urutan baris tidak meloncat saat user mengetik dalam mode edit
  const displayRows = useMemo(() => {
    if (isEditing) {
      return calculatedRows
    }
    return [...calculatedRows].sort((a, b) => b.marketValue - a.marketValue)
  }, [calculatedRows, isEditing])

  const handleLotChange = (ticker: string, newLotStr: string, currentAvgPrice: number) => {
    setDrafts((prev) => {
      const existing = prev[ticker]
      return {
        ...prev,
        [ticker]: {
          lotStr: newLotStr,
          priceStr: existing?.priceStr ?? (currentAvgPrice ? formatPrice(currentAvgPrice) : ''),
        },
      }
    })
    onHasEditsChange?.(true)
  }

  const handlePriceChange = (ticker: string, newPriceStr: string, currentLot: number) => {
    setDrafts((prev) => {
      const existing = prev[ticker]
      return {
        ...prev,
        [ticker]: {
          lotStr: existing?.lotStr ?? (currentLot ? formatLot(currentLot) : ''),
          priceStr: newPriceStr,
        },
      }
    })
    onHasEditsChange?.(true)
  }

  const handleLotBlur = (ticker: string) => {
    setDrafts((prev) => {
      const existing = prev[ticker]
      if (!existing) return prev
      const parsed = parseNumberInput(existing.lotStr)
      return {
        ...prev,
        [ticker]: {
          ...existing,
          lotStr: formatLot(parsed),
        },
      }
    })
  }

  const handlePriceBlur = (ticker: string) => {
    setDrafts((prev) => {
      const existing = prev[ticker]
      if (!existing) return prev
      const parsed = parseNumberInput(existing.priceStr)
      return {
        ...prev,
        [ticker]: {
          ...existing,
          priceStr: formatPrice(parsed),
        },
      }
    })
  }

  return (
    <div className="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
      <table className="w-full min-w-[760px] border-collapse">
        <thead>
          <tr className="bg-slate-50 text-slate-500">
            <th className="border-b border-slate-300 p-2 text-center text-xs">Ticker</th>
            <th className="border-b border-slate-300 p-2 text-right text-xs">Total Lot</th>
            <th className="border-b border-slate-300 p-2 text-right text-xs">Avg Price</th>
            <th className="border-b border-slate-300 p-2 text-right text-xs">Current Price</th>
            <th className="border-b border-slate-300 p-2 text-right text-xs">Market Value</th>
            <th className="border-b border-slate-300 p-2 text-right text-xs">P/L</th>
            <th className="border-b border-slate-300 p-2 text-right text-xs">P/L %</th>
            <th className="border-b border-slate-300 p-2 text-right text-xs">Weight</th>
          </tr>
        </thead>
        <tbody>
          {!rows || rows.length === 0 ? (
            <tr>
              <td colSpan={8} className="p-3 text-center text-sm text-slate-500">
                Tidak ada data ticker.
              </td>
            </tr>
          ) : (
            displayRows.map((row) => {
              const draft = drafts[row.ticker]
              const baseLot = toNumber(row.original.total_lot)
              const baseAvgPrice = toNumber(row.original.avg_price)
              const priceObj = prices[row.ticker]

              return (
                <tr key={row.ticker} className={isEditing ? 'hover:bg-slate-50/70 transition-colors' : undefined}>
                  <td className="border-b border-slate-100 p-2 text-center text-sm font-semibold">
                    {row.ticker}
                  </td>
                  {/* Kolom Total Lot - Dapat Diedit */}
                  <td className="border-b border-slate-100 p-2 text-right text-sm">
                    {isEditing ? (
                      <input
                        type="text"
                        inputMode="decimal"
                        aria-label={`Total lot untuk ${row.ticker}`}
                        value={draft?.lotStr ?? (baseLot ? formatLot(baseLot) : '')}
                        onChange={(e) => handleLotChange(row.ticker, e.target.value, row.avgPrice)}
                        onBlur={() => handleLotBlur(row.ticker)}
                        className="w-24 rounded border border-slate-300 bg-white px-2 py-1 text-right text-sm text-slate-900 shadow-sm focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
                        placeholder="0"
                      />
                    ) : (
                      formatLot(row.lot)
                    )}
                  </td>
                  {/* Kolom Avg Price - Dapat Diedit (Format x.xxx,xx) */}
                  <td className="border-b border-slate-100 p-2 text-right text-sm">
                    {isEditing ? (
                      <input
                        type="text"
                        inputMode="decimal"
                        aria-label={`Avg price untuk ${row.ticker}`}
                        value={draft?.priceStr ?? (baseAvgPrice ? formatPrice(baseAvgPrice) : '')}
                        onChange={(e) => handlePriceChange(row.ticker, e.target.value, row.lot)}
                        onBlur={() => handlePriceBlur(row.ticker)}
                        className="w-28 rounded border border-slate-300 bg-white px-2 py-1 text-right text-sm text-slate-900 shadow-sm focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
                        placeholder="0,00"
                      />
                    ) : (
                      formatPrice(row.avgPrice)
                    )}
                  </td>
                  {/* Kolom Current Price - Statis */}
                  <td className="border-b border-slate-100 p-2 text-right text-sm">
                    {priceObj || row.currentPrice > 0 ? formatPrice(row.currentPrice) : '—'}
                  </td>
                  {/* Kolom Market Value - Terpengaruh Perubahan */}
                  <td className="border-b border-slate-100 p-2 text-right text-sm">
                    {formatCurrency(row.marketValue)}
                  </td>
                  {/* Kolom P/L - Terpengaruh Perubahan */}
                  <td className={`border-b border-slate-100 p-2 text-right text-sm ${plClass(row.profitLoss)}`}>
                    {formatCurrency(row.profitLoss)}
                  </td>
                  {/* Kolom P/L % - Terpengaruh Perubahan */}
                  <td className={`border-b border-slate-100 p-2 text-right text-sm ${plClass(row.profitPct)}`}>
                    {formatPercent(row.profitPct)}
                  </td>
                  {/* Kolom Weight - Terpengaruh Perubahan */}
                  <td className="border-b border-slate-100 p-2 text-right text-sm">
                    {formatPercent(row.weight)}
                  </td>
                </tr>
              )
            })
          )}
        </tbody>
      </table>
    </div>
  )
}
