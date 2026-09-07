import { createRouter, createWebHistory } from 'vue-router'
import DashboardPage from './pages/DashboardPage.vue'
import BrokersPage from './pages/BrokersPage.vue'
import LoginPage from './pages/LoginPage.vue'
import SetupPage from './pages/SetupPage.vue'

const routes = [
  { path: '/login', name: 'login', component: LoginPage, meta: { public: true } },
  { path: '/setup', name: 'setup', component: SetupPage, meta: { requiresAuth: true } },
  { path: '/', name: 'dashboard', component: DashboardPage, meta: { requiresAuth: true, requiresSetup: true } },
  { path: '/manage', name: 'manage', component: BrokersPage, meta: { requiresAuth: true, requiresSetup: true } }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  const isAuthenticated = localStorage.getItem('isAuthenticated') === 'true'
  const isSetupCompleted = localStorage.getItem('isSetupCompleted') === 'true'

  if (to.meta.requiresAuth && !isAuthenticated) {
    next({ name: 'login' })
  } else if (to.name === 'login' && isAuthenticated) {
    next(isSetupCompleted ? { name: 'dashboard' } : { name: 'setup' })
  } else if (to.meta.requiresSetup && !isSetupCompleted && isAuthenticated) {
    next({ name: 'setup' })
  } else {
    next()
  }
})

export default router

