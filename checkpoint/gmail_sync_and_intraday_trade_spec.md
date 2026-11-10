# Spesifikasi Desain: Gmail Sync & Penanganan Transaksi Intraday (Same-Day Trade)

Dokumen ini merangkum rincian arsitektur, batasan sistem, dan formula kalkulasi komprehensif untuk sinkronisasi *Trade Confirmation* (TC) dari Gmail serta pengelolaan *Stock Holding* dan saldo *Broker Cash*.

---

## 1. Strategi Cutoff Gmail Sync (Onboarding Baseline vs Sync Kontinu)

### Masalah
* Jika user baru mendaftar hari ini ($T_0$) dan menginput portofolio awal (holding saham, harga rata-rata, dan saldo kas RDN), lalu Gmail sync menarik seluruh email historis tanpa batas waktu:
  * Terjadi *double counting* (lot bertambah ganda).
  * Saldo kas terdistorsi.
  * Rekonstruksi masa lalu secara parsial sangat rentan bias karena tidak mencakup instrumen non-TC (E-IPO, dividen kas/saham via KSEI, mutasi deposit/penarikan dana RDN antar bank).

### Keputusan & Spesifikasi
1. **Baseline Ground Truth ($T_0$)**:
   * Data portofolio yang diinput user pada saat registrasi/onboarding dianggap sebagai titik awal saldo yang valid dan final.
2. **Cutoff Tanggal Sync**:
   * Gmail Sync **hanya berjalan untuk transaksi pada atau setelah hari pendaftaran user** (`user.CreatedAt`).
   * Jika `user.LastGmailSyncAt == nil` (sinkronisasi pertama kali), sistem mengambil tanggal cutoff:
     $$\text{SyncStart} = \text{user.CreatedAt} - 1\text{ hari}$$
     *(Minus 1 hari sebagai buffer perbedaan zona waktu antara UTC dan WIB/WITA)*.
   * Query pencarian Gmail Messages API:
     ```
     subject:("trade confirmation" OR "konfirmasi transaksi" OR "contract note") has:attachment filename:pdf after:YYYY/MM/DD
     ```
   * Dengan pendekatan ini, seluruh email sebelum tanggal pendaftaran user otomatis terisolasi dan tidak akan menarik data usang.

---

## 2. Penanganan Saham yang Sudah Habis (Lot = 0)

### Masalah
* Saat ini endpoint `/my/stocks` mengambil seluruh baris data di tabel `stocks` tanpa memfilter jumlah lot. Jika user menjual habis suatu saham, data saham dengan `lot: 0` tetap tampil di daftar portofolio aktif dan *ticker breakdown*.

### Keputusan & Spesifikasi
1. **Penyaringan Portofolio Aktif (`/my/stocks`)**:
   * Repository `StocksRepository` (`GetAllStocks`, `GetStocks`) dan `BrokersRepository` (`GetBrokerDetails`, `GetAllBrokerHoldings`) menambahkan filter wajib:
     ```sql
     WHERE user_id = ? AND lot > 0
     ```
   * Saham yang sudah habis (`lot == 0`) **otomatis tersembunyi** dari daftar kepemilikan aktif dan ringkasan portofolio broker.
2. **Integritas Riwayat Transaksi (`/my/transactions`)**:
   * Seluruh catatan transaksi jual/beli tetap tersimpan utuh di tabel `transactions` sebagai audit trail riwayat transaksi.
3. **Pemberlakuan Beli Kembali (*Re-entry*)**:
   * Jika user membeli kembali saham yang sebelumnya telah berstatus `lot == 0`, sistem memperlakukannya sebagai posisi baru (*fresh position*):
     * `AvgPrice` di-reset murni ke harga beli baru (+ fee/tax), tanpa mencampuradukkan riwayat harga lama.

---

## 3. Logika Komprehensif: Netting Transaksi Harian & Penanganan Same-Day Trade

### Masalah pada Sistem Eksisting
1. **Urutan Eksekusi (*Execution Order*)**:
   * Pengurutan `SELL` sebelum `BUY` pada hari yang sama menyebabkan transaksi gagal (*error*: `insufficient lot`) jika user melakukan *day trading / scalping* (beli di pagi hari, jual di siang/sore hari saat posisi awal 0 lot).
2. **Validasi Saldo Kas (*Cash Barrier*)**:
   * Pengecekan saldo kas `if broker.Cash < amountDone` menolak transaksi beli jika pencatatan saldo internal belum terisi, padahal di bursa berlaku mekanisme *settlement netting* (T+2) dan dokumen Trade Confirmation PDF merupakan kontrak resmi bursa yang **sudah sah dieksekusi**.

---

### Alur Kalkulasi Komprehensif (Batch Netting per Dokumen TC)

Setiap dokumen Trade Confirmation PDF diproses sebagai satu kesatuan harian (*Daily Batch Reconciliation*):

