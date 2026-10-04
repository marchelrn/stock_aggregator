import { api } from './api'
import { normalizeArray } from './portfolio'
import type { Broker, UserResponse } from '../types'

const SESSION_KEYS = [
  'token',
  'isAuthenticated',
  'isSetupCompleted',
  'userName',
  'userEmail',
] as const

export function clearSession(): void {
  SESSION_KEYS.forEach((key) => localStorage.removeItem(key))
}

/** Cek exp pada payload JWT (tanpa verifikasi signature — itu tugas server). */
export function isTokenExpired(token: string): boolean {
  try {
    const base64 = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    const payload = JSON.parse(atob(base64))
    return typeof payload.exp === 'number' && payload.exp * 1000 <= Date.now()
  } catch {
    return true
  }
}

/** Sesi dianggap ada hanya jika flag aktif DAN token ada dan belum kedaluwarsa. */
export function hasValidSession(): boolean {
  if (localStorage.getItem('isAuthenticated') !== 'true') return false
  const token = localStorage.getItem('token')
  return !!token && !isTokenExpired(token)
}

/**
 * Sinkronkan data sesi dari backend ke localStorage:
 * - nama & email user
 * - isSetupCompleted (true jika sudah ada broker di server)
 *
 * Melempar error jika token tidak valid (request user gagal).
 */
export async function syncSessionFromServer(): Promise<{ setupCompleted: boolean }> {
  const [userResult, brokersResult] = await Promise.allSettled([
    api<UserResponse>('/api/auth/user'),
    api('/brokers'),
  ])

  if (userResult.status === 'rejected') {
    throw userResult.reason
  }

  const user = userResult.value?.data
  if (user?.name) localStorage.setItem('userName', user.name)
  if (user?.email) localStorage.setItem('userEmail', user.email)

  if (brokersResult.status === 'fulfilled') {
    const brokers = normalizeArray<Broker>(brokersResult.value)
    if (brokers.length > 0) {
      localStorage.setItem('isSetupCompleted', 'true')
    }
  }

  return { setupCompleted: localStorage.getItem('isSetupCompleted') === 'true' }
}
