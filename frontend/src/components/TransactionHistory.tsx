import { useEffect, useState } from 'react'
import { formatNumber, formatCurrency } from '../lib/formatters'
import { historyDate } from '../lib/historyDate'
import Pagination from './Pagination'
import { api } from '../lib/api'

interface HistoryItem {
  id: number
  kind: 'cash' | 'trade'
  date: string
  type: string
  broker_name: string
  amount: number
  tax: number
  ticker?: string
  lot?: number
  price?: number
}

interface HistoryPage {
  data: HistoryItem[]
  page: number
  total: number
}

function HistoryDate({ date }: { date: ReturnType<typeof historyDate> }) {
  if (!date.dateTime) return <span>{date.label}</span>
  return (
    <time dateTime={date.dateTime}>
      {date.label}
      <span className="block text-xs text-slate-500">{date.clock}</span>
    </time>
  )
}

const mutationBadgeClass: Record<string, string> = {
  DEPOSIT: 'text-emerald-600',
  WITHDRAW: 'text-rose-600',
  DIVIDEND: 'text-teal-600',
}

export default function TransactionHistory({ revision }: { revision: number }) {
  const [searchTerm, setSearchTerm] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(5)
  const [result, setResult] = useState<HistoryPage | null>(null)
  const [error, setError] = useState(false)

  useEffect(() => {
    const controller = new AbortController()
    setResult(null)
    setError(false)
    const query = new URLSearchParams({ page: String(page), page_size: String(pageSize), q: searchTerm })
    const timer = window.setTimeout(async () => {
      try {
        const response = await api<HistoryPage>(`/api/history?${query}`, { signal: controller.signal })
        if (!controller.signal.aborted) {
          setResult(response)
          setPage(response.page)
        }
      } catch {
        if (!controller.signal.aborted) setError(true)
      }
    }, searchTerm ? 200 : 0)
    return () => {
      window.clearTimeout(timer)
      controller.abort()
    }
  }, [page, pageSize, searchTerm, revision])

  return (
    <div>
      <form className="mt-3 flex flex-col gap-1 sm:flex-row" onSubmit={(e) => e.preventDefault()}>
        <input
          value={searchTerm}
          onChange={(e) => {
            setSearchTerm(e.target.value)
            setPage(1)
          }}
          type="text"
          placeholder="Cari saham, broker, atau mutasi kas"
          className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-teal-600 focus:outline-none"
        />
      </form>
      <div className="mt-3 overflow-hidden rounded-xl border border-slate-300 bg-white">
        <div className="overflow-auto">
          <table className="w-full min-w-[680px] table-fixed border-collapse">
            <thead>
              <tr className="bg-slate-50 text-slate-500">
                <th className="w-[24%] border-b border-slate-300 p-1 text-center text-xs">
                  Ticker / Broker
                </th>
                <th className="w-[6%] border-b border-slate-300 p-1 text-center text-xs">
                  Lot
                </th>
                <th className="w-[17%] border-b border-slate-300 p-1 text-center text-xs">
                  Price
                </th>
                <th className="w-[19%] border-b border-slate-300 p-1 text-center text-xs">
                  Amount Done
                </th>
                <th className="w-[12%] border-b border-slate-300 p-1 text-center text-xs">
                  Tax
                </th>
                <th className="w-[22%] border-b border-slate-300 p-1 text-center text-xs">
                  Tanggal
                </th>
              </tr>
            </thead>
            <tbody>
              {!result?.data.length ? (
                <tr>
                  <td colSpan={6} className="p-1 text-sm text-center text-slate-500">
                    {error ? 'Gagal memuat riwayat.' : result ? 'Tidak ada data transaksi.' : 'Memuat riwayat…'}
                  </td>
                </tr>
              ) : (
                result.data.map((row) => {
                  if (row.kind === 'cash') {
                    const badgeClass = mutationBadgeClass[row.type] || 'bg-slate-200 text-slate-700'
                    return (
                      <tr key={`cash-${row.id}`}>
                        <td className="border-b border-slate-100 p-3 text-center">
                          <div className="flex flex-col items-center gap-1">
                            <span
                              className={`px-2 py-0.5 rounded text-xs font-semibold ${badgeClass}`}
                            >
                              {row.type}
                            </span>
                            <div className="text-xs text-slate-500">{row.broker_name}</div>
                          </div>
                        </td>
                        <td className="border-b border-slate-100 p-1 text-sm text-center">-</td>
                        <td className="border-b border-slate-100 p-1 text-sm text-center">-</td>
                        <td className="border-b border-slate-100 p-1 text-sm text-center">
                          {formatCurrency(row.amount)}
                        </td>
                        <td className="border-b border-slate-100 p-1 text-sm text-center">-</td>
                        <td className="border-b border-slate-100 p-1 text-sm text-center">
                          <HistoryDate date={historyDate(row.date)} />
                        </td>
                      </tr>
                    )
                  }

                  const shares = (row.lot || 0) * 100
                  const price =
                    row.price ||
                    (shares > 0 ? (row.amount - (row.tax || 0)) / shares : 0)
                  return (
                    <tr key={`trade-${row.id}`}>
                      <td className="border-b border-slate-100 p-3 text-center">
                        <div className="flex flex-col items-center gap-1">
                          <div className="flex items-center gap-1">
                            <span className="font-semibold">{row.ticker}</span>
                            <span
                              className={`px-2 py-0.5 rounded text-xs font-semibold  ${row.type === 'BUY' ? 'text-emerald-600' : 'text-rose-600'}`}
                            >
                              {row.type}
                            </span>
                          </div>
                          <div className="text-xs text-slate-500">{row.broker_name}</div>
                        </div>
                      </td>
                      <td className="border-b border-slate-100 p-1 text-sm text-center">
                        {formatNumber(row.lot, 0)}
                      </td>
                      <td className="border-b border-slate-100 p-1 text-sm text-center">
                        {formatCurrency(price)}
                      </td>
                      <td className="border-b border-slate-100 p-1 text-sm text-center">
                        {formatCurrency(row.amount)}
                      </td>
                      <td className="border-b border-slate-100 p-1 text-sm text-center">
                        {formatCurrency(row.tax)}
                      </td>
                      <td className="border-b border-slate-100 p-1 text-sm text-center">
                        <HistoryDate date={historyDate(row.date)} />
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
        {result && <Pagination
          totalItems={result.total}
          page={result.page}
          pageSize={pageSize}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size)
            setPage(1)
          }}
        />}
      </div>
    </div>
  )
}
