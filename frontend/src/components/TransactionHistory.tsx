import { useEffect, useState } from 'react'
import { formatNumber, formatCurrency } from '../lib/formatters'
import Pagination from './Pagination'
import type { Transaction } from '../types'

interface TransactionHistoryProps {
  transactionHistory?: Transaction[]
}

const PAGE_SIZE_OPTIONS = [5, 10, 25, 50]

export default function TransactionHistory({ transactionHistory = [] }: TransactionHistoryProps) {
  const [searchTerm, setSearchTerm] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(5)

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
          placeholder="Search By Stocks"
          className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-teal-600 focus:outline-none"
        />
      </form>
      <div className="mt-3 overflow-hidden rounded-xl border border-slate-300 bg-white">
        <div className="overflow-auto">
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
                <th className="border-b border-slate-300 p-1 text-center text-xs">
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
