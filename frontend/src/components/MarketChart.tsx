import { useId } from 'react'
import { formatNumber } from '../lib/formatters'
import type { MarketDetail } from '../types'

export default function MarketChart({ ticker, history }: Pick<MarketDetail, 'ticker' | 'history'>) {
  const id = useId()
  const points = history.filter((point) => Number.isFinite(point.close) && Number.isFinite(point.time))
    .slice().sort((a, b) => a.time - b.time)
  if (points.length < 2) {
    return <p className="grid min-h-[220px] place-items-center text-sm text-slate-500">Histori belum cukup untuk menampilkan chart.</p>
  }

  const width = 640
  const height = 240
  const left = 72
  const right = 16
  const top = 16
  const bottom = 30
  const min = Math.min(...points.map((point) => point.close))
  const max = Math.max(...points.map((point) => point.close))
  const padding = (max - min) * 0.1 || Math.max(Math.abs(max) * 0.01, 1)
  const low = min - padding
  const high = max + padding
  const first = points[0]
  const last = points[points.length - 1]
  const x = (time: number) => left + (time - first.time) / (last.time - first.time || 1) * (width - left - right)
  const y = (price: number) => top + (high - price) / (high - low) * (height - top - bottom)
  const line = points.map((point) => `${x(point.time)},${y(point.close)}`).join(' ')
  const dateLabel = (time: number) => new Date(time * 1000).toLocaleDateString('id-ID', { timeZone: 'Asia/Jakarta', day: 'numeric', month: 'short' })
  const rising = last.close >= first.close

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="mt-4 w-full" role="img" aria-labelledby={id}>
      <title id={id}>{ticker}: histori harga harian satu bulan. Awal {formatNumber(first.close)}, terakhir {formatNumber(last.close)}.</title>
      {[0, 0.5, 1].map((fraction) => {
        const value = low + (high - low) * fraction
        return (
          <g key={fraction}>
            <line x1={left} x2={width - right} y1={y(value)} y2={y(value)} className="stroke-slate-200" />
            <text x={left - 8} y={y(value) + 4} textAnchor="end" className="fill-slate-500 text-[11px]">{formatNumber(value, 0)}</text>
          </g>
        )
      })}
      <polyline points={line} fill="none" strokeWidth={2} className={rising ? 'stroke-teal-600' : 'stroke-rose-600'} />
      {points.map((point) => (
        <circle key={point.time} cx={x(point.time)} cy={y(point.close)} r={3} className={rising ? 'fill-teal-600' : 'fill-rose-600'}>
          <title>{dateLabel(point.time)}: {formatNumber(point.close)}</title>
        </circle>
      ))}
      <text x={left} y={height - 6} className="fill-slate-500 text-[11px]">{dateLabel(first.time)}</text>
      <text x={width - right} y={height - 6} textAnchor="end" className="fill-slate-500 text-[11px]">{dateLabel(last.time)}</text>
    </svg>
  )
}
