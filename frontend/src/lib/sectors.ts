import type { Holding, TickerSummary } from '../types'

export const UNCLASSIFIED_SECTOR = 'Belum diklasifikasikan'

export interface SectorDistribution {
  sector: string
  value: number
  weight: number
}

// Gunakan harga yang sama dengan summary dashboard, termasuk fallback avg_price.
export function buildSectorDistribution(
  holdings: Holding[],
  tickers: TickerSummary[],
): SectorDistribution[] {
  const prices = new Map(tickers.map((ticker) => [ticker.ticker.toUpperCase(), ticker.current_price]))
  const tickerSectors = new Map(tickers.map((ticker) => [ticker.ticker.toUpperCase(), ticker.sector]))
  const values = new Map<string, number>()

  for (const holding of holdings) {
    const tickerKey = (holding.ticker || '').toUpperCase()
    if (!tickerKey) continue

    const price = prices.get(tickerKey) ?? holding.avg_price
    const value = Number(holding.lot) * 100 * Number(price)
    if (!Number.isFinite(value) || value <= 0) continue

    const sector = holding.sector?.trim() || tickerSectors.get(tickerKey)?.trim() || UNCLASSIFIED_SECTOR
    values.set(sector, (values.get(sector) ?? 0) + value)
  }

  const total = Array.from(values.values()).reduce((sum, value) => sum + value, 0)
  return Array.from(values, ([sector, value]) => ({
    sector,
    value,
    weight: total > 0 ? (value / total) * 100 : 0,
  })).sort((a, b) => b.value - a.value || a.sector.localeCompare(b.sector))
}
