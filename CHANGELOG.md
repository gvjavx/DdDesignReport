# Riwayat versi

## 0.4.2 · 30 Okt 2026

**Printer struk**: format `escpos` mencetak langsung ke printer thermal 58 dan 80 mm (ESC/POS): teks di kolom karakter sesuai kotaknya, rata dan dibungkus seperti di desain, tebal dan tinggi ganda (11 pt ke atas), garis menjadi `-`, QR dan Code 128 digambar printer, gambar sebagai titik, lalu kertas dipotong. Format `txt` adalah tata letak yang sama sebagai teks biasa, untuk pratinjau dan printer dot matrix. `"textColumns"` mengubah jumlah karakter per baris (default 32, 48, atau 80 menurut lebar kertas). Designer: ukuran kertas **Struk 58 mm** dan **Struk 80 mm**, **Karakter per baris** di tab Layout, dan ekspor TXT dan ESC/POS di aplikasi standalone.

**Excel**: chart ikut ke XLSX sebagai chart Excel sungguhan (batang, batang mendatar, garis, pie, donut), dengan datanya di dalam chart, sehingga bisa diubah di Excel. Gambar, barcode, dan chart yang berdampingan atau bertumpuk tetap berdampingan atau bertumpuk.

**JasperReports**:
- Ke `.jrxml`: `terbilang` menjadi closure Groovy; tabel dengan baris dari data dan sub-laporan menjadi `jr:list` atas daftarnya, dengan total di bawahnya yang menghitung item-itemnya dan ikut turun bila list memanjang; chart menjadi chart Jasper yang menjumlahkan nilai per label. Semuanya diuji di JasperReports 6.21.
- "Halaman X dari Y" benar di setiap halaman (sebelumnya nomor halaman kosong bila laporan tidak punya baris data), dan header serta footer halaman juga tercetak di halaman lanjutan band summary.
- Dari `.jrxml`: chart batang, garis, dan pie, list (menjadi sub-laporan), crosstab, perbandingan, `&& || !`, dan `kondisi ? a : b` ikut.
- `.jrxml` buatan ddreport kembali menjadi template yang sama persis (lewat properti `ddreport.*`): ketiga belas template siap pakai dicetak identik setelah bolak-balik. Sebelumnya setiap placeholder kembali sebagai Java dalam [kurung].

**Laporan besar**: PDF dan HTML digambar halaman demi halaman begitu tiap halaman selesai disusun. Memori puncak untuk 100 000 baris turun dari ±1,1 GB menjadi ±500 MB (PDF) dan dari ±1,4 GB menjadi ±400 MB (HTML), dengan hasil yang sama.

**Perbaikan**: tabel di bawah tabel data yang memanjang ikut turun di XLSX, XLS, dan CSV (sebelumnya baris tabel kehadiran di rapor terselip di antara baris nilai).

## 0.4.1 · 30 Okt 2026

**Template**: `"version"` (versi format template, sekarang 1). Designer menulisnya di setiap template. Ddreport menolak template yang versinya lebih baru dari yang bisa dibacanya, dengan pesan yang jelas, alih-alih mencetaknya tanpa bagian yang belum dikenalnya. Template tanpa `version` tetap dibaca seperti biasa. Dari Go: `ddreport.FormatVersion`.

**Server**:
- `GET /healthz` menjawab `ok` tanpa token, untuk load balancer dan pemantauan.
- Bila render yang menunggu dan berjalan sudah empat kali jumlah prosesor, render berikutnya langsung dijawab `503` dengan `Retry-After: 5`, bukan menunggu sampai batas waktu 5 menit.
- Satu baris log di stderr untuk setiap render, impor, dan akses My templates: metode, alamat, status, ukuran, dan lama (`POST /render?format=pdf 200 48213B 85ms`). Token dan isi laporan tidak dicatat.
- `DDREPORT_MAX_ROWS` (dari Go: `ddreport.MaxRows`): laporan dengan baris lebih dari itu ditolak dengan pesan error, alih-alih menghabiskan memori server.
- `ddreport -healthcheck 127.0.0.1:8080` keluar dengan kode 1 bila server tidak menjawab `/healthz`; image Docker memakainya sebagai HEALTHCHECK.

