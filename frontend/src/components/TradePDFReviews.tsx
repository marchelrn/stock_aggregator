import { useEffect, useState } from "react";
import { api } from "../lib/api";

type Review = { id: number; broker: string; filename: string; state: string };
type Fill = { ref: string; ticker: string; side: string; lots: string; shares: string; price: string; gross: string };
type Fee = { label: string; buy: string; sell: string };
type Draft = {
  date: string; rows: Fill[]; fees: Fee[];
  buy_gross: string; sell_gross: string; buy_net: string; sell_net: string;
};

const path = "/api/gmail/reviews";
const button = "min-h-11 rounded-lg border border-slate-300 px-3 py-2 text-sm font-semibold disabled:opacity-50";
const primary = button + " bg-teal-700 text-white";
const input = "mt-1 block min-h-11 w-full min-w-0 rounded border border-slate-300 bg-transparent px-2 py-1";
const fillFields = { ref: "Referensi", ticker: "Kode saham", lots: "Lot", shares: "Lembar", price: "Harga per lembar", gross: "Nilai bruto" } as const;
const totals = { buy_gross: "Bruto beli", buy_net: "Total bayar beli", sell_gross: "Bruto jual", sell_net: "Total terima jual" } as const;

async function imageData(file: File): Promise<string> {
  if (!["image/jpeg", "image/png"].includes(file.type) || file.size > 8 * 1024 * 1024)
    throw new Error("Pilih gambar PNG atau JPEG maksimal 8 MB.");
  return new Promise((resolve,reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error("Gambar tidak dapat dibuka."));
    reader.onload = () => resolve(String(reader.result).split(",")[1]);
    reader.readAsDataURL(file);
  });
}

