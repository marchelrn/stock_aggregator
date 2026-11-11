  import { formatCurrency, formatNumber, formatPercent } from '../lib/formatters'
import type { Holding, StockPriceMap, TickerSummary } from '../types'

interface HoldingTableProps {
  holdings?: Holding[]
  stockPrices?: StockPriceMap
  tickerSummaries?: TickerSummary[]
  totalPortfolioValue?: number
}

export default function HoldingTable({
  holdings = [],
  stockPrices = {},
  tickerSummaries = [],
  totalPortfolioValue = 0
}: HoldingTableProps) {
  // Create a map for quick ticker summary lookup
  const summaryMap = new Map<string, TickerSummary>()
  tickerSummaries.forEach(ts => summaryMap.set(ts.ticker.toUpperCase(), ts))

  const getStockPrice = (ticker: string): number => {
    const price = stockPrices[ticker.toUpperCase()]
    return price?.price ?? 0
  }

  const getMarketValue = (holding: Holding): number => {
    // First try to use ticker summary which has precomputed market_value
    const summary = summaryMap.get(holding.ticker.toUpperCase())
    if (summary) return summary.market_value
    // Fallback: compute from current price
    const currentPrice = getStockPrice(holding.ticker)
    return Number(holding.lot || 0) * 100 * currentPrice
  }

  const getWeight = (marketValue: number): number => {
    if (!totalPortfolioValue || totalPortfolioValue <= 0) return 0
    return (marketValue / totalPortfolioValue) * 100
  }

  return (
    <div className="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
      <table className="w-full min-w-[620px] border-collapse">
        <thead>
          <tr className="bg-slate-50 text-slate-500">
            <th className="w-[12%] border-b border-slate-300 p-2 text-center text-xs">Ticker</th>
            <th className="w-[12%] border-b border-slate-300 p-2 text-center text-xs">Broker</th>
            <th className="w-[12%] border-b border-slate-300 p-2 text-center text-xs">Sector</th>
            <th className="w-[12%] border-b border-slate-300 p-2 text-center text-xs">Market Value</th>
            <th className="w-[12%] border-b border-slate-300 p-2 text-center text-xs">Weight (%)</th>
          </tr>
        </thead>
        <tbody>
          {!holdings || holdings.length === 0 ? (
            <tr>
              <td colSpan={5} className="p-2 text-center text-sm text-slate-500">
                Pilih broker untuk melihat holding.
              </td>
            </tr>
          ) : (
            holdings.map((holding) => {
              const marketValue = getMarketValue(holding)
              const weight = getWeight(marketValue)
              return (
                <tr key={holding.id || `${holding.ticker}-${holding.broker_id}`}>
                  <td className="border-b border-slate-100 p-2 text-sm text-center font-semibold">
                    {holding.ticker}
                  </td>
                  <td className="border-b border-slate-100 text-sm text-center">
                    {holding.broker_name}
                  </td>
                  <td className="border-b border-slate-100 p-2 text-sm text-center">
                    {holding.sector || summaryMap.get(holding.ticker.toUpperCase())?.sector || '-'}
                  </td>
                  <td className="border-b border-slate-100 p-2 text-sm text-center">
                    {formatCurrency(marketValue)}
                  </td>
                  <td className="border-b border-slate-100 p-2 text-sm text-center">
                    {formatNumber(weight, 2)}%
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