```
       [ Dokumen Trade Confirmation PDF ]
                      │
                      ▼
 ┌──────────────────────────────────────────┐
 │ 1. Simpan Transaksi Individual          │
 │    - Catat setiap baris BUY/SELL ke      │
 │      tabel transactions (Audit Trail)    │
 └────────────────────┬─────────────────────┘
                      │
                      ▼
 ┌──────────────────────────────────────────┐
 │ 2. Netting Per (BrokerID, Ticker)        │
 │    - Total Buy Lot & Buy Amount          │
 │    - Total Sell Lot & Sell Amount        │
 │    - ΔLot = Total Buy Lot - Total Sell Lot│
 └────────────────────┬─────────────────────┘
                      │
                      ▼
 ┌──────────────────────────────────────────┐
 │ 3. Evaluasi State Holding & AvgPrice     │
 │    - Kasus A: Scalping / Habis (Lot = 0) │
 │    - Kasus B: Jual Sebagian (Avg Tetap)  │
 │    - Kasus C: Beli Baru / Tambah (Avg Baru)│
 │    - Kasus D: Jual Sebagian + Beli Lagi   │
 └────────────────────┬─────────────────────┘
                      │
                      ▼
 ┌──────────────────────────────────────────┐
 │ 4. Penyesuaian Saldo Kas Broker          │
 │    - Net Cash Delta = Net Sell - Net Buy │
 │    - broker.Cash += Net Cash Delta       │
 └──────────────────────────────────────────┘
```

#### A. Formula Matematika Perhitungan

1. **Agregasi Pembelian & Penjualan**:
   * Total Beli:
     $$\text{BuyLot} = \sum \text{Lot}_{\text{buy}}$$
     $$\text{BuyGross} = \sum (\text{Lot}_{\text{buy}} \times 100 \times \text{Price}_{\text{buy}})$$
     $$\text{BuyCost} = \text{BuyGross} + \text{Tax}_{\text{buy}}$$
   * Total Jual:
     $$\text{SellLot} = \sum \text{Lot}_{\text{sell}}$$
     $$\text{SellGross} = \sum (\text{Lot}_{\text{sell}} \times 100 \times \text{Price}_{\text{sell}})$$
     $$\text{SellProceeds} = \text{SellGross} - \text{Tax}_{\text{sell}}$$

2. **Perubahan Kas Bersih (Net Cash Settlement)**:
   $$\Delta \text{Cash} = \text{SellProceeds} - \text{BuyCost}$$
   $$\text{Broker.Cash}_{\text{baru}} = \text{Broker.Cash}_{\text{lama}} + \Delta \text{Cash}$$

3. **Perubahan Kepemilikan Lot**:
   $$\text{NewLot} = \text{CurrentLot} + \text{BuyLot} - \text{SellLot}$$

4. **Kalkulasi Average Price Sesuai Skenario**:
   * **Skenario 1 (Jual Habis / Scalping Day Trade, $\text{NewLot} == 0$)**:
     * `stock.Lot = 0`
     * `stock.AvgPrice = 0`
     * Tidak tampil di `/my/stocks`.
   * **Skenario 2 (Jual Sebagian Saja, $\text{BuyLot} == 0 \text{ dan } \text{SellLot} > 0$)**:
     * `stock.Lot = CurrentLot - SellLot`
     * `stock.AvgPrice` **tetap sama** dengan harga rata-rata sebelumnya (karena penjualan tidak mengubah basis modal sisa saham).
   * **Skenario 3 (Beli Murni / Penambahan Lot, $\text{SellLot} == 0 \text{ dan } \text{BuyLot} > 0$)**:
     * Jika `CurrentLot == 0`:
       $$\text{AvgPrice}_{\text{baru}} = \frac{\text{BuyCost}}{\text{BuyLot} \times 100}$$
     * Jika `CurrentLot > 0`:
       $$\text{AvgPrice}_{\text{baru}} = \frac{(\text{CurrentLot} \times 100 \times \text{CurrentAvgPrice}) + \text{BuyCost}}{\text{NewLot} \times 100}$$
   * **Skenario 4 (Intraday Jual & Beli Kembali, $\text{SellLot} > 0 \text{ dan } \text{BuyLot} > 0$)**:
     * Langkah 4a: Porsi jual mengurangi lot awal terlebih dahulu:
       $$\text{RemainingOldLot} = \max(0, \text{CurrentLot} - \text{SellLot})$$
     * Langkah 4b: Porsi beli membentuk rata-rata tertimbang bersama sisa lot lama:
       * Jika $\text{RemainingOldLot} == 0$:
         $$\text{AvgPrice}_{\text{baru}} = \frac{\text{BuyCost}}{\text{BuyLot} \times 100}$$
       * Jika $\text{RemainingOldLot} > 0$:
         $$\text{AvgPrice}_{\text{baru}} = \frac{(\text{RemainingOldLot} \times 100 \times \text{CurrentAvgPrice}) + \text{BuyCost}}{\text{NewLot} \times 100}$$

---

## 4. File Codebase yang Terdampak

1. [`api/service/gmail_service.go`](file:///home/lerch/Documents/stock_aggregator/api/service/gmail_service.go):
   * Tambahkan fallback query cutoff menggunakan `user.CreatedAt` saat `user.LastGmailSyncAt == nil`.
   * Perbarui dispatch pemrosesan trade agar menggunakan logika batch reconciliation atau urutan netting.
2. [`api/repository/stocks_repository.go`](file:///home/lerch/Documents/stock_aggregator/api/repository/stocks_repository.go):
   * Tambahkan klausul `AND lot > 0` pada query `GetAllStocks` dan `GetStocks`.
3. [`api/repository/brokers_repository.go`](file:///home/lerch/Documents/stock_aggregator/api/repository/brokers_repository.go):
   * Tambahkan preload filter `lot > 0` pada `GetBrokerDetails` dan `GetAllBrokerHoldings`.
4. [`api/repository/transaction_repository.go`](file:///home/lerch/Documents/stock_aggregator/api/repository/transaction_repository.go):
   * Perbaiki logika eksekusi transaksi: hindari penolakan transaksi valid dari TC (insufficient cash/lot) dan terapkan formula holding state machine yang akurat.
