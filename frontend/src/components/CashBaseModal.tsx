import { useEffect, useState } from 'react'
import { ChevronDown, ChevronUp, X } from 'lucide-react'
import { usePortfolio } from '../context/PortfolioContext'
import { formatCurrency } from '../lib/formatters'
import type { Broker, CashBreakdown } from '../types'

interface CashBaseModalProps {
  broker: Broker
  onClose: () => void
}

/** Format Date → nilai input datetime-local (YYYY-MM-DDTHH:mm). */
function toLocalInput(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const EVENT_BADGE: Record<string, string> = {
  BUY: 'bg-rose-100 text-rose-800',
  SELL: 'bg-emerald-100 text-emerald-800',
  DEPOSIT: 'bg-emerald-100 text-emerald-800',
  DIVIDEND: 'bg-teal-100 text-teal-800',
  WITHDRAW: 'bg-rose-100 text-rose-800',
}

/**
 * Modal ringkas "Atur Saldo Kas": user menetapkan saldo dasar pada waktu
 * tertentu, lalu cash = saldo dasar + total event setelah waktu itu.
 * Riwayat sebelum cutoff dianggap sudah termuat di saldo dasar.
 */
export default function CashBaseModal({ broker, onClose }: CashBaseModalProps) {
  const { getCashBreakdown, setBrokerCashBase } = usePortfolio()

  const [cashBase, setCashBase] = useState('')
  const [asOfLocal, setAsOfLocal] = useState(toLocalInput(new Date()))
  const [breakdown, setBreakdown] = useState<CashBreakdown | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [showEvents, setShowEvents] = useState(false)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    getCashBreakdown(broker.id).then((res) => {
      if (cancelled) return
      setBreakdown(res)
      if (res) {
        setCashBase(String(res.cash_base ?? 0))
        setAsOfLocal(res.cash_base_as_of ? toLocalInput(new Date(res.cash_base_as_of)) : toLocalInput(new Date()))
      }
      setLoading(false)
    })
    return () => {
      cancelled = true
    }
  }, [broker.id, getCashBreakdown])

  // Tutup dengan tombol Escape.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const baseActive = Boolean(breakdown?.cash_base_as_of)
  const events = breakdown?.events ?? []
  const eventsTotal = breakdown?.events_total ?? 0
  const projectedCash = (Number(cashBase) || 0) + eventsTotal

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    const baseValue = Number(cashBase)
    if (Number.isNaN(baseValue) || baseValue < 0) return
    setSaving(true)
    const iso = asOfLocal ? new Date(asOfLocal).toISOString() : new Date().toISOString()
    const res = await setBrokerCashBase(broker.id, baseValue, iso)
    if (res) setBreakdown(res)
    setSaving(false)
  }

  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center bg-slate-900/50 p-4"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label={`Atur saldo kas ${broker.name}`}
    >
      <div
        className="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-2xl border border-slate-300 bg-white p-5 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-3">
          <div>
            <h3 className="text-lg font-semibold text-slate-800">Atur Saldo Kas</h3>
            <p className="text-sm text-slate-500">
              {broker.name}
              {!loading && (
                <span className="ml-2 text-slate-400">· cash saat ini {formatCurrency(breakdown?.cash ?? broker.cash)}</span>
              )}
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Tutup"
            className="rounded p-1 text-slate-400 transition hover:bg-slate-100 hover:text-slate-600"
          >
            <X size={18} />
          </button>
        </div>

        <p className="mt-3 rounded-lg bg-slate-50 p-2.5 text-xs leading-relaxed text-slate-500">
          Cash = <strong className="text-slate-700">saldo dasar</strong> (is manual, mis. saldo RDN terakhir) +{' '}
          <strong className="text-slate-700">event setelah waktu dasar</strong>. Riwayat sebelum waktu dasar dianggap
          sudah termuat di saldo dasar.
        </p>

        {loading ? (
          <p className="mt-4 text-sm text-slate-500">Memuat rincian saldo...</p>
        ) : (
          <form onSubmit={handleSave}>
            <div className="mt-4 grid gap-3 sm:grid-cols-2">
              <div className="grid gap-1">
                <label className="text-xs font-semibold text-slate-700" htmlFor="cash-base-amount">
                  Saldo Dasar (Rp)
                </label>
                <input
                  id="cash-base-amount"
                  type="number"
                  min="0"
                  step="0.01"
                  value={cashBase}
                  onChange={(e) => setCashBase(e.target.value)}
                  className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-teal-600 focus:outline-none"
                  placeholder="0"
                  autoFocus
                />
              </div>
              <div className="grid gap-1">
                <label className="text-xs font-semibold text-slate-700" htmlFor="cash-base-asof">
                  Berlaku Sejak
                </label>
                <input
                  id="cash-base-asof"
                  type="datetime-local"
                  value={asOfLocal}
                  onChange={(e) => setAsOfLocal(e.target.value)}
                  className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-teal-600 focus:outline-none"
                />
              </div>
            </div>

            {/* Ringkasan: dasar + event = cash */}
            <div className="mt-3 grid grid-cols-3 gap-2 text-center">
              <div className="rounded-lg border border-slate-200 bg-slate-50 px-2 py-2">
                <div className="text-[10px] font-semibold uppercase tracking-wide text-slate-500">Saldo Dasar</div>
                <div className="mt-0.5 text-sm font-semibold text-slate-800">
                  {formatCurrency(Number(cashBase) || 0)}
                </div>
              </div>
              <div className="rounded-lg border border-slate-200 bg-slate-50 px-2 py-2">
                <div className="text-[10px] font-semibold uppercase tracking-wide text-slate-500">
                  Event ({events.length})
                </div>
                <div
                  className={`mt-0.5 text-sm font-semibold ${eventsTotal >= 0 ? 'text-emerald-700' : 'text-rose-700'}`}
                >
                  {eventsTotal >= 0 ? '+' : '-'}
                  {formatCurrency(Math.abs(eventsTotal))}
                </div>
              </div>
              <div className="rounded-lg border border-teal-200 bg-teal-50 px-2 py-2">
                <div className="text-[10px] font-semibold uppercase tracking-wide text-teal-700">Cash</div>
                <div className="mt-0.5 text-sm font-bold text-teal-800">{formatCurrency(projectedCash)}</div>
              </div>
            </div>

            {!baseActive && (
              <p className="mt-2 text-xs text-amber-700">
                Belum ada saldo dasar — cash masih dihitung dari replay seluruh riwayat.
              </p>
            )}

            {/* Rincian event (opsional) */}
            {events.length > 0 && (
              <div className="mt-3">
                <button
                  type="button"
                  onClick={() => setShowEvents((v) => !v)}
                  className="inline-flex items-center gap-1 text-xs font-semibold text-teal-700 hover:text-teal-800"
                >
                  {showEvents ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                  {showEvents ? 'Sembunyikan' : 'Lihat'} event setelah dasar ({events.length})
                </button>
                {showEvents && (
                  <div className="mt-2 max-h-44 overflow-auto rounded-lg border border-slate-200">
                    <table className="w-full border-collapse text-xs">
                      <thead className="sticky top-0 bg-slate-50 text-slate-500">
                        <tr>
                          <th className="border-b border-slate-200 p-2 text-left font-semibold">Tanggal</th>
                          <th className="border-b border-slate-200 p-2 text-left font-semibold">Keterangan</th>
                          <th className="border-b border-slate-200 p-2 text-right font-semibold">Dampak</th>
                        </tr>
                      </thead>
                      <tbody>
                        {events.map((ev, i) => (
                          <tr key={`${ev.date}-${i}`} className="hover:bg-slate-50/60">
                            <td className="border-b border-slate-100 p-2 whitespace-nowrap text-slate-500">
                              {new Date(ev.date).toLocaleDateString('id-ID')}
                            </td>
                            <td className="border-b border-slate-100 p-2">
                              <span
                                className={`mr-1.5 rounded px-1.5 py-0.5 text-[10px] font-bold ${
                                  EVENT_BADGE[ev.type] ?? 'bg-slate-100 text-slate-600'
                                }`}
                              >
                                {ev.type}
                              </span>
                              <span className="text-slate-700">{ev.description}</span>
                            </td>
                            <td
                              className={`border-b border-slate-100 p-2 text-right font-mono font-semibold ${
                                ev.delta >= 0 ? 'text-emerald-700' : 'text-rose-700'
                              }`}
                            >
                              {ev.delta >= 0 ? '+' : '-'}
                              {formatCurrency(Math.abs(ev.delta))}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            )}

            <div className="mt-4 flex justify-end gap-2 border-t border-slate-200 pt-3">
              <button
                type="button"
                onClick={onClose}
                className="rounded-lg border border-slate-300 bg-white px-3.5 py-2 text-xs font-semibold text-slate-600 transition hover:bg-slate-100"
              >
                Batal
              </button>
              <button
                type="submit"
                disabled={saving}
                className="rounded-lg bg-teal-700 px-4 py-2 text-xs font-semibold text-white transition hover:bg-teal-800 disabled:opacity-50"
              >
                {saving ? 'Menyimpan...' : 'Simpan Saldo Dasar'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}
