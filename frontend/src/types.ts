// Shared domain types for the Stock Portfolio Dashboard.

export interface Broker {
  id: number
  name: string
  cash: number
  /** Saldo acuan dari pengaturan manual terakhir. */
  cash_base?: number
  /** Batas aktivitas kas, tidak boleh mendahului waktu pendaftaran. */
  cash_base_as_of?: string | null
}

/** Satu baris riwayat yang memengaruhi cash setelah cutoff. */
export interface CashEvent {
  source: string // TRADE | MUTATION
  type: string // BUY/SELL/DEPOSIT/WITHDRAW/DIVIDEND
  delta: number
  date: string
  description: string
}

export interface CashBreakdown {
  broker_id: number
  broker_name: string
  cash: number
  cash_base: number
  cash_base_as_of: string | null
  events_total: number
  events: CashEvent[]
}

export interface CashBreakdownResponse {
  status_code: number
  message: string
  data: CashBreakdown
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
  quoted_at?: string
  stale?: boolean
  ticker: string
  sector?: string | null
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
  sector?: string | null
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
  tax: number
  stock: { ticker: string; lot: number; price?: number }
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

export interface ParsedTrade {
  ticker: string
  lot: number
  price: number
  type: string
  date: string
  broker_name: string
  amount_done: number
  tax: number
  ref_no?: string
}

export interface GmailSyncResponse {
  status_code: number
  message: string
  trades_found: number
  trades_synced: number
  parsed_trades: ParsedTrade[]
  TradesFound?: number
  TradesSynced?: number
  Message?: string
}

export interface GmailSyncStatusResponse {
  status_code: number
  message: string
  gmail_sync_enabled: boolean
  last_sync_at: string | null
  GmailSyncEnabled?: boolean
  LastSyncAt?: string | null
}

export interface CashMutationData {
  id: number
  user_id: number
  broker_id: number
  broker_name: string
  type: string // DEPOSIT | WITHDRAW | DIVIDEND
  amount: number
  date: string
  email_received_at?: string | null
  description: string
  ref_no: string
  created_at: string
  updated_at: string
}

export interface CashMutationsResponse {
  status_code: number
  message: string
  data: CashMutationData[]
  MutationsFound?: number
  MutationsSynced?: number
  Message?: string
}

export interface GmailCashSyncResponse {
  status_code: number
  message: string
  mutations_found: number
  mutations_synced: number
  parsed_mutations: ParsedCashMutationItem[]
}

/** Hasil sync historis saat setup: tiap tahap bisa berhasil atau gagal sendiri. */
export interface SetupSyncResponse {
  trade_warning?: string
  trade_sync?: GmailSyncResponse
  cash_sync?: GmailCashSyncResponse
  trade_error?: string
  cash_error?: string
}

export interface ParsedCashMutationItem {
  broker_name: string
  type: string
  amount: number
  date: string
  ref_no: string
  description?: string
}

export interface GmailCashSyncResponse {
  status_code: number
  message: string
  mutations_found: number
  mutations_synced: number
  parsed_mutations: ParsedCashMutationItem[]
}