**Asisten AI**: `ddreport -mcp` adalah server MCP (stdio) dengan tool `template_guide` (`llms.txt` dan JSON Schema), `check_template` (properti yang salah ketik, lalu dicoba dicetak), dan `render_template` (ke file). `ddreport.Guide` berisi `llms.txt` dari Go.

**JasperReports**: semua 13 template siap pakai, diekspor ke `.jrxml`, kini dikompilasi, diisi data contohnya, dan diekspor ke PDF oleh JasperReports 6.21 sungguhan. Yang diperbaiki dari ekspornya:
- Garis di tepi bawah band tidak lagi keluar dari band (JasperReports menolak laporan seperti itu).
- Field bertipe `java.lang.String`, bukan `Object`: `JsonDataSource` memberi `Object` berupa node JSON yang tidak bisa dihitung.
- Bold dan italic font bawaan ikut ke PDF (`pdfFontName`).
- Sel tabel tidak lagi bergaris ganda.
- Baris diurutkan menurut field grup lebih dulu, karena JasperReports hanya menggabungkan baris yang berurutan.
- Kotak teks yang terlalu pendek untuk satu baris (menurut ukuran JasperReports, sekitar 1,3 kali ukuran font) dipertinggi seperlunya, agar isinya tidak hilang.

**Font**:
- Folder `ddreport-fonts` (hasil `node fontpack/download.mjs`) dipakai dengan sendirinya bila `DDREPORT_FONTS` tidak di-set, jadi 100 font itu langsung muncul di designer.
- Designer: tombol **⋯** di sebelah menu font memilih font mana yang tampil di menu, dengan centang, cari, dan Semua / Tidak ada. Pilihan disimpan di browser (`localStorage`).

**PDF/A**: PDF/A-2b dari `"pdfA": true`, juga yang ditandatangani dan yang berisi teks Arab, lulus validasi veraPDF 1.30.


## 0.4.0 · 28 Okt 2026

**Designer**
- **My templates** di aplikasi standalone: simpan, buka, perbarui, ganti nama, dan hapus template sendiri beserta data contohnya, di samping template siap pakai. `ddreport -serve` menyimpannya di folder `-templates` (default `ddreport-templates`, di Docker `/data`), satu file JSON per template, dengan 20 versi sebelumnya yang bisa dibuka lewat tombol **History**. Tanpa server, template disimpan di browser, dan dipindah ke server saat server tersedia.
- `theme: 'inherit'`: designer memakai warna, font, dan sudut dari variabel Bootstrap (`--bs-*`), termasuk dark mode Bootstrap. Variabel CSS baru `--r` (sudut) dan `--font` (font antarmuka).
- Toolbar sendiri: `hide: ['ribbon']`, `designer.run(perintah)`, `designer.commands()`, `designer.getState()`, dan opsi `onState`. Contoh halaman Bootstrap lengkap: `designer/bootstrap.html`, juga di `/bootstrap.html` pada `ddreport -serve`.
- Insert > **Crosstab**, centang **PDF/A** di tab Layout, dan band grup sampai tingkat enam.
- Aplikasi standalone: semua font di `DDREPORT_FONTS` muncul di menu font dan tergambar di kanvas; **Open…** menerima `.jrxml`, dan **Export > JRXML**.

**Mesin cetak**
- Elemen **crosstab** (tabel silang / pivot): satu baris per nilai `rowField`, satu kolom per nilai `columnField`, setiap sel `measure` (`sum(total):0`, `count()`, `avg(x):2`, ...), lalu total baris, kolom, dan keseluruhan. Bekerja di semua format, per grup, dan dari daftar di data.
- Grouping sampai enam tingkat (sebelumnya tiga): `groupHeader4`–`groupHeader6` dan `groupFooter4`–`groupFooter6`, juga di schema dan sub-laporan.
- Arab, Ibrani, dan Thai di PDF: huruf Arab disambung (termasuk lam-alif dan huruf Persia), teks kanan-ke-kiri disusun dari kanan dengan angka dan kata Latin tetap kiri-ke-kanan, dan HTML diberi `dir="auto"`.
- Laporan besar: benchmark `go test -bench Large` (10 000 dan 100 000 baris, semua format, hasilnya di README). Memori sekitar 30% lebih kecil untuk PDF dan 40% untuk HTML, teks tanpa placeholder tidak lagi diproses, dan laporan tanpa `{{pages}}` atau band "halaman terakhir" disusun sekali, bukan dua kali.

