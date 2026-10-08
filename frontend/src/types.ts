// Shared domain types for the Stock Portfolio Dashboard.

export interface Broker {
  id: number
  name: string
  cash: number
}

export interface Holding {
  id?: number
  ticker: string
  lot: number
  avg_price: number
  broker_id: number
  broker_name: string
  sector?: string | null
}

export interface StockPrice {
  ticker: string
  price: number
  previous_close: number
  change: number
  change_percent: number
  currency: string
  updated_at?: string
}

export interface MarketDetail {
  ticker: string
  name: string
  currency: string
  exchange: string
  price: number | null
  previous_close: number | null
  change: number | null
  change_percent: number | null
  day_high: number | null
  day_low: number | null
  fifty_two_week_high: number | null
  fifty_two_week_low: number | null
  volume: number | null
  market_time: number | null
  fetched_at: string
  history: { time: number; close: number }[]
}

export type StockPriceMap = Record<string, StockPrice>

export interface TickerSummary {
  ticker: string
  total_lot: number
  total_shares: number
  total_cost: number
  avg_price: number
  current_price: number
  market_value: number
  invested_value: number
  profit_loss: number
  profit_pct: number
  weight: number
}

export interface BrokerSummary {
  id: number
  name: string
  cash: number
  stock_value: number
  total_value: number
  holdings: Holding[]
  weight: number
}

export interface PortfolioSummary {
  total_portfolio_value: number
  total_stock_value: number
  total_cash: number
  total_invested_value: number
  total_profit_loss: number
  total_profit_pct: number
  brokers: BrokerSummary[]
  by_ticker: TickerSummary[]
}

export interface Transaction {
  id: number
  type: string
  amount_done: number
  date: string
  stock: { ticker: string; lot: number }
  broker: { name: string }
}

export interface User {
  id: number
  name: string
  email: string
  auth_provider?: string
  created_at?: string
  updated_at?: string
}

// Bentuk respons GET /api/auth/user
export interface UserResponse {
  status_code: number
  message: string
  data: User
}

export interface TransactionPayload {
  type: string
  ticker: string
  broker_id: number
  broker_name: string
  lot: number
  price: number
}
