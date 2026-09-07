<template>
  <form class="grid gap-2" @submit.prevent="handleSubmit">
    <label for="base-url" class="text-sm text-slate-600">Base URL API</label>
    <div class="flex flex-col gap-2 sm:flex-row">
      <input
        id="base-url"
        v-model="url"
        type="url"
        placeholder="{{ import.meta.env.VITE_API_URL }}"
        required
        class="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
      />
      <button type="submit" class="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800">Apply</button>
    </div>
  </form>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useApi } from '../composables/useApi'

const emit = defineEmits(['apply'])

const { getBaseUrl, setBaseUrl } = useApi()

const url = ref('')

onMounted(() => {
  url.value = getBaseUrl()
})

const handleSubmit = () => {
  setBaseUrl(url.value)
  emit('apply')
}
</script>
