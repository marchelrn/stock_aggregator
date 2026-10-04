import { useState } from 'react'
import { formatNumber } from '../lib/formatters'
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
        <table className="w-full min-w-[800px] border-collapse">
          <thead>
            <tr className="bg-slate-50 text-slate-500">
              <th className="border-b border-slate-300 p-2 text-left text-xs">Ticker</th>
              <th className="border-b border-slate-300 p-2 text-left text-xs">Type</th>
              <th className="border-b border-slate-300 p-2 text-left text-xs">Lot Done</th>
              <th className="border-b border-slate-300 p-2 text-left text-xs">Amount Done</th>
              <th className="border-b border-slate-300 p-2 text-left text-xs">Broker</th>
              <th className="border-b border-slate-300 p-2 text-left text-xs">Date</th>
            </tr>
          </thead>
          <tbody>
            {transactionHistory.length === 0 ? (
              <tr>
                <td colSpan={6} className="p-3 text-sm text-center text-slate-500">
                  Tidak ada data transaksi.
                </td>
              </tr>
            ) : (
              transactionHistory.map((transaction) => (
                <tr key={transaction.id}>
                  <td className="border-b border-slate-100 p-2 text-sm font-semibold">
                    {transaction.stock.ticker}
                  </td>
                  <td className="border-b border-slate-100 p-2 text-sm">{transaction.type}</td>
                  <td className="border-b border-slate-100 p-2 text-sm">
                    {formatNumber(transaction.stock.lot, 0)}
                  </td>
                  <td className="border-b border-slate-100 p-2 text-sm">
                    {formatNumber(transaction.amount_done, 0)}
                  </td>
                  <td className="border-b border-slate-100 p-2 text-sm">
                    {transaction.broker.name}
                  </td>
                  <td className="border-b border-slate-100 p-2 text-sm">{transaction.date}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
