<template>
  <div class="mx-auto flex w-full max-w-5xl flex-col gap-4 px-4">
    <header class="grid gap-4 rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur md:grid-cols-[1.4fr_1fr] md:items-center">
      <div>
        <h1 class="text-2xl font-bold">Portfolio Dashboard</h1>
        <p class="mt-2 text-sm text-slate-500">Marchel's Portfolio</p>
      </div>
      <!-- <ConfigForm @apply="loadDashboard" /> -->
    </header>

    <section class="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
      <div class="flex items-center justify-between gap-2">
        <h2 class="text-xl font-semibold">Portfolio Summary</h2>
        <button class="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800" @click="loadDashboard">Refresh Data</button>
      </div>
      <StatusBar :message="status" :is-error="isError" />
      <KpiGrid :summary="state.summary" />
    </section>

    <section class="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
      <div class="flex items-center justify-between gap-2">
        <h2 class="text-xl font-semibold">Broker Allocation</h2>
      </div>
      <BrokerCards :summary="state.summary" />
    </section>

    <section class="rounded-2xl border border-slate-300 bg-white/80 p-4 shadow-sm backdrop-blur">
      <div class="flex items-center justify-between gap-2">
        <h2 class="text-xl font-semibold">Ticker Breakdown</h2>
      </div>
      <TickerTable :holdings="state.holdings" :summary="state.summary" @trade="handleTradeHolding" />
    </section>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { usePortfolio } from '../composables/usePortfolio'

import ConfigForm from '../components/ConfigForm.vue'
import StatusBar from '../components/StatusBar.vue'
import KpiGrid from '../components/KpiGrid.vue'
import BrokerCards from '../components/BrokerCards.vue'
import TickerTable from '../components/TickerTable.vue'

const {
  state,
  status,
  isError,
  loadDashboard
} = usePortfolio()

onMounted(() => {
  loadDashboard()
})
</script>
