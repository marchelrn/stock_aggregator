const amountFormat = new Intl.NumberFormat('id-ID', { maximumFractionDigits: 2 })

export function formatCashAmount(amount: number): string {
  return Number.isFinite(amount) ? amountFormat.format(amount) : ''
}

export function formatCashInput(input: string): string | null {
  const raw = input.replace(/\./g, '')
  if (!/^\d*(,\d{0,2})?$/.test(raw)) return null
  if (raw === '') return ''
  const [whole, fraction] = raw.split(',')
  const grouped = (whole || '0').replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  return fraction === undefined ? grouped : `${grouped},${fraction}`
}

export function parseCashAmount(input: string): number | null {
  if (!/^(?:\d+|\d{1,3}(?:\.\d{3})+)(?:,\d{0,2})?$/.test(input)) return null
  const amount = Number(input.replace(/\./g, '').replace(',', '.'))
  return Number.isFinite(amount) && amount >= 0 ? amount : null
}

export function formatPastedCash(input: string): string | null {
  const text = input.trim().replace(/^Rp\.?\s*/i, '')
  const local = parseCashAmount(text)
  if (local !== null && !/^0\.\d+$/.test(text)) return formatCashAmount(local)
  // Clipboard values from spreadsheets/APIs may use a decimal point.
  if (!/^\d+\.\d+$/.test(text)) return null
  const amount = Number(text)
  return Number.isFinite(amount) ? formatCashAmount(amount) : null
}

export function cashInputCaret(input: string, caret: number, formatted: string): number {
  let remaining = input.slice(0, caret).replace(/\./g, '').length
  if (remaining === 0) return 0
  if (input.replace(/\./g, '').startsWith(',')) remaining++
  for (let index = 0; index < formatted.length; index++) {
    if (formatted[index] !== '.') remaining--
    if (remaining === 0) return index + 1
  }
  return formatted.length
}
