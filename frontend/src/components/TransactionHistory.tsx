import { useEffect, useState } from 'react'
import { formatNumber, formatCurrency } from '../lib/formatters'
import type { Transaction } from '../types'

interface TransactionHistoryProps {
  transactionHistory?: Transaction[]
}

const PAGE_SIZE = 5

export default function TransactionHistory({ transactionHistory = [] }: TransactionHistoryProps) {
  const [searchTerm, setSearchTerm] = useState('')
  const [page, setPage] = useState(1)

  useEffect(() => {
    setPage(1)
  }, [searchTerm])

  const filtered = transactionHistory.filter((transaction) => {
    if (!searchTerm.trim()) return true
    const term = searchTerm.toLowerCase()
    return (
      transaction.stock?.ticker?.toLowerCase().includes(term) ||
      transaction.broker?.name?.toLowerCase().includes(term)
    )
  })

  const totalItems = filtered.length
  const totalPages = Math.max(1, Math.ceil(totalItems / PAGE_SIZE))
  const currentPage = Math.min(page, totalPages)
  const startIndex = (currentPage - 1) * PAGE_SIZE
  const pageItems = filtered.slice(startIndex, startIndex + PAGE_SIZE)

  return (
    <div>
      <form className="mt-3 flex flex-col gap-1 sm:flex-row" onSubmit={(e) => e.preventDefault()}>
        <input
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          type="text"
          placeholder="Search By Stocks"
          className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-teal-600 focus:outline-none"
        />
      </form>
      <div className="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
        <table className="w-full table-fixed border-collapse">
          <thead>
            <tr className="bg-slate-50 text-slate-500">
              <th className="border-b border-slate-300 p-1 text-center text-xs">
                Ticker / Broker
              </th>
              <th className="border-b border-slate-300 p-1 text-center text-xs">
                Lot
              </th>
              <th className="border-b border-slate-300 p-1 text-center text-xs">
                Price
              </th>
              <th className="border-b border-slate-300 p-1 text-center text-xs">
                Amount Done
              </th>
              <th className="border-b border-slate-300 p-1 text-center text-xs">
                Tax
              </th>
              <th className="border-b border-slate-300 p-w text-center text-xs">
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
              pageItems.map((transaction) => {
                const shares = (transaction.stock.lot || 0) * 100
                const price = transaction.stock?.price || (shares > 0 ? (transaction.amount_done - (transaction.tax || 0)) / shares : 0)
                return (
                  <tr key={transaction.id}>
                    <td className="border-b border-slate-100 p-3 text-center">
                      <div className="flex flex-col items-center gap-1">
                        <div className="flex items-center gap-1">
                          <span className="font-semibold">{transaction.stock.ticker}</span>
                          <span
                            className={`px-2 py-0.5 rounded text-xs font-semibold  ${transaction.type === 'BUY' ? 'bg-emerald-200 text-emerald-800' : 'bg-rose-200 text-rose-800'}`}
                          >
                            {transaction.type}
                          </span>
                        </div>
                        <div className="text-xs text-slate-500">
                          {transaction.broker.name}
                        </div>
                      </div>
                    </td>
                    <td className="border-b border-slate-100 p-1 text-sm text-center">
                      {formatNumber(transaction.stock.lot, 0)}
                    </td>
                    <td className="border-b border-slate-100 p-1 text-sm text-center">
                      {formatCurrency(price)}
                    </td>
                    <td className="border-b border-slate-100 p-1 text-sm text-center">
                      {formatCurrency(transaction.amount_done)}
                    </td>
                    <td className="border-b border-slate-100 p-1 text-sm text-center">
                      {formatCurrency(transaction.tax)}
                    </td>
                    <td className="border-b border-slate-100 p-1 text-sm text-center">
                      {new Date(transaction.date).toLocaleString()}
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
      <div className="mt-2 flex flex-wrap items-center justify-between gap-2 px-1 text-xs text-slate-500">
        <span>
          Menampilkan {totalItems === 0 ? 0 : startIndex + 1}&ndash;
          {Math.min(startIndex + PAGE_SIZE, totalItems)} dari {totalItems} data
        </span>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setPage(Math.max(1, currentPage - 1))}
            disabled={currentPage <= 1}
            className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40"
          >
            &larr; Sebelumnya
          </button>
          <span className="text-sm text-slate-600">
            Halaman {currentPage} / {totalPages}
          </span>
          <button
            type="button"
            onClick={() => setPage(Math.min(totalPages, currentPage + 1))}
            disabled={currentPage >= totalPages}
            className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40"
          >
            Berikutnya &rarr;
          </button>
        </div>
      </div>
    </div>
  )
}
