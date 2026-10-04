// Formatting helpers — port of the original useFormatters composable.

export function formatCurrency(value: number | string | null | undefined): string {
  const num = Number(value || 0)
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    maximumFractionDigits: 2,
  }).format(num)
}

export function formatNumber(value: number | string | null | undefined, digits = 2): string {
  return Number(value || 0).toLocaleString('id-ID', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })
}

export function formatPercent(value: number | string | null | undefined): string {
  const n = Number(value || 0)
  const signed = n > 0 ? `+${formatNumber(n, 2)}` : formatNumber(n, 2)
  return `${signed}%`
}

export function plClass(value: number | string | null | undefined): string {
  return Number(value || 0) >= 0 ? 'text-emerald-700 font-semibold' : 'text-rose-700 font-semibold'
}

export function useFormatters() {
  return { formatCurrency, formatNumber, formatPercent, plClass }
}
