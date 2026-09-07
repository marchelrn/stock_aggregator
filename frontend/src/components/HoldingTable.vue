<template>
  <div class="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
    <table class="w-full min-w-[620px] border-collapse">
      <thead>
        <tr class="bg-slate-50 text-slate-500">
          <th class="border-b border-slate-300 p-2 text-left text-xs">Ticker</th>
          <th class="border-b border-slate-300 p-2 text-left text-xs">Lot</th>
          <th class="border-b border-slate-300 p-2 text-left text-xs">Avg Price</th>
          <th class="border-b border-slate-300 p-2 text-left text-xs">Broker</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="!holdings || holdings.length === 0">
          <td colspan="4" class="p-4 text-center text-sm text-slate-500">Pilih broker untuk melihat holding.</td>
        </tr>
        <tr v-else v-for="holding in holdings" :key="holding.id || `${holding.ticker}-${holding.broker_id}`">
          <td class="border-b border-slate-100 p-2 text-sm font-semibold">{{ holding.ticker }}</td>
          <td class="border-b border-slate-100 p-2 text-sm">{{ formatNumber(holding.lot, 0) }}</td>
          <td class="border-b border-slate-100 p-2 text-sm">{{ formatCurrency(holding.avg_price) }}</td>
          <td class="border-b border-slate-100 text-sm">{{ holding.broker_name }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { useFormatters } from '../composables/useFormatters'

defineProps({
  holdings: {
    type: Array,
    default: () => []
  }
})

defineEmits(['trade'])

const { formatCurrency, formatNumber } = useFormatters()
</script>
