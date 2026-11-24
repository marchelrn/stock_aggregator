# Uji pengguna dari HP Android

Untuk peserta iPhone atau Android yang ingin langsung membuka tautan di browser, gunakan [panduan satu tautan HTTPS](iphone-android.md).

Peserta membuka aplikasi di Chrome pada HP mereka. Frontend, backend Go, dan PostgreSQL berjalan di laptop fasilitator. ADB meneruskan dua port dari HP ke laptop melalui Wi-Fi atau kabel USB. Cara ini mendukung login Google di localhost tanpa domain, Nginx, atau hosting publik. HP dan laptop tetap memerlukan internet untuk Google, Gmail, dan harga saham.

## 1. Siapkan laptop dan akun uji

Jalankan perintah dari direktori kerja branch `testing`. Siapkan Go sesuai `api/go.mod`, Node.js yang mendukung Vite proyek, npm, PostgreSQL, curl, dan Android SDK Platform Tools (`adb`). Pertahankan laptop menyala selama pengujian.

1. Siapkan database kosong khusus sesi, misalnya `createdb -h localhost -U NAMA_ROLE stock_uat`. Gunakan role PostgreSQL yang dapat membuat tabel di database tersebut. Server menjalankan migrasi dan penjadwal Gmail saat mulai.
2. Jika `api/.env` belum ada, buat dari contoh dengan `cp -n api/.env.example api/.env`. Isi `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, dan `DB_NAME=stock_uat`. Siapkan `JWT_SECRET` dan `ENCRYPTION_KEY` khusus uji. Untuk menghasilkan nilai baru, gunakan `openssl rand -hex 32` untuk JWT dan `openssl rand -hex 16` untuk kunci enkripsi 32 karakter. Simpan nilainya tetap selama sesi.
3. Gunakan OAuth client bertipe **Web application**, aktifkan Gmail API, dan isi `GOOGLE_CLIENT_ID` serta `GOOGLE_CLIENT_SECRET` di `api/.env`. Jika consent screen berstatus **Testing**, tambahkan alamat Google peserta ke **Test users**. Untuk uji pembacaan transaksi, pilih akun yang mempunyai email broker yang didukung dan pastikan peserta bersedia memberikan akses baca Gmail.
4. Daftarkan **Authorized redirect URI** persis `http://localhost:43030/auth/google/callback`. Jalur ini dipakai untuk login dan izin Gmail. Browser HP akan kembali ke frontend pada port 43000. [Panduan OAuth Google](https://developers.google.com/identity/protocols/oauth2/web-server)
5. Atur nilai berikut di `api/.env`. Simpan rahasia hanya di laptop fasilitator.

```dotenv
ENV=testing
PORT=43030
FRONTEND_URL=http://localhost:43000
GOOGLE_REDIRECT_URL=http://localhost:43030/auth/google/callback
```

## 2. Jalankan kedua layanan

**Terminal 1 — backend**

```bash
cd api
go run .
```

**Terminal 2 — frontend**

```bash
cd frontend
npm ci
VITE_ENV=testing VITE_API_URL=http://localhost:43030 npm run build
npm run preview -- --host 127.0.0.1 --port 43000 --strictPort
```

Gunakan `VITE_ENV=testing` saat **build**. Mode `DEVELOPMENT` di proyek ini memilih port 3030 secara otomatis. Jika alamat API berubah, ulangi build sebelum menjalankan preview lagi.

**Terminal 3 — cek di laptop**

```bash
curl -fsS http://localhost:43030/health
curl -fsS -o /dev/null -w '%{http_code}\n' http://localhost:43000/login
```

Hasil yang diharapkan adalah `{"status":"api is healthy"}` dan HTTP `200`. Gunakan jaringan tepercaya saat menjalankan backend. Konfigurasi server Go saat ini mendengarkan pada semua antarmuka laptop.

## 3. Hubungkan HP peserta

