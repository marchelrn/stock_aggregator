<template>
  <div>
    <form class="mt-3 flex flex-col gap-2 sm:flex-row" @submit.prevent="handleSubmit">
      <input
        v-model="tickers"
        type="text"
        placeholder="Search By Stocks"
        required
        class="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
      />
      <button type="submit" class="rounded-lg bg-teal-700 px-2 py-2 text-xs font-semibold text-white hover:bg-teal-800">Get Prices</button>
      <button type="button" class="rounded-lg bg-slate-500 px-3 py-2 text-sm font-semibold text-white hover:bg-slate-600" @click="handleClear">Clear</button>
    </form>
    <div class="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
      <table class="w-full min-w-[560px] border-collapse">
        <thead>
          <tr class="bg-slate-50 text-slate-500">
            <th class="border-b border-slate-300 p-2 text-left text-xs">Ticker</th>
            <th class="border-b border-slate-300 p-2 text-left text-xs">Price</th>
            <th class="border-b border-slate-300 p-2 text-left text-xs">Prev Close</th>
            <th class="border-b border-slate-300 p-2 text-left text-xs">Change</th>
            <th class="border-b border-slate-300 p-2 text-left text-xs">Change %</th>
            <th class="border-b border-slate-300 p-2 text-left text-xs">Currency</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="entries.length === 0">
            <td colspan="6" class="p-3 text-sm text-slate-500">Tidak ada data harga.</td>
          </tr>
          <tr v-else v-for="price in sortedEntries" :key="price.ticker">
            <td class="border-b border-slate-100 p-2 text-sm font-semibold">{{ price.ticker }}</td>
            <td class="border-b border-slate-100 p-2 text-sm">{{ formatNumber(price.price, 2) }}</td>
            <td class="border-b border-slate-100 p-2 text-sm">{{ formatNumber(price.previous_close, 2) }}</td>
            <td class="border-b border-slate-100 p-2 text-sm" :class="plClass(price.change)">{{ formatNumber(price.change, 2) }}</td>
            <td class="border-b border-slate-100 p-2 text-sm" :class="plClass(price.change_percent)">{{ formatPercent(price.change_percent) }}</td>
            <td class="border-b border-slate-100 p-2 text-sm">{{ price.currency }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useFormatters } from '../composables/useFormatters'

const props = defineProps({
  prices: {
    type: Object,
    default: () => ({})
  }
})

const emit = defineEmits(['fetch', 'clear'])

const { formatNumber, formatPercent, plClass } = useFormatters()

const tickers = ref('')

const entries = computed(() => Object.values(props.prices || {}))

const sortedEntries = computed(() => {
  if (!entries.value) return []
  return [...entries.value].sort((a, b) => a.ticker.localeCompare(b.ticker))
})

const handleSubmit = () => {
  const tickerList = tickers.value
    .split(',')
    .map(t => t.trim().toUpperCase())
    .filter(Boolean)
    .join(',')

  if (tickerList) {
    emit('fetch', tickerList)
  }
}

const handleClear = () => {
  tickers.value = ''
  emit('clear')
}

  
</script>
