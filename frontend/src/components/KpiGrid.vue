<template>
  <div v-if="summary" class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
    <article v-for="item in kpiItems" :key="item.label" class="rounded-xl border border-slate-300 bg-white p-3">
      <label class="block text-xs text-slate-500">{{ item.label }}</label>
      <strong class="mt-1 block text-base" :class="item.class">{{ item.value }}</strong>
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

const { formatCurrency, formatPercent, plClass } = useFormatters()
const kpiItems = computed(() => {
  if (!props.summary) return []

  return [
    { label: 'Total Portfolio', value: formatCurrency(props.summary.total_portfolio_value), class: '' },
    { label: 'Total Stock Value', value: formatCurrency(props.summary.total_stock_value), class: '' },
    { label: 'Total Cash', value: formatCurrency(props.summary.total_cash), class: '' },
    { label: 'Total Invested', value: formatCurrency(props.summary.total_invested_value), class: '' },
    { label: 'Total P/L', value: formatCurrency(props.summary.total_profit_loss), class: plClass(props.summary.total_profit_loss) },
    { label: 'Total P/L %', value: formatPercent(props.summary.total_profit_pct), class: plClass(props.summary.total_profit_pct) }
  ]
})
</script>
