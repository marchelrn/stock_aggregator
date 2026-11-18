import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'react-toastify'
import { GMAIL_CONNECTION_SESSION } from '../components/GmailConnection'
import { api, ApiError } from '../lib/api'

export default function GmailCallback() {
  const navigate = useNavigate()
  const handled = useRef(false)

  useEffect(() => {
    if (handled.current) return
    handled.current = true

    const params = new URLSearchParams(window.location.hash.slice(1))
    window.history.replaceState(window.history.state, '', window.location.pathname)
    const complete = async () => {
      let returnTo = localStorage.getItem('isSetupCompleted') === 'true' ? '/manage' : '/setup'
      let brokerName: string | undefined
      let syncAfterConsent = false
      try {
        const pending = sessionStorage.getItem(GMAIL_CONNECTION_SESSION)
        sessionStorage.removeItem(GMAIL_CONNECTION_SESSION)
        const saved = pending ? JSON.parse(pending) : null
        if (saved?.returnTo === '/setup' || saved?.returnTo === '/manage') returnTo = saved.returnTo
        const state = params.get('state')
        if (!state || saved?.state !== state || typeof saved.verifier !== 'string') {
          throw new Error('Sesi koneksi Gmail tidak ditemukan. Coba hubungkan kembali.')
        }
        if (returnTo === '/setup' && typeof saved.brokerName === 'string') brokerName = saved.brokerName
        const code = params.get('code')
        if (!code) throw new Error('Izin baca Gmail belum diberikan. Anda dapat mencoba lagi.')
        await api('/api/gmail/connect/complete', {
          method: 'POST',
          body: JSON.stringify({ code, state, verifier: saved.verifier }),
        })
        toast.success('Gmail terhubung.')
        syncAfterConsent = Boolean(brokerName)
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) return
        toast.error(err instanceof Error ? err.message : 'Gmail belum berhasil terhubung.')
      }
      navigate(returnTo, { replace: true, state: brokerName ? { brokerName, syncAfterConsent } : null })
    }
    void complete()
  }, [navigate])

  return <p role="status" className="p-4 text-center text-slate-700">Memeriksa izin baca Gmail…</p>
}
