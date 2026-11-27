import { Info } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import MarketChart from "../components/MarketChart";
import { api } from "../lib/api";
import { formatCurrency, formatPercent } from "../lib/formatters";

type BrokerPoint = {
  date: string;
  broker_id: number;
  broker_name: string;
  total_value: number | null;
  unpriced_holdings: number;
};

const dayMs = 86_400_000;
const displayDate = new Intl.DateTimeFormat("id-ID", {
  day: "numeric",
  month: "short",
  year: "numeric",
  timeZone: "UTC",
});
const dateLabel = (day: string) => displayDate.format(new Date(day));

function totalValue(points: BrokerPoint[]): number | null {
  if (points.length === 0 || points.some((point) => point.total_value === null))
    return null;
  return points.reduce((sum, point) => sum + (point.total_value ?? 0), 0);
}

function PerformanceInfo() {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const dismissOutside = (event: PointerEvent) => {
      if (event.target instanceof Node && !ref.current?.contains(event.target))
        setOpen(false);
    };
    const dismissOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", dismissOutside);
    document.addEventListener("keydown", dismissOnEscape);
    return () => {
      document.removeEventListener("pointerdown", dismissOutside);
      document.removeEventListener("keydown", dismissOnEscape);
    };
  }, [open]);

  return (
    <div
      ref={ref}
      className="sm:relative"
      onPointerEnter={(event) => {
        if (event.pointerType === "mouse") setOpen(true);
      }}
      onPointerLeave={(event) => {
        if (event.pointerType === "mouse") setOpen(false);
      }}
    >
      <button
        type="button"
        aria-label="Informasi kinerja portofolio"
        aria-describedby={open ? "performance-info" : undefined}
        className="grid size-11 shrink-0 place-items-center rounded-full text-slate-500 focus-visible:ring-2 sm:size-8"
        onFocus={(event) => {
          if (event.currentTarget.matches(":focus-visible")) setOpen(true);
        }}
        onBlur={() => setOpen(false)}
        onClick={() => setOpen(true)}
      >
        <Info size={15} aria-hidden="true" />
      </button>
      {open && (
        <div className="absolute left-0 top-full z-20 w-80 max-w-[calc(100vw-2rem)] pt-2">
          <div
            id="performance-info"
            role="tooltip"
            className="rounded-lg border border-slate-300 bg-white p-3.5 text-xs leading-relaxed text-slate-600 shadow-lg dark:border-neutral-700 dark:bg-neutral-900 dark:text-neutral-300"
          >
            <ul className="list-disc space-y-1.5 pl-4">
              <li>
                Sistem memerlukan dua data pergerakan harian untuk menghitung
                perubahan nilai portofolio.
              </li>
              <li>
                Sistem melakukan perhitungan performa setelah{" "}
                <i>Sync Trade Confirmation</i> data selesai, yaitu pada jam
                11:59 WITA.
              </li>
              <li>
                Perubahan nilai mencakup pergerakan harga dan perubahan saldo,
                termasuk setoran, penarikan, atau transfer.
              </li>
            </ul>
          </div>
        </div>
      )}
    </div>
  );
}

function changeLabel(current: number | null, previous: number | null) {
  if (current === null || previous === null) return "—";
  const change = current - previous;
  const amount = `${change > 0 ? "+" : ""}${formatCurrency(change)}`;
  return previous > 0
    ? `${amount} (${formatPercent((change / previous) * 100)})`
    : amount;
}

