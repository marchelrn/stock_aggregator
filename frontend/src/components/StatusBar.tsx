interface StatusBarProps {
  message?: string
  isError?: boolean
}

export default function StatusBar({ message = '', isError = false }: StatusBarProps) {
  return (
    <div
      className={`mt-3 flex min-h-10 items-center rounded-lg border px-3 py-2 text-sm ${
        isError
          ? 'border-rose-200 bg-rose-50 text-rose-700'
          : 'border-slate-300 bg-white/70 text-slate-600'
      }`}
      role="status"
      aria-live="polite"
    >
      {message}
    </div>
  )
}
