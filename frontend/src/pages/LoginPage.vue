<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-white p-4">
    <!-- Card -->
    <div class="w-full max-w-[440px] rounded-[24px] bg-white p-10 shadow-xl relative border border-slate-300 text-left">
  
      <!-- Headers -->
      <h2 class="mb-1 text-[22px] font-semibold text-slate-500 tracking-tight">Stock API.</h2>
      <h1 class="mb-8 text-[28px] font-bold text-slate-800 tracking-tight">Log in to your account</h1>
      
      <!-- Buttons -->
      <div class="space-y-3">
        <!-- Google Button -->
        <button 
          @click="handleLogin" 
          class="flex w-full items-center justify-center gap-3 rounded-xl border border-slate-300 bg-white px-4 py-3.5 text-[15px] font-semibold text-slate-700 transition-all hover:bg-slate-50 active:scale-[0.98] shadow-sm"
        >
          <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 48 48">
            <path fill="#FFC107" d="M43.611,20.083H42V20H24v8h11.303c-1.649,4.657-6.08,8-11.303,8c-6.627,0-12-5.373-12-12c0-6.627,5.373-12,12-12c3.059,0,5.842,1.154,7.961,3.039l5.657-5.657C34.046,6.053,29.268,4,24,4C12.955,4,4,12.955,4,24c0,11.045,8.955,20,20,20c11.045,0,20-8.955,20-20C44,22.659,43.862,21.35,43.611,20.083z"></path>
            <path fill="#FF3D00" d="M6.306,14.691l6.571,4.819C14.655,15.108,18.961,12,24,12c3.059,0,5.842,1.154,7.961,3.039l5.657-5.657C34.046,6.053,29.268,4,24,4C16.318,4,9.656,8.337,6.306,14.691z"></path>
            <path fill="#4CAF50" d="M24,44c5.166,0,9.86-1.977,13.409-5.192l-6.19-5.238C29.211,35.091,26.715,36,24,36c-5.202,0-9.619-3.317-11.283-7.946l-6.522,5.025C9.505,39.556,16.227,44,24,44z"></path>
            <path fill="#1976D2" d="M43.611,20.083H42V20H24v8h11.303c-0.792,2.237-2.231,4.166-4.087,5.571c0.001-0.001,0.002-0.001,0.003-0.002l6.19,5.238C36.971,39.205,44,34,44,24C44,22.659,43.862,21.35,43.611,20.083z"></path>
          </svg>
          Continue with Google
        </button>
      </div>

      <!-- Info Text styled like the SSO text in reference -->
      <div class="mt-8 pt-8 border-t border-slate-300">
        <div class="flex items-start gap-3 text-[14px] text-slate-500 leading-relaxed">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mt-0.5 shrink-0">
            <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
            <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
          </svg>
          <p>
            Pastikan Anda menggunakan Gmail utama yang terdaftar dan terhubung dengan akun sekuritas/broker Anda.
          </p>
        </div>
      </div>

    </div>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'

const router = useRouter()

const handleLogin = () => {
  // Dummy logic: set localStorage to indicate user is logged in
  localStorage.setItem('isAuthenticated', 'true')
  // For new users, we redirect to setup page. In a real app we check if they already have configuration.
  localStorage.setItem('isSetupCompleted', 'false')
  router.push('/setup')
}
</script>
