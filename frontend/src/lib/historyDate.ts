const localDate = new Intl.DateTimeFormat('id-ID', { dateStyle: 'long' })
const localTime = new Intl.DateTimeFormat('id-ID', {
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23',
  timeZoneName: 'short',
})

export function historyDate(value: string) {
  const instant = new Date(value)
  if (!value || Number.isNaN(instant.getTime())) {
    return { label: 'Tanggal tidak tersedia', clock: '', dateTime: undefined, time: -Infinity }
  }

  const iso = instant.toISOString()
  const dateOnly = /^\d{4}-\d{2}-\d{2}$/.test(value)
  const dateTime = dateOnly ? iso.slice(0, 10) : iso
  const date = dateOnly ? new Date(dateTime + 'T00:00:00') : instant

  return {
    label: localDate.format(date),
    clock: dateOnly ? '' : localTime.format(date),
    dateTime,
    time: date.getTime(),
  }
}
