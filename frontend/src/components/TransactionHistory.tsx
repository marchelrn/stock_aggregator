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
        <table className="w-full min-w-[700px] table-fixed border-collapse">
          <thead>
            <tr className="bg-slate-50 text-slate-500">
              <th className="w-[24%] border-b border-slate-300 p-4 text-left text-xs">
                Ticker / Broker
              </th>
              <th className="w-[8%] border-b border-slate-300 p-4 text-left text-xs">
                Lot
              </th>
              <th className="w-[12%] border-b border-slate-300 p-4 text-left text-xs">
                Est. Price
              </th>
              <th className="w-[16%] border-b border-slate-300 p-4 text-left text-xs">
                Amount Done
              </th>
              <th className="w-[20%] border-b border-slate-300 p-4 text-left text-xs">
                Date / Type
              </th>
            </tr>
          </thead>
          <tbody>
            {transactionHistory.length === 0 ? (
              <tr>
                <td colSpan={5} className="p-2 text-sm text-center text-slate-500">
                  Tidak ada data transaksi.
                </td>
              </tr>
            ) : (
              transactionHistory.map((transaction) => {
                const shares = (transaction.stock.lot || 0) * 100
                const estPrice = shares > 0 ? transaction.amount_done / shares : 0
                return (
                  <tr key={transaction.id}>
                    <td className="border-b border-slate-100 p-4">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold">{transaction.stock.ticker}</span>
                        <span
                          className={`px-2 py-0.5 rounded text-xs font-semibold ${transaction.type === 'BUY' ? 'bg-emerald-100 text-emerald-800' : 'bg-rose-100 text-rose-800'}`}
                        >
                          {transaction.type}
                        </span>
                      </div>
                      <div className="text-xs text-slate-500 mt-0.5">
                        {transaction.broker.name}
                      </div>
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
                      {transaction.date}
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
