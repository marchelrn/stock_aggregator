import { useEffect, useState } from 'react'
import { getBaseUrl, setBaseUrl } from '../lib/api'

interface ConfigFormProps {
  onApply?: () => void
}

export default function ConfigForm({ onApply }: ConfigFormProps) {
  const [url, setUrl] = useState('')

  useEffect(() => {
    setUrl(getBaseUrl())
  }, [])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setBaseUrl(url)
    onApply?.()
  }

  return (
    <form className="grid gap-2" onSubmit={handleSubmit}>
      <label htmlFor="base-url" className="text-sm text-slate-600">
        Base URL API
      </label>
      <div className="flex flex-col gap-2 sm:flex-row">
        <input
          id="base-url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          type="url"
          placeholder={getBaseUrl()}
          required
          className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
        />
        <button
          type="submit"
          className="rounded-lg bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800"
        >
          Apply
        </button>
      </div>
    </form>
  )
}