**Wi-Fi yang sama, Android 11 atau lebih baru.** Aktifkan Developer options dan Wireless debugging di HP. Pilih *Pair device with pairing code*. Ganti nilai contoh berikut dengan alamat dan port yang tertera di HP.

```bash
adb pair IP_HP:PORT_PAIRING
adb devices -l
```

Masukkan kode saat diminta. Jika HP belum muncul dengan status `device`, jalankan `adb connect IP_HP:PORT_KONEKSI`. Port koneksi ada di halaman utama Wireless debugging, sedangkan port pairing ada di dialog kode. Jika muncul `protocol fault`, gunakan Android SDK Platform Tools terbaru, buka dialog pairing baru, dan ulangi dengan port serta kode yang baru. [Panduan ADB Android](https://developer.android.com/tools/adb)

**Alternatif USB.** Aktifkan USB debugging, sambungkan kabel data, buka kunci HP, lalu izinkan komputer pada dialog Android. Jalankan `adb devices -l`. Cara ini juga cocok jika jaringan Wi-Fi memblokir koneksi antarperangkat.

Dari root proyek di terminal 3, gunakan kolom pertama keluaran `adb devices -l` sebagai SERIAL. Ulangi untuk setiap HP peserta.

```bash
bash user-testing/android.sh connect SERIAL
```

Di Chrome HP, buka **http://localhost:43030/health** lebih dahulu, lalu **http://localhost:43000/login**. Gunakan tab Chrome yang sama sampai login selesai supaya verifikasi sesi OAuth tetap tersedia. Jika halaman login muncul tetapi tombol Google gagal, cek health dari HP dan `adb -s SERIAL reverse --list`. Jika Google menampilkan `redirect_uri_mismatch`, cocokkan protokol, hostname, port, dan path dengan langkah 1.

## 4. Skenario peserta

| Tugas | Indikator berhasil |
| --- | --- |
| Login dengan akun Google peserta | Pengguna baru masuk Setup. Pengguna yang sudah menyelesaikan Setup dapat membuka Dashboard. |
| Pilih broker yang tersedia, isi informasi yang diminta, dan simpan | Saat akses Gmail diperlukan, Google meminta persetujuan. Setelah peserta menyetujui, aplikasi kembali ke Setup dan menunjukkan hasil sinkronisasi atau penjelasan jika data kosong. |
| Buka Review, periksa daftar, gunakan Back lalu selesaikan Review | Navigasi di HP mudah dijangkau dan peserta masuk halaman Broker. |
| Buka Dashboard, gunakan Refresh Data, lalu lihat History di Broker | Informasi harga dapat dibaca. Peserta dapat mencari transaksi dan berpindah halaman bila jumlah data cukup. |
| Buka Market, ubah tema, lalu Logout dari menu | Menu dapat dioperasikan dari HP dan Logout mengembalikan peserta ke Login. |

Catat **kode peserta, perangkat/browser, tugas, hasil aktual, berhasil/perlu bantuan/gagal, dan komentar peserta**. Gunakan kode peserta dalam catatan. Peserta sendiri yang memasukkan PIN dan memberikan izin Google. Untuk menguji tanpa memakai inbox pribadi, gunakan akun uji dengan contoh email broker. Hasil parsing bergantung pada isi akun tersebut.

## 5. Akhiri sesi

1. Logout di HP; jika peserta ingin menghentikan akses Gmail, cabut izin aplikasi melalui setelan koneksi pihak ketiga akun Google mereka. Logout aplikasi saja masih memungkinkan sinkronisasi terjadwal.
2. Dari root proyek, jalankan `bash user-testing/android.sh disconnect SERIAL` sebelum mencabut kabel atau mematikan Wireless debugging. Untuk Wi-Fi, putuskan juga koneksi dengan `adb disconnect IP_HP:PORT_KONEKSI`. Peserta dapat melupakan laptop di daftar perangkat yang dipasangkan.
3. Hentikan frontend dan backend dengan Ctrl+C di terminal masing-masing. Simpan atau hapus data uji sesuai kesepakatan dengan peserta.
