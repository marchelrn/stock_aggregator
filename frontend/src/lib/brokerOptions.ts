export const BROKERS = [
  {
    name: 'Stockbit Sekuritas',
    needsPin: false,
    disabled: false,
    gmail: { trades: true, withdrawal: true, deposit: true, dividend: true },
  },
  {
    name: 'Trimegah Sekuritas',
    needsPin: false,
    disabled: false,
    gmail: { trades: true, withdrawal: true, deposit: true, dividend: false },
  },
  {
    name: 'Mandiri Sekuritas',
    needsPin: true,
    disabled: false,
    gmail: { trades: true, withdrawal: false, deposit: false, dividend: true },
  },
  {
    name: 'Ajaib Sekuritas',
    needsPin: false,
    disabled: false,
    gmail: { trades: true, withdrawal: false, deposit: false, dividend: false },
  },
  {
    name: 'BNI Sekuritas',
    needsPin: false,
    disabled: true,
    gmail: { trades: false, withdrawal: false, deposit: false, dividend: false },
  },
]

export function findBrokerOption(name: string) {
  return BROKERS.find((broker) => broker.name.toLowerCase() === name.trim().toLowerCase())
}
