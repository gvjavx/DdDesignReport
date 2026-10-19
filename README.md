# DdDesignReport

Library reporting dengan designer drag and drop. Template didesain di browser, disimpan sebagai JSON, lalu di-render oleh satu core (Go) menjadi **PDF, HTML, XLSX, XLS, atau CSV** dari backend Go, Java, Kotlin, atau Node.js.

```
designer/   designer drag and drop (vanilla JS, tanpa dependency) + aplikasi standalone
ddreport.go core renderer (library Go)
fonts/      font bundel (TrueType, SIL OFL), disematkan ke binary
cmd/        binary `ddreport`, dipakai oleh wrapper di bawah dan untuk menjalankan designer
java/       wrapper Java / Kotlin
node/       wrapper Node.js
```

## Designer

Sebagai aplikasi terpisah, lengkap dengan preview:

```sh
go build -o ddreport ./cmd/ddreport
./ddreport -serve 127.0.0.1:8080      # buka http://127.0.0.1:8080
```

Sebagai library di frontend apa pun:

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
  preview: async t => {               // opsional, memunculkan tombol Preview
    const res = await fetch('/api/report/preview', { method: 'POST', body: JSON.stringify(t) })
    return res.text()                 // HTML hasil render backend (format "html")
  },
})
designer.getTemplate()                // JSON untuk dikirim ke backend
```

Designer punya ribbon seperti pengolah kata:

| Tab | Isi |
| --- | --- |
| **Home** | Undo/redo, cut/copy/paste/duplicate, font, ukuran, bold/italic/underline, warna teks, highlight, rata kiri/tengah/kanan/kanan-kiri, rata atas/tengah/bawah, spasi baris, wrap, perataan dan distribusi elemen, urutan depan/belakang |
| **Insert** | Text box, tabel (baris × kolom), gambar, kotak, garis, chart, barcode/QR, field data (nama field diambil dari data contoh) |
| **Layout** | Ukuran kertas (A4, A5, Letter, Legal, F4), orientasi, margin per sisi, warna dan gambar latar halaman, urutan data, grouping, pemisah angka, bahasa tanggal, pemisah CSV |
| **View** | Zoom, penggaris, snap, preview |

Tampilannya menyesuaikan lebar tempat designer dipasang: di layar sempit panel properti pindah ke bawah dan halaman dipaskan ke lebar. Zoom ada di bilah status bawah, dan bisa langsung di kanvas: `Ctrl` + scroll (atau pinch di trackpad) memperbesar di titik kursor, cubit dua jari di layar sentuh, serta `Ctrl` + `=` / `-` / `0`.

Klik tombol di tab Insert atau seret ke band. Klik ganda teks atau sel tabel untuk mengetik langsung di halaman; pilih sebagian teks lalu tekan **B**/**I**/**U** untuk memformat bagian itu saja. Shift-klik, atau seret kotak di area kosong, untuk memilih banyak elemen. Geser elemen untuk memindah (termasuk ke band lain), tarik pegangan biru di sudut kanan bawah untuk mengubah ukuran (juga dengan sentuhan), tarik tepi bawah band untuk mengubah tingginya. Untuk menghapus: klik tombol merah × di sudut elemen terpilih, seret elemen ke tempat sampah yang muncul saat menggeser, atau tekan `Delete`.

Pintasan: `Ctrl+Z` / `Ctrl+Y` (mengetik di satu kolom properti dihitung satu langkah undo), `Ctrl+C` / `X` / `V` / `D`, `Ctrl+A`, `Ctrl+B` / `I` / `U`, `Ctrl+=` / `Ctrl+-` / `Ctrl+0` (zoom), tombol panah (dengan `Shift` = 10pt), `Delete`. Tahan `Alt` saat menggeser untuk mematikan snap.

`Ctrl+C` / `Ctrl+X` menaruh elemen terpilih di clipboard sistem, jadi bisa ditempel di designer lain (tab atau laporan lain). Menempel teks biasa membuat text box baru. Untuk tabel, klik sebuah sel lalu pakai tombol di panel properti untuk menyisipkan atau menghapus baris dan kolom di sebelahnya. Klik kanvas kosong untuk mengatur band: tinggi, kepala/kaki halaman hanya di halaman pertama atau terakhir, dan band yang dimulai di halaman baru.

Aplikasi standalone juga punya tombol ekspor PDF, HTML, XLSX, XLS, dan CSV, serta antarmuka Indonesia lewat `?lang=id`. `Esc` saat mengetik membatalkan; **New** dan **Templates…** bisa dibatalkan dengan `Ctrl+Z`.

**Preview** (tombol mata di bilah status, atau tab View) membuka hasil cetak HTML dengan data contoh: tiap halaman tampil sebagai lembar terpisah, dipaskan ke lebar jendela, dengan jumlah halamannya. Preview dan ekspor butuh halaman dibuka dari `ddreport -serve`; jika tidak, designer mengatakannya.

## Template siap pakai

Delapan template ada di `designer/templates/` (faktur, laporan penjualan, struk kasir, slip gaji, sertifikat, surat resmi, daftar stok, label produk). Tiap file berisi `template` dan `data` contohnya. Di aplikasi standalone pilih lewat **Templates…**; `go test` mencetak semuanya ke setiap format.

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

Angka yang dikirim sebagai teks (`"18000"`, seperti dari BigDecimal atau kolom numeric database) dihitung sebagai angka di rumus, agregat, format `:N`, dan chart. Hasil hitungan tanpa `:N` tidak menampilkan sisa pembulatan (`0.1 + 0.2` menjadi `0.3`).

**Urutan**: `sortBy` mengurutkan baris, misalnya `"kategori, -total"` (`-` untuk menurun). Angka diurutkan sebagai angka.

**Grouping**: isi `groupBy` dengan nama field. Baris dengan nilai yang sama dikumpulkan menjadi satu grup, urut sesuai kemunculan pertama (atau sesuai `sortBy`). `groupPageBreak` memulai setiap grup di halaman baru.

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

**Tampil jika**: setiap elemen bisa punya `showIf`, misalnya `paid = 'yes'` atau `total > 0`. Jika kondisinya salah, elemen tidak dicetak (di Excel dan CSV selnya kosong). Kondisi yang salah tulis tetap menampilkan elemen, agar kesalahannya terlihat.

**Gambar**: PNG, JPEG, atau GIF sebagai data URI. `src` boleh berupa placeholder seperti `{{photo}}` yang nilainya data URI dari data, untuk foto produk atau tanda tangan; gambar yang kosong atau rusak dibiarkan kosong, tanpa menggagalkan laporan.

**Garis**: datar atau tegak sesuai bentuknya; `"slope": "down"` atau `"up"` untuk garis miring.

**Halaman**: `page.marginTop` / `marginRight` / `marginBottom` / `marginLeft`, `page.background` (warna), dan `page.backgroundImage` (gambar yang memenuhi halaman, misalnya kop surat atau watermark).

**Teks panjang**: centang `grow` pada elemen teks agar teks dibungkus dan band ikut bertambah tinggi. Elemen di bawahnya turun, kotak dan garis vertikal di sekelilingnya ikut memanjang.

**Chart**: `bar`, `hbar` (batang mendatar), `line`, `pie`, atau `donut`. `label` adalah nama field baris dan `value` nama field atau perhitungan (`qty * price`); nilai dijumlahkan per label. `bar`, `hbar`, dan `line` punya skala nilai yang selalu memuat nol, jadi nilai negatif tampil di bawah garis nol; label kategori ditampilkan selang-seling bila terlalu rapat. `pie` dan `donut` menggambar nilai negatif sebagai nol; legendanya selalu punya tempat di samping pie, dan irisan terkecil yang tidak muat digabung menjadi "Other" ("Lainnya" dengan `locale` `id`). Chart garis memberi titik di setiap nilai.

**Barcode**: elemen `barcode` dengan `format` `qr` (default) atau `code128`. Isinya ada di `text` dan boleh memuat placeholder, misalnya `https://example.com/verify/{{id}}`. `"showText": true` mencetak teksnya di bawah kode.

