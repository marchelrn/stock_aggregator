import { useMemo, useState } from 'react'
import { formatNumber, formatPercent, plClass } from '../lib/formatters'
import type { StockPriceMap } from '../types'

interface StockPricesProps {
  prices?: StockPriceMap
  onFetch?: (tickers: string) => void
  onClear?: () => void
}

export default function StockPrices({ prices = {}, onFetch, onClear }: StockPricesProps) {
  const [tickers, setTickers] = useState('')

  const sortedEntries = useMemo(() => {
    const entries = Object.values(prices || {})
    return [...entries].sort((a, b) => a.ticker.localeCompare(b.ticker))
  }, [prices])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const tickerList = tickers
      .split(',')
      .map((t) => t.trim().toUpperCase())
      .filter(Boolean)
      .join(',')

    if (tickerList) {
      onFetch?.(tickerList)
    }
  }

  const handleClear = () => {
    setTickers('')
    onClear?.()
  }

  return (
    <div>
      <form className="mt-3 flex flex-col gap-2 sm:flex-row" onSubmit={handleSubmit}>
        <input
          value={tickers}
          onChange={(e) => setTickers(e.target.value)}
          type="text"
          placeholder="Search By Stocks"
          required
          className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
        />
        <button
          type="submit"
          className="rounded-lg bg-teal-700 px-2 py-2 text-xs font-semibold text-white hover:bg-teal-800"
        >
          Get Prices
        </button>
        <button
          type="button"
          className="rounded-lg bg-slate-500 px-3 py-2 text-sm font-semibold text-white hover:bg-slate-600"
          onClick={handleClear}
        >
          Clear
        </button>
      </form>
      <div className="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
        <table className="w-full min-w-[560px] border-collapse">
          <thead>
            <tr className="bg-slate-50 text-slate-500">
              <th className="border-b border-slate-300 p-2 text-left text-xs">Ticker</th>
              <th className="border-b border-slate-300 p-2 text-left text-xs">Price</th>
              <th className="border-b border-slate-300 p-2 text-left text-xs">Prev Close</th>
              <th className="border-b border-slate-300 p-2 text-left text-xs">Change</th>
              <th className="border-b border-slate-300 p-2 text-left text-xs">Change %</th>
              <th className="border-b border-slate-300 p-2 text-left text-xs">Currency</th>
            </tr>
          </thead>
          <tbody>
            {sortedEntries.length === 0 ? (
              <tr>
                <td colSpan={6} className="p-3 text-sm text-slate-500">
                  Tidak ada data harga.
                </td>
              </tr>
            ) : (
              sortedEntries.map((price) => (
                <tr key={price.ticker}>
                  <td className="border-b border-slate-100 p-2 text-sm font-semibold">
                    {price.ticker}
                  </td>
                  <td className="border-b border-slate-100 p-2 text-sm">
                    {formatNumber(price.price, 2)}
                  </td>
                  <td className="border-b border-slate-100 p-2 text-sm">
                    {formatNumber(price.previous_close, 2)}
                  </td>
                  <td className={`border-b border-slate-100 p-2 text-sm ${plClass(price.change)}`}>
                    {formatNumber(price.change, 2)}
                  </td>
                  <td
                    className={`border-b border-slate-100 p-2 text-sm ${plClass(price.change_percent)}`}
                  >
                    {formatPercent(price.change_percent)}
                  </td>
                  <td className="border-b border-slate-100 p-2 text-sm">{price.currency}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