**Excel dan JasperReports**
- XLSX dan XLS membawa sel yang digabung ke bawah (`rowSpan`), juga bersama gabungan ke kanan. Sel tabel setinggi beberapa baris tidak lagi menarik baris di sebelahnya ke satu baris.
- XLSX membawa gambar dan barcode/QR sebagai gambar, di baris dan kolom yang sejajar dengannya; baris itu setinggi gambarnya. Gambar dari data (`{{photo}}`) ikut per baris.
- Ekspor ke `.jrxml` JasperReports 6 (`ddreport -format jrxml`, `ToJRXML`, format `"jrxml"` di semua wrapper): teks, tabel, gambar, kotak, garis, barcode/QR, grup, agregat, nomor halaman, serta format angka dan tanggal, dengan ekspresi Groovy. Yang tidak punya padanan dicatat di awal file.
- Impor `.jrxml` menjadi template (`ddreport -from-jrxml`, `FromJRXML`, `POST /import/jrxml`), dengan catatan tentang apa yang tidak ikut.

**PDF**
- PDF/A-2b: `"pdfA": true` di template. Semua font disematkan (Helvetica, Times, dan Courier dicetak dengan Roboto dan Lora), warna sRGB, metadata XMP, dan ID dokumen.
- Tanda tangan digital: `DDREPORT_SIGN_CERT` / `DDREPORT_SIGN_KEY` (PEM, RSA atau ECDSA) dan `DDREPORT_SIGN_REASON` menandatangani setiap PDF, juga dari `-serve`, Docker, Java, dan Node.js; `ddreport.SignPDF` dari Go. Tanda tangan tidak terlihat, `adbe.pkcs7.detached` SHA-256.

**Font**
- `node fontpack/download.mjs`: paket 100 font tambahan dari Google Fonts (lisensi OFL atau Apache) untuk `DDREPORT_FONTS`, termasuk Amiri dan Noto Naskh Arabic untuk Arab dan Sarabun untuk Thai.
- Font dari `AddFonts` dan `DDREPORT_FONTS` baru dibaca saat laporan memakainya; dengan paket 100 font, satu render lewat binary sekitar dua kali lebih cepat. File yang tidak bisa dibaca tetap menghentikan start dengan pesan error. `Family.Faces()` menyebut face yang ada tanpa membaca file; kode yang membaca `Families[x].Regular` langsung mendapat `nil` sampai font itu dipakai.
- `/fonts/` pada `-serve` juga menyajikan font dari folder `fonts/` di dalam `DDREPORT_FONTS`, dan `GET /fonts.json` mendaftar semua font.

**Server dan keamanan**
- Tanpa `DDREPORT_TOKEN`, `ddreport -serve` hanya melayani `localhost` dan alamat IP: situs yang mengarahkan namanya ke mesin Anda (DNS rebinding) tidak bisa memakai server itu atau My templates. Untuk nama domain, isi `DDREPORT_TOKEN`.
- Endpoint baru: `GET /mine`, `PUT` / `DELETE /mine/{id}`, `GET /mine/{id}/history[/{versi}]`, `POST /import/jrxml`, `GET /fonts.json`. Semuanya kecuali `/fonts.json` memakai token yang sama dengan `/render`.
- Flag baru: `-templates folder` dan `-from-jrxml`.

**Template**
- Galeri template komunitas: template baru lewat pull request, dengan `author` dan `url` di `designer/templates/index.json`; test memeriksa setiap template di semua format. Galeri website menampilkan pembuatnya dan cara mengirim template.
- Field template baru di JSON Schema: `pdfA`, band grup tingkat 4–6, dan elemen `crosstab` (`rowField`, `columnField`, `measure`, `totalLabel`).

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
