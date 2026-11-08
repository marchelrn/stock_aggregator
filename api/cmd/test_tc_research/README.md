# Pengujian TC: metadata-driven dan fallback Gemini

Modul penelitian yang dipertahankan:
- Baseline PDF asli: metadata sesuai, transaksi benar, AI tidak dipanggil.
- Perubahan format PDF: metadata gagal, Gemini mencoba ekstraksi objek transaksi.
- Verifikasi pengambilan PDF asli melalui Gmail (opsional).
- Unit test offline untuk engine, konfigurasi, perbandingan transaksi, dan provider Gemini.

Eksperimen Ollama, renderer Python, decoder OpenAI/Ollama, dan alat dump PDF/GORM lama sudah dihapus. `cmd/test_gmail_sync` dipertahankan **hanya sebagai bootstrap OAuth Gmail**, tanpa pencarian email, parsing PDF hardcoded, atau ekspor CSV.

## 1. Persiapan

Jalankan seluruh perintah dari direktori `api`, gunakan versi Go sesuai `go.mod`.

Fixture pribadi tetap di komputer masing-masing dan sengaja tidak masuk Git:

```text
testing/
  TC_1789262-20261019_1792405283-1.pdf
  TC_EDITED.pdf
  real_value.csv
```

Kedua PDF harus berisi transaksi sama jika eksperimen dimaksudkan mengisolasi perubahan layout. CSV memiliki header:

```csv
REF#,Board,Share,Lot,Quantity,Price,Buy,Sell,TransactionDate
```

Aturan CSV: ticker diawali empat huruf kapital pada `Share`; lot dan quantity integer positif; quantity = lot × 100; harga/nilai tanpa pemisah ribuan dan boleh dua digit desimal; tepat satu Buy/Sell positif; nilai bruto = quantity × price; tanggal `DD/MM/YYYY`. Nol `000` diterima. Reference disimpan sebagai string agar nol awal tidak hilang.

PDF, ground truth, laporan, token, binary, serta CSV debug tidak boleh di-commit. File ekspor lama `email_debug.csv`, `trade_results.csv`, dan binary `test_gmail_sync` dihapus dari tracking, **bukan dari penyimpanan lokal**. Riwayat commit lama tidak ditulis ulang oleh cleanup ini.

## 2. Unit test tanpa cloud/DB

```sh
go test ./... -count=1
go vet ./...
```

Pada cleanup terakhir, test runner TC, helper OAuth, service Gmail, dan metadata engine lulus, serta `go vet ./...` lulus. Suite `go test ./...` belum lulus penuh karena Windows Application Control memblokir executable `routes.test.exe` dari branch `testing`; tidak ada assertion failure yang dilaporkan pada modul yang diubah. Kebijakan Windows tidak diubah.

Tes memakai fake store, HTTP fake, serta dokumen sintetis. Tes PDF pribadi dilewati secara eksplisit jika fixture tidak tersedia; tes sintetis tetap dapat dijalankan pada checkout baru. Tes tidak membuka browser, menjalankan migrasi, atau mengirim dokumen ke Gemini.

## 3. Baseline: format tidak berubah

```sh
go run ./cmd/test_tc_research --case original --config-file migrations/templates/stockbit_v1.json --report testing/research-report-baseline-01.json
```

Hasil yang diharapkan:

```text
original: primary=match fallback_required=false expected=2 parsed=2 missing=0 unexpected=0
AI not executed.
```

`--case original` tidak membaca PDF edited. Untuk membuktikan AI tetap tidak dipanggil ketika fitur fallback diaktifkan, gunakan:

```sh
go run ./cmd/test_tc_research --case original --config-file migrations/templates/stockbit_v1.json --ai-fallback --gemini-model gemini-3-flash-preview --report testing/research-report-control-01.json
```

Jika baseline cocok, kasus original tidak memiliki objek `fallback` dan `ai_executed=false`. CSV mismatch saja tidak memicu AI saat metadata engine berhasil mengembalikan transaksi valid; ground truth tidak tersedia sebagai pemicu di produksi.

## 4. Format berubah: fallback Gemini

Di `api/.env` atau environment:

```dotenv
GEMINI_API_KEY=isi_key_anda
GEMINI_MODEL=gemini-3-flash-preview
```

Nama tampilan `Gemini_3_Flash` bukan ID API. `--gemini-model` dapat dipakai untuk override tanpa mengubah `.env`. API key Gemini berbeda dari kredensial OAuth Gmail. Jangan menaruh key di argumen CLI, frontend, log, atau Git.

