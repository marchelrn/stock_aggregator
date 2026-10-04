// API client — port of the original useApi composable.
// baseUrl is a module-level value so it is shared across the whole app,
// mirroring the singleton `ref` used in the Vue version.

const env = String(import.meta.env.VITE_ENV || '').toUpperCase()
const isDevelopment = env === 'DEVELOPMENT'
const localhostUrl = 'http://localhost:3030'
const renderUrl = import.meta.env.VITE_API_URL || 'https://stock-api-8ftf.onrender.com'

const defaultUrl = isDevelopment ? localhostUrl : renderUrl
let baseUrl = defaultUrl.replace(/\/$/, '')

// Dikirim ke window saat server membalas 401 untuk request yang membawa token
// (token kedaluwarsa / tidak valid), agar App bisa membersihkan sesi dan ke /login.
export const UNAUTHORIZED_EVENT = 'auth:unauthorized'

export class ApiError extends Error {
  status: number
  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export function getBaseUrl(): string {
  return baseUrl
}

export function setBaseUrl(url: string): void {
  baseUrl = url.trim().replace(/\/$/, '')
  localStorage.setItem('stock-api-base-url', baseUrl)
}

export async function api<T = any>(path: string, options: RequestInit = {}): Promise<T> {
  const url = `${baseUrl}${path}`
  const token = localStorage.getItem('token')

  const response = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(options.headers || {}),
    },
  })

  let payload: any
  try {
    payload = await response.json()
  } catch {
    payload = {}
  }

  if (!response.ok) {
    if (response.status === 401 && token) {
      window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
    }
    throw new ApiError(
      payload.error || payload.message || `Request failed: ${response.status}`,
      response.status,
    )
  }

  return payload as T
}

export function useApi() {
  return { getBaseUrl, setBaseUrl, api }
}
