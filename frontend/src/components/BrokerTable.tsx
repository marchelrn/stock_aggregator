import { formatCurrency } from '../lib/formatters'
import { X } from 'lucide-react'
import type { Broker } from '../types'

interface BrokerTableProps {
  brokers?: Broker[]
  selectedBrokerIds?: number[]
  onToggle?: (id: number) => void
  onRemove?: (broker: Broker) => void
}

export default function BrokerTable({
  brokers = [],
  selectedBrokerIds = [],
  onToggle,
  onRemove,
}: BrokerTableProps) {
  return (
    <div className="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
      <table className="w-full min-w-[560px] border-collapse">
        <thead>
          <tr className="bg-slate-50 text-slate-500">
            <th className="border-b border-slate-300 p-2 text-center text-xs">No</th>
            <th className="border-b border-slate-300 p-2 text-center text-xs">Name</th>
            <th className="border-b border-slate-300 p-2 text-center text-xs">Cash</th>
            <th className="border-b border-slate-300 p-2 text-center text-xs">View Holdings</th>
            <th className="border-b border-slate-300 p-2 text-center text-xs">Disconnect</th>
          </tr>
        </thead>
        <tbody>
          {!brokers || brokers.length === 0 ? (
            <tr>
              <td colSpan={5} className="p-3 text-sm text-slate-500">
                Belum ada broker.
              </td>
            </tr>
          ) : (
            brokers.map((broker, index) => (
              <tr key={broker.id}>
                <td className="border-b border-slate-100 p-2 text-sm text-center">
                  <code>{index + 1}</code>
                </td>
                <td className="border-b border-slate-100 p-2 text-sm text-center">{broker.name}</td>
                <td className="border-b border-slate-100 p-2 text-sm text-center">
                  {formatCurrency(broker.cash)}
                </td>
                <td className="border-b border-slate-100 p-2 text-sm text-center">
                  <label className="relative inline-flex cursor-pointer items-center">
                    <input
                      type="checkbox"
                      className="peer sr-only"
                      checked={selectedBrokerIds.includes(broker.id)}
                      onChange={() => onToggle?.(broker.id)}
                    />
                    <div className="peer h-5 w-9 rounded-full bg-slate-200 after:absolute after:left-[2px] after:top-[2px] after:h-4 after:w-4 after:rounded-full after:border after:border-slate-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-teal-600 peer-checked:after:translate-x-full peer-checked:after:border-white peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-teal-300"></div>
                  </label>
                </td>
                <td className="border-b border-slate-100 p-2 text-center">
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