**Privasi dan biaya:** Gemini mengirim PDF ke Google; Free Tier dapat menggunakan konten untuk peningkatan produk. PDF edited bukan otomatis anonim. Gunakan dokumen disamarkan bila tidak ingin data pribadi dikirim. Pastikan proyek di AI Studio tetap Free Tier tanpa billing berbayar. Pricing model yang diuji mencantumkan Standard Free Tier input/output gratis, tetapi API key tidak membuktikan tier proyek. Runner tidak mengaktifkan billing, mengganti model, atau retry otomatis. HTTP 429 berarti kuota habis/tidak tersedia.

Uji hanya PDF edited:

```sh
go run ./cmd/test_tc_research --case edited --config-file migrations/templates/stockbit_v1.json --ai-fallback --gemini-model gemini-3-flash-preview --ai-timeout 120s --ai-max-tokens 4096 --include-ai-trades --report testing/research-report-gemini-01.json
```

Atau uji original dan edited sekaligus dengan `--case all` (default). PDF asli yang cocok tidak dikirim ke Gemini. `--ai-provider gemini` tetap diterima untuk kompatibilitas, tetapi Gemini kini satu-satunya provider.

Alur: PDF → metadata parser → kegagalan dokumen yang diizinkan → Gemini PDF langsung + JSON Schema → decoding objek → validasi → perbandingan CSV. Tidak membutuhkan Ollama, Python, atau renderer gambar. PDF maksimum 20 MiB; tidak dipecah otomatis. Redirect HTTP ditolak agar key/dokumen tidak dialihkan.

Kontrak JSON: `trades[]` dengan reference, ticker, side (`BUY`/`SELL`), trade_date (`YYYY-MM-DD`), lot, quantity, price dan gross_amount. Uang berupa string desimal tanpa pemisah ribuan. Nilai null/field wajib hilang ditolak, bukan diisi dari CSV. Hanya final content dengan `finishReason=STOP` diterima; reasoning dan respons terpotong tidak dipakai. Tanggal, kuantitas, sisi, dan aritmetika diperiksa sebelum hasil dipetakan ke `[]dto.ParsedTrade`.

Secara default laporan hanya status/hitungan/diagnostik. `--include-ai-trades` menyertakan objek transaksi tervalidasi dalam laporan lokal; lindungi file tersebut. Raw PDF, API key, identitas nasabah, dan reasoning tidak dicetak. Laporan tujuan harus belum ada; gunakan nama baru pada pengulangan.

## 5. Metadata dari database

Tanpa `--config-file`, runner memuat snapshot template aktif dari DB secara readonly:

```sh
go run ./cmd/test_tc_research --case original
```

`--config-file` adalah **preview konfigurasi file**, bukan pengujian koneksi DB. Koneksi default memakai `ENV=production` + `DB_URL`, atau konfigurasi lokal `DB_USER`, `DB_PASS`, `DB_HOST`, `DB_PORT`, `DB_NAME`. Timeout pembacaan DB 5 detik; runner tidak menjalankan migrasi/AutoMigrate.

Migrasi `create_pdf_parsing_templates` sudah terdaftar pada startup API normal. Operator harus memeriksa DB tujuan, backup dan seluruh migrasi tertunda sebelum deployment. Startup `go run .` menjalankan **semua migrasi tertunda dan server**, sehingga bukan perintah unit test.

Tabel `pdf_parsing_templates` menyimpan key/version, active, broker, selector email, dan konfigurasi JSONB. Seed `stockbit` v1 berasal dari `migrations/templates/stockbit_v1.json`; kontraknya ada di `pdf_parsing_config.schema.json` pada folder yang sama. Pola broker/tanggal/baris, mapping field, locale angka, serta shares_per_lot adalah metadata DB, bukan percabangan kode engine. Regex Go/RE2 dipakai sebagai mekanisme generik, bukan aturan broker hardcoded.

Seed hanya mendukung format Stockbit asli pada fixture. Format/broker lain memerlukan template tersendiri. Engine menolak template tidak cocok/ambigu, konfigurasi invalid, tanggal tidak valid, tabel parsial, dan aritmetika salah. Engine produksi memuat ulang metadata tiap panggilan; runner memakai snapshot agar satu run konsisten. Error DB/config/template ambigu tidak disamarkan dengan AI.

Inspeksi versi aktif:

```sql
SELECT key, version, active, broker_name,
       email_sender, subject_contains, filename_suffix
FROM pdf_parsing_templates ORDER BY key, version;
```

Untuk perubahan, buat versi baru nonaktif dari JSON yang sudah diuji, kemudian aktifkan versi baru/nonaktifkan versi lama dalam satu transaksi. Jangan menimpa riwayat. Tidak ada UI/endpoint CRUD template publik. Koordinasikan penulis metadata agar tidak mengaktifkan template tumpang tindih.

