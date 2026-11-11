import { useEffect, useMemo, useRef, useState } from 'react'
import { usePortfolio } from '../context/PortfolioContext'
import { formatCurrency } from '../lib/formatters'
import { api } from '../lib/api'
import { normalizeArray } from '../lib/portfolio'
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

interface PendingStock {
  ticker: string
  lot: number
  avg_price: number
}

export default function BrokersPage() {
  const {
    state,
    loadDashboard,
    addBroker,
    addBrokerCash,
    addStock,
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
  const [configCash, setConfigCash] = useState<number | ''>('')
  const [configPin, setConfigPin] = useState('')

  // State untuk penambahan saham pada broker
  const [pendingStocks, setPendingStocks] = useState<PendingStock[]>([])
  const [stockTicker, setStockTicker] = useState('')
  const [stockLot, setStockLot] = useState<number | ''>(1)
  const [stockAvgPrice, setStockAvgPrice] = useState<number | ''>('')

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

  // Bobot alokasi per broker (% dari total portofolio) dari summary yang sama
  // dengan yang dipakai Dashboard, sehingga angkanya konsisten antar halaman.
  const brokerAllocations = useMemo(() => {
    const map: Record<number, number> = {}
    state.summary?.brokers.forEach((b) => {
      map[b.id] = b.weight
    })
    return map
  }, [state.summary])

  const filteredHoldings = useMemo(() => {
    if (selectedBrokerIds.length === 0) return []
    return state.holdings.filter((h) => selectedBrokerIds.includes(Number(h.broker_id)))
  }, [state.holdings, selectedBrokerIds])

  const tradeValueText = useMemo(
    () => formatCurrency(Number(tradeLot || 0) * 100 * Number(tradePrice || 0)),
    [tradeLot, tradePrice],
  )

  const handleAddPendingStock = (e?: React.MouseEvent | React.FormEvent) => {
    if (e) e.preventDefault()
    const cleanTicker = stockTicker.trim().toUpperCase()
    if (!cleanTicker) {
      setStatus('Kode saham (ticker) wajib diisi', true)
      return
    }
    const lotNum = Number(stockLot)
    if (!lotNum || lotNum <= 0) {
      setStatus('Jumlah lot harus lebih besar dari 0', true)
      return
    }
    const priceNum = Number(stockAvgPrice)
    if (!priceNum || priceNum <= 0) {
      setStatus('Harga rata-rata (avg price) harus lebih besar dari 0', true)
      return
    }

    if (pendingStocks.some((s) => s.ticker === cleanTicker)) {
      setPendingStocks((prev) =>
        prev.map((s) => (s.ticker === cleanTicker ? { ticker: cleanTicker, lot: lotNum, avg_price: priceNum } : s)),
      )
    } else {
      setPendingStocks((prev) => [
        ...prev,
        { ticker: cleanTicker, lot: lotNum, avg_price: priceNum },
      ])
    }

    setStockTicker('')
    setStockLot(1)
    setStockAvgPrice('')
  }

  const handleRemovePendingStock = (ticker: string) => {
    setPendingStocks((prev) => prev.filter((s) => s.ticker !== ticker))
  }

  const submitConfigureBroker = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!configBrokerName) {
      setStatus('Pilih broker terlebih dahulu', true)
      return
    }

    const existingBroker = state.brokers.find(
      (b) => b.name.toLowerCase() === configBrokerName.toLowerCase(),
    )

    if (configBrokerName === 'Mandiri Sekuritas' && !existingBroker && !configPin) {
      setStatus('PIN Trade Confirmation wajib diisi untuk Mandiri Sekuritas', true)
      return
    }

    let brokerId: number | null = null
    const cashValue = Number(configCash) || 0

    if (!existingBroker) {
      const ok = await addBroker(configBrokerName, cashValue, configPin)
      if (!ok) return
      try {
        const res = await api('/brokers')
        const updated = normalizeArray<Broker>(res)
        const found = updated.find(
          (b) => b.name.toLowerCase() === configBrokerName.toLowerCase(),
        )
        if (found) brokerId = Number(found.id)
      } catch {
        // fallback
      }
    } else {
      brokerId = Number(existingBroker.id)
      if (cashValue > 0) {
        await addBrokerCash(existingBroker, cashValue)
      }
    }

    if (brokerId && pendingStocks.length > 0) {
      let addedCount = 0
      for (const s of pendingStocks) {
        const ok = await addStock({
          ticker: s.ticker,
          lot: s.lot,
          avg_price: s.avg_price,
          broker_id: brokerId,
          broker_name: configBrokerName,
        })
        if (ok) addedCount++
      }
      setStatus(`Berhasil mengonfigurasi ${configBrokerName} dan ${addedCount} saham!`)
    } else if (existingBroker && cashValue > 0) {
      setStatus(`Berhasil memperbarui cash untuk ${configBrokerName}.`)
    }

    await loadDashboard()
    setConfigureOpen(false)
    setConfigBrokerName('')
    setConfigCash('')
    setConfigPin('')
    setPendingStocks([])
    setStockTicker('')
    setStockLot(1)
    setStockAvgPrice('')
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
            allocations={brokerAllocations}
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
          <HoldingTable
            holdings={filteredHoldings}
            stockPrices={state.liveStockPrices || state.stockPrices}
            tickerSummaries={state.summary?.by_ticker}
            totalPortfolioValue={state.summary?.total_portfolio_value}
          />
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
          onClick={() => {
            setConfigureOpen(false)
            setPendingStocks([])
          }}
        >
          <div
            className="w-full max-w-lg max-h-[90vh] overflow-y-auto rounded-2xl border border-slate-300 bg-white p-5 shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-2 flex items-center justify-between">
              <h3 className="text-lg font-semibold text-slate-800">Configure Broker & Saham</h3>
              <button
                className="rounded p-1 text-slate-500 hover:bg-slate-100"
                onClick={() => {
                  setConfigureOpen(false)
                  setPendingStocks([])
                }}
              >
                ✕
              </button>
            </div>

            <p className="mb-4 text-xs text-slate-500">
              Konfigurasikan sekuritas dan tambahkan saham yang Anda miliki saat ini di sekuritas tersebut.
            </p>

            <form className="grid gap-4" onSubmit={submitConfigureBroker}>
              <div className="grid gap-1.5">
                <label className="text-xs font-semibold text-slate-700" htmlFor="select-broker">
                  Pilih Sekuritas / Broker
                </label>
                <select
                  id="select-broker"
                  value={configBrokerName}
                  onChange={(e) => setConfigBrokerName(e.target.value)}
                  required
                  className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-teal-600 focus:outline-none"
                >
                  <option value="" disabled>
                    -- Pilih Sekuritas --
                  </option>
                  {state.brokers.length > 0 && (
                    <optgroup label="Broker Anda Saat Ini">
                      {state.brokers.map((b) => (
                        <option key={b.id} value={b.name}>
                          {b.name} (Tersimpan)
                        </option>
                      ))}
                    </optgroup>
                  )}
                  <optgroup label="Tambah Sekuritas Baru">
                    {['Stockbit Sekuritas', 'Mandiri Sekuritas', 'BNI Sekuritas']
                      .filter((name) => !state.brokers.some((b) => b.name.toLowerCase() === name.toLowerCase()))
                      .map((name) => (
                        <option key={name} value={name}>
                          {name}
                        </option>
                      ))}
                  </optgroup>
                </select>
              </div>

              <div className="grid gap-1.5">
                <label className="text-xs font-semibold text-slate-700" htmlFor="initial-cash">
                  {state.brokers.some((b) => b.name.toLowerCase() === configBrokerName.toLowerCase())
                    ? 'Tambah Saldo RDN / Cash (Opsional)'
                    : 'Initial Cash RDN (Opsional)'}
                </label>
                <input
                  id="initial-cash"
                  value={configCash}
                  onChange={(e) => setConfigCash(e.target.value === '' ? '' : Number(e.target.value))}
                  type="number"
                  min="0"
                  step="0.01"
                  className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-teal-600 focus:outline-none"
                  placeholder="0"
                />
              </div>

              {configBrokerName === 'Mandiri Sekuritas' &&
                !state.brokers.some((b) => b.name.toLowerCase() === 'mandiri sekuritas') && (
                  <div className="grid gap-1">
                    <label className="text-xs font-semibold text-slate-700" htmlFor="broker-pin">
                      PIN Trade Confirmation
                    </label>
                    <input
                      id="broker-pin"
                      value={configPin}
                      onChange={(e) => setConfigPin(e.target.value)}
                      type="password"
                      required
                      className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-teal-600 focus:outline-none"
                      placeholder="Masukkan PIN"
                    />
                    <span className="text-[11px] text-slate-500">
                      PIN Anda aman dan akan dienkripsi sebelum disimpan.
                    </span>
                  </div>
                )}

              {/* Section: Tambah Saham yang Dimiliki */}
              <div className="border-t border-slate-200 pt-3">
                <div className="mb-2">
                  <h4 className="text-xs font-semibold uppercase tracking-wider text-slate-700">
                    Saham yang Dimiliki (Opsional)
                  </h4>
                  <p className="text-[11px] text-slate-500">
                    Masukkan saham yang sudah ada di portofolio broker ini beserta harga beli rata-ratanya.
                  </p>
                </div>

                <div className="grid grid-cols-12 gap-2">
                  <div className="col-span-4">
                    <label className="block text-[10px] font-medium text-slate-500 mb-1">Ticker</label>
                    <input
                      type="text"
                      maxLength={6}
                      value={stockTicker}
                      onChange={(e) => setStockTicker(e.target.value.toUpperCase())}
                      placeholder="BBCA"
                      className="w-full rounded-lg border border-slate-300 px-2.5 py-1.5 text-xs font-semibold uppercase focus:border-teal-600 focus:outline-none"
                    />
                  </div>
                  <div className="col-span-3">
                    <label className="block text-[10px] font-medium text-slate-500 mb-1">Lot</label>
                    <input
                      type="number"
                      min="1"
                      value={stockLot}
                      onChange={(e) => setStockLot(e.target.value === '' ? '' : Number(e.target.value))}
                      placeholder="Lot"
                      className="w-full rounded-lg border border-slate-300 px-2.5 py-1.5 text-xs focus:border-teal-600 focus:outline-none"
                    />
                  </div>
                  <div className="col-span-3">
                    <label className="block text-[10px] font-medium text-slate-500 mb-1">Avg Price</label>
                    <input
                      type="number"
                      min="1"
                      step="0.01"
                      value={stockAvgPrice}
                      onChange={(e) => setStockAvgPrice(e.target.value === '' ? '' : Number(e.target.value))}
                      placeholder="Rp"
                      className="w-full rounded-lg border border-slate-300 px-2.5 py-1.5 text-xs focus:border-teal-600 focus:outline-none"
                    />
                  </div>
                  <div className="col-span-2 flex items-end">
                    <button
                      type="button"
                      onClick={handleAddPendingStock}
                      className="w-full rounded-lg bg-emerald-600 py-1.5 text-xs font-semibold text-white hover:bg-emerald-700 transition"
                    >
                      Tambah
                    </button>
                  </div>
                </div>

                {/* List Pending Stocks */}
                {pendingStocks.length > 0 && (
                  <div className="mt-3 space-y-1.5 rounded-lg border border-slate-200 bg-slate-50 p-2.5">
                    <div className="flex justify-between items-center text-[11px] font-semibold text-slate-600 mb-1">
                      <span>Daftar Saham ({pendingStocks.length})</span>
                      <span>Total: {formatCurrency(pendingStocks.reduce((sum, s) => sum + s.lot * 100 * s.avg_price, 0))}</span>
                    </div>
                    {pendingStocks.map((s) => (
                      <div
                        key={s.ticker}
                        className="flex items-center justify-between rounded-md bg-white border border-slate-200 px-2.5 py-1.5 text-xs"
                      >
                        <div className="flex items-center gap-2">
                          <span className="font-bold text-teal-800 bg-teal-50 px-1.5 py-0.5 rounded border border-teal-200 text-[11px]">
                            {s.ticker}
                          </span>
                          <span className="text-slate-600">{s.lot} lot</span>
                          <span className="text-slate-400">•</span>
                          <span className="text-slate-600 font-mono">@ {formatCurrency(s.avg_price)}</span>
                        </div>
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-slate-700 font-mono text-[11px]">
                            {formatCurrency(s.lot * 100 * s.avg_price)}
                          </span>
                          <button
                            type="button"
                            onClick={() => handleRemovePendingStock(s.ticker)}
                            className="text-rose-500 hover:text-rose-700 text-sm font-bold ml-1"
                            title="Hapus"
                          >
                            ✕
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              <div className="mt-3 flex justify-end gap-2 border-t border-slate-200 pt-3">
                <button
                  type="button"
                  className="rounded-lg bg-slate-200 px-3.5 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-300 transition"
                  onClick={() => {
                    setConfigureOpen(false)
                    setPendingStocks([])
                  }}
                >
                  Batal
                </button>
                <button
                  type="submit"
                  className="rounded-lg bg-teal-700 px-4 py-2 text-xs font-semibold text-white hover:bg-teal-800 transition"
                >
                  Simpan Konfigurasi
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  )
}
