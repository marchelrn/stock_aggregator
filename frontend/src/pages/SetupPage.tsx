import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'react-toastify'
import { usePortfolio } from '../context/PortfolioContext'

interface LocalHolding {
  ticker: string
  lot: number
  price: number
}

export default function SetupPage() {
  const navigate = useNavigate()
  const { addBroker, createTransaction, state } = usePortfolio()

  const [isBrokerAdded, setIsBrokerAdded] = useState(false)
  const [localHoldings, setLocalHoldings] = useState<LocalHolding[]>([])

  const [brokerName, setBrokerName] = useState('')
  const [brokerCash, setBrokerCash] = useState(0)
  const [brokerPin, setBrokerPin] = useState('')

  const [holdingTicker, setHoldingTicker] = useState('')
  const [holdingLot, setHoldingLot] = useState(1)
  const [holdingPrice, setHoldingPrice] = useState(0)

  const submitBroker = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!brokerName) return
    if (brokerName === 'Mandiri Sekuritas' && !brokerPin) {
      toast.error('PIN wajib diisi untuk Mandiri Sekuritas')
      return
    }

    const success = await addBroker(brokerName, brokerCash, brokerPin)
    if (success) {
      setIsBrokerAdded(true)
    }
  }

  const addLocalHolding = (e: React.FormEvent) => {
    e.preventDefault()
    if (!holdingTicker || holdingLot <= 0 || holdingPrice <= 0) return

    setLocalHoldings((prev) => [
      ...prev,
      { ticker: holdingTicker.toUpperCase(), lot: holdingLot, price: holdingPrice },
    ])

    setHoldingTicker('')
    setHoldingLot(1)
    setHoldingPrice(0)
  }

  const finishSetup = async () => {
    // Sync local holdings as initial BUY transactions if any.
    if (localHoldings.length > 0) {
      const broker = state.brokers.find((b) => b.name === brokerName)

      if (broker) {
        for (const holding of localHoldings) {
          await createTransaction({
            type: 'BUY',
            ticker: holding.ticker,
            broker_id: Number(broker.id),
            broker_name: broker.name,
            lot: holding.lot,
            price: holding.price,
          })
        }
      }
    }

    localStorage.setItem('isSetupCompleted', 'true')
    toast.success('Setup awal berhasil diselesaikan!')
    navigate('/')
  }

  return (
    <div className="mx-auto grid w-full max-w-3xl gap-6 px-4 py-8">
      <div className="rounded-2xl border border-slate-300 bg-white/80 p-6 shadow-sm backdrop-blur">
        <h1 className="text-2xl font-bold text-slate-800">Initial Setup</h1>
        <p className="mt-2 text-sm text-slate-500">
          Silakan konfigurasikan broker pertama Anda beserta cash dan saham awal yang Anda miliki
          saat ini.
        </p>
      </div>

      <section className="rounded-2xl border border-slate-300 bg-white/80 p-6 shadow-sm backdrop-blur">
        <h2 className="mb-4 text-lg font-semibold">1. Hubungkan Broker</h2>
        <form className="grid gap-3" onSubmit={submitBroker}>
          <label className="text-sm text-slate-600" htmlFor="select-broker">
            Pilih Sekuritas
          </label>
          <select
            id="select-broker"
            value={brokerName}
            onChange={(e) => setBrokerName(e.target.value)}
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
            Initial Cash (IDR)
          </label>
          <input
            id="initial-cash"
            value={brokerCash}
            onChange={(e) => setBrokerCash(Number(e.target.value))}
            type="number"
            min="0"
            step="0.01"
            className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
            placeholder="0"
          />

          {brokerName === 'Mandiri Sekuritas' && (
            <div className="grid gap-1">
              <label className="text-sm text-slate-600" htmlFor="broker-pin">
                PIN Trade Confirmation
              </label>
              <input
                id="broker-pin"
                value={brokerPin}
                onChange={(e) => setBrokerPin(e.target.value)}
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

          <button
            type="submit"
            className="mt-2 w-full rounded-lg bg-teal-700 px-4 py-2 font-semibold text-white hover:bg-teal-800 disabled:opacity-50"
            disabled={isBrokerAdded}
          >
            {isBrokerAdded ? 'Broker Tersimpan ✓' : 'Simpan Broker'}
          </button>
        </form>
      </section>

      {isBrokerAdded && (
        <section className="rounded-2xl border border-slate-300 bg-white/80 p-6 shadow-sm backdrop-blur">
          <h2 className="mb-4 text-lg font-semibold">2. Masukkan Portfolio Awal</h2>

          {localHoldings.length > 0 && (
            <div className="mb-4 overflow-hidden rounded-xl border border-slate-300 bg-slate-50">
              <table className="w-full text-left text-sm">
                <thead className="bg-slate-100 text-xs text-slate-500">
                  <tr>
                    <th className="p-2 pl-3">Ticker</th>
                    <th className="p-2">Lot</th>
                    <th className="p-2">Avg Price</th>
                  </tr>
                </thead>
                <tbody>
                  {localHoldings.map((h, idx) => (
                    <tr key={idx} className="border-t border-slate-300">
                      <td className="p-2 pl-3 font-semibold">{h.ticker}</td>
                      <td className="p-2">{h.lot}</td>
                      <td className="p-2">{h.price}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <form
            className="grid gap-3 rounded-xl border border-slate-300 bg-slate-50 p-4"
            onSubmit={addLocalHolding}
          >
            <div className="grid grid-cols-3 gap-3">
              <div className="grid gap-1">
                <label className="text-xs text-slate-600">Ticker</label>
                <input
                  value={holdingTicker}
                  onChange={(e) => setHoldingTicker(e.target.value)}
                  type="text"
                  placeholder="BBCA"
                  required
                  className="rounded-md border border-slate-300 px-2 py-1.5 text-sm uppercase"
                />
              </div>
              <div className="grid gap-1">
                <label className="text-xs text-slate-600">Lot</label>
                <input
                  value={holdingLot}
                  onChange={(e) => setHoldingLot(Number(e.target.value))}
                  type="number"
                  min="1"
                  required
                  className="rounded-md border border-slate-300 px-2 py-1.5 text-sm"
                />
              </div>
              <div className="grid gap-1">
                <label className="text-xs text-slate-600">Avg Price</label>
                <input
                  value={holdingPrice}
                  onChange={(e) => setHoldingPrice(Number(e.target.value))}
                  type="number"
                  min="1"
                  step="0.01"
                  required
                  className="rounded-md border border-slate-300 px-2 py-1.5 text-sm"
                />
              </div>
            </div>
            <button
              type="submit"
              className="w-full rounded-lg bg-slate-200 px-4 py-1.5 text-sm font-semibold text-slate-700 hover:bg-slate-300"
            >
              + Tambah Saham
            </button>
          </form>
        </section>
      )}

      {isBrokerAdded && (
        <div className="flex justify-end pt-2">
          <button
            onClick={finishSetup}
            className="rounded-lg bg-teal-700 px-6 py-3 font-bold text-white shadow-lg hover:bg-teal-800"
          >
            Selesai &amp; Buka Dashboard →
          </button>
        </div>
      )}
    </div>
  )
}
