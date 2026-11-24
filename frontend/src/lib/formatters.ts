// Formatting helpers, port of the original useFormatters composable.

export function formatCurrency(value: number | string | null | undefined): string {
  const num = Number(value || 0)
  return new Intl.NumberFormat('id-ID', {
    style: 'decimal',
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

export function formatLot(value: number | string | null | undefined): string {
  const num = Number(value || 0)
  const hasFraction = num % 1 !== 0
  return num.toLocaleString('id-ID', {
    minimumFractionDigits: hasFraction ? (Math.round(num * 10) === num * 10 ? 1 : 2) : 0,
    maximumFractionDigits: 2,
  })
}

export function formatPrice(value: number | string | null | undefined): string {
  const num = Number(value || 0)
  return num.toLocaleString('id-ID', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}

export function parseNumberInput(input: string | number | null | undefined): number {
  if (input == null) return 0
  if (typeof input === 'number') return Number.isFinite(input) ? input : 0
  const s = String(input).trim()
  if (!s) return 0

  if (s.includes(',')) {
    const cleaned = s.replace(/\./g, '').replace(',', '.')
    const val = parseFloat(cleaned)
    return Number.isFinite(val) ? Math.max(0, val) : 0
  }

  if (s.includes('.')) {
    const parts = s.split('.')
    if (parts.length === 2 && parts[1].length !== 3) {
      const val = parseFloat(s)
      return Number.isFinite(val) ? Math.max(0, val) : 0
    }
    const cleaned = s.replace(/\./g, '')
    const val = parseFloat(cleaned)
    return Number.isFinite(val) ? Math.max(0, val) : 0
  }

  const val = parseFloat(s)
  return Number.isFinite(val) ? Math.max(0, val) : 0
}

export function plClass(value: number | string | null | undefined): string {
  return Number(value || 0) >= 0 ? 'text-emerald-700 font-semibold' : 'text-rose-700 font-semibold'
}

export function useFormatters() {
  return { formatCurrency, formatNumber, formatPercent, plClass, formatLot, formatPrice }
}