function ReviewItem({ item, done }: { item: Review; done: (count: number) => void }) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [source, setSource] = useState<"image" | "pdf">("image");
  const [file, setFile] = useState<File | null>(null);
  const [consent, setConsent] = useState(false);
  const [confirmed, setConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const endpoint = path + "/" + item.id;
  async function run(action: () => Promise<void>) {
    setError(""); setBusy(true);
    try { await action(); } catch (err) { setError((err as Error).message); }
    finally { setBusy(false); }
  }
  function change(next: Draft) { setDraft(next); setConfirmed(false); }
  function field(label: string, value: string, update: (v: string) => void, type = "text") {
    return <label key={label} className="min-w-0 text-sm">{label}<input className={input} type={type} value={value ?? ""} onChange={e => update(e.target.value)} /></label>;
  }
  function start() {
    setOpen(true);
    if (item.state === "review" && !draft)
      void run(async () => setDraft(await api<Draft>(endpoint)));
  }
  async function extract() {
    const image = source === "image" && file ? await imageData(file) : undefined;
    const result = await api<Draft>(endpoint + "/extract", { method: "POST", body: JSON.stringify({ consent, source, image }) });
    setDraft(result); setFile(null);
  }
  async function save() {
    const res = await api<{ saved: number }>(endpoint + "/confirm", { method: "POST", body: JSON.stringify({ confirmed, draft }) });
    done(res.saved);
  }
  return <article className="min-w-0 border-t border-slate-300 py-3">
    <div className="flex flex-wrap items-center justify-between gap-2">
      <div className="min-w-0"><p className="font-semibold">{item.broker}</p><p className="break-all text-sm">{item.filename}</p></div>
      <button className={button} disabled={busy} aria-expanded={open} onClick={() => open ? setOpen(false) : start()}>{open ? "Tutup" : "Tinjau PDF"}</button>
    </div>
    {open && <fieldset disabled={busy} className="mt-3 min-w-0 space-y-3">
      {draft ? <>
        <p>Cocokkan hasil dengan PDF asli. Lengkapi kolom yang kosong. Gunakan titik untuk desimal.</p>
        {field("Tanggal transaksi", draft.date, date => change({ ...draft, date }), "date")}
        {draft.rows.map((row, i) => {
          const edit = (key: keyof Fill, value: string) => change({ ...draft, rows: draft.rows.map((r,j) => j === i ? { ...r, [key]: value } : r) });
          return <fieldset key={i} className="grid grid-cols-2 gap-2"><legend className="mb-2 font-semibold">Transaksi {i+1}</legend>
            <label className="text-sm">Jenis<select className={input} value={row.side} onChange={e => edit("side", e.target.value)}><option value="">Pilih</option><option value="BUY">Beli</option><option value="SELL">Jual</option></select></label>
            {(Object.keys(fillFields) as (keyof typeof fillFields)[]).map(k => field(fillFields[k], row[k], v => edit(k, v)))}
            <button className={button} onClick={() => change({ ...draft, rows: draft.rows.filter((_,j) => i!==j) })}>Hapus baris</button>
          </fieldset>;
        })}
        <button className={button} onClick={() => change({ ...draft, rows:[...draft.rows, { ref:"",ticker:"",side:"",lots:"",shares:"",price:"",gross:"" }] })}>Tambah transaksi</button>
        <fieldset className="space-y-2"><legend className="font-semibold">Rincian biaya (Rp)</legend>
          {(draft.fees ?? []).map((fee,i) => <div key={i} className="grid grid-cols-2 gap-2">
            {(["label","buy","sell"] as const).map((k,j) => field(["Nama biaya","Biaya beli","Biaya jual"][j], fee[k], v => change({ ...draft, fees:draft.fees.map((f,n) => n===i ? {...f,[k]:v} : f) })))}
            <button className={button} onClick={() => change({ ...draft, fees:draft.fees.filter((_,j)=>i!==j) })}>Hapus biaya</button>
          </div>)}
          <button className={button} onClick={() => change({ ...draft, fees:[...(draft.fees ?? []),{label:"",buy:"",sell:""}] })}>Tambah biaya</button>
        </fieldset>
        <fieldset className="grid grid-cols-2 gap-2"><legend className="font-semibold">Total tercetak (Rp)</legend>
          {(Object.keys(totals) as (keyof typeof totals)[]).map(k => field(totals[k],draft[k],v=>change({...draft,[k]:v})))}
        </fieldset>
        <label className="flex items-start gap-2"><input type="checkbox" checked={confirmed} onChange={e=>setConfirmed(e.target.checked)} />Saya sudah mencocokkan seluruh transaksi, biaya, dan total dengan dokumen asli.</label>
        <button className={primary} disabled={!confirmed || busy} onClick={() => void run(save)}>Simpan transaksi</button>
      </> : item.state === "review" ? <button className={button} onClick={start}>Buka hasil review</button> : <>
        <label className="block text-sm">Bagian yang dikirim ke Google Gemini<select className={input} value={source} onChange={e => { setSource(e.target.value as typeof source); setConsent(false); }}><option value="image">Potongan gambar pilihan saya</option><option value="pdf">Seluruh PDF dari Gmail</option></select></label>
        {source === "image" ? <label className="block">Pilih gambar tabel transaksi dan total. Potong atau tutupi nama dan nomor rekening sebelum mengirim.<input className="mt-2 block w-full text-sm" type="file" accept="image/png,image/jpeg" onChange={e => { setFile(e.target.files?.[0] ?? null); setConsent(false); }} /></label> : <p>Seluruh lampiran akan dikirim ke Google Gemini, termasuk nama, nomor rekening, dan rincian transaksi yang tercetak.</p>}
        <label className="flex items-start gap-2"><input type="checkbox" checked={consent} onChange={e => setConsent(e.target.checked)} />Saya setuju mengirim {source==="pdf" ? "PDF lengkap ini" : "gambar pilihan saya"} ke Google Gemini untuk membaca transaksi.</label>
        <button className={primary} disabled={!consent || busy || (source==="image" && !file)} onClick={() => void run(extract)}>Coba baca dengan Gemini</button>
      </>}
      {busy && <p role="status">Memproses dokumen…</p>}
      {error && <p role="alert">{error}</p>}
    </fieldset>}
  </article>;
}

export default function TradePDFReviews({ syncing, onSaved }: { syncing: boolean; onSaved: () => void }) {
  const [rows,setRows]=useState<Review[]>([]);
  const [message,setMessage]=useState("");
  const [retry,setRetry]=useState(0);
  const [error,setError]=useState("");
  useEffect(() => {
    if (syncing) return;
    const controller=new AbortController();
    setError("");
    api<Review[]>(path,{signal:controller.signal,cache:"no-store"}).then(data => {
      if (!controller.signal.aborted) setRows(data);
    }).catch(err=>{if (!controller.signal.aborted) setError((err as Error).message);});
    return ()=>controller.abort();
  },[syncing,retry]);
  if (!rows.length && !message && !error) return null;
  return <section className="mx-auto w-full max-w-5xl space-y-2 px-4 py-3">
    {rows.length>0 && <h2 className="text-lg font-semibold">PDF perlu ditinjau ({rows.length})</h2>}
    {rows.map(r=><ReviewItem key={r.id} item={r} done={count=>{
      setRows(all=>all.filter(item=>item.id!==r.id));
      setMessage(count ? count+" transaksi disimpan." : "Transaksi sudah tercatat."); onSaved();
    }}/>)}
    {message && <p role="status">{message}</p>}
    {error && <p role="alert">Daftar PDF belum dapat dimuat. <button className={button} onClick={()=>setRetry(n=>n+1)}>Coba lagi</button></p>}
  </section>;
}
