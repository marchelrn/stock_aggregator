import type { Holding, StockPrice, StockPriceMap } from '../types'

export interface WatchlistRow {
  ticker: string
  lot: number
  brokers: string[]
  quote: StockPrice | null
}

export function buildWatchlist(holdings: Holding[], prices: StockPriceMap): WatchlistRow[] {
  const rows = new Map<string, WatchlistRow>()
  for (const holding of holdings) {
    const ticker = holding.ticker.trim().toUpperCase()
    const lot = Number(holding.lot)
    if (!ticker || !Number.isFinite(lot) || lot <= 0) continue
    const row = rows.get(ticker) ?? { ticker, lot: 0, brokers: [], quote: prices[ticker] ?? null }
    row.lot += lot
    if (holding.broker_name && !row.brokers.includes(holding.broker_name)) {
      row.brokers.push(holding.broker_name)
    }
    rows.set(ticker, row)
  }
  return [...rows.values()].sort((a, b) => a.ticker.localeCompare(b.ticker))
}

export function getWatchlistMovers(rows: WatchlistRow[]) {
  const available = rows.filter((row) => row.quote && Number.isFinite(row.quote.change_percent))
  return {
    gainers: available.filter((row) => row.quote!.change_percent > 0)
      .sort((a, b) => b.quote!.change_percent - a.quote!.change_percent).slice(0, 3),
    losers: available.filter((row) => row.quote!.change_percent < 0)
      .sort((a, b) => a.quote!.change_percent - b.quote!.change_percent).slice(0, 3),
    unchanged: available.filter((row) => row.quote!.change_percent === 0).length,
    unavailable: rows.length - available.length,
  }
}

export function marketTime(value: string | number | null | undefined): string {
  if (value == null) return 'Tidak tersedia'
  const date = new Date(typeof value === 'number' ? value * 1000 : value)
  if (!Number.isFinite(date.getTime())) return 'Tidak tersedia'
  return date.toLocaleString('id-ID', { timeZone: 'Asia/Jakarta', dateStyle: 'medium', timeStyle: 'short' }) + ' WIB'
}
