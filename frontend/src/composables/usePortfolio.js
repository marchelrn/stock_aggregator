import { ref, reactive } from 'vue'
import { useApi } from './useApi'
import { useToast } from 'vue-toastification'

const state = reactive({
  summary: null,
  brokers: [],
  holdings: [],
  stockPrices: {},
  liveStockPrices: {},
  transactionHistory: []
})

const status = ref('')
const isError = ref(false)

const num = (v) => Number(v || 0)

const normalizeArray = (res) => {
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

const buildSummary = (brokers = [], holdings = [], priceMap = {}) => {
  const getCurrentPrice = (ticker, fallbackPrice) => {
    const key = String(ticker || '').toUpperCase()
    const priceRow = priceMap?.[key]
    if (priceRow && priceRow.price != null) return num(priceRow.price)
    return num(fallbackPrice)
  }

  const tickerMap = new Map()

  for (const h of holdings) {
    const ticker = String(h.ticker || '').toUpperCase()
    if (!ticker) continue

    const prev = tickerMap.get(ticker) || {
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
      weight: 0
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

  const byTicker = Array.from(tickerMap.values()).map((t) => {
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
      profit_pct: invested > 0 ? (pl / invested) * 100 : 0
    }
  })

  const brokerMap = new Map()

  for (const b of brokers) {
    brokerMap.set(num(b.id), {
      id: b.id,
      name: b.name,
      cash: num(b.cash),
      stock_value: 0,
      total_value: num(b.cash),
      holdings: []
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
        holdings: []
      })
    }

    const broker = brokerMap.get(brokerId)
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
    weight: totalStockValue > 0 ? (num(t.market_value) / totalStockValue) * 100 : 0
  }))

  const enrichedBrokers = brokersOut.map((b) => ({
    ...b,
    weight: totalPortfolioValue > 0 ? (num(b.total_value) / totalPortfolioValue) * 100 : 0
  }))

  return {
    total_portfolio_value: totalPortfolioValue,
    total_stock_value: totalStockValue,
    total_cash: totalCash,
    total_invested_value: totalInvestedValue,
    total_profit_loss: totalProfitLoss,
    total_profit_pct: totalProfitPct,
    brokers: enrichedBrokers,
    by_ticker: byTickerWithWeight
  }
}

export function usePortfolio() {
  const { api, baseUrl } = useApi()
  const toast = useToast()

  const setStatus = (message, error = false) => {
    status.value = message
    isError.value = error
    if (message !== 'Loading data...') {
      if (error) toast.error(message)
      else toast.success(message)
    }
  }

  const loadDashboard = async (options = {}) => {
    const { silent = false } = options
    if (!silent) setStatus('Loading data...')

    try {
      const [brokersResult, holdingsResult, historyResult] = await Promise.allSettled([
        api('/brokers'),
        api('/stocks'),
        api('/transactions')
      ])

      state.brokers = brokersResult.status === 'fulfilled' ? normalizeArray(brokersResult.value) : []
      state.holdings = holdingsResult.status === 'fulfilled' ? normalizeArray(holdingsResult.value) : []
      state.transactionHistory = historyResult.status === 'fulfilled' ? normalizeArray(historyResult.value) : []

      const uniqueTickers = [...new Set(state.holdings.map((h) => String(h.ticker || '').toUpperCase()).filter(Boolean))]

      state.stockPrices = {}
      if (uniqueTickers.length > 0) {
        const tickerQuery = uniqueTickers.join(',')
        const pricesResult = await Promise.allSettled([api(`/prices?ticker=${encodeURIComponent(tickerQuery)}`)])

        if (pricesResult[0].status === 'fulfilled') {
          state.stockPrices = pricesResult[0].value?.data || {}
        }
      }

      state.summary = buildSummary(state.brokers, state.holdings, state.stockPrices)

      if (brokersResult.status === 'rejected' || holdingsResult.status === 'rejected' || historyResult.status === 'rejected') {
        const msg = [
          brokersResult.status === 'rejected' ? `brokers: ${brokersResult.reason?.message || 'failed'}` : null,
          holdingsResult.status === 'rejected' ? `stocks: ${holdingsResult.reason?.message || 'failed'}` : null,
          historyResult.status === 'rejected' ? `transactions: ${historyResult.reason?.message || 'failed'}` : null
        ]
          .filter(Boolean)
          .join(' | ')

        if (!silent) setStatus(`Partial load from ${baseUrl.value} (${msg})`, true)
        return
      }

      if (!silent) setStatus(`Data loaded from ${baseUrl.value}`)
    } catch (error) {
      setStatus(error.message, true)
    }
  }

  const addBroker = async (name, cash, pin) => {
    try {
      const payload = { name, cash }
      if (pin) payload.pin = pin
      
      await api('/broker', {
        method: 'POST',
        body: JSON.stringify(payload)
      })
      setStatus(`Broker ${name} ditambahkan.`)
      await loadDashboard({ silent: true })
      return true
    } catch (error) {
      setStatus(error.message, true)
      return false
    }
  }

  const addBrokerCash = async (broker, cash) => {
    try {
      await api(`/broker/${broker.id}`, {
        method: 'PUT',
        body: JSON.stringify({ cash })
      })
      setStatus(`Cash sebesar Rp.${cash} berhasil ditambahkan ke broker ${broker.name} ${broker.id}.`)
      await loadDashboard({ silent: true })
      return true
    } catch (error) {
      setStatus(error.message, true)
      return false
    }
  }

  const deleteBroker = async (name) => {
    try {
      await api(`/broker/${encodeURIComponent(name)}`, { method: 'DELETE' })
      setStatus('Broker berhasil dihapus.')
      await loadDashboard({ silent: true })
      return true
    } catch (error) {
      setStatus(error.message, true)
      return false
    }
  }


  const createTransaction = async (payload) => {
    try {
      await api('/transaction', {
        method: 'POST',
        body: JSON.stringify(payload)
      })
      setStatus(`Transaksi ${payload.type} ${payload.ticker} berhasil.`)
      await loadDashboard({ silent: true })
      return true
    } catch (error) {
      setStatus(error.message, true)
      return false
    }
  }

  const transactionHistory = async () => {
    try {
      const res = await api(`/transactions`)
      if (!res.data || !Array.isArray(res.data)) {
        setStatus('Data transaksi tidak ditemukan.', true)
        return []
      }
      return res.data
    } catch (error) {
      setStatus(error.message, true)
      return []
    }
  }

  const fetchStockPrices = async (tickers) => {
    try {
      const res = await api(`/prices?ticker=${encodeURIComponent(tickers)}`)
      if (!res.data || Object.keys(res.data).length === 0) {
        setStatus(`Data Ticker ${tickers} tidak ditemukan.`, true)
        return false
      }

      state.liveStockPrices = res.data || {}
      setStatus(`Harga saham untuk ${tickers} berhasil diambil.`)
      return true
    } catch (error) {
      if (error.message.includes('No data found') || error.response.status === 400) {
        setStatus(`Ticker ${tickers} tidak terdaftar di sistem.`, true)
      } else if (error.response.status === 429 || error.message.includes('Too Many Requests')) {
        setStatus('Terlalu banyak permintaan, silakan coba lagi nanti.', true)
      }
      else if (error.message.includes('Network Error')) {
        setStatus('Koneksi internet terputus.', true)
      } else {
        setStatus('Terjadi kesalahan teknis, silakan coba lagi.', true)
      }
      return false
    }
  }

  const clearLiveStockPrices = () => {
    state.liveStockPrices = {}
  }

  return {
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
    transactionHistory,
    clearLiveStockPrices
  }
}
