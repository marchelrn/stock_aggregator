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
            <th className="w-[12%] border-b border-slate-300 p-2 text-center text-xs">Ticker</th>
            <th className="w-[12%] border-b border-slate-300 p-2 text-center text-xs">Market Value</th>
            <th className="w-[12%] border-b border-slate-300 p-2 text-center text-xs">Sector</th>
            <th className="w-[12%] border-b border-slate-300 p-2 text-center text-xs">Broker</th>
            <th className="w-[12%] border-b border-slate-300 p-2 text-center text-xs">Weight (%)</th>
          </tr>
        </thead>
        <tbody>
          {!holdings || holdings.length === 0 ? (
            <tr>
              <td colSpan={6} className="p-2 text-center text-sm text-slate-500">
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
                  {/*Market Value*/}
                </td>
                <td className="border-b border-slate-100 p-2 text-sm">
                  {/*Sector*/}
                </td>
                <td className="border-b border-slate-100 text-sm">{holding.broker_name}</td>
                <td>
                  {/*Weight %*/}
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}
