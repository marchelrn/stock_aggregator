import { ArrowLeft, Check, Minus } from 'lucide-react'
import { Link } from 'react-router-dom'
import ThemeToggle from '../components/ThemeToggle'
import { BROKERS } from '../lib/brokerOptions'

const columns = [
  { key: 'trades', label: 'Transaksi saham' },
  { key: 'withdrawal', label: 'Tarik dana' },
  { key: 'deposit', label: 'Setor dana' },
  { key: 'dividend', label: 'Dividen' },
] as const

export default function BrokerListPage() {
  const brokers = BROKERS.filter((broker) => !broker.disabled)

  return (
    <div className="mx-auto w-full max-w-4xl px-3 pb-10 sm:px-6 sm:pb-16">
      <header className="flex items-center justify-between gap-3 border-b border-slate-200 pb-3">
        <Link
          to="/login"
          className="inline-flex min-h-11 cursor-pointer items-center gap-2 rounded-sm text-sm font-semibold text-slate-700 hover:underline active:text-teal-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-teal-700"
        >
          <ArrowLeft size={16} aria-hidden="true" />
          Kembali ke login
        </Link>
        <ThemeToggle />
      </header>

      <h1 className="mt-8 text-3xl font-bold tracking-tight text-slate-800 sm:mt-12 sm:text-4xl">
        Broker yang didukung
      </h1>
      <p className="mt-3 max-w-lg text-sm leading-relaxed text-slate-600 sm:text-base">
        Lihat data saham BEI dan mutasi rekening dana nasabah yang bisa diimpor otomatis dari Gmail.
      </p>

      <table className="mt-7 w-full table-fixed border-collapse text-[11px] leading-snug sm:mt-9 sm:text-sm">
        <caption className="sr-only">Dukungan impor Gmail per broker</caption>
        <thead className="border-y border-slate-300 bg-slate-50 text-slate-700">
          <tr>
            <th scope="col" className="w-[33%] px-2 py-3 text-left font-semibold sm:px-4">Broker</th>
            {columns.map(({ key, label }) => (
              <th key={key} scope="col" className="px-0.5 py-3 text-center font-semibold sm:px-2">
                {label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {brokers.map(({ name, gmail }) => (
            <tr key={name} className="border-b border-slate-200">
              <th scope="row" className="px-2 py-4 text-left font-semibold text-slate-800 sm:px-4">
                {name}
              </th>
              {columns.map(({ key }) => (
                <td key={key} className="px-0.5 py-4 text-center">
                  {gmail[key] ? (
                    <Check aria-hidden="true" size={17} className="mx-auto text-teal-700" />
                  ) : (
                    <Minus aria-hidden="true" size={17} className="mx-auto text-slate-500" />
                  )}
                  <span className="sr-only">{gmail[key] ? 'Tersedia' : 'Belum tersedia'}</span>
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>

      <div className="mt-4 flex flex-wrap gap-x-5 gap-y-2 text-xs text-slate-600 sm:text-sm">
        <span className="inline-flex items-center gap-1.5">
          <Check aria-hidden="true" size={15} className="text-teal-700" /> Tersedia
        </span>
        <span className="inline-flex items-center gap-1.5">
          <Minus aria-hidden="true" size={15} /> Belum tersedia
        </span>
      </div>
    </div>
  )
}
