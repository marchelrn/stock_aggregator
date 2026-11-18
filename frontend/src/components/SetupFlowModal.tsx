import { useEffect, useRef, useState } from 'react'
import { AlertCircle, Check, Loader2, Plus, Trash2, X } from 'lucide-react'
import { formatCurrency } from '../lib/formatters'
import type { Broker, Holding } from '../types'

export type StepStatus = 'pending' | 'active' | 'done' | 'failed' | 'partial'

export interface SetupStep {
  key: string
  label: string
  status: StepStatus
  note?: string
}

export interface ManualHolding {
  ticker: string
  lot: number
  price: number
  brokerId: number
  brokerName: string
}

interface SetupFlowModalProps {
  stage: 'sync' | 'portfolio'
  running: boolean
  steps: SetupStep[]
  elapsed: number
  tradeSummary: string
  cashSummary: string
  tradeError: string
  cashError: string
  holdings: Holding[]
  brokers: Broker[]
  manualHoldings: ManualHolding[]
  finishing: boolean
  finishError: string
  onContinue: () => void
  onFinish: () => void
  onClose: () => void
  onAddManual: (holding: ManualHolding) => void
  onRemoveManual: (index: number) => void
}

function StepIcon({ status }: { status: StepStatus }) {
  if (status === 'active') {
    return <Loader2 className="h-4 w-4 animate-spin text-teal-700 motion-reduce:animate-none" aria-hidden="true" />
  }
  if (status === 'done') {
    return <Check className="h-4 w-4 text-emerald-700" aria-hidden="true" />
  }
  if (status === 'failed') {
    return <X className="h-4 w-4 text-rose-700" aria-hidden="true" />
  }
  if (status === 'partial') {
    return <AlertCircle className="h-4 w-4 text-amber-700" aria-hidden="true" />
  }
  return <span className="h-4 w-4 rounded-full border border-slate-300" aria-hidden="true" />
}

const stepTone: Record<StepStatus, string> = {
  pending: 'text-slate-500',
  active: 'text-slate-800',
  done: 'text-slate-700',
  failed: 'text-rose-800',
  partial: 'text-amber-800',
}

