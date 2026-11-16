import { formatCurrency, formatNumber } from '../lib/formatters'
import { Pencil, X } from 'lucide-react'
import type { Broker } from '../types'

interface BrokerTableProps {
  brokers?: Broker[]
  selectedBrokerIds?: number[]
  allocations?: Record<number, number>
  onToggle?: (id: number) => void
  onRemove?: (broker: Broker) => void
  onEditCash?: (broker: Broker) => void
}

export default function BrokerTable({
  brokers = [],
  selectedBrokerIds = [],
  allocations = {},
  onToggle,
  onRemove,
  onEditCash,
}: BrokerTableProps) {
  const isOnlyBroker = brokers.length === 1
  return (
    <div className="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
      <table className="w-full min-w-[560px] border-collapse">
        <thead>
          <tr className="bg-slate-50 text-slate-500">
            <th className="w-[5%] border-b border-slate-300 p-1 text-center text-xs">No</th>
            <th className="w-[20%] border-b border-slate-300 p-1 text-center text-xs">Name</th>
            <th className="w-[14%] border-b border-slate-300 p-1 pr-7 text-right text-xs">
              Cash
            </th>
            <th className="w-[18%] border-b border-slate-300 p-1 text-right text-xs">
              <div className='w-full text-right'>
                Broker's Allocations (%)
              </div>
            </th>
            <th className="w-[19%] border-b border-slate-300 p-1 text-center text-xs">View Holdings</th>
            <th className="w-[14%] border-b border-slate-300 p-1 text-center text-xs">Disconnect</th>
          </tr>
        </thead>
        <tbody>
          {!brokers || brokers.length === 0 ? (
            <tr>
              <td colSpan={6} className="p-3 text-sm text-slate-500">
                Belum ada broker.
              </td>
            </tr>
          ) : (
            brokers.map((broker, index) => (
              <tr key={broker.id}>
                <td className="border-b border-slate-100 p-1 text-sm text-center">
                  <code>{index + 1}</code>
                </td>
                <td className="border-b border-slate-100 p-1 text-sm text-center">{broker.name}</td>
                <td className="relative border-b border-slate-100 p-1 pr-7 text-sm">
                  <div className="w-full text-right tabular-nums">
                    {formatCurrency(broker.cash)}
                  </div>

                  <button
                    type="button"
                    onClick={() => onEditCash?.(broker)}
                    aria-label={`Atur saldo kas ${broker.name}`}
                    title="Atur saldo kas"
                    className="absolute right-0 top-1/2 inline-flex h-7 w-5 -translate-y-1/2 cursor-pointer items-center justify-center rounded-md"
                  >
                    <Pencil size={13} aria-hidden="true" />
                  </button>
                </td>
                <td className="border-b border-slate-100 p-1 text-sm">
                  <div className="w-full text-right tabular-nums">
                    {formatNumber(allocations[broker.id] ?? 0, 1)}%
                  </div>
                </td>
                <td className="border-b border-slate-100 p-1 text-sm text-center">
                  <label className={`relative inline-flex cursor-pointer items-center ${isOnlyBroker ? 'cursor-not-allowed' : 'cursor-pointer'}`}>
                    <input
                      type="checkbox"
                      className="peer sr-only"
                      checked={selectedBrokerIds.includes(broker.id)}
                      onChange={() => onToggle?.(broker.id)}
                      disabled={isOnlyBroker}
                    />
                    <div className="peer h-5 w-9 rounded-full bg-slate-200 after:absolute after:left-[2px] after:top-[2px] after:h-4 after:w-4 after:rounded-full after:border after:border-slate-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-teal-600 peer-checked:after:translate-x-full peer-checked:after:border-white peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-teal-300"></div>
                  </label>
                </td>
                <td className="border-b border-slate-100 p-1 text-center">
                  <button
                    type="button"
                    onClick={() => onRemove?.(broker)}
                    aria-label={`Disconnect ${broker.name}`}
                    title={`Disconnect ${broker.name}`}
                    className="inline-flex h-7 w-7 items-center justify-center rounded-md bg-rose-600 text-white transition hover:bg-rose-700 focus:outline-none focus:ring-2 focus:ring-rose-300"
                  >
                    <X size={16} />
                  </button>
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}
