<template>
  <div class="mx-auto flex w-full max-w-5xl flex-col gap-4 px-4">
    <section class="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
      <div class="flex items-center justify-between gap-2">
        <h2 class="text-xl font-semibold">Brokers</h2>
        <button class="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800" @click="configureModal.open = true">Configure Brokers</button>
      </div>
      <BrokerTable :brokers="state.brokers" :selectedBrokerId="selectedBrokerId" @toggle="handleToggleBroker" />
    </section>

    <section class="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
      <div class="flex items-center justify-between gap-2">
        <h2 class="text-xl font-semibold">Holdings by Broker Selected</h2>
        <span v-if="selectedBrokerId" class="text-sm text-slate-500">Filtered by selected broker</span>
      </div>
      <HoldingTable :holdings="filteredHoldings" @trade="handleTradeHolding" />
    </section>

    <section class="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
      <div class="flex items-center justify-between gap-2">
        <h2 class="text-xl font-semibold">History</h2>
      </div>
      <TransactionHistory :transactionHistory="state.transactionHistory" />
    </section>

    <section class="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
      <div class="flex items-center justify-between gap-2">
        <h2 class="text-xl font-semibold">Live Stock Prices</h2>
      </div>
      <StockPrices
        :prices="state.liveStockPrices"
        @fetch="handleFetchStockPrices"
        @clear="handleClearLiveStockPrices"
      />
    </section>
  </div>

  <div v-if="tradeModal.open" class="fixed inset-0 z-50 grid place-items-center bg-slate-900/50 p-4" @click="closeTradeModal">
    <div class="w-full max-w-md rounded-2xl border border-slate-300 bg-white p-4 shadow-2xl" @click.stop>
      <div class="mb-2 flex items-center justify-between">
        <h3 class="text-lg font-semibold">{{ tradeModal.type }} {{ tradeModal.holding?.ticker }}</h3>
        <button class="rounded p-1 text-slate-500 hover:bg-slate-100" @click="closeTradeModal">✕</button>
      </div>

      <p class="mb-3 text-sm text-slate-500">Ticker: <strong>{{ tradeModal.holding?.ticker }}</strong></p>

      <form class="grid gap-3" @submit.prevent="submitTradeModal">
        <label class="text-sm text-slate-600" for="trade-broker">Broker</label>
        <select id="trade-broker" v-model.number="tradeForm.brokerId" :disabled="tradeModal.type === 'SELL'" required class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm">
          <option v-for="b in state.brokers" :key="b.id" :value="b.id">{{ b.name }} (ID: {{ b.id }})</option>
        </select>

        <label class="text-sm text-slate-600" for="trade-lot">Lot</label>
        <input id="trade-lot" v-model.number="tradeForm.lot" type="number" min="1" required class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" />

        <label class="text-sm text-slate-600" for="trade-price">Harga per lembar</label>
        <input id="trade-price" v-model.number="tradeForm.price" type="number" min="1" step="0.01" required class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" />

        <div class="rounded-lg border border-slate-300 bg-slate-50 p-2 text-sm">
          Estimasi nilai transaksi: <strong>{{ tradeValueText }}</strong>
        </div>

        <div class="mt-1 flex justify-end gap-2">
          <button type="button" class="rounded-lg bg-slate-200 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-300" @click="closeTradeModal">Batal</button>
          <button type="submit" :class="tradeModal.type === 'BUY' ? 'rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800' : 'rounded-lg bg-rose-700 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-800'">
            Confirm {{ tradeModal.type }}
          </button>
        </div>
      </form>
    </div>
  </div>

  <div v-if="configureModal.open" class="fixed inset-0 z-50 grid place-items-center bg-slate-900/50 p-4" @click="configureModal.open = false">
    <div class="w-full max-w-md rounded-2xl border border-slate-300 bg-white p-4 shadow-2xl" @click.stop>
      <div class="mb-2 flex items-center justify-between">
        <h3 class="text-lg font-semibold">Configure Broker</h3>
        <button class="rounded p-1 text-slate-500 hover:bg-slate-100" @click="configureModal.open = false">✕</button>
      </div>

      <p class="mb-4 text-sm text-slate-500">Pilih sekuritas yang didukung untuk integrasi trade confirmation dari Gmail.</p>

      <form class="grid gap-3" @submit.prevent="submitConfigureBroker">
        <label class="text-sm text-slate-600" for="select-broker">Pilih Broker</label>
        <select id="select-broker" v-model="configureForm.brokerName" required class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm">
          <option value="" disabled>-- Pilih Sekuritas --</option>
          <option value="Mandiri Sekuritas">Mandiri Sekuritas (Growing/MOST)</option>
          <option value="Stockbit Sekuritas">Stockbit Sekuritas</option>
          <option value="BNI Sekuritas">BNI Sekuritas (BIONS)</option>
        </select>
        
        <label class="text-sm text-slate-600" for="initial-cash">Initial Cash (Opsional)</label>
        <input id="initial-cash" v-model.number="configureForm.cash" type="number" min="0" step="0.01" class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" placeholder="0" />

        <div v-if="configureForm.brokerName === 'Mandiri Sekuritas'" class="grid gap-1">
          <label class="text-sm text-slate-600" for="broker-pin">PIN Trade Confirmation</label>
          <input id="broker-pin" v-model="configureForm.pin" type="password" required class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" placeholder="Masukkan PIN" />
          <span class="text-xs text-slate-500">PIN Anda aman dan akan dienkripsi sebelum disimpan.</span>
        </div>

        <div class="mt-3 flex justify-end gap-2">
          <button type="button" class="rounded-lg bg-slate-200 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-300" @click="configureModal.open = false">Batal</button>
          <button type="submit" class="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800">
            Configure
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { usePortfolio } from '../composables/usePortfolio'
import { useFormatters } from '../composables/useFormatters'

