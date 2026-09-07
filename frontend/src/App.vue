<template>
  <div class="min-h-screen bg-white text-slate-800">
    <nav v-if="!isAuthRoute" class="border-b border-slate-300 bg-white/80 backdrop-blur">
      <div class="mx-auto flex max-w-5xl items-center gap-4 px-4 py-3">
        <span class="text-sm font-bold uppercase tracking-widest text-teal-700">Stock API</span>
        <router-link
          to="/"
          class="rounded-lg px-3 py-1.5 text-sm font-semibold transition"
          :class="$route.name === 'dashboard' ? 'bg-teal-700 text-white' : 'text-slate-600 hover:bg-slate-100'"
        >
          Dashboard
        </router-link>
        <router-link
          to="/manage"
          class="rounded-lg px-3 py-1.5 text-sm font-semibold transition"
          :class="$route.name === 'manage' ? 'bg-teal-700 text-white' : 'text-slate-600 hover:bg-slate-100'"
        >
          Broker
        </router-link>
        <div class="flex-1"></div>
        <button @click="logout" class="rounded-lg px-3 py-1.5 text-sm font-semibold text-rose-600 hover:bg-rose-50 transition">
          Logout
        </button>
      </div>
    </nav>
    <main class="w-full py-7">
      <router-view />
    </main>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

const isAuthRoute = computed(() => {
  return route.name === 'login' || route.name === 'setup'
})

const logout = () => {
  localStorage.removeItem('isAuthenticated')
  localStorage.removeItem('isSetupCompleted')
  router.push('/login')
}
</script>