**CSV, XLSX, dan XLS**: elemen teks dan sel tabel di band `detail` menjadi kolom. Teks di band lain (judul kolom, subtotal, total) masuk ke kolom yang sejajar dengannya. Di XLSX dan XLS, teks bold tetap bold, `{{f:2}}` menjadi format angka, dan lebar kolom mengikuti desain. `xls` ditulis sebagai XML Spreadsheet 2003; Excel menampilkan satu peringatan format saat membukanya. CSV diawali BOM UTF-8 agar Excel membaca huruf seperti `é` dengan benar; `"csvSeparator": ";"` untuk Excel berbahasa Indonesia atau Eropa yang memakai `;`. Di CSV, teks yang berawalan seperti formula (`=`, `@`, `+`, `-`) diberi tanda kutip tunggal di depan agar tidak dijalankan spreadsheet.

```json
{ "number": "INV-001", "customer": { "name": "Budi" }, "rows": [{ "name": "Kopi", "qty": 2 }] }
```

## Backend

**Go**

```go
import ddreport "github.com/gvjavx/DdDesignReport"

pdf, err := ddreport.Render(templateJSON, data, "pdf") // "pdf" | "html" | "xlsx" | "xls" | "csv"
```

Java, Kotlin, dan Node.js memanggil binary `ddreport`. Taruh di `PATH` atau set `DDREPORT_BIN`.

**Java** (salin `java/io/ddreport/DdReport.java` ke project, atau pasang sebagai jar dengan `mvn -f java/pom.xml install` lalu pakai dependency `io.ddreport:ddreport:0.1.0`)

```java
byte[] pdf = DdReport.render(templateJson, dataJson, "pdf");
// batas waktu default 1 menit; atau DdReport.render(templateJson, dataJson, "pdf", Duration.ofSeconds(30))
```

**Kotlin**

```kotlin
val pdf = DdReport.render(templateJson, dataJson, "pdf")
```

**Node.js**

```js
const { render, renderAsync } = require('ddreport')
const pdf = render(template, data, 'pdf')            // Buffer, sinkron
const pdf2 = await renderAsync(template, data, 'pdf') // tidak memblokir event loop; pakai ini di server
// batas waktu default 60 detik; ubah dengan { timeout: 30000 } sebagai argumen keempat
```

**HTTP**: `ddreport -serve` juga menerima `POST /render?format=pdf` dengan body `{"template": ..., "data": ...}`. Render berjalan paling banyak sejumlah prosesor sekaligus; permintaan lain menunggu giliran.

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

## Test

```sh
go test ./...                         # core, binary, template, dan test regresi
go build -o ddreport ./cmd/ddreport
export DDREPORT_BIN="$PWD/ddreport"   # PowerShell: $env:DDREPORT_BIN = "$PWD\ddreport.exe"
node --test node/test.js              # wrapper Node.js
javac -d out java/io/ddreport/DdReport.java java/DdReportTest.java
java -cp out DdReportTest             # wrapper Java
```

Designer: sajikan folder `designer/` dengan web server (`npx serve designer`) lalu buka `/test.html`; halaman itu menjalankan pemeriksaan otomatis pada designer.

`.github/workflows/test.yml` menjalankan semua test Go, Node.js, dan Java di setiap push.
