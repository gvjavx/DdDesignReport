# Riwayat versi

## 0.3.0 · 26 Okt 2026

**Server dan Docker**
- `DDREPORT_TOKEN`: `ddreport -serve` meminta `Authorization: Bearer <token>` untuk setiap render. Designer standalone memakai token dari alamatnya (`/#token=...`).
- `ddreport -serve` memberi peringatan bila alamatnya terbuka ke jaringan tanpa token.
- Image Docker `ghcr.io/gvjavx/ddreport` (amd64 dan arm64, sekitar 17 MB) di setiap rilis: render server untuk PHP, Python, .NET, atau bahasa apa pun lewat HTTP. README berisi contoh PHP, Laravel, Python, dan C#.

**Designer**
- **colour from data** dan **fill from data** di panel properti: warna yang dihitung per baris, misalnya `if(total < 0, '#dc2626', '#000000')`. Untuk tabel, warna teks isinya. Disembunyikan di mode mudah.

**Template**
- JSON Schema template: `designer/template.schema.json`, juga di paket npm (`ddreport-designer/template.schema.json`) dan di `/template.schema.json` pada `ddreport -serve`. Test memastikan schema selalu sama dengan mesin cetak dan menerima semua template siap pakai.
- `llms.txt`: cara menulis template untuk asisten AI, dengan contoh yang dicek oleh test.

**Mesin cetak**
- Warna sesuai data: `color`, `fill`, warna tabel, dan warna `cellStyle` boleh berupa rumus, misalnya `{{if(total < 0, '#dc2626', '#000000')}}`, dihitung per baris atau per item, juga di XLSX dan XLS.
- Template dari PHP (`json_decode($json, true)` lalu `json_encode`) yang menulis objek kosong sebagai `[]` tidak lagi ditolak.

## 0.2.1 · 22 Okt 2026

**Keamanan**: binary `ddreport` sekarang dibangun dengan Go 1.26.10. Binary 0.2.0 dibangun dengan Go 1.25.0 dan membawa 59 celah pustaka standar Go yang sudah diketahui (antara lain di `net/http`, `crypto/tls`, `crypto/x509`, dan `html/template`); yang paling berisiko adalah mode `-serve`. Perbarui ke 0.2.1, terutama bila memakai `-serve`. Tidak ada perubahan perilaku.

## 0.2.0 · 22 Okt 2026

**Designer**
- **Mode mudah** (`simple: true`): hanya alat yang dibutuhkan laporan pertama, setiap tombol dengan namanya, band grup baru muncul setelah Group by diisi. Aplikasi standalone memakainya secara default, dengan tombol untuk beralih ke mode lengkap.
- Halaman kosong punya tombol **Kotak teks**, **Tabel**, **Gambar**, dan **Chart** untuk langsung mulai.
- `hide`: sembunyikan tab, grup ribbon, perintah, elemen, atau properti per nama, misalnya `['Layout', 'chart', 'showIf']`. Perintah yang disembunyikan juga tidak jalan lewat pintasan keyboard.
- `messages`: ganti teks apa pun di antarmuka dengan kata-kata sendiri (atau bahasa lain).
- `destroy()`: melepas designer dari halaman, untuk React, Vue, dan aplikasi satu halaman lainnya.
- Tipe TypeScript (`designer.d.ts`), dan template siap pakai ikut di paket npm: `import invoice from 'ddreport-designer/templates/invoice.json'`.
- Properti posisi dan ukuran bernama *dari kiri*, *dari atas*, *lebar*, *tinggi* (bukan x, y, w, h); jenis elemen ikut diterjemahkan.
- Di layar HP, bilah atas aplikasi standalone turun ke baris berikutnya, tidak lagi terpotong.
- QR code dan Code 128 di kanvas adalah kode yang sama persis dengan hasil cetak, jadi bisa langsung dipindai; `{{field}}` di dalamnya diisi dari data contoh.
- Chart di kanvas punya skala nilai, label, dan format angka seperti hasil cetak, termasuk "Lainnya" untuk irisan pie yang tidak muat.
- Menu Field berisi `{{index}}` dan `{{terbilang(field)}}`.

**Template baru**: Kuitansi, Surat jalan, Rapor siswa, Daftar hadir, dan Laporan laba rugi (total 13).

**Mesin cetak**
- `{{terbilang(total)}}`: angka dalam kata bahasa Indonesia, misalnya untuk kuitansi. Template Kuitansi memakainya.
- `{{index}}` di band `detail`: nomor urut baris, mulai dari 1 lagi di setiap grup.

**Excel**: garis sel tabel (`grid` `all`, `rows`, dan `outer`), sel yang digabung ke kanan, dan teks rata tengah atau kanan ikut ke XLSX dan XLS. Dicoba dengan membukanya di Microsoft Excel.

**Java, Kotlin, dan Node.js**: `DdReport.Renderer` dan `createRenderer()` menjaga proses `ddreport` tetap berjalan untuk laporan demi laporan, sekitar lima kali lebih cepat per laporan kecil. Binary-nya mendapat mode `-stdio` untuk itu.

**Perbaikan**
- Pie dan donut tidak lagi mencantumkan irisan bernilai nol di legendanya.
- Gambar rusak di template langsung dilaporkan dengan pesan yang jelas di semua format, bukan `pdf: not a PNG buffer`.
- Designer: elemen rusak dari clipboard tidak lagi membuat designer berhenti bekerja; editor sub-laporan tidak lagi meninggalkan pendengar clipboard setiap kali ditutup.
- Designer di layar sentuh: mencubit untuk zoom di area kosong halaman tidak lagi membuang (atau mengganti) elemen yang dipilih.

**Pengembangan**: `node designer/run-tests.mjs` juga mencoba designer di layar sentuh HP dengan sentuhan sungguhan dari browser: ketuk di Chromium dan WebKit, serta geser, ubah ukuran, dan cubit di Chromium.

**Keamanan**
- `ddreport -serve` menolak `POST /render` dari halaman situs lain yang terbuka di browser yang sama (header `Origin` yang berbeda). Program yang memanggilnya langsung tidak terpengaruh.
- Designer hanya menampilkan gambar yang tertanam di template (data URI). Template dari orang lain tidak bisa membuat browser memuat alamat luar lewat latar halaman, gambar, atau warna isi.

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
