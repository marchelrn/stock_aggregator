# Uji Safari iPhone dan Chrome Android melalui satu tautan

Fasilitator menjalankan frontend, API, dan database uji di laptop. Peserta cukup membuka satu tautan HTTPS di browser. Pilih cara akses sebelum mengatur Google OAuth.

- **Tautan sementara untuk peserta tanpa instalasi.** Gunakan dev domain milik akun ngrok. Saat tunnel hidup, alamat dapat dijangkau melalui internet. Gunakan database pengujian dan matikan tunnel setelah sesi. Domain akun dapat dipakai kembali pada sesi berikutnya. [Dokumentasi ngrok](https://ngrok.com/docs/gateway/endpoints/http)
- **Akses privat.** Gunakan Tailscale Serve jika peserta bersedia memasang Tailscale dan bergabung ke jaringan uji. Akses mengikuti aturan jaringan tersebut. Gunakan hostname HTTPS perangkat yang ditampilkan oleh Serve. [Dokumentasi Tailscale Serve](https://tailscale.com/docs/features/tailscale-serve)

## Siapkan laptop

1. Siapkan PostgreSQL, database uji, dan kredensial Google sesuai bagian 1 di [panduan utama](README.md). Instal Nginx dan alat akses yang dipilih pada laptop. Untuk ngrok, autentikasi agen dengan akun fasilitator sesuai [petunjuk instalasi](https://ngrok.com/docs/gateway/endpoints/agent-cli-quickstart).
2. Tentukan alamat HTTPS yang akan dipakai peserta. Contoh di bawah memakai `https://DOMAIN-UJI`; ganti seluruh contoh dengan domain sebenarnya. Pada OAuth client Web application di Google Cloud, tambahkan **Authorized redirect URI** berikut secara persis. Gunakan akun peserta yang terdaftar sebagai test user jika aplikasi Google masih dalam mode Testing. [Panduan Google OAuth](https://developers.google.com/identity/protocols/oauth2/web-server)

   ```text
   https://DOMAIN-UJI/auth/google/callback
   ```

3. Di `api/.env`, isi nilai berikut selain konfigurasi database dan kredensial. Lalu jalankan backend dari root worktree testing dalam terminal pertama.

   ```dotenv
   ENV=testing
   PORT=3030
   FRONTEND_URL=https://DOMAIN-UJI
   GOOGLE_REDIRECT_URL=https://DOMAIN-UJI/auth/google/callback
   ```

   ```bash
   cd ~/Documents/Skripsi/stock-aggregator-testing/api
   go run .
   ```

4. Pada terminal kedua, mulai dari root proyek. Build dengan alamat API relatif agar Safari dan Chrome memakai domain yang sedang dibuka.

   ```bash
   cd ~/Documents/Skripsi/stock-aggregator-testing/frontend
   npm ci
   env VITE_ENV=testing VITE_API_URL=/backend npm run build
   npm run preview -- --host 127.0.0.1 --port 43000 --strictPort
   ```

5. Pada terminal ketiga di root proyek, jalankan Nginx sebagai pengguna biasa. Biarkan terminal ini terbuka. Nginx meneruskan `/backend/` dan callback Google `/auth/google/callback` ke API pada port 3030 dan halaman seperti `/login` dan `/auth/callback` ke frontend.

   **Fish**

   ```fish
   cd ~/Documents/Skripsi/stock-aggregator-testing
   set uat_nginx_dir (mktemp -d)
   nginx -p "$uat_nginx_dir/" -c "$PWD/user-testing/mobile-nginx.conf" -t
   and nginx -p "$uat_nginx_dir/" -c "$PWD/user-testing/mobile-nginx.conf" -g 'daemon off;'
   ```

   **Bash**

   ```bash
   cd ~/Documents/Skripsi/stock-aggregator-testing
   uat_nginx_dir=$(mktemp -d)
   nginx -p "$uat_nginx_dir/" -c "$PWD/user-testing/mobile-nginx.conf" -t
   nginx -p "$uat_nginx_dir/" -c "$PWD/user-testing/mobile-nginx.conf" -g 'daemon off;'
   ```

6. Di terminal keempat, cek kedua layanan melalui Nginx.

   ```bash
   curl -f http://127.0.0.1:43080/backend/health
   curl -fsS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:43080/login
   ```

   Hasil yang diharapkan adalah JSON status API dan kode 200. Jika login menampilkan halaman tetapi permintaan mengarah ke localhost HP, ulangi build pada langkah 4.

## Buka akses untuk sesi

Pilih salah satu perintah berikut di laptop. Untuk tautan ngrok, pastikan domain HTTPS yang ditampilkan cocok dengan konfigurasi Google dan backend.

```bash
ngrok http 127.0.0.1:43080 --inspect=false
```

Untuk sesi privat, aktifkan HTTPS pada tailnet dan jalankan:

```bash
tailscale serve 43080
```

Peserta membuka **https://DOMAIN-UJI/backend/health** sekali untuk memeriksa akses, lalu **https://DOMAIN-UJI/login** di Safari atau Chrome. Pada ngrok gratis, browser dapat menampilkan halaman konfirmasi sebelum masuk. Lanjutkan login dan izin Gmail di tab yang sama. Jangan beralih dari domain uji ke IP laptop di tengah alur login. Lanjutkan dengan skenario peserta di [panduan utama](README.md).

Setelah sesi, logout di HP, hentikan ngrok atau Tailscale Serve dengan Ctrl+C, lalu hentikan Nginx, frontend, dan backend. Ikuti langkah pencabutan akses Gmail di panduan utama jika peserta ingin menghentikan sinkronisasi.
