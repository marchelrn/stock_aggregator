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
      <form className="mt-3       flex flex-col gap-2 sm:flex-row" onSubmit={(e) => e.preventDefault()}>
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
              <th className="w-[15%] border-b border-slate-300 p-4 text-left text-xs">
                Ordered Items
              </th>
              <th className="w-[10%] border-b border-slate-300 p-4 text-left text-xs">
                Lot Done
              </th>
              <th className="w-[10%] border-b border-slate-300 p-4 text-left text-xs">
                Price
              </th>
              <th className="w-[14%] border-b border-slate-300 p-4 text-left text-xs">
                Amount Done
              </th>
              <th className="w-[11%] border-b border-slate-300 p-4 text-left text-xs">
                Total Fee
              </th>
              <th className="w-[12%] border-b border-slate-300 p-4 text-left text-xs">
                Net Amount
              </th>
              <th className="w-[10%] border-b border-slate-300 p-4 text-left text-xs">
                Date
              </th>
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
                  <td className="border-b border-slate-100 p-4 text-sm font-semibold">
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
