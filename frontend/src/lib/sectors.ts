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
  const values = new Map<string, number>()

  for (const holding of holdings) {
    const price = prices.get(holding.ticker.toUpperCase()) ?? holding.avg_price
    const value = Number(holding.lot) * 100 * Number(price)
    if (!Number.isFinite(value) || value <= 0) continue

    const sector = holding.sector?.trim() || UNCLASSIFIED_SECTOR
    values.set(sector, (values.get(sector) ?? 0) + value)
  }

  const total = Array.from(values.values()).reduce((sum, value) => sum + value, 0)
  return Array.from(values, ([sector, value]) => ({
    sector,
    value,
    weight: (value / total) * 100,
  })).sort((a, b) => b.value - a.value || a.sector.localeCompare(b.sector))
}