export default function SetupFlowModal({
  stage,
  running,
  steps,
  elapsed,
  tradeSummary,
  cashSummary,
  tradeError,
  cashError,
  holdings,
  brokers,
  manualHoldings,
  finishing,
  finishError,
  onContinue,
  onFinish,
  onClose,
  onAddManual,
  onRemoveManual,
}: SetupFlowModalProps) {
  const panelRef = useRef<HTMLDivElement>(null)
  const bodyRef = useRef<HTMLDivElement>(null)
  const liveRef = useRef({ running, finishing, onClose })
  liveRef.current = { running, finishing, onClose }

  const [brokerId, setBrokerId] = useState<number>(brokers[0]?.id ? Number(brokers[0].id) : 0)
  const [ticker, setTicker] = useState('')
  const [lot, setLot] = useState('1')
  const [price, setPrice] = useState('')

  // Broker bisa selesai dimuat setelah modal terbuka; samakan pilihan default.
  useEffect(() => {
    if (brokers.length === 0) return
    if (brokers.some((broker) => Number(broker.id) === brokerId)) return
    setBrokerId(Number(brokers[0].id))
  }, [brokers, brokerId])

  useEffect(() => {
    const opener = document.activeElement
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    panelRef.current?.focus()

    // Escape dan kunci scroll ditangani di sini karena overlay ini bukan
    // elemen dialog native.
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      if (liveRef.current.running || liveRef.current.finishing) return
      event.preventDefault()
      liveRef.current.onClose()
    }
    document.addEventListener('keydown', handleKeyDown)

    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      document.body.style.overflow = previousOverflow
      if (opener instanceof HTMLElement && opener.isConnected) opener.focus()
    }
  }, [])

  useEffect(() => {
    bodyRef.current?.scrollTo({ top: 0 })
  }, [stage])

  const selectedBroker = brokers.find((broker) => Number(broker.id) === brokerId)

  const submitManual = (event: React.FormEvent) => {
    event.preventDefault()
    const lotValue = Number(lot)
    const priceValue = Number(price)
    if (!selectedBroker || !ticker.trim() || lotValue <= 0 || priceValue <= 0) return
    onAddManual({
      ticker: ticker.trim().toUpperCase(),
      lot: lotValue,
      price: priceValue,
      brokerId: Number(selectedBroker.id),
      brokerName: selectedBroker.name,
    })
    setTicker('')
    setLot('1')
    setPrice('')
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 p-4"
      onClick={(event) => {
        if (event.target !== event.currentTarget || running || finishing) return
        onClose()
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="setup-modal-title"
        tabIndex={-1}
        onKeyDown={(event) => {
          if (event.key !== 'Tab') return
          const controls = event.currentTarget.querySelectorAll<HTMLElement>(
            'a[href], button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled)',
          )
          const first = controls[0]
          const last = controls[controls.length - 1]
          if (!first) {
            event.preventDefault()
          } else if (event.shiftKey && document.activeElement === first) {
            event.preventDefault()
            last.focus()
          } else if (!event.shiftKey && document.activeElement === last) {
            event.preventDefault()
            first.focus()
          }
        }}
        className="flex max-h-[calc(100dvh_-_2rem)] w-full max-w-xl flex-col overflow-hidden rounded-2xl border border-slate-300 bg-white text-slate-800 shadow-xl focus:outline-none"
      >
      <div className="flex items-start justify-between gap-3 border-b border-slate-200 px-5 py-4">
        <div className="min-w-0">
          <h2 id="setup-modal-title" className="text-lg font-semibold">
            {stage === 'sync' ? 'Sinkronisasi riwayat' : 'Portofolio awal'}
          </h2>
          <p className="mt-1 text-sm text-slate-600">
            {stage === 'sync'
              ? 'Email transaksi dan mutasi kas sedang dibaca dari Gmail.'
              : 'Periksa saham yang berhasil dibaca, lalu lengkapi yang belum terbaca.'}
          </p>
        </div>
        <button
          type="button"
          aria-label="Tutup"
          onClick={onClose}
          disabled={running || finishing}
          className="-mr-2 -mt-1 inline-flex min-h-11 min-w-11 shrink-0 cursor-pointer items-center justify-center rounded-lg text-slate-600 hover:bg-slate-100 active:bg-slate-200 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-40"
        >
          <X size={18} aria-hidden="true" />
        </button>
      </div>

      <div ref={bodyRef} className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
        {stage === 'sync' ? (
          <div aria-live="polite">
            <ol className="grid gap-3">
              {steps.map((step) => (
                <li key={step.key} className="flex items-start gap-2 text-sm">
                  <span className="mt-0.5 shrink-0">
                    <StepIcon status={step.status} />
                  </span>
                  <span className="min-w-0">
                    <span className={`block ${stepTone[step.status]}`}>{step.label}</span>
                    {step.note && <span className="block text-xs text-slate-500">{step.note}</span>}
                    {step.key === 'sync' && tradeSummary && (
                      <span className="block text-xs text-slate-500">{tradeSummary}</span>
                    )}
                    {step.key === 'sync' && cashSummary && (
                      <span className="block text-xs text-slate-500">{cashSummary}</span>
                    )}
                  </span>
                </li>
              ))}
            </ol>

            {running && (
              <p className="mt-4 rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-600">
                Sudah berjalan {elapsed} detik. Durasi tergantung jumlah email, bisa beberapa
                menit. Biarkan jendela ini terbuka.
              </p>
            )}

            {(tradeError || cashError) && (
              <div className="mt-4 grid gap-1 rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-800">
                {tradeError && <p>Transaksi: {tradeError}</p>}
                {cashError && <p>Mutasi kas: {cashError}</p>}
              </div>
            )}
          </div>
        ) : (
          <div className="grid gap-5">
            {(tradeError || cashError) && (
              <div role="alert" className="grid gap-1 rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-800">
                <p className="font-semibold">Sinkronisasi tidak lengkap.</p>
                {tradeError && <p>Transaksi: {tradeError}</p>}
                {cashError && <p>Mutasi kas: {cashError}</p>}
              </div>
            )}

            <section>
              <div className="flex flex-wrap items-baseline justify-between gap-2">
                <h3 className="text-sm font-semibold text-slate-800">Hasil Parsing Email</h3>
                <span className="text-xs tabular-nums text-slate-500">
                  {holdings.length} posisi
                </span>
              </div>

              {holdings.length > 0 ? (
                <ul className="mt-2 rounded-xl border border-slate-300">
                  {holdings.map((holding, index) => (
                    <li
                      key={`${holding.broker_id}-${holding.ticker}-${index}`}
                      className="flex items-center justify-between gap-3 border-b border-slate-200 px-3 py-2 last:border-b-0"
                    >
                      <span className="min-w-0">
                        <span className="block text-sm font-semibold">{holding.ticker}</span>
                        <span className="block truncate text-xs text-slate-500">
                          {holding.broker_name}
                        </span>
                      </span>
                      <span className="shrink-0 text-right text-xs tabular-nums text-slate-600">
                        <span className="block">{holding.lot} lot</span>
                        <span className="block">{formatCurrency(holding.avg_price)}</span>
                      </span>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="mt-2 rounded-xl border border-dashed border-slate-300 px-3 py-4 text-sm text-slate-600">
                  Belum ada saham yang terbaca otomatis dari email.
                </p>
              )}
            </section>

            <section>
              <h3 className="text-sm font-semibold text-slate-800">
                Tambahkan saham yang belum terbaca
              </h3>
              <form onSubmit={submitManual} className="mt-2 grid gap-3">
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="grid gap-1 text-xs text-slate-600">
                    Broker
                    <select
                      value={brokerId}
                      onChange={(event) => setBrokerId(Number(event.target.value))}
                      disabled={finishing || brokers.length === 0}
                      className="min-h-11 w-full min-w-0 rounded-lg border border-slate-300 bg-white px-2 text-sm text-slate-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:bg-slate-50"
                    >
                      {brokers.map((broker) => (
                        <option key={broker.id} value={broker.id}>
                          {broker.name}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="grid gap-1 text-xs text-slate-600">
                    Ticker
                    <input
                      value={ticker}
                      onChange={(event) => setTicker(event.target.value)}
                      type="text"
                      placeholder="BBCA"
                      required
                      autoComplete="off"
                      disabled={finishing}
                      className="min-h-11 w-full min-w-0 rounded-lg border border-slate-300 px-2 text-sm uppercase text-slate-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:bg-slate-50"
                    />
                  </label>
                  <label className="grid gap-1 text-xs text-slate-600">
                    Lot
                    <input
                      value={lot}
                      onChange={(event) => setLot(event.target.value)}
                      type="number"
                      min="1"
                      required
                      disabled={finishing}
                      className="min-h-11 w-full min-w-0 rounded-lg border border-slate-300 px-2 text-sm tabular-nums text-slate-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:bg-slate-50"
                    />
                  </label>
                  <label className="grid gap-1 text-xs text-slate-600">
                    Harga rata-rata
                    <input
                      value={price}
                      onChange={(event) => setPrice(event.target.value)}
                      type="number"
                      min="1"
                      step="0.01"
                      required
                      disabled={finishing}
                      className="min-h-11 w-full min-w-0 rounded-lg border border-slate-300 px-2 text-sm tabular-nums text-slate-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:bg-slate-50"
                    />
                  </label>
                </div>
                <button
                  type="submit"
                  disabled={finishing || !selectedBroker}
                  className="inline-flex min-h-11 w-full cursor-pointer items-center justify-center gap-1.5 rounded-lg bg-slate-200 px-4 text-sm font-semibold text-slate-700 hover:bg-slate-300 active:bg-slate-400 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  Tambah saham
                </button>
              </form>

              {manualHoldings.length > 0 && (
                <ul className="mt-3 rounded-xl border border-slate-300 bg-slate-50">
                  {manualHoldings.map((holding, index) => (
                    <li
                      key={`${holding.ticker}-${index}`}
                      className="flex items-center justify-between gap-3 border-b border-slate-200 px-3 py-2 last:border-b-0"
                    >
                      <span className="min-w-0">
                        <span className="block text-sm font-semibold">{holding.ticker}</span>
                        <span className="block truncate text-xs text-slate-500">
                          {holding.brokerName} · {holding.lot} lot · {formatCurrency(holding.price)}
                        </span>
                      </span>
                      <button
                        type="button"
                        onClick={() => onRemoveManual(index)}
                        disabled={finishing}
                        aria-label={`Hapus ${holding.ticker}`}
                        className="inline-flex min-h-11 min-w-11 shrink-0 cursor-pointer items-center justify-center rounded-lg text-slate-500 hover:bg-rose-50 hover:text-rose-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-40"
                      >
                        <Trash2 size={16} aria-hidden="true" />
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          </div>
        )}
      </div>

      {finishError && (
        <p role="alert" className="border-t border-rose-200 bg-rose-50 px-5 py-3 text-sm text-rose-800">
          {finishError}
        </p>
      )}

      <div className="flex flex-col-reverse gap-2 border-t border-slate-200 px-5 py-4 sm:flex-row sm:justify-end">
        {stage === 'portfolio' && (
          <button
            type="button"
            onClick={onClose}
            disabled={finishing}
            className="min-h-11 cursor-pointer rounded-lg border border-slate-300 px-4 text-sm font-semibold text-slate-700 hover:bg-slate-50 active:bg-slate-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            Tutup
          </button>
        )}
        {stage === 'sync' ? (
          <button
            type="button"
            onClick={onContinue}
            disabled={running}
            className="min-h-11 cursor-pointer rounded-lg bg-teal-700 px-4 text-sm font-semibold text-white hover:bg-teal-800 active:bg-teal-900 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {running ? 'Sinkronisasi berjalan...' : 'Lihat portofolio awal'}
          </button>
        ) : (
          <button
            type="button"
            onClick={onFinish}
            disabled={finishing}
            className="min-h-11 cursor-pointer rounded-lg bg-teal-700 px-4 text-sm font-semibold text-white hover:bg-teal-800 active:bg-teal-900 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {finishing ? 'Menyimpan...' : 'Dashboard'}
          </button>
        )}
      </div>
      </div>
    </div>
  )
}
