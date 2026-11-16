import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { X } from 'lucide-react'
import { usePortfolio } from '../context/PortfolioContext'
import { cashInputCaret, formatCashAmount, formatCashInput, formatPastedCash, parseCashAmount } from '../lib/cashInput'
import type { Broker } from '../types'

interface CashBaseModalProps {
  broker: Broker
  onClose: () => void
}

export default function CashBaseModal({ broker, onClose }: CashBaseModalProps) {
  const { getCashBreakdown, setBrokerCashBase } = usePortfolio()
  const dialogRef = useRef<HTMLDialogElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const [cash, setCash] = useState('')
  const [loading, setLoading] = useState(true)
  const [loadFailed, setLoadFailed] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [retry, setRetry] = useState(0)

  useLayoutEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return
    const opener = document.activeElement
    dialog.showModal()
    return () => {
      dialog.close()
      if (opener instanceof HTMLElement && opener.isConnected) opener.focus()
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setLoadFailed(false)
    setError('')
    getCashBreakdown(broker.id).then((result) => {
      if (cancelled) return
      if (result) {
        setCash(formatCashAmount(result.cash))
      } else {
        setLoadFailed(true)
        setError('Saldo belum berhasil dimuat. Coba lagi.')
      }
      setLoading(false)
    })
    return () => { cancelled = true }
  }, [broker.id, getCashBreakdown, retry])

  useEffect(() => {
    if (!loading) inputRef.current?.focus()
  }, [loading])

  const handleSave = async (event: React.FormEvent) => {
    event.preventDefault()
    if (saving || loading) return
    const value = parseCashAmount(cash)
    if (value === null) {
      setError('Masukkan saldo Rp0 atau lebih.')
      inputRef.current?.focus()
      return
    }
    setError('')
    setSaving(true)
    try {
      const result = await setBrokerCashBase(broker.id, value)
      if (result) {
        onClose()
      } else {
        setError('Saldo belum tersimpan. Coba lagi.')
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <dialog
      ref={dialogRef}
      aria-labelledby="cash-modal-title"
      aria-describedby="cash-modal-description"
      onKeyDown={(event) => {
        if (event.key !== 'Tab') return
        const controls = event.currentTarget.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled)')
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
      onCancel={(event) => {
        event.preventDefault()
        if (!saving) onClose()
      }}
      onClick={(event) => {
        if (event.target !== event.currentTarget || saving) return
        const rect = event.currentTarget.getBoundingClientRect()
        if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) onClose()
      }}
      className="fixed m-auto max-h-[calc(100dvh_-_2rem)] w-[calc(100%_-_2rem)] max-w-sm overflow-y-auto rounded-2xl border border-slate-300 bg-white p-5 text-slate-800 shadow-xl backdrop:bg-slate-900/50"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 id="cash-modal-title" className="text-lg font-semibold">Atur saldo kas</h2>
          <p className="mt-1 break-words text-sm text-slate-600">{broker.name}</p>
        </div>
        <button
          type="button"
          aria-label="Tutup"
          onClick={onClose}
          disabled={saving}
          className="-mr-2 -mt-2 inline-flex min-h-11 min-w-11 shrink-0 cursor-pointer items-center justify-center rounded-lg text-slate-600 hover:bg-slate-100 active:bg-slate-200 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <X size={18} aria-hidden="true" />
        </button>
      </div>

      <p id="cash-modal-description" className="mt-4 text-sm leading-relaxed text-slate-600">
        Isi saldo RDN saat ini. Histori lama tidak mengubah saldo ini.
      </p>

      {loading ? (
        <p role="status" className="py-6 text-sm text-slate-600">Memuat saldo...</p>
      ) : loadFailed ? (
        <div className="mt-4">
          <p role="alert" className="text-sm text-rose-700">{error}</p>
          <button type="button" onClick={() => setRetry((current) => current + 1)}
            className="mt-2 min-h-11 cursor-pointer rounded-lg border border-slate-300 px-4 text-sm font-semibold hover:bg-slate-50 active:bg-slate-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700">
            Coba lagi
          </button>
        </div>
      ) : (
        <form onSubmit={handleSave} className="mt-5" aria-busy={saving}>
          <label htmlFor="cash-amount" className="block text-sm font-semibold text-slate-700">Saldo kas saat ini (Rp)</label>
          <input
            ref={inputRef}
            id="cash-amount"
            type="text"
            inputMode="decimal"
            lang="id"
            autoComplete="off"
            placeholder="0,00"
            required
            value={cash}
            disabled={saving}
            onChange={(event) => {
              const input = event.currentTarget
              const formatted = formatCashInput(input.value)
              if (formatted === null) {
                setError('Gunakan angka dan koma, maksimal 2 desimal.')
                return
              }
              const caret = cashInputCaret(input.value, input.selectionStart ?? input.value.length, formatted)
              input.value = formatted
              setCash(formatted)
              input.setSelectionRange(caret, caret)
              setError('')
            }}
            onKeyDown={(event) => {
              const input = event.currentTarget
              const start = input.selectionStart ?? 0
              if (start !== input.selectionEnd) return
              const backward = event.key === 'Backspace' && start > 0 && cash[start - 1] === '.'
              const forward = event.key === 'Delete' && cash[start] === '.'
              if (!backward && !forward) return
              event.preventDefault()
              const from = backward ? start - 2 : start
              const next = cash.slice(0, from) + cash.slice(from + 2)
              const formatted = formatCashInput(next)
              if (formatted === null) return
              const caret = cashInputCaret(next, from, formatted)
              input.value = formatted
              setCash(formatted)
              input.setSelectionRange(caret, caret)
              setError('')
            }}
            onPaste={(event) => {
              event.preventDefault()
              const input = event.currentTarget
              const pasted = formatPastedCash(event.clipboardData.getData('text'))
              if (pasted === null) {
                setError('Nominal tidak valid. Contoh: 1.500.000,50.')
                return
              }
              const next = cash.slice(0, input.selectionStart ?? 0) + pasted + cash.slice(input.selectionEnd ?? cash.length)
              const formatted = formatCashInput(next)
              if (formatted === null) {
                setError('Gunakan angka dan koma, maksimal 2 desimal.')
                return
              }
              const caret = cashInputCaret(next, (input.selectionStart ?? 0) + pasted.length, formatted)
              input.value = formatted
              setCash(formatted)
              input.setSelectionRange(caret, caret)
              setError('')
            }}
            onBlur={() => {
              const value = parseCashAmount(cash)
              if (value !== null) setCash(formatCashAmount(value))
            }}
            aria-invalid={Boolean(error)}
            aria-describedby={error ? 'cash-error' : undefined}
            className="mt-2 min-h-11 w-full min-w-0 rounded-lg border border-slate-300 bg-white px-3 py-2 text-base tabular-nums focus:border-teal-700 focus:outline focus:outline-2 focus:outline-offset-2 focus:outline-teal-700 disabled:cursor-not-allowed disabled:bg-slate-50"
          />
          {error && <p id="cash-error" role="alert" className="mt-2 text-sm text-rose-700">{error}</p>}
          <div className="mt-6 flex justify-end gap-2">
            <button type="button" onClick={onClose} disabled={saving}
              className="min-h-11 cursor-pointer rounded-lg border border-slate-300 px-4 text-sm font-semibold text-slate-700 hover:bg-slate-50 active:bg-slate-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-50">
              Batal
            </button>
            <button type="submit" disabled={saving}
              className="min-h-11 cursor-pointer rounded-lg bg-teal-700 px-4 text-sm font-semibold text-white hover:bg-teal-800 active:bg-teal-900 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-50">
              {saving ? 'Menyimpan...' : 'Simpan'}
            </button>
          </div>
        </form>
      )}
    </dialog>
  )
}
