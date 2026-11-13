import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { toast } from 'react-toastify'
import { api, getBaseUrl } from '../lib/api'
import { buildSummary, normalizeArray } from '../lib/portfolio'
import type {
  Broker,
  CashMutationData,
  Holding,
  PortfolioSummary,
  StockPriceMap,
  Transaction,
  TransactionPayload,
  User,
  UserResponse,
  GmailSyncResponse,
  GmailSyncStatusResponse,
  CashMutationsResponse,
} from '../types'

interface PortfolioState {
  summary: PortfolioSummary | null
  brokers: Broker[]
  holdings: Holding[]
  stockPrices: StockPriceMap
  liveStockPrices: StockPriceMap
  transactionHistory: Transaction[]
  user: User | null
}

interface PortfolioContextValue {
  state: PortfolioState
  status: string
  isError: boolean
  setStatus: (message: string, error?: boolean) => void
  loadDashboard: (options?: { silent?: boolean }) => Promise<void>
  addBroker: (name: string, cash: number, pin?: string) => Promise<boolean>
  addBrokerCash: (broker: Broker, cash: number) => Promise<boolean>
  deleteBroker: (name: string) => Promise<boolean>
  createTransaction: (payload: TransactionPayload) => Promise<boolean>
  fetchStockPrices: (tickers: string) => Promise<boolean>
  loadTransactionHistory: () => Promise<Transaction[]>
  clearLiveStockPrices: () => void
  addStock: (stock: {
    ticker: string
    lot: number
    avg_price: number
    broker_id: number
    broker_name?: string
  }) => Promise<boolean>
  syncStockSectors: () => Promise<boolean>
  syncTradeConfirmations: (fullSync?: boolean) => Promise<GmailSyncResponse | null>
  syncCashMutations: () => Promise<CashMutationsResponse | null>
  loadCashMutations: () => Promise<CashMutationData[]>
  getGmailSyncStatus: () => Promise<GmailSyncStatusResponse | null>
}

const initialState: PortfolioState = {
  summary: null,
  brokers: [],
  holdings: [],
  stockPrices: {},
  liveStockPrices: {},
  transactionHistory: [],
  user: null,
}

const PortfolioContext = createContext<PortfolioContextValue | null>(null)

