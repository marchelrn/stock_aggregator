<template>
  <div class="mt-3 grid grid-cols-1 gap-3 lg:grid-cols-2">
    <p v-if="!brokers || brokers.length === 0" class="text-sm text-slate-500">Belum ada data broker.</p>
    <article v-else v-for="broker in sortedBrokers" :key="broker.id" class="rounded-xl border border-slate-300 bg-white p-3">
      <h3 class="font-semibold">{{ broker.name }}</h3>
      <div class="mt-2 grid gap-1 font-mono text-xs text-slate-600">
        <span>Total Value: {{ formatCurrency(broker.total_value) }}</span>
        <span>Stock Value: {{ formatCurrency(broker.stock_value) }}</span>
        <span>Cash: {{ formatCurrency(broker.cash) }}</span>
        <span>Weight: {{ formatPercent(broker.weight) }}</span>
      </div>
      <div class="mt-2 h-2 overflow-hidden rounded-full bg-slate-200">
        <span class="block h-full bg-gradient-to-r from-teal-700 to-emerald-500" :style="{ width: `${Math.max(0, Math.min(100, broker.weight))}%` }"></span>
      </div>
    </article>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useFormatters } from '../composables/useFormatters'

const props = defineProps({
  summary: {
    type: Object,
    default: null
  }
})

const { formatCurrency, formatPercent } = useFormatters()

const brokers = computed(() => props.summary?.brokers || [])

const sortedBrokers = computed(() => {
  return [...brokers.value].sort((a, b) => b.total_value - a.total_value)
})
</script>
