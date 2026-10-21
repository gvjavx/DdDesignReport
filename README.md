<img src="logo.svg" alt="" width="72">

# DdDesignReport

Library reporting dengan designer drag and drop. Template didesain di browser, disimpan sebagai JSON, lalu di-render oleh satu core (Go) menjadi **PDF, HTML, XLSX, XLS, atau CSV** dari backend Go, Java, Kotlin, atau Node.js.

Yang baru di setiap versi: [CHANGELOG.md](CHANGELOG.md).

```
designer/   designer drag and drop (vanilla JS, tanpa dependency) + aplikasi standalone
ddreport.go core renderer (library Go)
fonts/      font bundel (TrueType, SIL OFL), disematkan ke binary
cmd/        binary `ddreport`, dipakai oleh wrapper di bawah dan untuk menjalankan designer
java/       wrapper Java / Kotlin
node/       wrapper Node.js
```

## Designer

Sebagai aplikasi terpisah, lengkap dengan preview. Unduh binary untuk sistem Anda dari [GitHub Release](https://github.com/gvjavx/DdDesignReport/releases/latest) (`ddreport-linux-amd64`, `ddreport-darwin-arm64`, `ddreport-windows-amd64.exe`, ...) dan ganti namanya menjadi `ddreport`, atau build sendiri:

```sh
go build -o ddreport ./cmd/ddreport
./ddreport -serve 127.0.0.1:8080      # buka http://127.0.0.1:8080
```

Sebagai library di frontend apa pun (`npm install ddreport-designer`):

```js
import { createDesigner } from 'ddreport-designer'

const designer = createDesigner(document.getElementById('designer'), {
  template: savedTemplate,            // opsional
  onChange: t => save(t),             // dipanggil setiap ada perubahan
  lang: 'id',                         // opsional: antarmuka Indonesia (default 'en')
  theme: 'auto',                      // opsional: 'light' (default), 'dark', atau 'auto'
  fontBase: '/fonts/',                // opsional: URL font bawaan (`ddreport -serve` menyajikannya di /fonts/)
  fonts: ['Inter'],                   // opsional: font tambahan yang didaftarkan di backend
  fields: ['name', 'qty', 'price'],   // opsional: nama field data untuk menu Field; ubah dengan designer.setFields()
  data: sampleData,                   // opsional: data contoh; chart di kanvas digambar darinya; ubah dengan designer.setData()
  preview: async t => {               // opsional, memunculkan tombol Preview
    const res = await fetch('/api/report/preview', { method: 'POST', body: JSON.stringify(t) })
    return res.text()                 // HTML hasil render backend (format "html")
  },
})
designer.getTemplate()                // JSON untuk dikirim ke backend
designer.destroy()                    // saat designer dilepas dari halaman (React, Vue, ...)
```

Paketnya membawa tipe TypeScript dan 13 template siap pakai: `import invoice from 'ddreport-designer/templates/invoice.json'`, lalu `designer.setTemplate(invoice.template)`. Contoh untuk React dan Vue ada di [README paket npm-nya](release/designer-README.md).

**Menyesuaikan designer untuk project Anda**:

| Opsi | Arti |
| --- | --- |
| `simple: true` | Mode mudah untuk pemula: alat yang jarang dipakai disembunyikan, setiap tombol diberi nama, band grup baru muncul setelah Group by diisi |
| `hide: ['Layout', 'chart', 'showIf']` | Sembunyikan per nama: tab (`Layout`), grup ribbon (`Arrange`, atau `Layout.Data` untuk grup di satu tab saja), perintah (`delete`, `preview`), elemen (`chart`, `subreport`), atau pengaturan (`sortBy`, `showIf`). Perintah yang disembunyikan juga tidak jalan lewat pintasan |
| `messages: { 'Text box': 'Label' }` | Ganti teks antarmuka mana pun, dengan teks Inggrisnya sebagai kunci (daftar lengkapnya `ID` di `designer.js`) |
| CSS `#designer.ddr { --a: #e11d48 }` | Warna: `--a` (aksen), `--a-soft`, `--bg`, `--surface`, `--text`, `--muted`, `--line`, `--stage`, `--hover` |

Designer punya ribbon seperti pengolah kata:

| Tab | Isi |
| --- | --- |
| **Home** | Undo/redo, cut/copy/paste/duplicate, font, ukuran, bold/italic/underline, warna teks, highlight, rata kiri/tengah/kanan/kanan-kiri, rata atas/tengah/bawah, spasi baris, wrap, perataan dan distribusi elemen, urutan depan/belakang |
| **Insert** | Text box, tabel (baris × kolom), gambar, kotak, garis, chart, barcode/QR, field data (nama field diambil dari data contoh) |
| **Layout** | Ukuran kertas (A4, A5, Letter, Legal, F4), orientasi, margin per sisi, warna dan gambar latar halaman, urutan data, grouping, pemisah angka, bahasa tanggal, pemisah CSV |
| **View** | Zoom, penggaris, snap, preview |

Chart di kanvas digambar dari data contoh (opsi `data`) dengan skala nilai, label, dan format angka seperti hasil cetak. Barcode di kanvas adalah kode yang sama persis dengan hasil cetak (QR dan Code 128, modul demi modul), jadi bisa langsung dipindai; `{{field}}` di dalamnya diisi dari data contoh. Tampilannya menyesuaikan lebar tempat designer dipasang: di layar sempit panel properti pindah ke bawah dan halaman dipaskan ke lebar. Zoom ada di bilah status bawah, dan bisa langsung di kanvas: `Ctrl` + scroll (atau pinch di trackpad) memperbesar di titik kursor, cubit dua jari di layar sentuh, serta `Ctrl` + `=` / `-` / `0`.

Klik tombol di tab Insert atau seret ke band. Klik ganda teks atau sel tabel untuk mengetik langsung di halaman; pilih sebagian teks lalu tekan **B**/**I**/**U** untuk memformat bagian itu saja. Shift-klik, atau seret kotak di area kosong, untuk memilih banyak elemen. Geser elemen untuk memindah (termasuk ke band lain), tarik pegangan biru di sudut kanan bawah untuk mengubah ukuran (juga dengan sentuhan), tarik tepi bawah band untuk mengubah tingginya. Untuk menghapus: klik tombol merah × di sudut elemen terpilih, seret elemen ke tempat sampah yang muncul saat menggeser, atau tekan `Delete`.

Pintasan: `Ctrl+Z` / `Ctrl+Y` (mengetik di satu kolom properti dihitung satu langkah undo), `Ctrl+C` / `X` / `V` / `D`, `Ctrl+A`, `Ctrl+B` / `I` / `U`, `Ctrl+=` / `Ctrl+-` / `Ctrl+0` (zoom), tombol panah (dengan `Shift` = 10pt), `Delete`. Tahan `Alt` saat menggeser untuk mematikan snap.

`Ctrl+C` / `Ctrl+X` menaruh elemen terpilih di clipboard sistem, jadi bisa ditempel di designer lain (tab atau laporan lain). Menempel teks biasa membuat text box baru. Untuk tabel, klik sebuah sel lalu pakai tombol di panel properti untuk menyisipkan atau menghapus baris dan kolom di sebelahnya. Klik kanvas kosong untuk mengatur band: tinggi, kepala/kaki halaman hanya di halaman pertama atau terakhir, dan band yang dimulai di halaman baru.

Aplikasi standalone juga punya tombol ekspor PDF, HTML, XLSX, XLS, dan CSV, antarmuka Indonesia lewat `?lang=id`, dan **Mode mudah** yang aktif sejak awal (hapus centangnya untuk semua alat). `Esc` saat mengetik membatalkan; **New** dan **Templates…** bisa dibatalkan dengan `Ctrl+Z`.

**Preview** (tombol mata di bilah status, atau tab View) membuka hasil cetak HTML dengan data contoh: tiap halaman tampil sebagai lembar terpisah, dipaskan ke lebar jendela, dengan jumlah halamannya. Preview dan ekspor butuh halaman dibuka dari `ddreport -serve`; jika tidak, designer mengatakannya.

## Template siap pakai

Tiga belas template ada di `designer/templates/` (faktur, laporan penjualan, struk kasir, slip gaji, sertifikat, surat resmi, daftar stok, label produk, kuitansi, surat jalan, rapor siswa, daftar hadir, laporan laba rugi). Tiap file berisi `template` dan `data` contohnya. Di aplikasi standalone pilih lewat **Templates…**; `go test` mencetak semuanya ke setiap format.

## Template

| Band | Muncul |
| --- | --- |
| `header`, `columnHeader`, `footer` | di setiap halaman; `"pages": "first"` hanya di halaman pertama, `"last"` (footer) hanya di halaman terakhir |
| `groupHeader`, `groupFooter` | sebelum dan sesudah setiap grup; tanpa `groupBy`, sekali di sekitar semua baris. Judul grup diulang di atas halaman tempat grupnya berlanjut |
| `detail` | untuk setiap baris data |
| `summary` | sekali, setelah baris terakhir |

`"newPage": true` pada `groupHeader`, `detail`, `groupFooter`, atau `summary` memulai band itu di halaman baru. Band yang lebih tinggi dari satu halaman (misalnya surat panjang dengan Wrap) berlanjut ke halaman berikutnya, baris demi baris; tidak ada teks yang terpotong. Footer yang ikut memanjang tetap di dalam margin bawah.

**Placeholder** di dalam teks:

| Placeholder | Arti |
| --- | --- |
| `{{field}}`, `{{customer.name}}` | field dari baris, lalu dari data utama |
| `{{page}}`, `{{pages}}` | nomor dan jumlah halaman |
| `{{sum(f)}}`, `{{avg(f)}}`, `{{min(f)}}`, `{{max(f)}}`, `{{count()}}` | agregat: per grup di band grup dan detail, seluruh baris di band lain |
| `{{qty * price}}`, `{{sum(qty * price)}}`, `{{(price - disc) / 1.11}}` | perhitungan dengan `+ - * /` dan kurung, juga di dalam agregat |
| `{{unit-price}}`, `{{[unit-price] * qty}}` | nama field yang bukan satu kata: langsung jika berdiri sendiri, dalam kurung siku di dalam perhitungan |
| `{{price:2}}`, `{{sum(price):0}}` | angka dengan N desimal dan pemisah ribuan (`thousands` / `decimal` di template, default `,` dan `.`; `"thousands": "none"` tanpa pemisah ribuan) |
| `{{date:dd MMMM yyyy}}`, `{{now:dd/MM/yyyy HH:mm}}` | tanggal dengan pola `yyyy yy MMMM MMM MM M dd d EEEE EEE HH H mm ss`, teks dalam `'kutip'`. `{{now}}` adalah waktu laporan dibuat. Nama bulan dan hari Indonesia dengan `"locale": "id"` |
| `{{if(paid, 'LUNAS', 'BELUM')}}`, `{{upper(name)}}`, `{{round(x, 2)}}` | fungsi `if`, `upper`, `lower`, `round`; perbandingan `= != < > <= >=`, `and`, `or`, `not`, teks dalam `'kutip'` |
| `{{terbilang(total)}}`, `{{upper(terbilang(total))}} RUPIAH` | angka dalam kata bahasa Indonesia: 2750000 menjadi "dua juta tujuh ratus lima puluh ribu" (desimal dibulatkan) |
| `{{index}}` | nomor urut baris di band `detail`, mulai dari 1 lagi di setiap grup; di tabel data dan sub-laporan, nomor item |

Angka yang dikirim sebagai teks (`"18000"`, seperti dari BigDecimal atau kolom numeric database) dihitung sebagai angka di rumus, agregat, format `:N`, dan chart. Hasil hitungan tanpa `:N` tidak menampilkan sisa pembulatan (`0.1 + 0.2` menjadi `0.3`).

**Urutan**: `sortBy` mengurutkan baris, misalnya `"kategori, -total"` (`-` untuk menurun). Angka diurutkan sebagai angka.

**Grouping**: isi `groupBy` dengan nama field. Baris dengan nilai yang sama dikumpulkan menjadi satu grup, urut sesuai kemunculan pertama (atau sesuai `sortBy`). Sampai tiga tingkat: `"region, city, store"`; tingkat kedua memakai band `groupHeader2` / `groupFooter2`, tingkat ketiga `groupHeader3` / `groupFooter3`, dan agregat di tiap band menghitung grup tingkatnya. Judul semua grup yang sedang berjalan diulang di atas halaman baru. Di designer, band tingkat 2 dan 3 muncul begitu Group by berisi dua atau tiga field.

**Sheet per grup**: `"sheetPerGroup": true` membuat XLSX dan XLS berisi satu sheet untuk setiap grup tingkat pertama, dinamai sesuai grupnya (dengan aturan nama sheet Excel: maksimal 31 huruf, tanpa `[]:*?/\`, tidak kembar). Tiap sheet berisi judul kolom, baris, dan totalnya sendiri. `groupPageBreak` memulai setiap grup di halaman baru.

**Format teks**: `font` (lihat [Font](#font)), `fontSize`, `lineHeight`, `bold`, `italic`, `underline`, `color`, `fill` (highlight), `align` (`left`, `center`, `right`, `justify`), `valign` (`top`, `middle`, `bottom`). Sebagian teks bisa diformat dengan tag `<b>`, `<i>`, `<u>`: `Total <b>{{sum(price):2}}</b>`.

**Tabel**: elemen `table` dengan `cells` (teks per sel, boleh berisi placeholder), `cols` (lebar relatif kolom), `aligns` (rata per kolom), `border`, dan `fill`. Tabel satu baris di band `detail` menjadi baris data berbingkai.

Gaya tabel, semuanya bisa diatur di panel properti:

| Properti | Arti |
| --- | --- |
| `header`, `headerFill`, `headerColor` | baris pertama menjadi judul: tebal, dengan warna latar dan teks sendiri |
| `footer`, `footerFill`, `footerColor` | baris terakhir menjadi baris total (footer). Di designer, mencentang *total row* menambah satu baris di bawah |
| `stripe` | warna latar baris isi selang-seling |
| `textColor` | warna teks isi; `color` adalah warna garis |
| `grid` | garis yang digambar: `all` (default), `outer` (bingkai luar saja), `rows` (garis antar-baris saja) |
| `heights` | tinggi relatif baris, seperti `cols`: `[2,1,1]` membuat baris pertama dua kali lebih tinggi |
| `padding` | jarak teks dari tepi sel dalam pt (default 3 mendatar, 2 tegak) |
| `cellStyle` | gaya per sel, dengan kunci `"baris,kolom"` dari 0: `fill`, `color`, `align`, `bold`, `italic`, serta `colSpan` / `rowSpan` untuk menggabung sel |
| `data` | nama daftar di data, misalnya `items`: baris di antara judul dan baris total diulang untuk setiap item (lihat di bawah) |
| `headerLine`, `footerLine`, `frame` | garis lebih tebal (pt) di bawah baris judul, di atas baris total, dan di sekeliling tabel; garis judul ikut diulang di halaman berikutnya, bingkai tertutup di setiap halaman |

**Tabel data** (`data`): satu tabel untuk daftar di dalam data, misalnya item sebuah pesanan. Baris pertama (`header`) dan terakhir (`footer`) dicetak sekali; baris di antaranya diulang per item. Di baris yang diulang, `{{field}}` adalah field item itu dan `{{index}}` nomor urutnya; di judul dan baris total, agregat menghitung semua item (`{{sum(qty * price):0}}`). Taruh di band `detail` untuk daftar per baris data (pesanan dengan itemnya), atau di `summary`/`header` untuk daftar di data utama.

```json
{ "type": "table", "data": "items", "header": true, "footer": true, "border": 1, "grow": true,
  "cells": [["No", "Barang", "Jumlah"],
            ["{{index}}", "{{name}}", "{{qty * price:0}}"],
            ["Total", "", "{{sum(qty * price):0}}"]],
  "cellStyle": { "2,0": { "colSpan": 2, "align": "right" } } }
```

Tabel yang lebih panjang dari satu halaman berlanjut ke halaman berikutnya: baris tidak pernah terpotong di tengah, dan baris judul diulang di atas setiap halaman. Dengan `grow`, setiap baris setinggi selnya yang paling panjang. Di XLSX dan XLS, warna latar, warna teks, rata tengah/kanan, garis sel (sesuai `grid`: `all`, `rows`, atau `outer`), dan sel yang digabung ke kanan ikut terbawa, dan tabel data menghasilkan satu baris spreadsheet per item.

Di designer: klik sebuah sel, lalu bagian **Sel** di panel kanan mengatur warna, rata, tebal/miring, serta **Gabung ke kanan** / **Gabung ke bawah** / **Pisahkan** (teks sel yang tergabung ikut dipindahkan). Seret garis antar kolom atau antar baris pada tabel terpilih untuk mengubah lebar dan tingginya. Saat mengetik di sel, `Enter` menyimpan, `Shift+Enter` membuat baris baru, `Tab` / `Shift+Tab` pindah ke sel berikutnya / sebelumnya. Sel yang disalin dari Excel bisa ditempel langsung ke tabel mulai dari sel yang diklik.

**Tampil jika**: setiap elemen bisa punya `showIf`, misalnya `paid = 'yes'` atau `total > 0`. Jika kondisinya salah, elemen tidak dicetak (di Excel dan CSV selnya kosong). Kondisi yang salah tulis tetap menampilkan elemen, agar kesalahannya terlihat.

**Gambar**: PNG, JPEG, atau GIF sebagai data URI. `src` boleh berupa placeholder seperti `{{photo}}` yang nilainya data URI dari data, untuk foto produk atau tanda tangan; gambar yang kosong atau rusak dibiarkan kosong, tanpa menggagalkan laporan.

**Garis**: datar atau tegak sesuai bentuknya; `"slope": "down"` atau `"up"` untuk garis miring.

**Halaman**: `page.marginTop` / `marginRight` / `marginBottom` / `marginLeft`, `page.background` (warna), dan `page.backgroundImage` (gambar yang memenuhi halaman, misalnya kop surat atau watermark).

**Teks panjang**: centang `grow` pada elemen teks agar teks dibungkus dan band ikut bertambah tinggi. Elemen di bawahnya turun, kotak dan garis vertikal di sekelilingnya ikut memanjang.

**Chart**: `bar`, `hbar` (batang mendatar), `line`, `pie`, atau `donut`. `label` adalah nama field baris dan `value` nama field atau perhitungan (`qty * price`); nilai dijumlahkan per label. `bar`, `hbar`, dan `line` punya skala nilai yang selalu memuat nol, jadi nilai negatif tampil di bawah garis nol; label kategori ditampilkan selang-seling bila terlalu rapat. `pie` dan `donut` menggambar nilai negatif sebagai nol; legendanya selalu punya tempat di samping pie, irisan bernilai nol tidak dicantumkan, dan irisan terkecil yang tidak muat digabung menjadi "Other" ("Lainnya" dengan `locale` `id`). Chart garis memberi titik di setiap nilai.

**Barcode**: elemen `barcode` dengan `format` `qr` (default) atau `code128`. Isinya ada di `text` dan boleh memuat placeholder, misalnya `https://example.com/verify/{{id}}`. `"showText": true` mencetak teksnya di bawah kode. QR code diberi ruang kosong dua modul di sekelilingnya (di dalam kotaknya) agar mudah dipindai di samping elemen lain.

**Sub-laporan**: elemen `subreport` dengan `data` (nama daftar, misalnya `lines`) dan `report` (template dengan band sendiri: `header`, `columnHeader`, `groupHeader`... sampai tiga tingkat, `detail`, `summary`; `footer` tidak dipakai, dan `sortBy` / `groupBy` boleh diisi). Laporan itu dicetak untuk setiap item di dalam kotak elemen, dan band induknya ikut memanjang. Di band `detail` setiap baris mendapat sub-laporannya sendiri, misalnya setiap pesanan dengan rincian per jenisnya. Field induk tetap bisa dipakai di dalamnya (`{{number}}`), `{{index}}` adalah nomor item. Sub-laporan yang lebih panjang dari satu halaman berlanjut tanpa memotong band, dengan header dan judul kolomnya diulang. Di designer: tab Insert > **Sub-report**, lalu **Edit sub-report…** (atau klik ganda) membuka designer untuk band-bandnya, selebar kotaknya.

**CSV, XLSX, dan XLS**: elemen teks dan sel tabel di band `detail` menjadi kolom. Teks di band lain (judul kolom, subtotal, total) masuk ke kolom yang sejajar dengannya. Di XLSX dan XLS, teks bold tetap bold, `{{f:2}}` menjadi format angka, teks rata tengah atau kanan tetap begitu, sel tabel bergaris tetap bergaris, sel yang digabung ke kanan (`colSpan`) tetap tergabung, dan lebar kolom mengikuti desain. Gabungan ke bawah (`rowSpan`) tidak ikut, dan sel hanya digabung bila kolom yang ditutupnya jelas, agar tidak ada nilai yang tersembunyi; CSV tidak mengenal gabungan. `xls` ditulis sebagai XML Spreadsheet 2003; Excel menampilkan satu peringatan format saat membukanya. CSV diawali BOM UTF-8 agar Excel membaca huruf seperti `é` dengan benar; `"csvSeparator": ";"` untuk Excel berbahasa Indonesia atau Eropa yang memakai `;`. Di CSV, teks yang berawalan seperti formula (`=`, `@`, `+`, `-`) diberi tanda kutip tunggal di depan agar tidak dijalankan spreadsheet.

```json
{ "number": "INV-001", "customer": { "name": "Budi" }, "rows": [{ "name": "Kopi", "qty": 2 }] }
```

## Backend

**Go**

```go
import ddreport "github.com/gvjavx/DdDesignReport"

pdf, err := ddreport.Render(templateJSON, data, "pdf") // "pdf" | "html" | "xlsx" | "xls" | "csv"
```

Java, Kotlin, dan Node.js memanggil binary `ddreport`. Paketnya membawa binary untuk Windows, Linux, dan macOS (x64 dan arm64); `DDREPORT_BIN` menunjuk binary lain bila perlu. Binary-nya juga ada di setiap [GitHub Release](https://github.com/gvjavx/DdDesignReport/releases).

**Java** (Maven Central, `io.github.gvjavx:ddreport`): tambahkan wrapper-nya dan binary untuk sistem tempat aplikasi berjalan, dengan classifier `linux-x64`, `linux-arm64`, `windows-x64`, `windows-arm64`, `darwin-x64`, atau `darwin-arm64`.

```xml
<dependency><groupId>io.github.gvjavx</groupId><artifactId>ddreport</artifactId><version>0.2.0</version></dependency>
<dependency><groupId>io.github.gvjavx</groupId><artifactId>ddreport</artifactId><version>0.2.0</version><classifier>linux-x64</classifier></dependency>
```

```kotlin
// Gradle
implementation("io.github.gvjavx:ddreport:0.2.0")
runtimeOnly("io.github.gvjavx:ddreport:0.2.0:linux-x64")
```

Package Java-nya tetap `io.ddreport` (`import io.ddreport.DdReport`).

```java
byte[] pdf = DdReport.render(templateJson, dataJson, "pdf");
// batas waktu default 1 menit; atau DdReport.render(templateJson, dataJson, "pdf", Duration.ofSeconds(30))

// di server: satu proses yang terus berjalan, tanpa memulai proses baru untuk setiap laporan
try (DdReport.Renderer renderer = new DdReport.Renderer()) {
    byte[] report = renderer.render(templateJson, dataJson, "pdf");
}
```

`DdReport.Renderer` dipakai bergantian oleh beberapa thread; untuk mencetak paralel, buat satu `Renderer` per thread. Laporan yang gagal atau terlalu lama hanya menggagalkan laporan itu, dan laporan berikutnya memakai proses baru.

**Kotlin**

```kotlin
val pdf = DdReport.render(templateJson, dataJson, "pdf")
```

**Node.js** (`npm install ddreport`; binary untuk sistemnya ikut terpasang)

```js
const { render, renderAsync } = require('ddreport')
const pdf = render(template, data, 'pdf')            // Buffer, sinkron
const pdf2 = await renderAsync(template, data, 'pdf') // tidak memblokir event loop; pakai ini di server
// batas waktu default 60 detik; ubah dengan { timeout: 30000 } sebagai argumen keempat

// di server: proses yang terus berjalan, sekitar lima kali lebih cepat per laporan kecil
const { createRenderer } = require('ddreport')
const renderer = createRenderer({ processes: 2 })    // opsional: { timeout: 30000 }
const pdf3 = await renderer.render(template, data, 'pdf')
renderer.close()                                      // saat server berhenti
```

Laporan menunggu giliran jika semua proses sedang bekerja. Proses yang berhenti atau terlalu lama hanya menggagalkan laporannya sendiri; proses baru menggantikannya. Proses yang menganggur tidak menahan Node.js tetap berjalan.

Keduanya memakai `ddreport -stdio`: binary terus berjalan dan membaca permintaan satu per satu, masing-masing sebuah baris `<format> <panjang>` lalu `{"template": ..., "data": ...}` sepanjang itu; jawabannya baris `ok <panjang>` lalu dokumennya, atau `error <panjang>` lalu pesannya. Panjang di setiap permintaan menjaga agar isi satu permintaan tidak pernah terbaca sebagai permintaan berikutnya.

**HTTP**: `ddreport -serve` juga menerima `POST /render?format=pdf` dengan body `{"template": ..., "data": ...}`. Render berjalan paling banyak sejumlah prosesor sekaligus; permintaan lain menunggu giliran. Permintaan dari halaman situs lain di browser (header `Origin` yang berbeda) ditolak; panggil dari backend Anda, bukan langsung dari browser.

### Font

`Helvetica`, `Times`, dan `Courier` adalah font bawaan PDF. Selain itu ikut dibundel: **Roboto**, **Open Sans**, **Lato**, **Montserrat**, **Lora**, **Playfair Display**, dan **Great Vibes** (lisensi SIL OFL, di `fonts/`). Font bundel mencakup huruf Latin, Kiril, Yunani, dan Vietnam; di PDF dan HTML huruf yang dipakai ikut disematkan, jadi hasilnya sama di mana pun dibuka.

Font lain bisa didaftarkan dengan file `Nama-Regular.ttf`, `Nama-Bold.ttf`, `Nama-Italic.ttf`, `Nama-BoldItalic.ttf` (hanya Regular yang wajib):

```go
ddreport.AddFonts(os.DirFS("fonts-saya"))                       // atau:
ddreport.Families["Inter"] = &ddreport.Family{Regular: regular, Bold: bold}
```

Untuk binary (Java, Kotlin, Node.js, `-serve`): set `DDREPORT_FONTS` ke folder berisi file-file itu. Agar namanya muncul di designer, berikan opsi `fonts: ['Inter']`.

### Font non-Latin di PDF

Font bawaan PDF hanya mencakup huruf Latin. Untuk aksara lain, berikan font TrueType:

```go
ddreport.Font, _ = os.ReadFile("NotoSans-Regular.ttf")
ddreport.FontBold, _ = os.ReadFile("NotoSans-Bold.ttf") // opsional
```

Untuk binary (Java, Kotlin, Node.js, `-serve`): set `DDREPORT_FONT` dan `DDREPORT_FONT_BOLD` ke path file `.ttf`.

## Rilis

Tag versi menjalankan `.github/workflows/release.yml`: binary untuk setiap sistem dibangun dan dilampirkan ke GitHub Release, paketnya dicoba, lalu diterbitkan ke npm (`ddreport`, `@ddreport/<sistem>`, `ddreport-designer`) dan Maven Central (`io.github.gvjavx:ddreport`).

```sh
git tag vX.Y.Z && git push origin vX.Y.Z
```

npm memakai *trusted publishing*: setiap paket di npmjs.com mencantumkan `gvjavx/DdDesignReport` dan `release.yml` sebagai trusted publisher, sehingga CI menerbitkan tanpa token. Secret `NPM_TOKEN` hanya dibutuhkan untuk versi pertama paket npm yang baru, karena trusted publisher baru bisa dipasang setelah paketnya ada; pasang sementara, lalu hapus lagi setelah trusted publisher-nya terpasang. Untuk Maven dibutuhkan secret `MAVEN_CENTRAL_USERNAME`, `MAVEN_CENTRAL_PASSWORD` (user token Central Portal), `MAVEN_GPG_PRIVATE_KEY`, dan `MAVEN_GPG_PASSPHRASE`. Registry yang secret-nya belum ada dilewati; versi yang sudah terbit tidak diterbitkan lagi. `node release/build.mjs 0.1.0` membangun semuanya di `dist/` secara lokal.

## Test

```sh
go test ./...                         # core, binary, template, dan test regresi
go build -o ddreport ./cmd/ddreport
export DDREPORT_BIN="$PWD/ddreport"   # PowerShell: $env:DDREPORT_BIN = "$PWD\ddreport.exe"
node --test node/test.js              # wrapper Node.js
npm install --no-save playwright && npx playwright install chromium firefox webkit
node designer/run-tests.mjs           # designer di Chromium, Firefox, dan WebKit (mesin Safari), lalu di layar sentuh HP
javac -d out java/io/ddreport/DdReport.java java/DdReportTest.java
java -cp out DdReportTest             # wrapper Java
```

Designer: sajikan folder `designer/` dengan web server (`npx serve designer`) lalu buka `/test.html`; halaman itu menjalankan pemeriksaan otomatis pada designer.

Pengetesan manual: `./run-local.sh` (port lain: `./run-local.sh 9000`) membangun binary dari kode saat ini dan membuka designer di http://127.0.0.1:8080, lengkap dengan Preview, ekspor, dan template siap pakai. `Ctrl+C` menghentikannya.

`.github/workflows/test.yml` menjalankan semua test Go, Node.js, dan Java di Linux dan Windows, serta test designer di Chromium, Firefox, dan WebKit, di setiap push.