**Konfigurasikan selector sebelum sync mailbox nyata.** Selector seed kosong berarti wildcard. `email_sender` harus mailbox tanpa display name; subject substring dan filename suffix case-insensitive. Discovery produksi memakai PDF + gabungan sender DB bila semua sender terisi; subject/suffix dievaluasi lokal agar tidak bergantung kosakata subjek tetap. Pesan di luar selector dilewati; PDF eligible yang gagal diparsing menghentikan sync tanpa memajukan checkpoint. `TradesSynced=0` karena service belum menyimpan transaksi hasil parsing.

## 6. Gmail opsional

Konfigurasi backend: `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`; token CLI `.gmail_token.json`. Token tersimpan di DB aplikasi tidak otomatis digunakan runner CLI.

Bootstrap OAuth jika token belum ada:

```sh
go run ./cmd/test_gmail_sync
```

Helper hanya login/refresh dan menyimpan token secara privat. Redirect yang harus diizinkan di Google OAuth: `http://localhost:9090/callback`. Login memiliki state acak, PKCE, listener loopback, dan batas waktu 5 menit. Tidak mencari email, membuat CSV, atau menjalankan parser lama.

Uji pengambilan PDF asli:

```sh
go run ./cmd/test_tc_research --case original --config-file migrations/templates/stockbit_v1.json --gmail --report testing/research-report-gmail-01.json
```

Runner mencari maksimal 20 pesan/4 halaman, mendukung MIME root/nested, inline/downloaded attachment, dan base64 padded/raw. Nama file dan SHA256 harus identik dengan fixture asli. Metadata From/Subject/filename nyata diteruskan ke parser. Batas pengambilan 60 detik, HTTP 15 detik. Verifikasi SHA tidak membuktikan autentisitas pengirim.

Opsi pencarian: `--message-id ID_GMAIL_API`, `--query 'has:attachment filename:pdf'`, dan `--token-file lokasi_token.json`; semuanya memerlukan `--gmail`. Query khusus tidak menghapus verifikasi nama/SHA. Jika memilih `--case edited --gmail`, original tetap dibaca untuk baseline Gmail dan kasus `gmail_original` tetap ditambahkan.

## 7. Interpretasi dan batas validasi

- `primary`: match, mismatch, empty, atau parse_error.
- `fallback_required`: hasil utama bukan match terhadap CSV; lihat objek fallback untuk pemanggilan nyata.
- `fallback.attempted`: cabang fallback dicoba. `ai_executed` adalah flag historis permintaan HTTP AI dicoba, bukan jaminan inferensi berhasil.
- `fallback.status`: match, mismatch, invalid_output, atau request_error.
- `missing/unexpected`: pencocokan tanpa bergantung urutan dan tanpa memakai ulang transaksi duplikat.
- Field pembanding: tanggal, ticker, side, lot, price dan gross amount; toleransi uang 0,01. Reference/Board/nama perusahaan/broker tidak dievaluasi pada perbandingan utama.
- Pada parse_error/invalid_output, hitungan bukan ukuran akurasi karena belum ada transaksi tervalidasi.
- Exit `0`: pengujian selesai dan hasil yang dievaluasi cocok; edited gagal tanpa AI boleh menjadi observasi. Exit `1`: baseline/fallback gagal evaluasi. Exit `2`: kegagalan operasional, termasuk DB, kuota/request, atau laporan.

Pengujian nyata sebelumnya: metadata original cocok dua transaksi; edited tidak cocok. REST Gemini `gemini-3-flash-preview` mengekstrak kedua transaksi edited dengan benar, STOP, tanpa missing/unexpected, sekitar 9,3 detik. Inferensi itu dilakukan sebagai **REST smoke test terpisah** setelah executable `go run` diblokir Windows Application Control; bukan bukti runner Go/DB/Gmail end-to-end. Laporan privat tidak dikomit. Unit test HTTP fake/engine dan `go vet` adalah validasi terpisah.

Jika Windows memblokir executable, jangan menonaktifkan proteksi atau membuat pengecualian luas cache Go. Gunakan lingkungan pengembangan/izin yang disetujui administrator. Menjalankan PowerShell Administrator tidak otomatis melewati kebijakan.

Dua sampel hanya demonstrasi. Keberhasilan JSON Schema bukan jaminan kebenaran transaksi; evaluasi beberapa dokumen, versi, serta pengulangan sebelum menarik kesimpulan penelitian. Ground truth tidak tersedia di produksi; salah field semantik dapat lolos validasi aritmetika.
