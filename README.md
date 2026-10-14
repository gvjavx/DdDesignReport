# DdDesignReport

Library reporting dengan designer drag and drop. Template didesain di browser, disimpan sebagai JSON, lalu di-render oleh satu core (Go) menjadi **PDF, HTML, XLSX, XLS, atau CSV** dari backend Go, Java, Kotlin, atau Node.js.

```
designer/   designer drag and drop (vanilla JS, tanpa dependency) + aplikasi standalone
ddreport.go core renderer (library Go)
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
| **Home** | Undo/redo, cut/copy/paste/duplicate, font, ukuran, bold/italic/underline, warna teks, highlight, rata kiri/tengah/kanan, spasi baris, wrap, perataan dan distribusi elemen, urutan depan/belakang |
| **Insert** | Text box, tabel (baris × kolom), gambar, kotak, garis, chart, barcode/QR, field data |
| **Layout** | Ukuran kertas (A4, A5, Letter, Legal, F4), orientasi, margin per sisi, warna dan gambar latar halaman, grouping, pemisah angka |
| **View** | Zoom, penggaris, snap, preview |

Tampilannya menyesuaikan lebar tempat designer dipasang: di layar sempit panel properti pindah ke bawah dan halaman dipaskan ke lebar. Zoom ada di bilah status bawah.

Klik tombol di tab Insert atau seret ke band. Klik ganda teks atau sel tabel untuk mengetik langsung di halaman; pilih sebagian teks lalu tekan **B**/**I**/**U** untuk memformat bagian itu saja. Shift-klik, atau seret kotak di area kosong, untuk memilih banyak elemen. Geser elemen untuk memindah (termasuk ke band lain), tarik pegangan biru di sudut kanan bawah untuk mengubah ukuran (juga dengan sentuhan), tarik tepi bawah band untuk mengubah tingginya. Untuk menghapus: klik tombol merah × di sudut elemen terpilih, seret elemen ke tempat sampah yang muncul saat menggeser, atau tekan `Delete`.

Pintasan: `Ctrl+Z` / `Ctrl+Y`, `Ctrl+C` / `X` / `V` / `D`, `Ctrl+A`, `Ctrl+B` / `I` / `U`, tombol panah (dengan `Shift` = 10pt), `Delete`. Tahan `Alt` saat menggeser untuk mematikan snap.

Aplikasi standalone juga punya tombol ekspor PDF, HTML, XLSX, XLS, dan CSV, serta antarmuka Indonesia lewat `?lang=id`. `Esc` saat mengetik membatalkan; **New** dan **Templates…** bisa dibatalkan dengan `Ctrl+Z`.

## Template siap pakai

Delapan template ada di `designer/templates/` (faktur, laporan penjualan, struk kasir, slip gaji, sertifikat, surat resmi, daftar stok, label produk). Tiap file berisi `template` dan `data` contohnya. Di aplikasi standalone pilih lewat **Templates…**; `go test` mencetak semuanya ke setiap format.

## Template

| Band | Muncul |
| --- | --- |
| `header`, `columnHeader`, `footer` | di setiap halaman |
| `groupHeader`, `groupFooter` | sebelum dan sesudah setiap grup (jika `groupBy` diisi) |
| `detail` | untuk setiap baris data |
| `summary` | sekali, setelah baris terakhir |

**Placeholder** di dalam teks:

| Placeholder | Arti |
| --- | --- |
| `{{field}}`, `{{customer.name}}` | field dari baris, lalu dari data utama |
| `{{page}}`, `{{pages}}` | nomor dan jumlah halaman |
| `{{sum(f)}}`, `{{avg(f)}}`, `{{min(f)}}`, `{{max(f)}}`, `{{count()}}` | agregat: per grup di band grup dan detail, seluruh baris di band lain |
| `{{qty * price}}`, `{{sum(qty * price)}}`, `{{(price - disc) / 1.11}}` | perhitungan dengan `+ - * /` dan kurung, juga di dalam agregat |
| `{{unit-price}}`, `{{[unit-price] * qty}}` | nama field yang bukan satu kata: langsung jika berdiri sendiri, dalam kurung siku di dalam perhitungan |
| `{{price:2}}`, `{{sum(price):0}}` | angka dengan N desimal dan pemisah ribuan (`thousands` / `decimal` di template, default `,` dan `.`) |

**Grouping**: isi `groupBy` dengan nama field. Baris dengan nilai yang sama dikumpulkan menjadi satu grup, urut sesuai kemunculan pertama. `groupPageBreak` memulai setiap grup di halaman baru.

**Format teks**: `font` (`Helvetica`, `Times`, `Courier`), `fontSize`, `lineHeight`, `bold`, `italic`, `underline`, `color`, `fill` (highlight). Sebagian teks bisa diformat dengan tag `<b>`, `<i>`, `<u>`: `Total <b>{{sum(price):2}}</b>`.

**Tabel**: elemen `table` dengan `cells` (teks per sel, boleh berisi placeholder), `cols` (lebar relatif kolom), `aligns` (rata per kolom), `border`, dan `fill`. Tabel satu baris di band `detail` menjadi baris data berbingkai.

**Halaman**: `page.marginTop` / `marginRight` / `marginBottom` / `marginLeft`, `page.background` (warna), dan `page.backgroundImage` (gambar yang memenuhi halaman, misalnya kop surat atau watermark).

**Teks panjang**: centang `grow` pada elemen teks agar teks dibungkus dan band ikut bertambah tinggi. Elemen di bawahnya turun, kotak dan garis vertikal di sekelilingnya ikut memanjang.

**Chart**: `bar`, `hbar` (batang mendatar), `line`, `pie`, atau `donut`. `label` adalah nama field baris dan `value` nama field atau perhitungan (`qty * price`); nilai dijumlahkan per label.

**Barcode**: elemen `barcode` dengan `format` `qr` (default) atau `code128`. Isinya ada di `text` dan boleh memuat placeholder, misalnya `https://example.com/verify/{{id}}`.

**CSV, XLSX, dan XLS**: elemen teks dan sel tabel di band `detail` menjadi kolom. Teks di band lain (judul kolom, subtotal, total) masuk ke kolom yang sejajar dengannya. Di XLSX dan XLS, teks bold tetap bold, `{{f:2}}` menjadi format angka, dan lebar kolom mengikuti desain. `xls` ditulis sebagai XML Spreadsheet 2003; Excel menampilkan satu peringatan format saat membukanya. Di CSV, teks yang berawalan seperti formula (`=`, `@`, `+`, `-`) diberi tanda kutip tunggal di depan agar tidak dijalankan spreadsheet.

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
```

**Kotlin**

```kotlin
val pdf = DdReport.render(templateJson, dataJson, "pdf")
```

**Node.js**

```js
const { render } = require('ddreport')
const pdf = render(template, data, 'pdf') // Buffer
```

**HTTP**: `ddreport -serve` juga menerima `POST /render?format=pdf` dengan body `{"template": ..., "data": ...}`.

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
