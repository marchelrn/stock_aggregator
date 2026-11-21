import { useEffect, useRef, useState } from "react";
import { Trash2, ArrowLeft, ArrowRight } from "lucide-react";
import { formatCurrency } from "../lib/formatters";
import type { Broker, Holding } from "../types";

export interface ManualHolding {
  ticker: string;
  lot: number;
  price: number;
  brokerId: number;
  brokerName: string;
}

interface SetupReviewProps {
  syncError: string;
  holdings: Holding[];
  brokers: Broker[];
  manualHoldings: ManualHolding[];
  finishing: boolean;
  finishError: string;
  onFinish: () => void;
  onBack: () => void;
  onAddManual: (holding: ManualHolding) => void;
  onRemoveManual: (index: number) => void;
}

export default function SetupReview({
  holdings,
  brokers,
  manualHoldings,
  finishing,
  finishError,
  syncError,
  onBack,
  onFinish,
  onAddManual,
  onRemoveManual,
}: SetupReviewProps) {
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    heading.current?.focus();
  }, []);

  const [brokerId, setBrokerId] = useState<number>(
    brokers[0]?.id ? Number(brokers[0].id) : 0,
  );
  const [ticker, setTicker] = useState("");
  const [lot, setLot] = useState("1");
  const [price, setPrice] = useState("");

  useEffect(() => {
    if (brokers.length === 0) return;
    if (brokers.some((broker) => Number(broker.id) === brokerId)) return;
    setBrokerId(Number(brokers[0].id));
  }, [brokers, brokerId]);

  const selectedBroker = brokers.find(
    (broker) => Number(broker.id) === brokerId,
  );

  const submitManual = (event: React.FormEvent) => {
    event.preventDefault();
    const lotValue = Number(lot);
    const priceValue = Number(price);
    if (!selectedBroker || !ticker.trim() || lotValue <= 0 || priceValue <= 0)
      return;
    onAddManual({
      ticker: ticker.trim().toUpperCase(),
      lot: lotValue,
      price: priceValue,
      brokerId: Number(selectedBroker.id),
      brokerName: selectedBroker.name,
    });
    setTicker("");
    setLot("1");
    setPrice("");
  };

  return (
    <section className="mx-auto grid w-full max-w-3xl gap-6 px-5 py-8 sm:px-10">
      <header>
        <button
          type="button"
          onClick={onBack}
          disabled={finishing}
          className="-ml-2 mb-2 hidden sm:inline-flex min-h-11 cursor-pointer items-center gap-2 rounded-md px-2 text-sm font-medium text-slate-600 hover:bg-slate-100 hover:text-slate-900 active:bg-slate-200 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <ArrowLeft size={16} aria-hidden="true" />
          Back
        </button>
        <h1
          ref={heading}
          tabIndex={-1}
          className="text-2xl font-semibold tracking-tight"
        >
          Portofolio awal
        </h1>
        <p className="mt-2 text-slate-600">
          Periksa saham dari email broker dan tambahkan posisi yang belum
          terbaca.
        </p>
      </header>
      {syncError && (
        <p role="alert" className="rounded-lg bg-rose-50 p-3 text-rose-800">
          {syncError}
        </p>
      )}
      <section>
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <h3 className="text-sm font-semibold text-slate-800">
            Hasil Parsing Email
          </h3>
          <span className="text-xs tabular-nums text-slate-500">
            {holdings.length} Emiten dari {brokers.length} Broker
          </span>
        </div>

        {holdings.length > 0 ? (
          <ul className="mt-2 rounded-xl border border-slate-300">
            {holdings.map((holding, index) => (
              <li
                key={`${holding.broker_id}-${holding.ticker}-${index}`}
                className="flex items-center justify-between gap-3 border-b border-slate-200 px-3 py-2 last:border-b-0"
              >
                <span className="min-w-0">
                  <span className="block text-sm font-semibold">
                    {holding.ticker}
                  </span>
                  <span className="block truncate text-xs text-slate-500">
                    {holding.broker_name}
                  </span>
                </span>
                <span className="shrink-0 text-right text-xs tabular-nums text-slate-600">
                  <span className="block">{holding.lot} lot</span>
                  <span className="block">
                    {formatCurrency(holding.avg_price)}
                  </span>
                </span>
              </li>
            ))}
          </ul>
        ) : (
          <p className="mt-2 rounded-xl border border-dashed border-slate-300 px-3 py-4 text-sm text-slate-600">
            Belum ada saham yang terbaca otomatis dari email.
          </p>
        )}
      </section>

      <section>
        <h3 className="text-sm font-semibold text-slate-800">
          Tambahkan Ticker secara manual
        </h3>
        <form onSubmit={submitManual} className="mt-2 grid gap-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="grid gap-1 text-xs text-slate-600">
              Broker
              <select
                value={brokerId}
                onChange={(event) => setBrokerId(Number(event.target.value))}
                disabled={finishing || brokers.length === 0}
                className="min-h-11 w-full min-w-0 rounded-lg border border-slate-300 bg-white px-2 text-sm text-slate-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:bg-slate-50"
              >
                {brokers.map((broker) => (
                  <option key={broker.id} value={broker.id}>
                    {broker.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="grid gap-1 text-xs text-slate-600">
              Ticker
              <input
                value={ticker}
                onChange={(event) => setTicker(event.target.value)}
                type="text"
                placeholder="BBCA"
                required
                autoComplete="off"
                disabled={finishing}
                className="min-h-11 w-full min-w-0 rounded-lg border border-slate-300 px-2 text-sm uppercase text-slate-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:bg-slate-50"
              />
            </label>
            <label className="grid gap-1 text-xs text-slate-600">
              Lot
              <input
                value={lot}
                onChange={(event) => setLot(event.target.value)}
                type="number"
                min="1"
                required
                disabled={finishing}
                className="min-h-11 w-full min-w-0 rounded-lg border border-slate-300 px-2 text-sm tabular-nums text-slate-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:bg-slate-50"
              />
            </label>
            <label className="grid gap-1 text-xs text-slate-600">
              Harga rata-rata
              <input
                value={price}
                onChange={(event) => setPrice(event.target.value)}
                type="number"
                min="1"
                step="0.01"
                required
                disabled={finishing}
                className="min-h-11 w-full min-w-0 rounded-lg border border-slate-300 px-2 text-sm tabular-nums text-slate-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:bg-slate-50"
              />
            </label>
          </div>
          <button
            type="submit"
            disabled={finishing || !selectedBroker}
            className="inline-flex min-h-11 w-full cursor-pointer items-center justify-center gap-1.5 rounded-lg bg-slate-200 px-4 text-sm font-semibold text-slate-700 hover:bg-slate-300 active:bg-slate-400 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            Add Ticker
          </button>
        </form>

        {manualHoldings.length > 0 && (
          <ul className="mt-3 rounded-xl border border-slate-300 bg-slate-50">
            {manualHoldings.map((holding, index) => (
              <li
                key={`${holding.ticker}-${index}`}
                className="flex items-center justify-between gap-3 border-b border-slate-200 px-3 py-2 last:border-b-0"
              >
                <span className="min-w-0">
                  <span className="block text-sm font-semibold">
                    {holding.ticker}
                  </span>
                  <span className="block truncate text-xs text-slate-500">
                    {holding.brokerName} · {holding.lot} lot ·{" "}
                    {formatCurrency(holding.price)}
                  </span>
                </span>
                <button
                  type="button"
                  onClick={() => onRemoveManual(index)}
                  disabled={finishing}
                  aria-label={`Hapus ${holding.ticker}`}
                  className="inline-flex min-h-11 min-w-11 shrink-0 cursor-pointer items-center justify-center rounded-lg text-slate-500 hover:bg-rose-50 hover:text-rose-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-40"
                >
                  <Trash2 size={16} aria-hidden="true" />
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>
      {finishError && (
        <p
          role="alert"
          className="border-t border-rose-200 bg-rose-50 px-5 py-3 text-sm text-rose-800"
        >
          {finishError}
        </p>
      )}

      <div className="sticky bottom-0 z-10 -mx-5 flex items-center justify-between gap-2 border-t border-slate-200 bg-white px-5 pt-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] sm:static sm:mx-0 sm:justify-end sm:py-4">
        <button
          type="button"
          onClick={onBack}
          disabled={finishing}
          className="inline-flex min-h-11 items-center gap-2 rounded-md px-2 text-sm font-medium text-slate-600 hover:bg-slate-100 disabled:opacity-50 sm:hidden"
        >
          <ArrowLeft size={16} aria-hidden="true" />
          Back
        </button>

        <button
          type="button"
          onClick={onFinish}
          disabled={finishing}
          className="inline-flex min-h-11 cursor-pointer items-center justify-center gap-2 rounded-lg px-4 text-sm font-semibold text-white hover:bg-teal-800 active:bg-teal-900 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {finishing ? "Finishing..." : "Finish Reviewing"}
          <ArrowRight size={16} aria-hidden="true" className="sm:hidden" />
        </button>
      </div>
    </section>
  );
}
