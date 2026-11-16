interface PaginationProps {
  totalItems: number
  page: number
  pageSize: number
  onPageChange: (page: number) => void
  onPageSizeChange: (size: number) => void
  pageSizeOptions?: number[]
}

const DEFAULT_PAGE_SIZE_OPTIONS = [5, 10, 25, 50]

/**
 * Susun deret tombol halaman: selalu tampilkan halaman pertama & terakhir,
 * jendela 5 nomor di sekitar halaman aktif, sisanya ellipsis.
 * Contoh (total 40, aktif 1): 1 2 3 4 5 … 40
 */
function buildPageItems(current: number, total: number): Array<number | 'ellipsis'> {
  if (total <= 7) {
    return Array.from({ length: total }, (_, i) => i + 1)
  }
  const pages = new Set<number>([1, total])
  for (let p = current - 2; p <= current + 2; p++) {
    if (p >= 1 && p <= total) pages.add(p)
  }
  if (current <= 3) {
    for (let p = 1; p <= 5; p++) pages.add(p)
  }
  if (current >= total - 2) {
    for (let p = total - 4; p <= total; p++) {
      if (p >= 1) pages.add(p)
    }
  }
  const sorted = [...pages].sort((a, b) => a - b)
  const items: Array<number | 'ellipsis'> = []
  let prev = 0
  for (const p of sorted) {
    if (p - prev > 1) items.push('ellipsis')
    items.push(p)
    prev = p
  }
  return items
}

export default function Pagination({
  totalItems,
  page,
  pageSize,
  onPageChange,
  onPageSizeChange,
  pageSizeOptions = DEFAULT_PAGE_SIZE_OPTIONS,
}: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize))
  const currentPage = Math.min(Math.max(1, page), totalPages)
  const start = totalItems === 0 ? 0 : (currentPage - 1) * pageSize + 1
  const end = Math.min(currentPage * pageSize, totalItems)
  const items = buildPageItems(currentPage, totalPages)

  const buttonBase =
    'min-w-8 rounded-lg px-2 py-1.5 text-sm font-medium transition focus:outline-none focus:ring-2 focus:ring-teal-300'

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-t border-slate-200 bg-slate-50 px-4 py-2.5">
      <span className="text-sm text-slate-600">
        Menampilkan {start}&ndash;{end} dari {totalItems} data
      </span>
      <div className="flex flex-wrap items-center gap-3">
        <select
          value={pageSize}
          onChange={(e) => onPageSizeChange(Number(e.target.value))}
          aria-label="Jumlah data per halaman"
          className="rounded-lg border border-slate-300 bg-white px-2 py-1.5 text-sm text-slate-600 focus:border-teal-600 focus:outline-none"
        >
          {pageSizeOptions.map((size) => (
            <option key={size} value={size}>
              {size} per halaman
            </option>
          ))}
        </select>
        <nav className="flex items-center gap-1" aria-label="Navigasi halaman">
          <button
            type="button"
            onClick={() => onPageChange(1)}
            disabled={currentPage <= 1}
            aria-label="Halaman pertama"
            className={`${buttonBase} text-slate-600 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent`}
          >
            &laquo;
          </button>
          {items.map((item, index) =>
            item === 'ellipsis' ? (
              <span key={`ellipsis-${index}`} className="px-1 text-sm text-slate-400">
                &hellip;
              </span>
            ) : (
              <button
                key={item}
                type="button"
                onClick={() => onPageChange(item)}
                aria-current={item === currentPage ? 'page' : undefined}
                className={`${buttonBase} ${
                  item === currentPage
                    ? 'bg-teal-700 text-white'
                    : 'text-slate-600 hover:bg-slate-100'
                }`}
              >
                {item}
              </button>
            )
          )}
          <button
            type="button"
            onClick={() => onPageChange(totalPages)}
            disabled={currentPage >= totalPages}
            aria-label="Halaman terakhir"
            className={`${buttonBase} text-slate-600 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent`}
          >
            &raquo;
          </button>
        </nav>
      </div>
    </div>
  )
}
