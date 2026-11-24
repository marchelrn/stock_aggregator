export const BROKERS = [
  { name: 'Stockbit Sekuritas', needsPin: false, disabled: false },
  { name: 'Trimegah Sekuritas', needsPin: false, disabled: false },
  { name: 'Mandiri Sekuritas', needsPin: true, disabled: false },
  { name: 'Ajaib Sekuritas', needsPin: false, disabled: false },
  { name: 'BNI Sekuritas', needsPin: false, disabled: true },
]

export function findBrokerOption(name: string) {
  return BROKERS.find((broker) => broker.name.toLowerCase() === name.trim().toLowerCase())
}
