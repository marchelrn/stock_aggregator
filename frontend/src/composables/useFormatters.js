export function useFormatters() {
  const formatCurrency = (value) => {
    const num = Number(value || 0)
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      maximumFractionDigits: 2
    }).format(num)
  }

  const formatNumber = (value, digits = 2) => {
    return Number(value || 0).toLocaleString('id-ID', {
      minimumFractionDigits: digits,
      maximumFractionDigits: digits
    })
  }

  const formatPercent = (value) => {
    const n = Number(value || 0)
    const signed = n > 0 ? `+${formatNumber(n, 2)}` : formatNumber(n, 2)
    return `${signed}%`
  }

  const plClass = (value) => {
    return Number(value || 0) >= 0 ? 'text-emerald-700 font-semibold' : 'text-rose-700 font-semibold'
  }

  return {
    formatCurrency,
    formatNumber,
    formatPercent,
    plClass
  }
}
