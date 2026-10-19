# Riwayat versi

## 0.1.3 · 20 Okt 2026

- Semua paket npm terbit lewat *trusted publishing*: CI memakai identitas GitHub Actions, tanpa token npm.
- README dan website: pasang dari npm dan Maven Central, binary dari GitHub Release.

## 0.1.2 · 20 Okt 2026

- `ddreport` dan `ddreport-designer` pertama kali terbit di npm.
- Binary per sistem untuk npm sekarang bernama `@ddreport/<sistem>` (misalnya `@ddreport/win32-x64`), termasuk Windows. npm menolak nama `ddreport-win32-*`.

## 0.1.1 · 20 Okt 2026

- Pertama kali terbit di Maven Central: `io.github.gvjavx:ddreport`, dengan jar binary per sistem (classifier `linux-x64`, `windows-x64`, `darwin-arm64`, ...).

## 0.1.0 · 19 Okt 2026

Rilis pertama: binary `ddreport` untuk Windows, Linux, dan macOS (x64 dan arm64) di GitHub Release. Dibanding versi awal (14 Okt):

**Designer**
- Tampilan baru: ribbon ikon, tema terang/gelap/otomatis, warna aksen dari CSS, dan tata letak yang menyesuaikan layar sempit.
- Zoom kanvas (Ctrl+roda, cubit, Ctrl +/-/0), halaman selalu di tengah, pegangan untuk mengubah ukuran elemen (juga dengan sentuhan).
- Hapus elemen dengan tombol di sudutnya atau dengan menyeretnya ke tempat sampah.
- Preview menampilkan halaman sebagai lembar, jumlah halaman, serta status memuat dan error.
- Menu field dari data, clipboard sistem, dan satu langkah undo per field yang diketik.

**Tabel**
- Tabel data: baris di antara header dan total berulang untuk setiap item dari daftar di data, dengan `{{index}}`.
- Gabung sel (ke kanan dan ke bawah), warna latar, warna teks, rata, bold, dan italic per sel.
- Header, total, baris belang, garis tebal (`headerLine`, `footerLine`, `frame`).
- Tabel panjang berlanjut ke halaman berikutnya tanpa memotong baris, dengan header yang diulang.
- Di designer: seret lebar kolom dan tinggi baris, tempel sel dari spreadsheet, Tab untuk pindah sel.

**Kelompok dan sub-laporan**
- Pengelompokan sampai tiga tingkat, masing-masing dengan subtotal; header kelompok diulang di halaman baru.
- Sub-laporan: laporan lengkap (header, kelompok, detail, ringkasan) untuk setiap item dari sebuah daftar.
- Satu worksheet XLSX/XLS per kelompok (`sheetPerGroup`).

**Mesin cetak**
- Ekspresi baru: perbandingan, `and`/`or`/`not`, `if`, `upper`, `lower`, `round`, format tanggal (`{{date:dd MMMM yyyy}}`), `{{now}}`, nama bulan bahasa Indonesia.
- `showIf`, gambar dari data, `sortBy`, band halaman pertama dan terakhir, `newPage`, rata vertikal, justify, garis diagonal.
- Teks yang lebih panjang dari satu halaman berlanjut ke halaman berikutnya; footer yang tumbuh tetap di dalam margin.
- Angka yang dikirim sebagai teks ikut dihitung, dan hasil hitungan bebas sisa biner (0.1 + 0.2 = 0.3).
- Font TrueType bawaan, serta font sendiri lewat `AddFonts` atau `DDREPORT_FONTS`.
- Chart: skala nilai dengan angka negatif, label yang tidak bertumpuk, legenda pie yang selalu muat. Di designer, chart dan barcode digambar dari data contoh.
- HTML memakai pemenggalan baris yang sama dengan PDF; CSV diawali BOM UTF-8 dan separatornya bisa diatur.
- XLSX/XLS membawa warna latar dan warna teks sel.

**Backend**
- Node.js: `renderAsync`, yang tidak memblokir event loop.
- Java dan Node.js berhenti setelah satu menit secara default (bisa diubah) dan memberi tahu bahwa batas waktunya habis.
- `ddreport -serve` mencetak paling banyak satu laporan per CPU sekaligus.

**Pengujian**
- CI menjalankan test Go, Node.js, dan Java di Linux dan Windows, serta test designer di Chromium, Firefox, dan WebKit.
