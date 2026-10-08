import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import MarketChart from '../components/MarketChart'
import { api } from '../lib/api'
import { formatCurrency, formatNumber, formatPercent, plClass } from '../lib/formatters'
import { buildWatchlist, getWatchlistMovers, marketTime } from '../lib/market'
import { normalizeArray } from '../lib/portfolio'
import type { WatchlistRow } from '../lib/market'
import type { Holding, MarketDetail, StockPriceMap } from '../types'

const REFRESH_INTERVAL = 5 * 60 * 1000
const panelClass = 'rounded-2xl border border-slate-300 bg-white p-4 shadow-sm'
const money = (value: number | null | undefined) => value == null ? '—' : formatCurrency(value)
const number = (value: number | null | undefined) => value == null ? '—' : formatNumber(value, 0)

function Movers({ title, rows }: { title: string; rows: WatchlistRow[] }) {
  return (
    <section className={panelClass}>
      <h2 className="font-semibold">{title}</h2>
      {rows.length === 0 ? <p className="mt-3 text-sm text-slate-500">Tidak ada emiten pada kategori ini.</p> : (
        <ul className="mt-3 space-y-2">
          {rows.map((row) => (
            <li key={row.ticker} className="flex items-center justify-between gap-3 text-sm">
              <span className="font-semibold">{row.ticker}</span>
              <span className={plClass(row.quote!.change_percent)}>{formatPercent(row.quote!.change_percent)}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

export default function MarketPage() {
  const [rows, setRows] = useState<WatchlistRow[]>([])
  const [selected, setSelected] = useState('')
  const [detail, setDetail] = useState<MarketDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [detailLoading, setDetailLoading] = useState(false)
  const [error, setError] = useState('')
  const [detailError, setDetailError] = useState('')
  const [warning, setWarning] = useState('')
  const [loadedAt, setLoadedAt] = useState<string | null>(null)
  const [revision, setRevision] = useState(0)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState('ticker')

  useEffect(() => {
    const controller = new AbortController()
    const timeout = window.setTimeout(() => {
      controller.abort()
      setError('Permintaan watchlist melewati batas 30 detik. Silakan Refresh kembali.')
      setLoading(false)
    }, 30000)
    setLoading(true)
    setError('')
    setWarning('')
    // Holdings menjadi sumber watchlist, bukan input ticker manual atau cache browser.
    async function load() {
      try {
        const result = await api('/my/stocks', { signal: controller.signal })
        if (controller.signal.aborted) return
        const holdings = normalizeArray<Holding>(result)
        const watchlist = buildWatchlist(holdings, {})
        let prices: StockPriceMap = {}
        if (watchlist.length > 0) {
          try {
            const response = await api<{ data?: StockPriceMap }>(
              `/prices?ticker=${encodeURIComponent(watchlist.map((row) => row.ticker).join(','))}`,
              { signal: controller.signal },
            )
            prices = response.data ?? {}
          } catch (cause) {
            if (controller.signal.aborted) return
            setWarning(`Harga belum tersedia: ${(cause as Error).message}`)
          }
        }
        if (controller.signal.aborted) return
        const next = buildWatchlist(holdings, prices)
        const missing = next.filter((row) => !row.quote).map((row) => row.ticker)
        if (missing.length > 0) {
          setWarning(`Sebagian harga tidak tersedia (${missing.join(', ')}). Data yang hilang tidak dihitung sebagai movers.`)
        }
        setRows(next)
        setSelected((current) => next.some((row) => row.ticker === current) ? current : next[0]?.ticker ?? '')
        setLoadedAt(new Date().toISOString())
      } catch (cause) {
        if (!controller.signal.aborted) setError(`Gagal memuat watchlist: ${(cause as Error).message}`)
      } finally {
        if (!controller.signal.aborted) {
          window.clearTimeout(timeout)
          setLoading(false)
        }
      }
    }
    void load()
    return () => {
      window.clearTimeout(timeout)
      controller.abort()
    }
  }, [revision])

  useEffect(() => {
    if (!autoRefresh) return
    const timer = window.setInterval(() => {
      if (document.visibilityState === 'visible') setRevision((current) => current + 1)
    }, REFRESH_INTERVAL)
    return () => window.clearInterval(timer)
  }, [autoRefresh])

  useEffect(() => {
    if (!selected) {
      setDetail(null)
      return
    }
    const controller = new AbortController()
    const timeout = window.setTimeout(() => {
      controller.abort()
      setDetailLoading(false)
      setDetailError('Permintaan detail melewati batas 20 detik. Silakan Refresh kembali.')
    }, 20000)
    setDetail(null)
    setDetailError('')
    setDetailLoading(true)
    api<{ data: MarketDetail }>(`/api/market/${encodeURIComponent(selected)}`, { signal: controller.signal })
      .then((response) => {
        if (!controller.signal.aborted) setDetail(response.data)
      })
      .catch((cause) => {
        if (!controller.signal.aborted) setDetailError(`Detail emiten tidak tersedia: ${(cause as Error).message}`)
      })
      .finally(() => {
        window.clearTimeout(timeout)
        if (!controller.signal.aborted) setDetailLoading(false)
      })
    return () => {
      window.clearTimeout(timeout)
      controller.abort()
    }
  }, [selected, revision])

  const movers = useMemo(() => getWatchlistMovers(rows), [rows])
  const visibleRows = useMemo(() => {
    const filtered = rows.filter((row) => row.ticker.includes(query.trim().toUpperCase()))
    if (sort === 'change') filtered.sort((a, b) => (b.quote?.change_percent ?? -Infinity) - (a.quote?.change_percent ?? -Infinity))
    return filtered
  }, [rows, query, sort])

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-4 px-4">
      <header className={`${panelClass} flex flex-wrap items-start justify-between gap-4`}>
        <div>
          <h1 className="text-2xl font-bold">Market</h1>
          <p className="mt-2 text-sm text-slate-500">Watchlist otomatis dari saham yang Anda miliki, digabung dari seluruh broker.</p>
          <p className="mt-1 text-xs text-slate-500">Watchlist dimuat: {marketTime(loadedAt)}</p>
        </div>
        <div className="flex flex-col items-end gap-2">
          <button type="button" onClick={() => setRevision((current) => current + 1)} disabled={loading}
            className="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800 disabled:opacity-50">
            {loading ? 'Memuat...' : 'Refresh'}
          </button>
          <label className="flex items-center gap-2 text-xs text-slate-600">
            <input type="checkbox" checked={autoRefresh} onChange={(event) => setAutoRefresh(event.target.checked)} />
            Refresh otomatis 5 menit
          </label>
        </div>
      </header>

      <p className="rounded-xl border border-amber-200 bg-amber-50 p-3 text-xs leading-relaxed text-amber-900">
        Sumber: Yahoo Finance. Data dapat tertunda, bukan running trade real-time. Cache backend berlaku 5 menit;
        tombol Refresh tidak memaksa Yahoo memberikan harga baru. Movers hanya mencakup watchlist Anda, bukan seluruh BEI.
      </p>
      {error && <p role="alert" className="rounded-xl bg-rose-50 p-3 text-sm text-rose-700">{error}{rows.length > 0 && ' Data sebelumnya masih ditampilkan.'}</p>}
      {warning && <p role="status" className="rounded-xl bg-amber-50 p-3 text-sm text-amber-800">{warning}</p>}

      <div className="grid gap-3 sm:grid-cols-3">
        <Movers title="Top Gainers · Watchlist" rows={movers.gainers} />
        <Movers title="Top Losers · Watchlist" rows={movers.losers} />
        <section className={panelClass}>
          <h2 className="font-semibold">Ringkasan Watchlist</h2>
          <dl className="mt-3 grid grid-cols-2 gap-2 text-sm">
            <dt className="text-slate-500">Emiten dimiliki</dt><dd className="text-right font-semibold">{rows.length}</dd>
            <dt className="text-slate-500">Tidak berubah</dt><dd className="text-right">{movers.unchanged}</dd>
            <dt className="text-slate-500">Harga belum tersedia</dt><dd className="text-right">{movers.unavailable}</dd>
          </dl>
        </section>
      </div>

      <section className={panelClass} aria-busy={loading}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-lg font-semibold">My Watchlist</h2>
          <div className="flex flex-wrap gap-2">
            <input aria-label="Filter ticker watchlist" placeholder="Filter ticker..." value={query} onChange={(event) => setQuery(event.target.value)}
              className="w-40 rounded-lg border border-slate-300 px-3 py-2 text-sm" />
            <select aria-label="Urutkan watchlist" value={sort} onChange={(event) => setSort(event.target.value)} className="rounded-lg border border-slate-300 px-2 py-2 text-sm">
              <option value="ticker">Ticker A–Z</option><option value="change">Perubahan terbesar</option>
            </select>
          </div>
        </div>
        <div className="mt-3 overflow-x-auto">
          <table className="w-full min-w-[640px] text-sm">
            <thead className="bg-slate-50 text-xs text-slate-500">
              <tr>{['Emiten', 'Harga terakhir', 'Perubahan', 'Prev Close', 'Lot dimiliki', 'Broker'].map((label) => <th key={label} className="p-3 text-left">{label}</th>)}</tr>
            </thead>
            <tbody>
              {visibleRows.map((row) => (
                <tr key={row.ticker} className={row.ticker === selected ? 'bg-teal-50' : 'hover:bg-slate-50'}>
                  <td className="border-b border-slate-100 p-3"><button type="button" aria-pressed={row.ticker === selected} onClick={() => setSelected(row.ticker)} className="font-semibold text-teal-800 underline decoration-dotted underline-offset-4">{row.ticker}</button></td>
                  <td className="border-b border-slate-100 p-3" title={`Cache harga: ${marketTime(row.quote?.updated_at)}`}>{money(row.quote?.price)}</td>
                  <td className={`border-b border-slate-100 p-3 ${row.quote ? plClass(row.quote.change_percent) : 'text-slate-500'}`}>{row.quote ? `${formatNumber(row.quote.change)} (${formatPercent(row.quote.change_percent)})` : '—'}</td>
                  <td className="border-b border-slate-100 p-3">{money(row.quote?.previous_close)}</td>
                  <td className="border-b border-slate-100 p-3">{formatNumber(row.lot, 0)}</td>
                  <td className="border-b border-slate-100 p-3 text-xs text-slate-600">{row.brokers.join(', ') || '—'}</td>
                </tr>
              ))}
              {visibleRows.length === 0 && <tr><td colSpan={6} className="p-6 text-center text-slate-500">
                {loading ? 'Memuat saham yang dimiliki...' : rows.length > 0 ? 'Tidak ada ticker yang cocok.' : error ? 'Watchlist belum tersedia.' : <>Belum ada saham yang dimiliki. <Link to="/manage" className="text-teal-700 underline">Kelola broker</Link></>}
              </td></tr>}
            </tbody>
          </table>
        </div>
      </section>

      {selected && <section className={panelClass} aria-busy={detailLoading}>
        <h2 className="text-lg font-semibold">{selected} · Detail Emiten</h2>
        {detailLoading && <p role="status" className="mt-4 text-sm text-slate-500">Mengambil detail dan histori harga...</p>}
        {detailError && <p role="alert" className="mt-4 text-sm text-rose-700">{detailError} Gunakan Refresh untuk mencoba lagi.</p>}
        {detail && <>
          <p className="mt-1 text-sm text-slate-500">{detail.name || selected} · {detail.exchange || 'Bursa tidak tersedia'} · {detail.currency || 'Mata uang tidak tersedia'}</p>
          <p className="mt-3 text-2xl font-bold">{money(detail.price)} <span className={`text-sm ${detail.change_percent == null ? 'text-slate-500' : plClass(detail.change_percent)}`}>{detail.change_percent == null ? '—' : formatPercent(detail.change_percent)}</span></p>
          <p className="mt-1 text-xs text-slate-500">Waktu harga Yahoo: {marketTime(detail.market_time)} · Diambil server: {marketTime(detail.fetched_at)}</p>
          <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {[
              ['Previous Close', money(detail.previous_close)],
              ['Low / High Hari Ini', `${money(detail.day_low)} / ${money(detail.day_high)}`],
              ['Low / High 52 Minggu', `${money(detail.fifty_two_week_low)} / ${money(detail.fifty_two_week_high)}`],
              ['Volume (lembar)', number(detail.volume)],
            ].map(([label, value]) => <div key={label} className="rounded-lg bg-slate-50 p-3"><p className="text-xs text-slate-500">{label}</p><p className="mt-1 text-sm font-semibold">{value}</p></div>)}
          </div>
          <h3 className="mt-5 text-sm font-semibold">Histori harga · 1 bulan</h3>
          <p className="mt-1 text-xs text-slate-500">Titik harga close harian dari Yahoo; titik terakhir dapat berubah selama sesi perdagangan. Hover titik untuk melihat harga.</p>
          <MarketChart ticker={detail.ticker} history={detail.history} />
          <a href={`https://finance.yahoo.com/quote/${encodeURIComponent(selected + '.JK')}/`} target="_blank" rel="noopener noreferrer" className="mt-3 inline-block text-xs font-semibold text-teal-700 underline">Lihat emiten di Yahoo Finance ↗</a>
        </>}
      </section>}
    </div>
  )
}
