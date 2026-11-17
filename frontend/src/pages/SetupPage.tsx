import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'react-toastify'
import { usePortfolio } from '../context/PortfolioContext'
import SetupFlowModal, { type ManualHolding } from '../components/SetupFlowModal'
import { formatCurrency } from '../lib/formatters'
import { api } from '../lib/api'
import './SetupPage.css'

const BROKERS = [
  { name: 'Stockbit Sekuritas', needsPin: false, disabled: false },
  { name: 'Trimegah Sekuritas', needsPin: false, disabled: false },
  { name: 'Mandiri Sekuritas', needsPin: true, disabled: false },
  { name: 'BNI Sekuritas', needsPin: false, disabled: true },
]

export default function SetupPage() {
  const navigate = useNavigate()
  const { state, addBroker, loadDashboard, syncSetupHistorical, createTransaction } = usePortfolio()
  const [brokerName, setBrokerName] = useState('Stockbit Sekuritas')
  const [cash, setCash] = useState('')
  const [pin, setPin] = useState('')
  const [showForm, setShowForm] = useState(true)
  const [running, setRunning] = useState(false)
  const [initialLoading, setInitialLoading] = useState(true)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [elapsed, setElapsed] = useState(0)
  const [review, setReview] = useState(false)
  const [manualHoldings, setManualHoldings] = useState<ManualHolding[]>([])
  const [finishing, setFinishing] = useState(false)
  const [finishError, setFinishError] = useState('')
  const busy = useRef(false)
  const selectRef = useRef<HTMLSelectElement>(null)
  const option = BROKERS.find((broker) => broker.name === brokerName)!
  const savedBroker = state.brokers.find((broker) => broker.name === brokerName)
  const existing = Boolean(savedBroker)
  const cost = state.holdings.reduce((sum, holding) => sum + holding.lot * 100 * holding.avg_price, 0)
  const totalCash = state.brokers.reduce((sum, broker) => sum + broker.cash, 0)

  useEffect(() => {
    let active = true
    loadDashboard({ silent: true }).finally(() => { if (active) setInitialLoading(false) })
    return () => { active = false }
  }, [loadDashboard])

  useEffect(() => {
    if (!running) return
    const timer = window.setInterval(() => setElapsed((value) => value + 1), 1000)
    return () => window.clearInterval(timer)
  }, [running])

  const connectBroker = async (event: React.FormEvent) => {
    event.preventDefault()
    if (busy.current || initialLoading) return
    busy.current = true
    setRunning(true)
    setError('')
    setElapsed(0)
    setMessage(`Menyiapkan ${brokerName}…`)
    try {
      if (savedBroker && pin) {
        await api(`/broker/${savedBroker.id}`, { method: 'PUT', body: JSON.stringify({ pin }) })
        setPin('')
      }
      if (!existing && !await addBroker(brokerName, Number(cash) || 0, pin)) {
        setError(`Broker ${brokerName} belum berhasil disimpan. Silakan coba lagi.`)
        setMessage('')
        return
      }
      setMessage(`Menyinkronkan ${brokerName}…`)
      const result = await syncSetupHistorical()
      await loadDashboard({ silent: true })
      if (!result) {
        setError('Sinkronisasi gagal. Broker tetap tersimpan. Coba sinkronkan ulang atau lengkapi saham secara manual.')
        setMessage('')
        return
      }
      if (result.trade_error || result.cash_error) {
        setError([result.trade_error && `Transaksi: ${result.trade_error}`, result.cash_error && `Mutasi kas: ${result.cash_error}`].filter(Boolean).join(' '))
        setMessage('Sinkronisasi belum lengkap. Anda dapat mencoba lagi.')
        return
      }
      setMessage(`${brokerName} tersinkron.`)
      setShowForm(false)
      setCash('')
      setPin('')
    } catch {
      setError('Terjadi kesalahan saat memuat hasil. Coba sinkronkan ulang.')
      setMessage('')
    } finally {
      busy.current = false
      setRunning(false)
    }
  }

  const finishSetup = async () => {
    if (busy.current) return
    busy.current = true
    setFinishing(true)
    setFinishError('')
    const failed: ManualHolding[] = []
    try {
      for (const holding of manualHoldings) {
        const ok = await createTransaction({ type: 'BUY', ticker: holding.ticker, broker_id: holding.brokerId, broker_name: holding.brokerName, lot: holding.lot, price: holding.price })
        if (!ok) failed.push(holding)
      }
      setManualHoldings(failed)
      if (failed.length) {
        setFinishError(`Saham ${failed.map((holding) => holding.ticker).join(', ')} gagal disimpan. Periksa saldo kas broker, lalu coba lagi.`)
        return
      }
      localStorage.setItem('isSetupCompleted', 'true')
      toast.success('Setup awal selesai. Portofolio siap dipakai.')
      navigate('/manage')
    } finally {
      busy.current = false
      setFinishing(false)
    }
  }

  const openForm = () => {
    const available = BROKERS.find((broker) => !broker.disabled && !state.brokers.some((saved) => saved.name === broker.name))
    if (available) setBrokerName(available.name)
    setShowForm(true)
    setCash('')
    setPin('')
    window.setTimeout(() => selectRef.current?.focus(), 0)
  }

  return (
    <div className="broker-setup">
      <header className="setup-header">
        <span className="setup-brand">Stock API</span>
        <nav aria-label="Progres konfigurasi">
          <ol className="setup-progress">
            <li aria-current={review ? undefined : 'step'}>Hubungkan broker</li>

            <li aria-current={review ? 'step' : undefined}>Periksa portofolio</li>
          </ol>
        </nav>
      </header>
      <div className="setup-columns">
        <section className="setup-config" aria-labelledby="setup-title">
          <div>
            <h1 id="setup-title">Hubungkan broker</h1>
            <p className="setup-intro">Impor transaksi dan saldo kas dari Gmail Anda.</p>
          </div>

          {(initialLoading || state.brokers.length > 0) && <div className="setup-connected" aria-busy={initialLoading}>
            <h2>Broker terhubung <span>{state.brokers.length}</span></h2>
            {initialLoading ? <p className="setup-muted" role="status">Memuat broker Anda…</p> : state.brokers.length === 0 ? (
              <p className="setup-muted">Belum ada broker terhubung. Pilih broker pertama di bawah.</p>
            ) : <ul>{state.brokers.map((broker) => (
              <li key={broker.id}>
                <div><strong>{broker.name}</strong><span>Saldo kas</span></div>
                <span className="setup-number">{formatCurrency(broker.cash)}</span>
              </li>
            ))}</ul>}
          </div>}

          {showForm && <form onSubmit={connectBroker} className="setup-form">
            <label htmlFor="setup-broker">Sekuritas</label>
            <select ref={selectRef} id="setup-broker" value={brokerName} disabled={running || initialLoading} onChange={(event) => { setBrokerName(event.target.value); setCash(''); setPin(''); setError('') }}>
              {BROKERS.map((broker) => <option key={broker.name} value={broker.name} disabled={broker.disabled}>{broker.name}{broker.disabled ? ' (belum tersedia)' : ''}</option>)}
            </select>
            {!existing && <>
              <label htmlFor="setup-cash">Saldo kas awal (IDR)</label>
              <input id="setup-cash" type="number" min="0" step="0.01" placeholder="0" value={cash} disabled={running} onChange={(event) => setCash(event.target.value)} />
            </>}
            {option.needsPin && <>
              <label htmlFor="setup-pin">PIN trade confirmation</label>
              <input id="setup-pin" type="password" required={!existing} autoComplete="off" placeholder={existing ? 'Isi untuk memperbarui PIN' : undefined} value={pin} disabled={running} onChange={(event) => setPin(event.target.value)} />
            </>}
            <button className="setup-secondary" type="submit" disabled={running || initialLoading}>{running ? 'Menyinkronkan…' : existing ? 'Sinkronkan ulang' : 'Hubungkan broker'}</button>
          </form>}

          <div className="setup-feedback" aria-live="polite" aria-atomic="true">
            {message && !running && <p role="status">{message}</p>}
          </div>
          {error && <p className="setup-error" role="alert">{error}</p>}
          <div className="setup-actions">
            {!showForm && <button type="button" className="setup-secondary" disabled={running} onClick={openForm}>Tambah broker lain</button>}
            <button type="button" className="setup-primary" disabled={running || initialLoading || state.brokers.length === 0} onClick={() => setReview(true)}>Lanjutkan</button>
          </div>
        </section>

        <aside className="setup-portfolio" aria-labelledby="portfolio-title">
          <h2 id="portfolio-title">Portofolio Anda</h2>
          <dl className="setup-totals">
            <div><dt>Nilai pembelian · IDR</dt><dd>{formatCurrency(cost)}</dd></div>
            <div><dt>Saldo kas · IDR</dt><dd>{formatCurrency(totalCash)}</dd></div>
          </dl>
          <div className="setup-holdings-header"><h3>Daftar Ticker</h3><span>{state.holdings.length} posisi</span></div>
          {running && <div className="setup-loading" role="status"><strong>Membaca saham dari email…</strong><span>{elapsed} detik · Dapat memerlukan beberapa menit.</span></div>}
          {initialLoading ? <p role="status" className="setup-empty">Memuat portofolio…</p> : state.holdings.length === 0 ? (
            <div className="setup-empty"><strong>Belum ada saham terbaca</strong><p>{state.brokers.length ? 'Sinkronkan ulang atau lengkapi saham pada langkah berikutnya.' : 'Saham akan muncul setelah broker terhubung.'}</p></div>
          ) : <ul className="setup-holdings">{state.holdings.map((holding, index) => (
            <li key={`${holding.broker_id}-${holding.ticker}-${index}`}>
              <div><strong>{holding.ticker}</strong><span>{holding.broker_name}</span></div>
              <div className="setup-number"><strong>{holding.lot} lot</strong><span>Rata-rata {formatCurrency(holding.avg_price)}</span></div>
            </li>
          ))}</ul>}
        </aside>
      </div>
      {review && <SetupFlowModal stage="portfolio" running={false} steps={[]} elapsed={0} tradeSummary="" cashSummary="" tradeError={error} cashError="" holdings={state.holdings} brokers={state.brokers} manualHoldings={manualHoldings} finishing={finishing} finishError={finishError} onContinue={() => {}} onFinish={finishSetup} onClose={() => setReview(false)} onAddManual={(holding) => { setFinishError(''); setManualHoldings((prev) => [...prev, holding]) }} onRemoveManual={(index) => { setFinishError(''); setManualHoldings((prev) => prev.filter((_, position) => position !== index)) }} />}
    </div>
  )
}
