import { useEffect, useState } from 'react'
import { formatNumber, formatCurrency } from '../lib/formatters'
import Pagination from './Pagination'
import type { CashMutationData, Transaction } from '../types'

interface TransactionHistoryProps {
  transactionHistory?: Transaction[]
  cashMutations?: CashMutationData[]
}

const PAGE_SIZE_OPTIONS = [5, 10, 25, 50]

// Baris gabungan: transaksi saham dan mutasi kas (deposit, penarikan, dividen)
// ditampilkan di satu tabel agar riwayat keuangan terbaca berurutan.
type HistoryRow =
  | { kind: 'trade'; time: number; trade: Transaction }
  | { kind: 'cash'; time: number; mutation: CashMutationData }

const parseTime = (value: string): number => {
  const time = new Date(value).getTime()
  return Number.isNaN(time) ? 0 : time
}

const mutationBadgeClass: Record<string, string> = {
  DEPOSIT: 'text-emerald-600',
  WITHDRAW: 'text-rose-600',
  DIVIDEND: 'text-teal-600',
}

export default function TransactionHistory({
  transactionHistory = [],
  cashMutations = [],
}: TransactionHistoryProps) {
  const [searchTerm, setSearchTerm] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(5)

  useEffect(() => {
    setPage(1)
  }, [searchTerm])

  const rows: HistoryRow[] = [
    ...transactionHistory.map((trade) => ({ kind: 'trade' as const, time: parseTime(trade.date), trade })),
    ...cashMutations.map((mutation) => ({ kind: 'cash' as const, time: parseTime(mutation.date), mutation })),
  ].sort((a, b) => b.time - a.time)

  const filtered = rows.filter((row) => {
    if (!searchTerm.trim()) return true
    const term = searchTerm.toLowerCase()
    if (row.kind === 'trade') {
      return (
        row.trade.stock?.ticker?.toLowerCase().includes(term) ||
        row.trade.broker?.name?.toLowerCase().includes(term) ||
        row.trade.type?.toLowerCase().includes(term)
      )
    }
    return (
      row.mutation.broker_name?.toLowerCase().includes(term) ||
      row.mutation.type?.toLowerCase().includes(term) ||
      row.mutation.description?.toLowerCase().includes(term) ||
      row.mutation.ref_no?.toLowerCase().includes(term)
    )
  })

  const totalItems = filtered.length
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize))
  const currentPage = Math.min(page, totalPages)
  const startIndex = (currentPage - 1) * pageSize
  const pageItems = filtered.slice(startIndex, startIndex + pageSize)

  return (
    <div>
      <form className="mt-3 flex flex-col gap-1 sm:flex-row" onSubmit={(e) => e.preventDefault()}>
        <input
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
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
                  Date
                </th>
              </tr>
            </thead>
            <tbody>
              {pageItems.length === 0 ? (
                <tr>
                  <td colSpan={6} className="p-1 text-sm text-center text-slate-500">
                    Tidak ada data transaksi.
                  </td>
                </tr>
              ) : (
                pageItems.map((row) => {
                  if (row.kind === 'cash') {
                    const { mutation } = row
                    const badgeClass = mutationBadgeClass[mutation.type] || 'bg-slate-200 text-slate-700'
                    return (
                      <tr key={`cash-${mutation.id}`}>
                        <td className="border-b border-slate-100 p-3 text-center">
                          <div className="flex flex-col items-center gap-1">
                            <span
                              className={`px-2 py-0.5 rounded text-xs font-semibold ${badgeClass}`}
                            >
                              {mutation.type}
                            </span>
                            <div className="text-xs text-slate-500">{mutation.broker_name}</div>
                          </div>
                        </td>
                        <td className="border-b border-slate-100 p-1 text-sm text-center">-</td>
                        <td className="border-b border-slate-100 p-1 text-sm text-center">-</td>
                        <td className="border-b border-slate-100 p-1 text-sm text-center">
                          {formatCurrency(mutation.amount)}
                        </td>
                        <td className="border-b border-slate-100 p-1 text-sm text-center">-</td>
                        <td className="border-b border-slate-100 p-1 text-sm text-center">
                          {new Date(mutation.date).toLocaleString()}
                        </td>
                      </tr>
                    )
                  }

                  const { trade } = row
                  const shares = (trade.stock.lot || 0) * 100
                  const price =
                    trade.stock?.price ||
                    (shares > 0 ? (trade.amount_done - (trade.tax || 0)) / shares : 0)
                  return (
                    <tr key={`trade-${trade.id}`}>
                      <td className="border-b border-slate-100 p-3 text-center">
                        <div className="flex flex-col items-center gap-1">
                          <div className="flex items-center gap-1">
                            <span className="font-semibold">{trade.stock.ticker}</span>
                            <span
                              className={`px-2 py-0.5 rounded text-xs font-semibold  ${trade.type === 'BUY' ? 'text-emerald-600' : 'text-rose-600'}`}
                            >
                              {trade.type}
                            </span>
                          </div>
                          <div className="text-xs text-slate-500">{trade.broker.name}</div>
                        </div>
                      </td>
                      <td className="border-b border-slate-100 p-1 text-sm text-center">
                        {formatNumber(trade.stock.lot, 0)}
                      </td>
                      <td className="border-b border-slate-100 p-1 text-sm text-center">
                        {formatCurrency(price)}
                      </td>
                      <td className="border-b border-slate-100 p-1 text-sm text-center">
                        {formatCurrency(trade.amount_done)}
                      </td>
                      <td className="border-b border-slate-100 p-1 text-sm text-center">
                        {formatCurrency(trade.tax)}
                      </td>
                      <td className="border-b border-slate-100 p-1 text-sm text-center">
                        {new Date(trade.date).toLocaleString()}
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
        <Pagination
          totalItems={totalItems}
          page={currentPage}
          pageSize={pageSize}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size)
            setPage(1)
          }}
          pageSizeOptions={PAGE_SIZE_OPTIONS}
        />
      </div>
    </div>
  )
}
