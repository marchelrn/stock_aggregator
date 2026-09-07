<template>
  <div>
    <button class="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800" @click="toggleForm('Broker')">
      {{ showForm ? 'Close Broker Form' : 'Add New Broker' }}
    </button>
    <button class="ml-3 rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800" @click="toggleForm('Cash')">
      {{ showCashForm ? 'Close Cash Form' : 'Add Broker Cash' }}
    </button>

    <form v-if="showForm" class="mt-3 grid content-start gap-2 rounded-xl border border-slate-300 bg-white p-3" id="addBroker" @submit.prevent="handleSubmit">
      <h3 class="text-base font-semibold">Add Broker</h3>
      <label class="text-sm text-slate-600" for="broker-name">Broker Name</label>
      <input id="broker-name" v-model="name" type="text" required class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" />
      <label class="text-sm text-slate-600" for="broker-cash">Cash</label>
      <input id="broker-cash" v-model.number="cash" type="number" min="0" step="0.01" class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" />
      <button type="submit" class="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800">Add Broker</button>
    </form>

    
    <form v-if="showCashForm" class="mt-3 grid content-start gap-2 rounded-xl border border-slate-300 bg-white p-3" id="addCash" @submit.prevent="handleCashSubmit">
      <h3 class="text-base font-semibold">Add Broker Cash</h3>
      <label class="text-sm text-slate-600" for="tx-broker">Broker Name</label>
      <select id="tx-broker" v-model.number="brokerId" required class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm">
      <option v-if="brokerOptions.length === 0" value="">Tidak ada broker tersedia</option>
      <option v-else v-for="broker in brokerOptions" :key="broker.id" :value="broker.id">{{ broker.name }} (ID: {{ broker.id }})</option>
    </select>
      <label class="text-sm text-slate-600" for="cash-amount">Cash Amount</label>
      <input id="cash-amount" v-model.number="cashAmount" type="number" min="0" step="0.01" class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" />
      <button type="submit" class="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800">Add Cash</button>
    </form>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'

const props = defineProps({
  brokers: { type: Array, default: () => [] }
})

const emit = defineEmits(['submit', 'cashSubmit'])

const name = ref('')
const cash = ref(0)
const brokerId = ref(null)
const cashAmount = ref(0)
const showForm = ref(false)
const showCashForm = ref(false)

const toggleForm = (formType) => {
  if (formType === 'Broker') {
    showForm.value = !showForm.value
    showCashForm.value = false
  } else if (formType === 'Cash') {
    showCashForm.value = !showCashForm.value
    showForm.value = false
  }
} 

const brokerOptions = computed(() => {
  return props.brokers || []
})

watch(showCashForm, (isOpen) => {
  if (!isOpen) return
  brokerId.value = brokerOptions.value[0] ? Number(brokerOptions.value[0].id) : null
})

watch(brokerOptions, (list) => {
  if (!list.some((b) => Number(b.id) === Number(brokerId.value))) {
    brokerId.value = list[0] ? Number(list[0].id) : null
  }0
})

const handleSubmit = async () => {
  try {
    await emit('submit', { name: name.value.trim(), cash: cash.value })
    name.value = ''
    cash.value = 0
    showForm.value = false
  } catch (error) {
    console.error('Error submitting broker form:', error)
  }
}

const handleCashSubmit = async () => {
  try {
    await emit('cashSubmit', { 
      brokerId: brokerId.value,
      cashAmount: cashAmount.value 
    })
    brokerId.value = null
    cashAmount.value = 0
  } catch (error) {
    console.error('Error submitting cash form:', error)
  }
} 
</script>