export default function PerformancePage() {
  const [points, setPoints] = useState<BrokerPoint[] | null>(null);
  const [error, setError] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [period, setPeriod] = useState(30);
  const [broker, setBroker] = useState("");
  const [selectedDate, setSelectedDate] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    api<{ data: BrokerPoint[] }>("/api/portfolio/performance", {
      signal: controller.signal,
    })
      .then(({ data }) => setPoints(data))
      .catch(() => {
        if (!controller.signal.aborted) setError(true);
      });
    return () => controller.abort();
  }, [attempt]);

  const { brokers, sessions } = useMemo(() => {
    const names = new Map<number, string>();
    const sessions = new Map<string, BrokerPoint[]>();
    for (const point of points ?? []) {
      names.set(point.broker_id, point.broker_name);
      const existing = sessions.get(point.date) ?? [];
      existing.push(point);
      sessions.set(point.date, existing);
    }
    return {
      brokers: [...names.entries()].sort((a, b) => a[1].localeCompare(b[1])),
      sessions: [...sessions.entries()].sort(([a], [b]) => a.localeCompare(b)),
    };
  }, [points]);

  const cutoff = new Date(Date.now() + 8 * 3_600_000 - (period - 1) * dayMs)
    .toISOString()
    .slice(0, 10);
  const filteredSessions = sessions
    .map(
      ([day, rows]) =>
        [
          day,
          broker
            ? rows.filter((row) => String(row.broker_id) === broker)
            : rows,
        ] as const,
    )
    .filter(([, rows]) => rows.length > 0);
  const visibleSessions = filteredSessions.filter(([day]) => day >= cutoff);
  const current =
    visibleSessions.find(([day]) => day === selectedDate) ??
    visibleSessions[visibleSessions.length - 1];
  const previous =
    current && filteredSessions[filteredSessions.indexOf(current) - 1];
  const currentValue = totalValue(current?.[1] ?? []);
  const previousValue = totalValue(previous?.[1] ?? []);
  const history = visibleSessions.flatMap(([day, rows]) => {
    const value = totalValue(rows);
    return value === null
      ? []
      : [{ time: Date.parse(day) / 1000, close: value }];
  });
  const partial = visibleSessions.some(([, rows]) =>
    rows.some((row) => row.unpriced_holdings > 0),
  );
  const selectClass =
    "min-h-11 rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-800";

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-5 px-4">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="relative flex items-center gap-1">
          <h1 className="text-2xl font-bold">Portfolio Performance</h1>
          <PerformanceInfo />
        </div>
        <div className="flex flex-wrap gap-3">
          <div className="flex flex-col gap-1 text-xs font-medium text-slate-600">
            <label htmlFor="performance-period">Periode</label>
            <select
              id="performance-period"
              className={selectClass}
              value={period}
              onChange={(event) => {
                setPeriod(Number(event.target.value));
                setSelectedDate("");
              }}
            >
              {[7, 30, 90].map((days) => (
                <option key={days} value={days}>
                  {days} hari
                </option>
              ))}
            </select>
          </div>
          <div className="flex min-w-0 flex-col gap-1 text-xs font-medium text-slate-600">
            <label htmlFor="performance-broker">Broker</label>
            <select
              id="performance-broker"
              className={selectClass + " max-w-[16rem]"}
              value={broker}
              onChange={(event) => {
                setBroker(event.target.value);
                setSelectedDate("");
              }}
            >
              <option value="">Semua broker</option>
              {brokers.map(([id, name]) => (
                <option key={id} value={id}>
                  {name}
                </option>
              ))}
            </select>
          </div>
        </div>
      </header>

      {error ? (
        <div
          role="alert"
          className="flex flex-wrap items-center gap-3 rounded-xl border border-slate-300 p-4"
        >
          <p>Riwayat kinerja belum dapat dimuat.</p>
          <button
            type="button"
            className="min-h-11 rounded-lg border border-slate-300 px-3 text-sm font-semibold"
            onClick={() => {
              setError(false);
              setAttempt((value) => value + 1);
            }}
          >
            Coba lagi
          </button>
        </div>
      ) : points === null ? (
        <p role="status" className="text-sm text-slate-600">
          Memuat riwayat kinerja…
        </p>
      ) : !current ? (
        <p className="rounded-xl border border-slate-300 p-5 text-sm text-slate-600">
          {points.length === 0
            ? "Belum ada catatan nilai portofolio. Data akan muncul setelah pencatatan otomatis pada malam hari bursa."
            : "Tidak ada catatan untuk broker dan periode yang dipilih."}
        </p>
      ) : (
        <>
          <section
            aria-labelledby="performance-value"
            className="rounded-xl border border-slate-300 bg-white p-4 sm:p-6"
          >
            <h2 id="performance-value" className="text-lg font-semibold">
              Nilai portofolio · IDR
            </h2>
            <div className="mt-3 flex flex-wrap items-baseline gap-x-4 gap-y-1">
              <p className="break-words text-2xl font-semibold tabular-nums sm:text-3xl">
                {currentValue === null
                  ? "Belum lengkap"
                  : formatCurrency(currentValue)}
              </p>
              {previous && currentValue !== null && previousValue !== null && (
                <p className="text-sm tabular-nums text-slate-600">
                  {changeLabel(currentValue, previousValue)} sejak{" "}
                  {dateLabel(previous[0])}
                </p>
              )}
            </div>
            <MarketChart
              ticker={broker ? current[1][0].broker_name : "Portofolio"}
              history={history}
              description="nilai tercatat dalam IDR"
            />
          </section>

          <section
            aria-labelledby="performance-daily"
            className="rounded-xl border border-slate-300 bg-white p-4 sm:p-6"
          >
            <div className="flex flex-wrap items-center justify-between gap-3">
              <h2 id="performance-daily" className="text-lg font-semibold">
                Perubahan per broker
              </h2>
              <div className="flex items-center gap-2 text-sm text-slate-600">
                <label htmlFor="performance-date">Tanggal</label>
                <select
                  id="performance-date"
                  className={selectClass}
                  value={current[0]}
                  onChange={(event) => setSelectedDate(event.target.value)}
                >
                  {visibleSessions
                    .slice()
                    .reverse()
                    .map(([day]) => (
                      <option key={day} value={day}>
                        {dateLabel(day)}
                      </option>
                    ))}
                </select>
              </div>
            </div>
            <ul className="mt-3 divide-y divide-slate-200 border-y border-slate-200">
              {current[1].map((row) => {
                const prior = previous?.[1].find(
                  (point) => point.broker_id === row.broker_id,
                );
                return (
                  <li
                    key={row.broker_id}
                    className="grid items-start gap-2 py-4 sm:grid-cols-[1fr_2fr]"
                  >
                    <h3 className="break-words font-semibold">
                      {row.broker_name}
                    </h3>
                    <dl className="grid grid-cols-2 gap-3 text-sm">
                      <div className="min-w-0">
                        <dt className="text-slate-600">Total nilai · IDR</dt>
                        <dd className="break-words font-semibold tabular-nums">
                          {row.total_value === null
                            ? `Harga belum lengkap (${row.unpriced_holdings})`
                            : formatCurrency(row.total_value)}
                        </dd>
                      </div>
                      <div className="min-w-0">
                        <dt className="text-slate-600">Perubahan · IDR</dt>
                        <dd className="break-words font-semibold tabular-nums">
                          {changeLabel(
                            row.total_value,
                            prior?.total_value ?? null,
                          )}
                        </dd>
                      </div>
                    </dl>
                  </li>
                );
              })}
            </ul>
          </section>
        </>
      )}
    </div>
  );
}
