<template>
  <form class="grid content-start gap-2 rounded-xl border border-slate-300 bg-white p-3" @submit.prevent="handleSubmit">
    <h3 class="text-base font-semibold">Add Transaction</h3>

    <small v-if="validationMessage" :class="validationOk ? 'text-emerald-700' : 'text-rose-700'">{{ validationMessage }}</small>

    <label class="text-sm text-slate-600" for="tx-type">Type</label>
    <select id="tx-type" v-model="type" required class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm">
      <option value="BUY">BUY</option>
      <option value="SELL">SELL</option>
    </select>

    <label class="text-sm text-slate-600" for="tx-broker">Broker</label>
    <select id="tx-broker" v-model.number="brokerId" required class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm">
      <option v-if="brokerOptions.length === 0" value="">Tidak ada broker tersedia</option>
      <option v-else v-for="broker in brokerOptions" :key="broker.id" :value="broker.id">{{ broker.name }} (ID: {{ broker.id }})</option>
    </select>

    <label class="text-sm text-slate-600" for="tx-ticker">Ticker</label>
    <input id="tx-ticker" v-model="ticker" type="text" placeholder="Contoh: BBRI" required @blur="validateBuyTicker" class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" />

    <label class="text-sm text-slate-600" for="tx-lot">Lot</label>
    <input id="tx-lot" v-model.number="lot" type="number" required class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" />

    <label class="text-sm text-slate-600" for="tx-price">Harga per lembar</label>
    <input id="tx-price" v-model.number="price" type="number" step="0.01" required class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" />

    <button type="submit" :disabled="submitting" class="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800 disabled:cursor-not-allowed disabled:opacity-60">
      {{ submitting ? 'Submitting...' : 'Submit Transaction' }}
    </button>
  </form>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useApi } from '../composables/useApi'

const props = defineProps({ brokers: { type: Array, default: () => [] }, holdings: { type: Array, default: () => [] } })
const emit = defineEmits(['submit'])
const { api } = useApi()

const type = ref('BUY')
const ticker = ref('')
const brokerId = ref(null)
const lot = ref(1)
const price = ref(0)
const submitting = ref(false)
const validationMessage = ref('')
const validationOk = ref(false)

const normalizedTicker = computed(() => String(ticker.value || '').trim().toUpperCase())

const sellRows = computed(() => (props.holdings || []).map((h) => ({
  ticker: String(h.ticker || '').toUpperCase(),
  broker_id: Number(h.broker_id),
  broker_name: h.broker_name,
  avg_price: Number(h.avg_price || 0)
})))

const brokerOptions = computed(() => {
  if (type.value === 'BUY') return props.brokers || []
  const ids = new Set(sellRows.value.map((r) => r.broker_id))
  return (props.brokers || []).filter((b) => ids.has(Number(b.id)))
})

console.log("broker" , props.brokers.length)

watch(type, (nextType) => {
  validationMessage.value = ''
  validationOk.value = false
  if (nextType === 'SELL') {
    const first = brokerOptions.value[0]
    brokerId.value = first ? Number(first.id) : null
  }
})

watch(brokerOptions, (list) => {
  if (!list.some((b) => Number(b.id) === Number(brokerId.value))) {
    brokerId.value = list[0] ? Number(list[0].id) : null
  }
})

watch([type, brokerId, normalizedTicker], () => {
  if (!normalizedTicker.value) {
    validationMessage.value = ''
    validationOk.value = false
    return
  }
  if (type.value === 'SELL') {
    const row = sellRows.value.find((r) => r.ticker === normalizedTicker.value && Number(r.broker_id) === Number(brokerId.value))
    if (row) {
      validationOk.value = true
      validationMessage.value = 'Ticker tersedia untuk SELL di broker ini'
    } else {
      validationOk.value = false
      validationMessage.value = 'Ticker ini tidak ada di holding broker terpilih'
    }
  }
})

const validateBuyTicker = async () => {
  if (type.value !== 'BUY') return true
  if (!normalizedTicker.value) return false
  try {
    await api(`/price/${encodeURIComponent(normalizedTicker.value)}`)
    validationOk.value = true
    validationMessage.value = 'Ticker valid (terdaftar / bisa diambil dari IHSG via Yahoo)'
    return true
  } catch {
    validationOk.value = false
    validationMessage.value = 'Ticker tidak valid / tidak ditemukan di IHSG'
    return false
  }
}

const handleSubmit = async () => {
  if (submitting.value) return
  if (!brokerId.value) return (validationOk.value = false), (validationMessage.value = 'Broker wajib dipilih')
  if (!normalizedTicker.value) return (validationOk.value = false), (validationMessage.value = 'Ticker wajib diisi')

  const lotValue = Number(lot.value)
  if (!Number.isFinite(lotValue) || lotValue <= 0) return (validationOk.value = false), (validationMessage.value = 'Lot harus lebih besar dari 0')

  const priceValue = Number(price.value)
  if (!Number.isFinite(priceValue) || priceValue <= 0) return (validationOk.value = false), (validationMessage.value = 'Harga per lembar harus lebih besar dari 0')

  const broker = (props.brokers || []).find((b) => Number(b.id) === Number(brokerId.value))
  if (!broker) return (validationOk.value = false), (validationMessage.value = 'Broker tidak ditemukan')

  if (type.value === 'BUY') {
    const ok = await validateBuyTicker()
    if (!ok) return
  }

  if (type.value === 'SELL') {
    const exists = sellRows.value.some((r) => r.ticker === normalizedTicker.value && Number(r.broker_id) === Number(brokerId.value))
    if (!exists) return (validationOk.value = false), (validationMessage.value = 'SELL ditolak: ticker tidak ada di holding broker ini')
  }

  submitting.value = true
  try {
    await emit('submit', {
      type: type.value,
      ticker: normalizedTicker.value,
      broker_id: Number(broker.id),
      broker_name: broker.name,
      lot: lotValue,
      price: priceValue
    })
    ticker.value = ''
    lot.value = 1
    price.value = 0
    validationMessage.value = ''
    validationOk.value = false
  } finally {
    submitting.value = false
  }
}
</script>
