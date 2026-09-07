<template>
  <div class="mt-3 overflow-auto rounded-xl border border-slate-300 bg-white">
    <table class="w-full min-w-[560px] border-collapse">
      <thead>
        <tr class="bg-slate-50 text-slate-500">
          <th class="border-b border-slate-300 p-2 text-center text-xs">No</th>
          <th class="border-b border-slate-300 p-2 text-center text-xs">Name</th>
          <th class="border-b border-slate-300 p-2 text-center text-xs">Cash</th>
          <th class="border-b border-slate-300 p-2 text-center text-xs">View Holdings</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="!brokers || brokers.length === 0">
          <td colspan="5" class="p-3 text-sm text-slate-500">Belum ada broker.</td>
        </tr>
        <tr v-else v-for="(broker, index) in brokers" :key="broker.id">
          <td class="border-b border-slate-100 p-2 text-sm text-center"><code>{{ index + 1 }}</code></td>
          <td class="border-b border-slate-100 p-2 text-sm text-center">{{ broker.name }}</td>
          <td class="border-b border-slate-100 p-2 text-sm text-center">{{ formatCurrency(broker.cash) }}</td>
          <td class="border-b border-slate-100 p-2 text-sm text-center">
            <label class="relative inline-flex cursor-pointer items-center">
              <input type="checkbox" class="peer sr-only" :checked="selectedBrokerId === broker.id" @change="$emit('toggle', broker.id)">
              <div class="peer h-5 w-9 rounded-full bg-slate-200 after:absolute after:left-[2px] after:top-[2px] after:h-4 after:w-4 after:rounded-full after:border after:border-slate-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-teal-600 peer-checked:after:translate-x-full peer-checked:after:border-white peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-teal-300"></div>
            </label>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup>
import { useFormatters } from '../composables/useFormatters'

defineProps({
  brokers: {
    type: Array,
    default: () => []
  },
  selectedBrokerId: {
    type: Number,
    default: null
  }
})

defineEmits(['toggle'])



const { formatCurrency } = useFormatters()
</script>
