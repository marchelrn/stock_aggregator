<template>
  <div class="mx-auto grid w-full max-w-3xl gap-6 px-4 py-8">
    <div class="rounded-2xl border border-slate-300 bg-white/80 p-6 shadow-sm backdrop-blur">
      <h1 class="text-2xl font-bold text-slate-800">Initial Setup</h1>
      <p class="mt-2 text-sm text-slate-500">Silakan konfigurasikan broker pertama Anda beserta cash dan saham awal yang Anda miliki saat ini.</p>
    </div>

    <section class="rounded-2xl border border-slate-300 bg-white/80 p-6 shadow-sm backdrop-blur">
      <h2 class="mb-4 text-lg font-semibold">1. Hubungkan Broker</h2>
      <form class="grid gap-3" @submit.prevent="submitBroker">
        <label class="text-sm text-slate-600" for="select-broker">Pilih Sekuritas</label>
        <select id="select-broker" v-model="brokerForm.name" required class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm">
          <option value="" disabled>-- Pilih Sekuritas --</option>
          <option value="Mandiri Sekuritas">Mandiri Sekuritas (Growing/MOST)</option>
          <option value="Stockbit Sekuritas">Stockbit Sekuritas</option>
          <option value="BNI Sekuritas">BNI Sekuritas (BIONS)</option>
        </select>
        
        <label class="text-sm text-slate-600" for="initial-cash">Initial Cash (IDR)</label>
        <input id="initial-cash" v-model.number="brokerForm.cash" type="number" min="0" step="0.01" class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" placeholder="0" />

        <div v-if="brokerForm.name === 'Mandiri Sekuritas'" class="grid gap-1">
          <label class="text-sm text-slate-600" for="broker-pin">PIN Trade Confirmation</label>
          <input id="broker-pin" v-model="brokerForm.pin" type="password" required class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" placeholder="Masukkan PIN" />
          <span class="text-xs text-slate-500">PIN Anda aman dan akan dienkripsi sebelum disimpan.</span>
        </div>

        <button type="submit" class="mt-2 w-full rounded-lg bg-teal-700 px-4 py-2 font-semibold text-white hover:bg-teal-800 disabled:opacity-50" :disabled="isBrokerAdded">
          {{ isBrokerAdded ? 'Broker Tersimpan ✓' : 'Simpan Broker' }}
        </button>
      </form>
    </section>

    <section v-if="isBrokerAdded" class="rounded-2xl border border-slate-300 bg-white/80 p-6 shadow-sm backdrop-blur">
      <h2 class="mb-4 text-lg font-semibold">2. Masukkan Portfolio Awal</h2>
      
      <div v-if="localHoldings.length > 0" class="mb-4 overflow-hidden rounded-xl border border-slate-300 bg-slate-50">
        <table class="w-full text-left text-sm">
          <thead class="bg-slate-100 text-xs text-slate-500">
            <tr>
              <th class="p-2 pl-3">Ticker</th>
              <th class="p-2">Lot</th>
              <th class="p-2">Avg Price</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(h, idx) in localHoldings" :key="idx" class="border-t border-slate-300">
              <td class="p-2 pl-3 font-semibold">{{ h.ticker }}</td>
              <td class="p-2">{{ h.lot }}</td>
              <td class="p-2">{{ h.price }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <form class="grid gap-3 rounded-xl border border-slate-300 bg-slate-50 p-4" @submit.prevent="addLocalHolding">
        <div class="grid grid-cols-3 gap-3">
          <div class="grid gap-1">
            <label class="text-xs text-slate-600">Ticker</label>
            <input v-model="holdingForm.ticker" type="text" placeholder="BBCA" required class="rounded-md border border-slate-300 px-2 py-1.5 text-sm uppercase" />
          </div>
          <div class="grid gap-1">
            <label class="text-xs text-slate-600">Lot</label>
            <input v-model.number="holdingForm.lot" type="number" min="1" required class="rounded-md border border-slate-300 px-2 py-1.5 text-sm" />
          </div>
          <div class="grid gap-1">
            <label class="text-xs text-slate-600">Avg Price</label>
            <input v-model.number="holdingForm.price" type="number" min="1" step="0.01" required class="rounded-md border border-slate-300 px-2 py-1.5 text-sm" />
          </div>
        </div>
        <button type="submit" class="w-full rounded-lg bg-slate-200 px-4 py-1.5 text-sm font-semibold text-slate-700 hover:bg-slate-300">
          + Tambah Saham
        </button>
      </form>
    </section>

    <div v-if="isBrokerAdded" class="flex justify-end pt-2">
      <button @click="finishSetup" class="rounded-lg bg-teal-700 px-6 py-3 font-bold text-white shadow-lg hover:bg-teal-800">
        Selesai & Buka Dashboard →
      </button>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { usePortfolio } from '../composables/usePortfolio'
import { useToast } from 'vue-toastification'

const router = useRouter()
const toast = useToast()
const { addBroker, createTransaction, state } = usePortfolio()

const isBrokerAdded = ref(false)
const localHoldings = ref([])

const brokerForm = reactive({
  name: '',
  cash: 0,
  pin: ''
})

const holdingForm = reactive({
  ticker: '',
  lot: 1,
  price: 0
})

const submitBroker = async () => {
  if (!brokerForm.name) return
  if (brokerForm.name === 'Mandiri Sekuritas' && !brokerForm.pin) {
    toast.error('PIN wajib diisi untuk Mandiri Sekuritas')
    return
  }
  
  // Create broker
  const success = await addBroker(brokerForm.name, brokerForm.cash, brokerForm.pin)
  if (success) {
    isBrokerAdded.value = true
  }
}

const addLocalHolding = () => {
  if (!holdingForm.ticker || holdingForm.lot <= 0 || holdingForm.price <= 0) return
  
  localHoldings.value.push({
    ticker: holdingForm.ticker.toUpperCase(),
    lot: holdingForm.lot,
    price: holdingForm.price
  })
  
  holdingForm.ticker = ''
  holdingForm.lot = 1
  holdingForm.price = 0
}

const finishSetup = async () => {
  // Sync local holdings as initial BUY transactions if any
  if (localHoldings.value.length > 0) {
    // We get the recently added broker from the state
    const broker = state.brokers.find(b => b.name === brokerForm.name)
    
    if (broker) {
      for (const holding of localHoldings.value) {
        await createTransaction({
          type: 'BUY',
          ticker: holding.ticker,
          broker_id: Number(broker.id),
          broker_name: broker.name,
          lot: holding.lot,
          price: holding.price
        })
      }
    }
  }
  
  // Set setup as completed and go to dashboard
  localStorage.setItem('isSetupCompleted', 'true')
  toast.success('Setup awal berhasil diselesaikan!')
  router.push('/')
}
</script>