export function PortfolioProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<PortfolioState>(initialState)
  const [status, setStatusText] = useState('')
  const [isError, setIsError] = useState(false)

  const setStatus = useCallback((message: string, error = false) => {
    setStatusText(message)
    setIsError(error)
    if (message !== 'Loading data...') {
      if (error) toast.error(message)
      else toast.success(message)
    }
  }, [])

  const loadDashboard = useCallback(
    async (options: { silent?: boolean } = {}) => {
      const { silent = false } = options
      if (!silent) setStatus('Loading data...')

      try {
        const [userInfo, brokersResult, holdingsResult, historyResult] = await Promise.allSettled([
          api('/api/auth/user'),
          api('/brokers'),
          api('/my/stocks'),
          api('/transactions'),
        ])

        const user: User | null =
          userInfo.status === 'fulfilled' ? ((userInfo.value as UserResponse)?.data ?? null) : null

        if (user?.name) localStorage.setItem('userName', user.name)
        if (user?.email) localStorage.setItem('userEmail', user.email)
        const brokers = brokersResult.status === 'fulfilled' ? normalizeArray<Broker>(brokersResult.value) : []
        const holdings = holdingsResult.status === 'fulfilled' ? normalizeArray<Holding>(holdingsResult.value) : []
        const history = historyResult.status === 'fulfilled' ? normalizeArray<Transaction>(historyResult.value) : []

        const uniqueTickers = [
          ...new Set(holdings.map((h) => String(h.ticker || '').toUpperCase()).filter(Boolean)),
        ]

        let stockPrices: StockPriceMap = {}
        if (uniqueTickers.length > 0) {
          const tickerQuery = uniqueTickers.join(',')
          const pricesResult = await Promise.allSettled([
            api<{ data?: StockPriceMap }>(`/prices?ticker=${encodeURIComponent(tickerQuery)}`),
          ])
          if (pricesResult[0].status === 'fulfilled') {
            stockPrices = pricesResult[0].value?.data || {}
          }
        }

        const summary = buildSummary(brokers, holdings, stockPrices)

        setState((prev) => ({
          ...prev,
          user,
          brokers,
          holdings,
          transactionHistory: history,
          stockPrices,
          summary,
        }))

        if (
          brokersResult.status === 'rejected' ||
          holdingsResult.status === 'rejected' ||
          historyResult.status === 'rejected'
        ) {
          const msg = [
            brokersResult.status === 'rejected'
              ? `brokers: ${(brokersResult.reason as Error)?.message || 'failed'}`
              : null,
            holdingsResult.status === 'rejected'
              ? `stocks: ${(holdingsResult.reason as Error)?.message || 'failed'}`
              : null,
            historyResult.status === 'rejected'
              ? `transactions: ${(historyResult.reason as Error)?.message || 'failed'}`
              : null,
          ]
            .filter(Boolean)
            .join(' | ')

          if (!silent) setStatus(`Partial load from ${getBaseUrl()} (${msg})`, true)
          return
        }

        if (!silent) setStatus(`Data loaded from ${getBaseUrl()}`)
      } catch (error) {
        setStatus((error as Error).message, true)
      }
    },
    [setStatus],
  )

  const addBroker = useCallback(
    async (name: string, cash: number, pin?: string): Promise<boolean> => {
      try {
        const payload: Record<string, unknown> = { name, cash }
        if (pin) payload.pin = pin

        await api('/broker', { method: 'POST', body: JSON.stringify(payload) })
        setStatus(`Broker ${name} ditambahkan.`)
        await loadDashboard({ silent: true })
        return true
      } catch (error) {
        setStatus((error as Error).message, true)
        return false
      }
    },
    [setStatus, loadDashboard],
  )

  const addBrokerCash = useCallback(
    async (broker: Broker, cash: number): Promise<boolean> => {
      try {
        await api(`/broker/${broker.id}`, { method: 'PUT', body: JSON.stringify({ cash }) })
        setStatus(`Cash sebesar Rp.${cash} berhasil ditambahkan ke broker ${broker.name} ${broker.id}.`)
        await loadDashboard({ silent: true })
        return true
      } catch (error) {
        setStatus((error as Error).message, true)
        return false
      }
    },
    [setStatus, loadDashboard],
  )

  const deleteBroker = useCallback(
    async (name: string): Promise<boolean> => {
      try {
        await api(`/broker/${encodeURIComponent(name)}`, { method: 'DELETE' })
        setStatus('Broker berhasil dihapus.')
        await loadDashboard({ silent: true })
        return true
      } catch (error) {
        setStatus((error as Error).message, true)
        return false
      }
    },
    [setStatus, loadDashboard],
  )

  const createTransaction = useCallback(
    async (payload: TransactionPayload): Promise<boolean> => {
      try {
        await api('/transaction', { method: 'POST', body: JSON.stringify(payload) })
        setStatus(`Transaksi ${payload.type} ${payload.ticker} berhasil.`)
        await loadDashboard({ silent: true })
        return true
      } catch (error) {
        setStatus((error as Error).message, true)
        return false
      }
    },
    [setStatus, loadDashboard],
  )

  const loadTransactionHistory = useCallback(async (): Promise<Transaction[]> => {
    try {
      const res = await api<{ data?: Transaction[] }>('/transactions')
      if (!res.data || !Array.isArray(res.data)) {
        setStatus('Data transaksi tidak ditemukan.', true)
        return []
      }
      return res.data
    } catch (error) {
      setStatus((error as Error).message, true)
      return []
    }
  }, [setStatus])

  const fetchStockPrices = useCallback(
    async (tickers: string): Promise<boolean> => {
      try {
        const res = await api<{ data?: StockPriceMap; not_found?: string[] }>(
          `/prices?ticker=${encodeURIComponent(tickers)}`,
        )
        const data = res.data || {}
        const notFound = res.not_found || []

        if (Object.keys(data).length === 0) {
          setStatus(`Data Ticker ${tickers} tidak ditemukan.`, true)
          return false
        }

        // Sebagian ticker tidak dikembalikan oleh Yahoo → anggap permintaan gagal,
        // jangan tampilkan hasil sebagian seolah-olah semuanya valid.
        if (notFound.length > 0) {
          setState((prev) => ({ ...prev, liveStockPrices: {} }))
          setStatus(`Ticker tidak ditemukan: ${notFound.join(', ')}. Periksa kembali kode ticker.`, true)
          return false
        }

        setState((prev) => ({ ...prev, liveStockPrices: data }))
        setStatus(`Harga saham untuk ${tickers} berhasil diambil.`)
        return true
      } catch (error) {
        const message = (error as Error)?.message || ''
        if (message.includes('No data found') || message.includes('400')) {
          setStatus(`Ticker ${tickers} tidak terdaftar di sistem.`, true)
        } else if (message.includes('429') || message.includes('Too Many Requests')) {
          setStatus('Terlalu banyak permintaan, silakan coba lagi nanti.', true)
        } else if (message.includes('Network Error') || message.includes('Failed to fetch')) {
          setStatus('Koneksi internet terputus.', true)
        } else {
          setStatus('Terjadi kesalahan teknis, silakan coba lagi.', true)
        }
        return false
      }
    },
    [setStatus],
  )

  const clearLiveStockPrices = useCallback(() => {
    setState((prev) => ({ ...prev, liveStockPrices: {} }))
  }, [])

  const addStock = useCallback(
    async (stock: {
      ticker: string
      lot: number
      avg_price: number
      broker_id: number
      broker_name?: string
    }): Promise<boolean> => {
      try {
        await api('/my/stock', {
          method: 'POST',
          body: JSON.stringify({
            ticker: stock.ticker.toUpperCase().trim(),
            lot: Number(stock.lot),
            avg_price: Number(stock.avg_price),
            broker_id: Number(stock.broker_id),
            broker_name: stock.broker_name || '',
          }),
        })
        return true
      } catch (error) {
        // Fallback to PUT if the stock holding already exists for this broker
        try {
          await api(`/my/stock/${encodeURIComponent(stock.ticker.toUpperCase().trim())}`, {
            method: 'PUT',
            body: JSON.stringify({
              ticker: stock.ticker.toUpperCase().trim(),
              lot: Number(stock.lot),
              avg_price: Number(stock.avg_price),
              broker_id: Number(stock.broker_id),
              broker_name: stock.broker_name || '',
            }),
          })
          return true
        } catch (updateErr) {
          setStatus((error as Error).message || (updateErr as Error).message, true)
          return false
        }
      }
    },
    [setStatus],
  )

  const syncStockSectors = useCallback(async (): Promise<boolean> => {
    try {
      setStatus('Menyinkronkan data sektor dari Yahoo Finance...')
      await api('/my/stocks/sync-sectors', { method: 'POST' })
      setStatus('Data sektor berhasil disinkronkan dari Yahoo Finance.')
      await loadDashboard({ silent: true })
      return true
    } catch (error) {
      setStatus((error as Error).message, true)
      return false
    }
  }, [setStatus, loadDashboard])

  const syncTradeConfirmations = useCallback(
    async (fullSync = false): Promise<GmailSyncResponse | null> => {
      try {
        setStatus(fullSync ? 'Sinkronisasi penuh trade confirmation dari Gmail...' : 'Menyinkronkan trade confirmation dari Gmail...')
        const path = fullSync ? '/api/gmail/sync?full=true' : '/api/gmail/sync'
        const res = await api<GmailSyncResponse>(path, { method: 'POST' })
        const tradesSynced = res.TradesSynced ?? res.trades_synced ?? 0
        const tradesFound = res.TradesFound ?? res.trades_found ?? 0
        if (tradesSynced > 0 || tradesFound > 0) {
          await loadDashboard({ silent: true })
        }
        setStatus(res.Message ?? res.message ?? 'Sync completed')
        return res
      } catch (error) {
        setStatus((error as Error).message, true)
        return null
      }
    },
    [setStatus, loadDashboard],
  )

  const syncCashMutations = useCallback(async (): Promise<CashMutationsResponse | null> => {
    try {
      setStatus('Menyinkronkan mutasi kas (deposit/penarikan) dari Gmail...')
      const res = await api<CashMutationsResponse>('/api/gmail/sync-cash', { method: 'POST' })
      const mutationsSynced = res.MutationsSynced ?? 0
      const mutationsFound = res.MutationsFound ?? 0
      if (mutationsSynced > 0 || mutationsFound > 0) {
        await loadDashboard({ silent: true })
      }
      setStatus(res.Message ?? 'Sync completed')
      return res
    } catch (error) {
      setStatus((error as Error).message, true)
      return null
    }
  }, [setStatus, loadDashboard])

  const getGmailSyncStatus = useCallback(async (): Promise<GmailSyncStatusResponse | null> => {
    try {
      const res = await api<GmailSyncStatusResponse>('/api/gmail/status')
      return res
    } catch (error) {
      setStatus((error as Error).message, true)
      return null
    }
  }, [setStatus])

  const loadCashMutations = useCallback(async (): Promise<CashMutationData[]> => {
    try {
      const res = await api<{ data?: CashMutationData[] }>('/api/gmail/cash-mutations')
      return res.data ?? []
    } catch (error) {
      setStatus((error as Error).message, true)
      return []
    }
  }, [setStatus])

  const value = useMemo<PortfolioContextValue>(
    () => ({
      state,
      status,
      isError,
      setStatus,
      loadDashboard,
      addBroker,
      addBrokerCash,
      deleteBroker,
      createTransaction,
      fetchStockPrices,
      loadTransactionHistory,
      clearLiveStockPrices,
      addStock,
      syncStockSectors,
      syncTradeConfirmations,
      syncCashMutations,
      loadCashMutations,
      getGmailSyncStatus,
    }),
    [
      state,
      status,
      isError,
      setStatus,
      loadDashboard,
      addBroker,
      addBrokerCash,
      deleteBroker,
      createTransaction,
      fetchStockPrices,
      loadTransactionHistory,
      clearLiveStockPrices,
      addStock,
      syncStockSectors,
      syncTradeConfirmations,
      syncCashMutations,
      loadCashMutations,
      getGmailSyncStatus,
    ],
  )

  return <PortfolioContext.Provider value={value}>{children}</PortfolioContext.Provider>
}

export function usePortfolio(): PortfolioContextValue {
  const ctx = useContext(PortfolioContext)
  if (!ctx) {
    throw new Error('usePortfolio must be used within a PortfolioProvider')
  }
  return ctx
}
