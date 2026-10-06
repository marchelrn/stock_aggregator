import { useEffect, useMemo, useRef, useState } from 'react'
import { usePortfolio } from '../context/PortfolioContext'
import { formatCurrency } from '../lib/formatters'
import type { Broker, Holding } from '../types'
import BrokerTable from '../components/BrokerTable'
import HoldingTable from '../components/HoldingTable'
import TransactionHistory from '../components/TransactionHistory'
import StockPrices from '../components/StockPrices'

interface TradeModalState {
  open: boolean
  type: 'BUY' | 'SELL'
  holding: Holding | null
}

export default function BrokersPage() {
  const {
    state,
    loadDashboard,
    addBroker,
    deleteBroker,
    createTransaction,
    fetchStockPrices,
    clearLiveStockPrices,
    setStatus,
  } = usePortfolio()

  // Broker yang holding-nya ditampilkan. Boleh lebih dari satu (atau kosong).
  const [selectedBrokerIds, setSelectedBrokerIds] = useState<number[]>([])
  // Broker yang sudah pernah dilihat, agar broker baru otomatis tercentang sekali
  // tanpa memaksa ulang pilihan user yang sengaja mematikan toggle.
  const knownBrokerIds = useRef<Set<number>>(new Set())

  const [brokerToRemove, setBrokerToRemove] = useState<Broker | null>(null)
  const [removing, setRemoving] = useState(false)

  const [configureOpen, setConfigureOpen] = useState(false)
  const [configBrokerName, setConfigBrokerName] = useState('')
  const [configCash, setConfigCash] = useState(0)
  const [configPin, setConfigPin] = useState('')

  const [tradeModal, setTradeModal] = useState<TradeModalState>({
    open: false,
    type: 'BUY',
    holding: null,
  })
  const [tradeBrokerId, setTradeBrokerId] = useState<number | null>(null)
  const [tradeLot, setTradeLot] = useState(1)
  const [tradePrice, setTradePrice] = useState(0)

  useEffect(() => {
    loadDashboard()
  }, [loadDashboard])

  // Sinkronkan pilihan dengan daftar broker:
  // - broker baru otomatis dicentang (semua tampil secara default)
  // - broker yang sudah dihapus dibuang dari pilihan
  useEffect(() => {
    const currentIds = state.brokers.map((b) => b.id)
    const newIds = currentIds.filter((id) => !knownBrokerIds.current.has(id))
    newIds.forEach((id) => knownBrokerIds.current.add(id))

    setSelectedBrokerIds((prev) => {
      const stillExists = prev.filter((id) => currentIds.includes(id))
      const next = [...stillExists, ...newIds]
      const unchanged = next.length === prev.length && next.every((id, i) => id === prev[i])
      return unchanged ? prev : next
    })
  }, [state.brokers])

  const handleToggleBroker = (id: number) => {
    setSelectedBrokerIds((current) =>
      current.includes(id) ? current.filter((x) => x !== id) : [...current, id],
    )
  }

  const confirmRemoveBroker = async () => {
    if (!brokerToRemove || removing) return
    setRemoving(true)
    const success = await deleteBroker(brokerToRemove.name)
    setRemoving(false)
    // Pilihan broker yang dihapus dibersihkan otomatis oleh efek sinkronisasi di atas.
    if (success) setBrokerToRemove(null)
  }

  const filteredHoldings = useMemo(() => {
    if (selectedBrokerIds.length === 0) return []
    return state.holdings.filter((h) => selectedBrokerIds.includes(Number(h.broker_id)))
  }, [state.holdings, selectedBrokerIds])

  const tradeValueText = useMemo(
    () => formatCurrency(Number(tradeLot || 0) * 100 * Number(tradePrice || 0)),
    [tradeLot, tradePrice],
  )

  const submitConfigureBroker = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!configBrokerName) {
      setStatus('Pilih broker terlebih dahulu', true)
      return
    }
    if (configBrokerName === 'Mandiri Sekuritas' && !configPin) {
      setStatus('PIN Trade Confirmation wajib diisi untuk Mandiri Sekuritas', true)
      return
    }

    await addBroker(configBrokerName, configCash || 0, configPin)

    setConfigureOpen(false)
    setConfigBrokerName('')
    setConfigCash(0)
    setConfigPin('')
  }

  const closeTradeModal = () => {
    setTradeModal({ open: false, type: 'BUY', holding: null })
    setTradeBrokerId(null)
  }

  const submitTradeModal = async (e: React.FormEvent) => {
    e.preventDefault()
    const broker = state.brokers.find((b) => Number(b.id) === Number(tradeBrokerId))
    if (!broker || !tradeModal.holding) return

    await createTransaction({
      type: tradeModal.type,
      ticker: tradeModal.holding.ticker,
      broker_id: Number(broker.id),
      broker_name: broker.name,
      lot: Number(tradeLot),
      price: Number(tradePrice),
    })
    closeTradeModal()
  }

  const handleFetchStockPrices = async (tickers: string) => {
    if (!tickers) {
      setStatus('Masukkan minimal satu ticker.', true)
      return
    }
    await fetchStockPrices(tickers)
  }

  return (
    <>
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-4 px-4">
        <section className="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-xl font-semibold">Brokers</h2>
            <button
              className="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800"
              onClick={() => setConfigureOpen(true)}
            >
              Configure Brokers
            </button>
          </div>
          <BrokerTable
            brokers={state.brokers}
            selectedBrokerIds={selectedBrokerIds}
            onToggle={handleToggleBroker}
            onRemove={setBrokerToRemove}
          />
        </section>

        <section className="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-xl font-semibold">Holdings by Broker Selected</h2>
            {state.brokers.length > 0 && (
              <span className="text-sm text-slate-500">
                {selectedBrokerIds.length} of {state.brokers.length} Brokers
              </span>
            )}
          </div>
          <HoldingTable holdings={filteredHoldings} />
        </section>

        <section className="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-xl font-semibold">History</h2>
          </div>
          <TransactionHistory transactionHistory={state.transactionHistory} />
        </section>

        <section className="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-xl font-semibold">Live Stock Prices</h2>
          </div>
          <StockPrices
            prices={state.liveStockPrices}
            onFetch={handleFetchStockPrices}
            onClear={clearLiveStockPrices}
          />
        </section>
      </div>

      {brokerToRemove && (
        <div
          className="fixed inset-0 z-50 grid place-items-center bg-slate-900/50 p-4"
          onClick={() => !removing && setBrokerToRemove(null)}
        >
          <div
            className="w-full max-w-md rounded-2xl border border-slate-300 bg-white p-4 shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-2 flex items-center justify-between">
              <h3 className="text-lg font-semibold">Disconnect Broker</h3>
            </div>

            <p className="text-sm text-slate-600">
              Putuskan koneksi <strong>{brokerToRemove.name}</strong>? <br />Data portofolio dari broker
              ini tidak akan diambil lagi.
            </p>
            <p className="mt-2 rounded-lg border border-rose-200 bg-rose-50 p-2 text-xs text-rose-700">
              Broker beserta seluruh <i>holding</i> saham di dalamnya akan dihapus dan tidak bisa
              dikembalikan. Riwayat transaksi tetap tersimpan.
            </p>

            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                className="rounded-lg bg-slate-200 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-300 disabled:opacity-50"
                onClick={() => setBrokerToRemove(null)}
                disabled={removing}
              >
                Batal
              </button>
              <button
                type="button"
                className="rounded-lg bg-rose-700 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-800 disabled:opacity-50"
                onClick={confirmRemoveBroker}
                disabled={removing}
              >
                {removing ? 'Memutuskan...' : 'Disconnect'}
              </button>
            </div>
          </div>
        </div>
      )}

      {tradeModal.open && (
        <div
          className="fixed inset-0 z-50 grid place-items-center bg-slate-900/50 p-4"
          onClick={closeTradeModal}
        >
          <div
            className="w-full max-w-md rounded-2xl border border-slate-300 bg-white p-4 shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-2 flex items-center justify-between">
              <h3 className="text-lg font-semibold">
                {tradeModal.type} {tradeModal.holding?.ticker}
              </h3>
              <button
                className="rounded p-1 text-slate-500 hover:bg-slate-100"
                onClick={closeTradeModal}
              >
                ✕
              </button>
            </div>

            <p className="mb-3 text-sm text-slate-500">
              Ticker: <strong>{tradeModal.holding?.ticker}</strong>
            </p>

            <form className="grid gap-3" onSubmit={submitTradeModal}>
              <label className="text-sm text-slate-600" htmlFor="trade-broker">
                Broker
              </label>
              <select
                id="trade-broker"
                value={tradeBrokerId ?? ''}
                onChange={(e) =>
                  setTradeBrokerId(e.target.value === '' ? null : Number(e.target.value))
                }
                disabled={tradeModal.type === 'SELL'}
                required
                className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
              >
                {state.brokers.map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name} (ID: {b.id})
                  </option>
                ))}
              </select>

              <label className="text-sm text-slate-600" htmlFor="trade-lot">
                Lot
              </label>
              <input
                id="trade-lot"
                value={tradeLot}
                onChange={(e) => setTradeLot(Number(e.target.value))}
                type="number"
                min="1"
                required
                className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
              />

              <label className="text-sm text-slate-600" htmlFor="trade-price">
                Harga per lembar
              </label>
              <input
                id="trade-price"
                value={tradePrice}
                onChange={(e) => setTradePrice(Number(e.target.value))}
                type="number"
                min="1"
                step="0.01"
                required
                className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
              />

              <div className="rounded-lg border border-slate-300 bg-slate-50 p-2 text-sm">
                Estimasi nilai transaksi: <strong>{tradeValueText}</strong>
              </div>

              <div className="mt-1 flex justify-end gap-2">
                <button
                  type="button"
                  className="rounded-lg bg-slate-200 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-300"
                  onClick={closeTradeModal}
                >
                  Batal
                </button>
                <button
                  type="submit"
                  className={
                    tradeModal.type === 'BUY'
                      ? 'rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800'
                      : 'rounded-lg bg-rose-700 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-800'
                  }
                >
                  Confirm {tradeModal.type}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {configureOpen && (
        <div
          className="fixed inset-0 z-50 grid place-items-center bg-slate-900/50 p-4"
          onClick={() => setConfigureOpen(false)}
        >
          <div
            className="w-full max-w-md rounded-2xl border border-slate-300 bg-white p-4 shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-2 flex items-center justify-between">
              <h3 className="text-lg font-semibold">Configure Broker</h3>
              <button
                className="rounded p-1 text-slate-500 hover:bg-slate-100"
                onClick={() => setConfigureOpen(false)}
              >
                ✕
              </button>
            </div>

            <p className="mb-4 text-sm text-slate-500">
              Pilih sekuritas yang didukung untuk integrasi trade confirmation dari Gmail.
            </p>

            <form className="grid gap-3" onSubmit={submitConfigureBroker}>
              <label className="text-sm text-slate-600" htmlFor="select-broker">
                Pilih Broker
              </label>
              <select
                id="select-broker"
                value={configBrokerName}
                onChange={(e) => setConfigBrokerName(e.target.value)}
                required
                className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
              >
                <option value="" disabled>
                  -- Pilih Sekuritas --
                </option>
                <option value="Mandiri Sekuritas">Mandiri Sekuritas (Growing/MOST)</option>
                <option value="Stockbit Sekuritas">Stockbit Sekuritas</option>
                <option value="BNI Sekuritas">BNI Sekuritas (BIONS)</option>
              </select>

              <label className="text-sm text-slate-600" htmlFor="initial-cash">
                Initial Cash (Opsional)
              </label>
              <input
                id="initial-cash"
                value={configCash}
                onChange={(e) => setConfigCash(Number(e.target.value))}
                type="number"
                min="0"
                step="0.01"
                className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
                placeholder="0"
              />

              {configBrokerName === 'Mandiri Sekuritas' && (
                <div className="grid gap-1">
                  <label className="text-sm text-slate-600" htmlFor="broker-pin">
                    PIN Trade Confirmation
                  </label>
                  <input
                    id="broker-pin"
                    value={configPin}
                    onChange={(e) => setConfigPin(e.target.value)}
                    type="password"
                    required
                    className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
                    placeholder="Masukkan PIN"
                  />
                  <span className="text-xs text-slate-500">
                    PIN Anda aman dan akan dienkripsi sebelum disimpan.
                  </span>
                </div>
              )}

              <div className="mt-3 flex justify-end gap-2">
                <button
                  type="button"
                  className="rounded-lg bg-slate-200 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-300"
                  onClick={() => setConfigureOpen(false)}
                >
                  Batal
                </button>
                <button
                  type="submit"
                  className="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800"
                >
                  Configure
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  )
}