import BrokerTable from '../components/BrokerTable.vue'
import HoldingTable from '../components/HoldingTable.vue'
import TransactionHistory from '../components/TransactionHistory.vue'
import StockPrices from '../components/StockPrices.vue'

const {
  state,
  loadDashboard,
  addBroker,
  fetchStockPrices,
  clearLiveStockPrices,
  setStatus
} = usePortfolio()

const { formatCurrency } = useFormatters()

const selectedBrokerId = ref(null)

const handleToggleBroker = (id) => {
  if (selectedBrokerId.value === id) {
    selectedBrokerId.value = null // Deselect if clicked again (though radio input might not re-trigger natively, we handle logic)
  } else {
    selectedBrokerId.value = id
  }
}

watch(() => state.brokers, (brokers) => {
  if (brokers && brokers.length > 0 && selectedBrokerId.value === null) {
    selectedBrokerId.value = brokers[0].id
  }
}, { immediate: true })

const filteredHoldings = computed(() => {
  if (selectedBrokerId.value === null) return []
  return state.holdings.filter(h => Number(h.broker_id) === selectedBrokerId.value)
})

const configureModal = reactive({
  open: false
})

const configureForm = reactive({
  brokerName: '',
  cash: 0,
  pin: ''
})

const submitConfigureBroker = async () => {
  if (!configureForm.brokerName) {
    setStatus('Pilih broker terlebih dahulu', true)
    return
  }
  
  // Jika Mandiri Sekuritas, pastikan PIN diisi (tambahan keamanan logic di sisi frontend)
  if (configureForm.brokerName === 'Mandiri Sekuritas' && !configureForm.pin) {
    setStatus('PIN Trade Confirmation wajib diisi untuk Mandiri Sekuritas', true)
    return
  }
  
  await addBroker(configureForm.brokerName, configureForm.cash || 0, configureForm.pin)
  
  configureModal.open = false
  configureForm.brokerName = ''
  configureForm.cash = 0
  configureForm.pin = ''
}

const tradeModal = reactive({
  open: false,
  type: 'BUY',
  holding: null
})

const tradeForm = reactive({
  brokerId: null,
  lot: 1,
  price: 0
})

const selectedTradeBroker = computed(() => {
  return state.brokers.find((b) => Number(b.id) === Number(tradeForm.brokerId)) || null
})

const tradeValueText = computed(() => {
  const lot = Number(tradeForm.lot || 0)
  const price = Number(tradeForm.price || 0)
  return formatCurrency(lot * 100 * price)
})

const handleTradeHolding = ({ type, holding }) => {
  tradeModal.open = true
  tradeModal.type = type
  tradeModal.holding = holding
  tradeForm.brokerId = Number(holding.broker_id)
  tradeForm.lot = 1
  tradeForm.price = Number(holding.avg_price || 0)
}

const closeTradeModal = () => {
  tradeModal.open = false
  tradeModal.holding = null
  tradeForm.brokerId = null
}

const handleFetchStockPrices = async (tickers) => {
  if (!tickers) {
    setStatus('Masukkan minimal satu ticker.', true)
    return
  }
  await fetchStockPrices(tickers)
}

const handleClearLiveStockPrices = () => {
  clearLiveStockPrices()
}

onMounted(() => {
  loadDashboard()
})
</script>
