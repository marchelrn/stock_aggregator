import { formatCurrency, formatNumber } from '../lib/formatters'
import type { Holding } from '../types'

interface HoldingTableProps {
  holdings?: Holding[]
}

export default function HoldingTable({ holdings = [] }: HoldingTableProps) {
  return (
    <div className="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
      <table className="w-full min-w-[620px] border-collapse">
        <thead>
          <tr className="bg-slate-50 text-slate-500">
            <th className="border-b border-slate-300 p-2 text-left text-xs">Ticker</th>
            <th className="border-b border-slate-300 p-2 text-left text-xs">Lot</th>
            <th className="border-b border-slate-300 p-2 text-left text-xs">Avg Price</th>
            <th className="border-b border-slate-300 p-2 text-left text-xs">Broker</th>
          </tr>
        </thead>
        <tbody>
          {!holdings || holdings.length === 0 ? (
            <tr>
              <td colSpan={4} className="p-4 text-center text-sm text-slate-500">
                Pilih broker untuk melihat holding.
              </td>
            </tr>
          ) : (
            holdings.map((holding) => (
              <tr key={holding.id || `${holding.ticker}-${holding.broker_id}`}>
                <td className="border-b border-slate-100 p-2 text-sm font-semibold">
                  {holding.ticker}
                </td>
                <td className="border-b border-slate-100 p-2 text-sm">
                  {formatNumber(holding.lot, 0)}
                </td>
                <td className="border-b border-slate-100 p-2 text-sm">
                  {formatCurrency(holding.avg_price)}
                </td>
                <td className="border-b border-slate-100 text-sm">{holding.broker_name}</td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}
