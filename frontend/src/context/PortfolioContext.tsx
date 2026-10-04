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
  Holding,
  PortfolioSummary,
  StockPriceMap,
  Transaction,
  TransactionPayload,
  User,
  UserResponse,
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
    }),
    [state, status, isError],
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
