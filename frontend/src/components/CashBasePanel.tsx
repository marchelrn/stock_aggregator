import { useEffect, useState } from 'react'
import { usePortfolio } from '../context/PortfolioContext'
import { formatCurrency } from '../lib/formatters'
import type { Broker, CashBreakdown } from '../types'

interface CashBasePanelProps {
  brokers: Broker[]
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
 * Panel "Atur Saldo Kas": user menetapkan saldo dasar (cash_base) pada
 * waktu tertentu; selanjutnya cash = cash_base + total event setelah waktu
 * itu. Riwayat sebelum cutoff dianggap sudah termuat di saldo dasar.
 */
export default function CashBasePanel({ brokers }: CashBasePanelProps) {
  const { getCashBreakdown, setBrokerCashBase } = usePortfolio()

  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [cashBase, setCashBase] = useState('')
  const [asOfLocal, setAsOfLocal] = useState('')
  const [breakdown, setBreakdown] = useState<CashBreakdown | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  // Pilih broker pertama secara default.
  useEffect(() => {
    if (selectedId === null && brokers.length > 0) {
      setSelectedId(Number(brokers[0].id))
    }
  }, [brokers, selectedId])

  // Muat rincian + isi form setiap kali broker terpilih berganti.
  useEffect(() => {
    if (selectedId === null) return
    let cancelled = false
    setLoading(true)
    getCashBreakdown(selectedId).then((res) => {
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
  }, [selectedId, getCashBreakdown])

  if (brokers.length === 0) return null

  const baseActive = Boolean(breakdown?.cash_base_as_of)
  const eventsCount = breakdown?.events.length ?? 0

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (selectedId === null) return
    const baseValue = Number(cashBase)
    if (Number.isNaN(baseValue) || baseValue < 0) {
      return
    }
    setSaving(true)
    const iso = asOfLocal ? new Date(asOfLocal).toISOString() : new Date().toISOString()
    const res = await setBrokerCashBase(selectedId, baseValue, iso)
    if (res) setBreakdown(res)
    setSaving(false)
  }

  const resetForm = () => {
    if (!breakdown) return
    setCashBase(String(breakdown.cash_base ?? 0))
    setAsOfLocal(
      breakdown.cash_base_as_of ? toLocalInput(new Date(breakdown.cash_base_as_of)) : toLocalInput(new Date()),
    )
  }

  return (
    <section className="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-xl font-semibold">Atur Saldo Kas</h2>
        {baseActive && (
          <span className="rounded bg-teal-50 px-2 py-0.5 text-xs font-semibold text-teal-800 ring-1 ring-teal-200">
            Saldo dasar aktif
          </span>
        )}
      </div>
      <p className="mt-1 text-sm text-slate-500">
        Cash = <strong>Saldo Dasar</strong> (diatur manual, misalnya dari mutasi terakhir di aplikasi broker) + total
        event (trade, deposit, penarikan, dividen) <strong>setelah</strong> waktu dasar. Riwayat sebelum waktu dasar
        dianggap sudah termuat di saldo dasar.
      </p>

      <div className="mt-3 grid gap-3 md:grid-cols-3">
        {/* Daftar broker */}
        <div className="space-y-1.5">
          {brokers.map((b) => {
            const active = Number(b.id) === selectedId
            return (
              <button
                key={b.id}
                type="button"
                onClick={() => setSelectedId(Number(b.id))}
                className={`w-full rounded-lg border px-3 py-2 text-left transition ${
                  active
                    ? 'border-teal-600 bg-teal-50 ring-1 ring-teal-200'
                    : 'border-slate-200 bg-white hover:bg-slate-50'
                }`}
              >
                <span className="block text-sm font-semibold text-slate-800">{b.name}</span>
                <span className="block text-xs text-slate-500">Cash: {formatCurrency(b.cash)}</span>
              </button>
            )
          })}
        </div>

        {/* Form + rincian */}
        <div className="md:col-span-2">
          {loading ? (
            <div className="rounded-xl border border-slate-200 bg-slate-50 p-4 text-sm text-slate-500">
              Memuat rincian saldo...
            </div>
          ) : (
            <form onSubmit={handleSave} className="rounded-xl border border-slate-200 bg-slate-50 p-3">
              <div className="grid gap-3 sm:grid-cols-2">
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

              <div className="mt-3 rounded-lg border border-slate-200 bg-white p-3 text-sm">
                <div className="flex items-center justify-between">
                  <span className="text-slate-600">Saldo dasar</span>
                  <span className="font-semibold text-slate-800">
                    {formatCurrency(Number(cashBase) || 0)}
                  </span>
                </div>
                <div className="mt-1 flex items-center justify-between">
                  <span className="text-slate-600">
                    Event setelah dasar ({eventsCount})
                  </span>
                  <span
                    className={`font-semibold ${
                      (breakdown?.events_total ?? 0) >= 0 ? 'text-emerald-700' : 'text-rose-700'
                    }`}
                  >
                    {formatCurrency(breakdown?.events_total ?? 0)}
                  </span>
                </div>
                <div className="mt-2 flex items-center justify-between border-t border-slate-200 pt-2">
                  <span className="font-semibold text-slate-700">Cash</span>
                  <span className="font-bold text-teal-800">{formatCurrency(breakdown?.cash ?? 0)}</span>
                </div>
              </div>

              {!baseActive && (
                <p className="mt-2 rounded-lg border border-amber-200 bg-amber-50 p-2 text-xs text-amber-800">
                  Belum ada saldo dasar untuk broker ini — cash masih dihitung dari replay seluruh riwayat.
                </p>
              )}

              <div className="mt-3 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={resetForm}
                  className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-xs font-semibold text-slate-600 hover:bg-slate-100"
                >
                  Kembalikan
                </button>
                <button
                  type="submit"
                  disabled={saving}
                  className="rounded-lg bg-teal-700 px-4 py-2 text-xs font-semibold text-white hover:bg-teal-800 disabled:opacity-50"
                >
                  {saving ? 'Menyimpan...' : 'Simpan Saldo Dasar'}
                </button>
              </div>
            </form>
          )}

          {breakdown && breakdown.events.length > 0 && (
            <div className="mt-3 max-h-56 overflow-auto rounded-xl border border-slate-200">
              <table className="w-full border-collapse text-xs">
                <thead className="sticky top-0 bg-slate-50 text-slate-500">
                  <tr>
                    <th className="border-b border-slate-200 p-2 text-left font-semibold">Tanggal</th>
                    <th className="border-b border-slate-200 p-2 text-left font-semibold">Keterangan</th>
                    <th className="border-b border-slate-200 p-2 text-right font-semibold">Dampak Kas</th>
                  </tr>
                </thead>
                <tbody>
                  {breakdown.events.map((ev, i) => (
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
      </div>
    </section>
  )
}
