import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { GmailSyncStatusResponse } from '../types'

export const GMAIL_CONNECTION_SESSION = 'gmail-connection'
export type GmailConnectionReturn = '/setup' | '/manage'

export async function startGmailConnection(returnTo: GmailConnectionReturn, brokerName?: string) {
  const { url, state, verifier } = await api<{ url: string; state: string; verifier: string }>(
    '/api/gmail/connect', { method: 'POST' },
  )
  sessionStorage.setItem(GMAIL_CONNECTION_SESSION, JSON.stringify({ state, verifier, returnTo, brokerName }))
  window.location.assign(url)
}

export function useGmailConnectionStatus() {
  const [connected, setConnected] = useState<boolean | null>(null)
  const [statusError, setStatusError] = useState(false)

  useEffect(() => {
    const controller = new AbortController()
    api<GmailSyncStatusResponse>('/api/gmail/status', { signal: controller.signal })
      .then((status) => setConnected(status.gmail_sync_enabled))
      .catch(() => { if (!controller.signal.aborted) setStatusError(true) })
    return () => controller.abort()
  }, [])

  return { connected, statusError }
}

export default function GmailConnection({
  connected, statusError, returnTo, disabled = false,
}: ReturnType<typeof useGmailConnectionStatus> & { returnTo: GmailConnectionReturn; disabled?: boolean }) {
  const [starting, setStarting] = useState(false)
  const [error, setError] = useState('')

  const connect = async () => {
    setStarting(true)
    setError('')
    try {
      await startGmailConnection(returnTo)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Koneksi Gmail gagal dimulai.')
      setStarting(false)
    }
  }

  return (
    <section aria-label="Sinkronisasi Gmail" className="flex flex-col items-start gap-3 rounded-xl border border-slate-300 bg-white p-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0">
        <h2 className="font-semibold text-slate-900">
          {connected === true ? 'Gmail terhubung' : connected === false ? 'Gmail belum terhubung' : 'Sinkronisasi Gmail'}
        </h2>
        <p className="mt-1 text-sm text-slate-600">
          {statusError ? 'Status koneksi Gmail tidak dapat dimuat.'
            : connected === null ? 'Memeriksa koneksi Gmail…'
            : connected ? 'Akses baca digunakan untuk mengimpor transaksi dan mutasi kas dari email broker.'
            : 'Izinkan akses baca Gmail untuk mengimpor Trade Confirmation dan mutasi kas dari email broker.'}
        </p>
        {error && <p role="alert" className="mt-2 text-sm text-rose-700">{error}</p>}
      </div>
      {(connected !== null || statusError) && (
        <button type="button" onClick={connect} disabled={disabled || starting}
          className="min-h-11 shrink-0 cursor-pointer rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800 active:bg-teal-900 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-60">
          {starting ? 'Membuka Google…' : connected ? 'Hubungkan ulang Gmail' : 'Hubungkan Gmail'}
        </button>
      )}
    </section>
  )
}
