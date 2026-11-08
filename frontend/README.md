# Frontend - Stock Portfolio Dashboard

Frontend ini dibuat sebagai dashboard untuk backend `stock_api`.
Teknologi yang dipakai: **Vue.js 3** dengan **Vite** sebagai build tool.

## Fitur

- Ringkasan portfolio dari `GET /portfolio/summary`
- Breakdown broker (allocation + weight)
- Breakdown ticker (market value, P/L, weight)
- List broker dan holding
- Tambah broker (`POST /broker`)
- Tambah holding (`POST /holding`)
- Hapus broker (`DELETE /broker/:id`)
- Hapus holding (`DELETE /holding/:id`)
- Lookup harga multi ticker (`GET /stock?tickers=A,B,C`)
- Konfigurasi `Base URL API` langsung dari UI

## Halaman Market (`/market`)

- Watchlist otomatis berdasarkan holding dengan lot positif dari `GET /my/stocks`.
- Ticker yang sama dari beberapa broker digabung; filter dan urutan perubahan tersedia.
- Harga watchlist menggunakan `GET /prices?ticker=A,B,C`, dengan harga yang hilang ditandai, bukan dianggap nol.
- Top gainers/losers hanya mencakup watchlist, bukan seluruh saham BEI.
- Klik ticker untuk detail dari `GET /api/market/:ticker` (JWT wajib): nama emiten, harga, previous close, low/high harian dan 52 minggu, volume, waktu harga, serta chart close harian satu bulan.
- Refresh otomatis setiap lima menit saat tab terlihat; dapat dimatikan. Cache backend juga lima menit.
- Sumber Yahoo Finance dapat tertunda dan tidak menjamin data real-time. Tidak memerlukan API key; ketersediaan/rate limit dan izin penggunaan tetap mengikuti penyedia. Nilai yang tidak tersedia ditampilkan `—`.
- Frontend dan backend terbaru harus di-deploy bersama untuk endpoint detail ini.

**Batasan kepemilikan:** halaman mengikuti holdings yang dikembalikan backend. Repository holdings saat ini belum memfilter `user_id`, sehingga watchlist belum terisolasi per akun pada deployment multi-user. JWT pada endpoint Market tidak memperbaiki keterbatasan endpoint holdings yang sudah ada.

Tes helper watchlist dan movers:

```sh
node --test tests/market.test.cjs
```

## Struktur Project

```
frontend/
├── index.html              # Entry point HTML
├── package.json            # Dependencies dan scripts
├── vite.config.js          # Vite configuration
└── src/
    ├── main.js             # Vue app entry point
    ├── App.vue             # Root component
    ├── styles.css          # Global styles
    ├── components/         # Vue components
    │   ├── BrokerCards.vue
    │   ├── BrokerForm.vue
    │   ├── BrokerTable.vue
    │   ├── ConfigForm.vue
    │   ├── HoldingForm.vue
    │   ├── HoldingTable.vue
    │   ├── KpiGrid.vue
    │   ├── StatusBar.vue
    │   ├── StockPrices.vue
    │   └── TickerTable.vue
    └── composables/        # Vue composables (shared logic)
        ├── useApi.js       # API client
        ├── useFormatters.js # Number/currency formatters
        └── usePortfolio.js # Portfolio state management
```

## Cara Menjalankan

### Development Mode

1. Pastikan Node.js terinstall (v18+ recommended)
2. Install dependencies:
   ```bash
   cd frontend
   npm install
   ```
3. Jalankan development server:
   ```bash
   npm run dev
   ```
4. Buka browser di `http://localhost:3000`
5. Pastikan backend jalan di `http://localhost:8080` (atau ubah Base URL di UI)

### Production Build

```bash
npm run build
```

File hasil build akan ada di folder `dist/`.

### Preview Production Build

```bash
npm run preview
```

## Endpoint Backend yang Dipakai

- `GET /portfolio/summary`
- `GET /broker`
- `POST /broker`
- `DELETE /broker/:id`
- `GET /holding`
- `POST /holding`
- `DELETE /holding/:id`
- `GET /stock?tickers=A,B,C`

## Catatan

- Frontend ini menggunakan Vue 3 Composition API dengan `<script setup>` syntax
- State management menggunakan Vue composables (tidak perlu Vuex/Pinia untuk skala ini)