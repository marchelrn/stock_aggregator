<template>
  <div>
    <form action="" class="mt-3 flex flex-col gap-2 sm:flex-row">
      <input
      v-model="transactions"
      type="text"
      placeholder="Search By Stocks"
      required
      class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
      />
    </form>
    <div class="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
      <table class="w-full min-w-[1300px] border-collapse">
        <thead>
          <tr class="bg-slate-50 text-slate-500">
            <th class="border-b border-slate-300 p-2 text-left text-xs">Ticker</th>
            <th class="border-b border-slate-300 p-2 text-left text-xs">Type</th>
            <th class="border-b border-slate-300 p-2 text-left text-xs">Lot Done</th>
            <th class="border-b border-slate-300 p-2 text-left text-xs">Amount Done</th>
            <th class="border-b border-slate-300 p-2 text-left text-xs">Broker</th>
            <!-- <th class="border-b border-slate-300 p-2 text-left text-xs">Total Fee</th> -->
            <th class="border-b border-slate-300 p-2 text-left text-xs">Date</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="transactionHistory.length === 0">
            <td colspan="6" class="p-3 text-sm text-slate-500">Tidak ada data transaksi.</td>
          </tr>
          <tr v-else v-for="transaction in transactionHistory" :key="transaction.id">
            <td class="border-b border-slate-100 p-2 text-sm font-semibold">{{ transaction.stock.ticker }}</td>
            <td class="border-b border-slate-100 p-2 text-sm">{{ transaction.type }}</td>
            <td class="border-b border-slate-100 p-2 text-sm">{{ formatNumber(transaction.stock.lot, 0) }}</td>
            <td class="border-b border-slate-100 p-2 text-sm">{{ formatNumber(transaction.amount_done, 0) }}</td>
            <td class="border-b border-slate-100 p-2 text-sm">{{ transaction.broker.name }}</td>
            <!-- <td class="border-b border-slate-100 p-2 text-sm">{{ formatNumber(transaction.fee, 0) }}</td> -->
            <td class="border-b border-slate-100 p-2 text-sm">{{ transaction.date }}</td>
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
  transactionHistory: {
    type: Array,
    default: () => []
  }
})

const { formatNumber, formatPercent, plClass } = useFormatters()
const transactions = ref('')



</script>