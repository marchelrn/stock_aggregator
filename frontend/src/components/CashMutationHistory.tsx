import { useState } from 'react'
import { formatCurrency } from '../lib/formatters'
import type { CashMutationData } from '../types'

interface CashMutationHistoryProps {
  cashMutations?: CashMutationData[]
}

export default function CashMutationHistory({ cashMutations = [] }: CashMutationHistoryProps) {
  const [searchTerm, setSearchTerm] = useState('')

  const filtered = cashMutations.filter((m) => {
    if (!searchTerm.trim()) return true
    const term = searchTerm.toLowerCase()
    return (
      m.broker_name?.toLowerCase().includes(term) ||
      m.type?.toLowerCase().includes(term) ||
      m.ref_no?.toLowerCase().includes(term) ||
      m.description?.toLowerCase().includes(term)
    )
  })

  return (
    <div>
      <form className="mt-3 flex flex-col gap-1 sm:flex-row" onSubmit={(e) => e.preventDefault()}>
        <input
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          type="text"
          placeholder="Cari broker atau keterangan mutasi..."
          className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-teal-600 focus:outline-none"
        />
      </form>
      <div className="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
        <table className="w-full table-fixed border-collapse">
          <thead>
            <tr className="bg-slate-50 text-slate-500">
              <th className="border-b border-slate-300 p-2 text-center text-xs w-1/4">
                Broker & Tipe
              </th>
              <th className="border-b border-slate-300 p-2 text-center text-xs w-1/4">
                Nominal
              </th>
              <th className="border-b border-slate-300 p-2 text-center text-xs w-1/4">
                Keterangan / Ref No
              </th>
              <th className="border-b border-slate-300 p-2 text-center text-xs w-1/4">
                Tanggal
              </th>
            </tr>
          </thead>
          <tbody>
            {filtered.length === 0 ? (
              <tr>
                <td colSpan={4} className="p-4 text-sm text-center text-slate-500">
                  Tidak ada data mutasi kas.
                </td>
              </tr>
            ) : (
              filtered.map((mutation) => {
                const isDeposit = mutation.type === 'DEPOSIT'
                return (
                  <tr key={mutation.id} className="hover:bg-slate-50/50 transition">
                    <td className="border-b border-slate-100 p-3 text-center">
                      <div className="flex flex-col items-center gap-1">
                        <div className="flex items-center gap-1.5">
                          <span className="font-semibold text-sm">{mutation.broker_name}</span>
                          <span
                            className={`px-2 py-0.5 rounded text-[11px] font-bold ${
                              isDeposit ? 'bg-emerald-100 text-emerald-800' : 'bg-rose-100 text-rose-800'
                            }`}
                          >
                            {mutation.type}
                          </span>
                        </div>
                      </div>
                    </td>
                    <td className="border-b border-slate-100 p-2 text-sm text-center font-semibold">
                      <span className={isDeposit ? 'text-emerald-700' : 'text-rose-700'}>
                        {isDeposit ? '+' : '-'}{formatCurrency(mutation.amount)}
                      </span>
                    </td>
                    <td className="border-b border-slate-100 p-2 text-xs text-center text-slate-600">
                      <div>{mutation.description || '-'}</div>
                      {mutation.ref_no && (
                        <div className="text-[11px] font-mono text-slate-400">Ref: {mutation.ref_no}</div>
                      )}
                    </td>
                    <td className="border-b border-slate-100 p-2 text-xs text-center text-slate-500">
                      {new Date(mutation.date).toLocaleString()}
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
