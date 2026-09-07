import { ref } from 'vue'

const env = String(import.meta.env.VITE_ENV || '').toUpperCase()
const isDevelopment = env === 'DEVELOPMENT'
const localhostUrl = 'http://localhost:3030'
const renderUrl = import.meta.env.VITE_API_URL || 'https://stock-api-8ftf.onrender.com'

const defaultUrl = isDevelopment ? localhostUrl : renderUrl
const baseUrl = ref(defaultUrl.replace(/\/$/, ''))
export function useApi() {
  const setBaseUrl = (url) => {
    baseUrl.value = url.trim().replace(/\/$/, '')
    localStorage.setItem('stock-api-base-url', baseUrl.value)
  }

  const getBaseUrl = () => baseUrl.value

  const api = async (path, options = {}) => {
    const url = `${baseUrl.value}${path}`
    const response = await fetch(url, {
      headers: {
        'Content-Type': 'application/json',
        ...(options.headers || {})
      },
      ...options
    })

    let payload
    try {
      payload = await response.json()
    } catch {
      payload = {}
    }

    if (!response.ok) {
      throw new Error(payload.error || payload.message || `Request failed: ${response.status}`)
    }

    return payload
  }

  return {
    baseUrl,
    setBaseUrl,
    getBaseUrl,
    api
  }
}
