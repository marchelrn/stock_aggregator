<template>
  <div class="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
    <table class="w-full min-w-[760px] border-collapse">
      <thead>
        <tr class="bg-slate-50 text-slate-500">
          <th class="border-b border-slate-300 p-2 text-left text-xs">Ticker</th>
          <th class="border-b border-slate-300 p-2 text-left text-xs">Total Lot</th>
          <th class="border-b border-slate-300 p-2 text-left text-xs">Avg Price</th>
          <th class="border-b border-slate-300 p-2 text-left text-xs">Current Price</th>
          <th class="border-b border-slate-300 p-2 text-left text-xs">Market Value</th>
          <th class="border-b border-slate-300 p-2 text-left text-xs">P/L</th>
          <th class="border-b border-slate-300 p-2 text-left text-xs">P/L %</th>
          <th class="border-b border-slate-300 p-2 text-left text-xs">Weight</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="!rows || rows.length === 0">
          <td colspan="8" class="p-3 text-sm text-slate-500">Tidak ada data ticker.</td>
        </tr>
        <tr v-else v-for="row in sortedRows" :key="row.ticker">
          <td class="border-b border-slate-100 p-2 text-sm font-semibold">{{ row.ticker }}</td>
          <td class="border-b border-slate-100 p-2 text-sm">{{ formatNumber(row.total_lot, 0) }}</td>
          <td class="border-b border-slate-100 p-2 text-sm">{{ formatCurrency(row.avg_price) }}</td>
          <td class="border-b border-slate-100 p-2 text-sm">{{ formatCurrency(getCurrentPrice(row)) }}</td>
          <td class="border-b border-slate-100 p-2 text-sm">{{ formatCurrency(getMarketValue(row)) }}</td>
          <td class="border-b border-slate-100 -ml-10 text-sm" :class="plClass(getProfitLoss(row))">{{ formatCurrency(getProfitLoss(row)) }}</td>
          <td class="border-b border-slate-100 p-2 text-sm" :class="plClass(getProfitPct(row))">{{ formatPercent(getProfitPct(row)) }}</td>
          <td class="border-b border-slate-100 p-2 text-sm">{{ formatPercent(getWeight(row)) }}</td>
        </tr>
      </tbody>
    </table>
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

const { formatCurrency, formatNumber, formatPercent, plClass } = useFormatters()

const rows = computed(() => props.summary?.by_ticker || [])

const sortedRows = computed(() => {
  return [...rows.value].sort((a, b) => getMarketValue(b) - getMarketValue(a))
})

const toNumber = (value) => Number(value || 0)
const getShares = (row) => toNumber(row.total_lot) * 100

const getCurrentPrice = (row) => {
  if (row.current_price != null) return toNumber(row.current_price)
  const shares = getShares(row)
  if (shares > 0 && row.market_value != null) return toNumber(row.market_value) / shares
  return toNumber(row.avg_price)
}

const getInvestedValue = (row) => {
  if (row.invested_value != null) return toNumber(row.invested_value)
  return getShares(row) * toNumber(row.avg_price)
}

const getMarketValue = (row) => {
  if (row.current_price != null) return getShares(row) * getCurrentPrice(row)
  if (row.market_value != null) return toNumber(row.market_value)
  return getInvestedValue(row)
}

const getProfitLoss = (row) => getMarketValue(row) - getInvestedValue(row)

const getProfitPct = (row) => {
  const invested = getInvestedValue(row)
  if (invested <= 0) return 0
  return (getProfitLoss(row) / invested) * 100
}

const getWeight = (row) => {
  const totalStockValue = toNumber(props.summary?.total_stock_value)
  if (totalStockValue <= 0) return 0
  return (getMarketValue(row) / totalStockValue) * 100
}
</script>
