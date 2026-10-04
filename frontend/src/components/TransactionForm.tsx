import { useEffect, useMemo, useState } from 'react'
import { api } from '../lib/api'
import type { Broker, Holding, TransactionPayload } from '../types'

interface TransactionFormProps {
  brokers?: Broker[]
  holdings?: Holding[]
  onSubmit?: (payload: TransactionPayload) => void | Promise<void>
}

export default function TransactionForm({
  brokers = [],
  holdings = [],
  onSubmit,
}: TransactionFormProps) {
  const [type, setType] = useState('BUY')
  const [ticker, setTicker] = useState('')
  const [brokerId, setBrokerId] = useState<number | null>(null)
  const [lot, setLot] = useState(1)
  const [price, setPrice] = useState(0)
  const [submitting, setSubmitting] = useState(false)
  const [validationMessage, setValidationMessage] = useState('')
  const [validationOk, setValidationOk] = useState(false)

  const normalizedTicker = useMemo(() => String(ticker || '').trim().toUpperCase(), [ticker])

  const sellRows = useMemo(
    () =>
      (holdings || []).map((h) => ({
        ticker: String(h.ticker || '').toUpperCase(),
        broker_id: Number(h.broker_id),
        broker_name: h.broker_name,
        avg_price: Number(h.avg_price || 0),
      })),
    [holdings],
  )

  const brokerOptions = useMemo(() => {
    if (type === 'BUY') return brokers || []
    const ids = new Set(sellRows.map((r) => r.broker_id))
    return (brokers || []).filter((b) => ids.has(Number(b.id)))
  }, [type, brokers, sellRows])

  // Reset validation when the type changes; preselect a broker for SELL.
  useEffect(() => {
    setValidationMessage('')
    setValidationOk(false)
    if (type === 'SELL') {
      const first = brokerOptions[0]
      setBrokerId(first ? Number(first.id) : null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [type])

  // Keep the selected broker valid as the option list changes.
  useEffect(() => {
    if (!brokerOptions.some((b) => Number(b.id) === Number(brokerId))) {
      setBrokerId(brokerOptions[0] ? Number(brokerOptions[0].id) : null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [brokerOptions])

  // Live SELL validation against the selected broker's holdings.
  useEffect(() => {
    if (!normalizedTicker) {
      setValidationMessage('')
      setValidationOk(false)
      return
    }
    if (type === 'SELL') {
      const row = sellRows.find(
        (r) => r.ticker === normalizedTicker && Number(r.broker_id) === Number(brokerId),
      )
      if (row) {
        setValidationOk(true)
        setValidationMessage('Ticker tersedia untuk SELL di broker ini')
      } else {
        setValidationOk(false)
        setValidationMessage('Ticker ini tidak ada di holding broker terpilih')
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [type, brokerId, normalizedTicker])

  const validateBuyTicker = async (): Promise<boolean> => {
    if (type !== 'BUY') return true
    if (!normalizedTicker) return false
    try {
      await api(`/price/${encodeURIComponent(normalizedTicker)}`)
      setValidationOk(true)
      setValidationMessage('Ticker valid (terdaftar / bisa diambil dari IHSG via Yahoo)')
      return true
    } catch {
      setValidationOk(false)
      setValidationMessage('Ticker tidak valid / tidak ditemukan di IHSG')
      return false
    }
  }

  const fail = (message: string): false => {
    setValidationOk(false)
    setValidationMessage(message)
    return false
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (submitting) return
    if (!brokerId) return fail('Broker wajib dipilih')
    if (!normalizedTicker) return fail('Ticker wajib diisi')

    const lotValue = Number(lot)
    if (!Number.isFinite(lotValue) || lotValue <= 0) return fail('Lot harus lebih besar dari 0')

    const priceValue = Number(price)
    if (!Number.isFinite(priceValue) || priceValue <= 0)
      return fail('Harga per lembar harus lebih besar dari 0')

    const broker = (brokers || []).find((b) => Number(b.id) === Number(brokerId))
    if (!broker) return fail('Broker tidak ditemukan')

    if (type === 'BUY') {
      const ok = await validateBuyTicker()
      if (!ok) return
    }

    if (type === 'SELL') {
      const exists = sellRows.some(
        (r) => r.ticker === normalizedTicker && Number(r.broker_id) === Number(brokerId),
      )
      if (!exists) return fail('SELL ditolak: ticker tidak ada di holding broker ini')
    }

    setSubmitting(true)
    try {
      await onSubmit?.({
        type,
        ticker: normalizedTicker,
        broker_id: Number(broker.id),
        broker_name: broker.name,
        lot: lotValue,
        price: priceValue,
      })
      setTicker('')
      setLot(1)
      setPrice(0)
      setValidationMessage('')
      setValidationOk(false)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form
      className="grid content-start gap-2 rounded-xl border border-slate-300 bg-white p-3"
      onSubmit={handleSubmit}
    >
      <h3 className="text-base font-semibold">Add Transaction</h3>

      {validationMessage && (
        <small className={validationOk ? 'text-emerald-700' : 'text-rose-700'}>
          {validationMessage}
        </small>
      )}

      <label className="text-sm text-slate-600" htmlFor="tx-type">
        Type
      </label>
      <select
        id="tx-type"
        value={type}
        onChange={(e) => setType(e.target.value)}
        required
        className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
      >
        <option value="BUY">BUY</option>
        <option value="SELL">SELL</option>
      </select>

      <label className="text-sm text-slate-600" htmlFor="tx-broker">
        Broker
      </label>
      <select
        id="tx-broker"
        value={brokerId ?? ''}
        onChange={(e) => setBrokerId(e.target.value === '' ? null : Number(e.target.value))}
        required
        className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
      >
        {brokerOptions.length === 0 ? (
          <option value="">Tidak ada broker tersedia</option>
        ) : (
          brokerOptions.map((broker) => (
            <option key={broker.id} value={broker.id}>
              {broker.name} (ID: {broker.id})
            </option>
          ))
        )}
      </select>

      <label className="text-sm text-slate-600" htmlFor="tx-ticker">
        Ticker
      </label>
      <input
        id="tx-ticker"
        value={ticker}
        onChange={(e) => setTicker(e.target.value)}
        onBlur={validateBuyTicker}
        type="text"
        placeholder="Contoh: BBRI"
        required
        className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
      />

      <label className="text-sm text-slate-600" htmlFor="tx-lot">
        Lot
      </label>
      <input
        id="tx-lot"
        value={lot}
        onChange={(e) => setLot(Number(e.target.value))}
        type="number"
        required
        className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
      />

      <label className="text-sm text-slate-600" htmlFor="tx-price">
        Harga per lembar
      </label>
      <input
        id="tx-price"
        value={price}
        onChange={(e) => setPrice(Number(e.target.value))}
        type="number"
        step="0.01"
        required
        className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
      />

      <button
        type="submit"
        disabled={submitting}
        className="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800 disabled:cursor-not-allowed disabled:opacity-60"
      >
        {submitting ? 'Submitting...' : 'Submit Transaction'}
      </button>
    </form>
  )
}
