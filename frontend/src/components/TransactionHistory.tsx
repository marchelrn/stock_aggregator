import { useState } from 'react'
import { formatNumber, formatCurrency } from '../lib/formatters'
import type { Transaction } from '../types'

interface TransactionHistoryProps {
  transactionHistory?: Transaction[]
}

export default function TransactionHistory({ transactionHistory = [] }: TransactionHistoryProps) {
  const [transactions, setTransactions] = useState('')

  return (
    <div>
      <form className="mt-3 flex flex-col gap-2 sm:flex-row" onSubmit={(e) => e.preventDefault()}>
        <input
          value={transactions}
          onChange={(e) => setTransactions(e.target.value)}
          type="text"
          placeholder="Search By Stocks"
          required
          className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
        />
      </form>
      <div className="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
        <table className="w-full min-w-[800px] table-fixed border-collapse">
          <thead>
            <tr className="bg-slate-50 text-slate-500">
              <th className="w-[12%] border-b border-slate-300 p-4 text-left text-xs">
                Ticker
              </th>
              <th className="w-[8%] border-b border-slate-300 p-4 text-left text-xs">
                Lot
              </th>
              <th className="w-[10%] border-b border-slate-300 p-4 text-left text-xs">
                Est. Price
              </th>
              <th className="w-[12%] border-b border-slate-300 p-4 text-left text-xs">
                Amount Done
              </th>
              <th className="w-[15%] border-b border-slate-300 p-4 text-left text-xs">
                Date / Type
              </th>
              <th className="w-[12%] border-b border-slate-300 p-4 text-left text-xs">
                Broker
              </th>
            </tr>
          </thead>
          <tbody>
            {transactionHistory.length === 0 ? (
              <tr>
                <td colSpan={6} className="p-2 text-sm text-center text-slate-500">
                  Tidak ada data transaksi.
                </td>
              </tr>
            ) : (
              transactionHistory.map((transaction) => {
                const shares = (transaction.stock.lot || 0) * 100
                const estPrice = shares > 0 ? transaction.amount_done / shares : 0
                return (
                  <tr key={transaction.id}>
                    <td className="border-b border-slate-100 p-4 text-sm font-semibold">
                      {transaction.stock.ticker}
                    </td>
                    <td className="border-b border-slate-100 p-2 text-sm">
                      {formatNumber(transaction.stock.lot, 0)}
                    </td>
                    <td className="border-b border-slate-100 p-2 text-sm">
                      {formatCurrency(estPrice)}
                    </td>
                    <td className="border-b border-slate-100 p-2 text-sm">
                      {formatCurrency(transaction.amount_done)}
                    </td>
                    <td className="border-b border-slate-100 p-2 text-sm">
                      <div className="flex items-center gap-2">
                        <span className="whitespace-nowrap">{transaction.date}</span>
                        <span
                          className={`px-2 py-0.5 rounded text-xs font-semibold ${transaction.type === 'BUY' ? 'bg-emerald-100 text-emerald-800' : 'bg-rose-100 text-rose-800'}`}
                        >
                          {transaction.type}
                        </span>
                      </div>
                    </td>
                    <td className="border-b border-slate-100 p-2 text-sm">
                      {transaction.broker.name}
                    </td>
                  </tr>
                )
            }
            )
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
