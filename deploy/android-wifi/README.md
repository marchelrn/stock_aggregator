# Android melalui Wi-Fi dan Nginx

Untuk Android 11 atau lebih baru, aktifkan **Developer options → Wireless debugging** pada HP dan gunakan jaringan Wi-Fi yang sama dengan komputer. Jalankan API lokal pada port 3030. Gunakan konfigurasi OAuth lokal proyek dengan frontend `http://localhost:3000` dan redirect Google terdaftar `http://localhost:3030/auth/google/callback`.

Dari root proyek, jalankan `bash deploy/android-wifi/start.sh`. Nginx menyajikan build frontend pada loopback 8088 dan meneruskan API melalui loopback 8089. Jalankan ulang skrip setelah mengubah frontend.

Pada HP pilih **Pair device with pairing code**. Di terminal komputer jalankan:

```sh
adb pair IP_HP:PORT_PAIRING
adb connect IP_HP:PORT_KONEKSI
adb devices -l
adb reverse tcp:3000 tcp:8088
adb reverse tcp:3030 tcp:8089
```

Masukkan kode pairing langsung saat ADB memintanya. Port koneksi ditampilkan pada layar utama Wireless debugging, sedangkan port pairing ada di dialog kode. Jika ada beberapa perangkat, gunakan `adb -s SERIAL reverse ...` untuk memilih HP yang benar.

Buka **http://localhost:3000** di Chrome Android. Jika koneksi ADB terputus, sambungkan kembali lalu ulangi kedua perintah reverse. Untuk menghentikan akses HP, matikan Wireless debugging. Untuk menghentikan Nginx proyek:

```sh
nginx -p "${XDG_CACHE_HOME:-$HOME/.cache}/stock-aggregator/android-nginx/" -c "$PWD/deploy/android-wifi/nginx.conf" -s quit
```
