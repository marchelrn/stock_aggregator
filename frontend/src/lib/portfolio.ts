// Pure portfolio aggregation helpers — ported from the original usePortfolio composable.
import type {
  Broker,
  BrokerSummary,
  Holding,
  PortfolioSummary,
  StockPriceMap,
  TickerSummary,
} from '../types'

const num = (v: unknown): number => Number(v || 0)

export function normalizeArray<T = any>(res: any): T[] {
  if (!res) return []
  if (Array.isArray(res)) return res

  if (Array.isArray(res.data)) return res.data
  if (Array.isArray(res.stocks_data)) return res.stocks_data
  if (Array.isArray(res.brokers_data)) return res.brokers_data

  if (Array.isArray(res?.data?.data)) return res.data.data
  if (Array.isArray(res?.data?.brokers)) return res.data.brokers
  if (Array.isArray(res?.data?.brokers_data?.brokers)) return res.data.brokers_data.brokers
  if (Array.isArray(res?.brokers_data?.brokers)) return res.brokers_data.brokers

  return []
}

export function buildSummary(
  brokers: Broker[] = [],
  holdings: Holding[] = [],
  priceMap: StockPriceMap = {},
): PortfolioSummary {
  const getCurrentPrice = (ticker: unknown, fallbackPrice: unknown): number => {
    const key = String(ticker || '').toUpperCase()
    const priceRow = priceMap?.[key]
    if (priceRow && priceRow.price != null) return num(priceRow.price)
    return num(fallbackPrice)
  }

  const tickerMap = new Map<string, TickerSummary>()

  for (const h of holdings) {
    const ticker = String(h.ticker || '').toUpperCase()
    if (!ticker) continue

    const prev: TickerSummary =
      tickerMap.get(ticker) || {
        ticker,
        total_lot: 0,
        total_shares: 0,
        total_cost: 0,
        avg_price: 0,
        current_price: 0,
        market_value: 0,
        invested_value: 0,
        profit_loss: 0,
        profit_pct: 0,
        weight: 0,
      }

    const shares = num(h.lot) * 100
    const avgPrice = num(h.avg_price)
    const currentPrice = getCurrentPrice(ticker, avgPrice)

    prev.total_lot += num(h.lot)
    prev.total_shares += shares
    prev.total_cost += shares * avgPrice
    prev.market_value += shares * currentPrice

    tickerMap.set(ticker, prev)
  }

  const byTicker: TickerSummary[] = Array.from(tickerMap.values()).map((t) => {
    const invested = t.total_cost
    const market = t.market_value
    const avgPrice = t.total_shares > 0 ? invested / t.total_shares : 0
    const currentPrice = t.total_shares > 0 ? market / t.total_shares : avgPrice
    const pl = market - invested

    return {
      ...t,
      avg_price: avgPrice,
      current_price: currentPrice,
      invested_value: invested,
      market_value: market,
      profit_loss: pl,
      profit_pct: invested > 0 ? (pl / invested) * 100 : 0,
    }
  })

  const brokerMap = new Map<number, BrokerSummary>()

  for (const b of brokers) {
    brokerMap.set(num(b.id), {
      id: b.id,
      name: b.name,
      cash: num(b.cash),
      stock_value: 0,
      total_value: num(b.cash),
      holdings: [],
      weight: 0,
    })
  }

  for (const h of holdings) {
    const brokerId = num(h.broker_id)
    if (!brokerMap.has(brokerId)) {
      brokerMap.set(brokerId, {
        id: brokerId,
        name: h.broker_name || `Broker ${brokerId}`,
        cash: 0,
        stock_value: 0,
        total_value: 0,
        holdings: [],
        weight: 0,
      })
    }

    const broker = brokerMap.get(brokerId)!
    const shares = num(h.lot) * 100
    const currentPrice = getCurrentPrice(h.ticker, h.avg_price)
    const marketValue = shares * currentPrice

    broker.stock_value += marketValue
    broker.total_value = broker.cash + broker.stock_value
    broker.holdings.push(h)
  }

  const brokersOut = Array.from(brokerMap.values())

  const totalCash = brokersOut.reduce((sum, b) => sum + num(b.cash), 0)
  const totalStockValue = byTicker.reduce((sum, t) => sum + num(t.market_value), 0)
  const totalInvestedValue = byTicker.reduce((sum, t) => sum + num(t.invested_value), 0)
  const totalPortfolioValue = totalCash + totalStockValue
  const totalProfitLoss = totalStockValue - totalInvestedValue
  const totalProfitPct = totalInvestedValue > 0 ? (totalProfitLoss / totalInvestedValue) * 100 : 0

  const byTickerWithWeight = byTicker.map((t) => ({
    ...t,
    weight: totalStockValue > 0 ? (num(t.market_value) / totalStockValue) * 100 : 0,
  }))

  const enrichedBrokers = brokersOut.map((b) => ({
    ...b,
    weight: totalPortfolioValue > 0 ? (num(b.total_value) / totalPortfolioValue) * 100 : 0,
  }))

  return {
    total_portfolio_value: totalPortfolioValue,
    total_stock_value: totalStockValue,
    total_cash: totalCash,
    total_invested_value: totalInvestedValue,
    total_profit_loss: totalProfitLoss,
    total_profit_pct: totalProfitPct,
    brokers: enrichedBrokers,
    by_ticker: byTickerWithWeight,
  }
}
