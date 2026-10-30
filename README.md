<img src="logo.svg" alt="" width="72">

# DdDesignReport

Library reporting dengan designer drag and drop. Template didesain di browser, disimpan sebagai JSON, lalu di-render oleh satu core (Go) menjadi **PDF, HTML, XLSX, XLS, atau CSV** dari backend Go, Java, Kotlin, Node.js, atau bahasa apa pun lewat HTTP. Template juga bisa dibawa ke dan dari JasperReports (`.jrxml`).

**Untuk software house:** pasang designer-nya di aplikasi Anda, dan klien bisa mengubah sendiri kop faktur, kolom slip gaji, atau tata letak rapor mereka. Tidak ada lagi tiket "tolong geser logonya" untuk Anda. Mode mudah, tombol yang bisa disembunyikan, dan teks antarmuka yang bisa diganti membuatnya cocok untuk admin dan staf yang bukan programmer, dan dokumen lokal sudah ada: kuitansi dengan terbilang, kertas F4, nama bulan Indonesia, slip gaji, surat jalan, dan rapor.

Butuh dibuatkan template atau dipasangkan? Lihat [jasa](https://gvjavx.github.io/DdDesignReport-website/#jasa).

Yang baru di setiap versi: [CHANGELOG.md](CHANGELOG.md). English: [README.en.md](README.en.md).

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
| CSS `#designer.ddr { --a: #e11d48 }` | Warna: `--a` (aksen), `--a-soft`, `--bg`, `--surface`, `--text`, `--muted`, `--line`, `--stage`, `--hover`; sudut `--r` dan font `--font`. Setiap bagian punya class (`.ddr-tabs`, `.ddr-b`, `.ddr-page`, `.ddr-side`, `.ddr-status`, ...) yang bisa ditimpa dengan `.ddr .ddr-b { ... }` |
| `theme: 'inherit'` | Ikut tema halaman: warna, font, dan sudut dari variabel Bootstrap (`--bs-primary`, `--bs-body-bg`, ...), termasuk dark mode Bootstrap. Framework lain cukup mendefinisikan variabel `--bs-*` itu |
| `hide: ['ribbon']`, `designer.run('bold')`, `onState` | Toolbar sendiri: ribbon disembunyikan, tombol Anda (Bootstrap atau apa pun) menjalankan perintah designer, dan `onState` memberi tahu tombol mana yang aktif. Daftar perintah: `designer.commands()`. Contoh lengkap dengan komponen Bootstrap: `designer/bootstrap.html` (di `ddreport -serve`: `/bootstrap.html`) |

Designer punya ribbon seperti pengolah kata:

| Tab | Isi |
| --- | --- |
| **Home** | Undo/redo, cut/copy/paste/duplicate, font, ukuran, bold/italic/underline, warna teks, highlight, rata kiri/tengah/kanan/kanan-kiri, rata atas/tengah/bawah, spasi baris, wrap, perataan dan distribusi elemen, urutan depan/belakang |
| **Insert** | Text box, tabel (baris × kolom), gambar, kotak, garis, chart, barcode/QR, sub-laporan, crosstab, field data (nama field diambil dari data contoh) |
| **Layout** | Ukuran kertas (A4, A5, Letter, Legal, F4), orientasi, margin per sisi, warna dan gambar latar halaman, urutan data, grouping, pemisah angka, bahasa tanggal, pemisah CSV |
| **View** | Zoom, penggaris, snap, preview |

Chart di kanvas digambar dari data contoh (opsi `data`) dengan skala nilai, label, dan format angka seperti hasil cetak. Barcode di kanvas adalah kode yang sama persis dengan hasil cetak (QR dan Code 128, modul demi modul), jadi bisa langsung dipindai; `{{field}}` di dalamnya diisi dari data contoh. Tampilannya menyesuaikan lebar tempat designer dipasang: di layar sempit panel properti pindah ke bawah dan halaman dipaskan ke lebar. Zoom ada di bilah status bawah, dan bisa langsung di kanvas: `Ctrl` + scroll (atau pinch di trackpad) memperbesar di titik kursor, cubit dua jari di layar sentuh, serta `Ctrl` + `=` / `-` / `0`.

Klik tombol di tab Insert atau seret ke band. Klik ganda teks atau sel tabel untuk mengetik langsung di halaman; pilih sebagian teks lalu tekan **B**/**I**/**U** untuk memformat bagian itu saja. Shift-klik, atau seret kotak di area kosong, untuk memilih banyak elemen. Geser elemen untuk memindah (termasuk ke band lain), tarik pegangan biru di sudut kanan bawah untuk mengubah ukuran (juga dengan sentuhan), tarik tepi bawah band untuk mengubah tingginya. Untuk menghapus: klik tombol merah × di sudut elemen terpilih, seret elemen ke tempat sampah yang muncul saat menggeser, atau tekan `Delete`.

Pintasan: `Ctrl+Z` / `Ctrl+Y` (mengetik di satu kolom properti dihitung satu langkah undo), `Ctrl+C` / `X` / `V` / `D`, `Ctrl+A`, `Ctrl+B` / `I` / `U`, `Ctrl+=` / `Ctrl+-` / `Ctrl+0` (zoom), tombol panah (dengan `Shift` = 10pt), `Delete`. Tahan `Alt` saat menggeser untuk mematikan snap.

`Ctrl+C` / `Ctrl+X` menaruh elemen terpilih di clipboard sistem, jadi bisa ditempel di designer lain (tab atau laporan lain). Menempel teks biasa membuat text box baru. Untuk tabel, klik sebuah sel lalu pakai tombol di panel properti untuk menyisipkan atau menghapus baris dan kolom di sebelahnya. Klik kanvas kosong untuk mengatur band: tinggi, kepala/kaki halaman hanya di halaman pertama atau terakhir, dan band yang dimulai di halaman baru.

Aplikasi standalone juga punya tombol ekspor PDF, HTML, XLSX, XLS, dan CSV, antarmuka Indonesia lewat `?lang=id`, dan **Mode mudah** yang aktif sejak awal (hapus centangnya untuk semua alat). `Esc` saat mengetik membatalkan; **New** dan **Templates…** bisa dibatalkan dengan `Ctrl+Z`.

**My templates** menyimpan template Anda sendiri di samping yang siap pakai: simpan laporan yang sedang dirancang dengan nama, lalu buka, perbarui, ganti nama, atau hapus. Tiap template ikut membawa data contohnya. `ddreport -serve` menyimpannya sebagai satu file JSON per template di folder `ddreport-templates` (ubah dengan `-templates folder`), jadi semua yang membuka server itu melihat daftar yang sama. Setiap kali template ditimpa, versi sebelumnya disimpan (20 terakhir): tombol **History** membuka versi lama, lalu **Update** menjadikannya versi sekarang lagi. Lewat HTTP: `GET /mine`, `PUT /mine/{id}`, `DELETE /mine/{id}`, `GET /mine/{id}/history`, `GET /mine/{id}/history/{versi}`, dengan token yang sama seperti `/render` bila `DDREPORT_TOKEN` diisi. Di image Docker foldernya `/data`: `docker run -v ddreport:/data ...` agar tidak hilang. Halaman yang dibuka tanpa server menyimpannya di browser.

**Preview** (tombol mata di bilah status, atau tab View) membuka hasil cetak HTML dengan data contoh: tiap halaman tampil sebagai lembar terpisah, dipaskan ke lebar jendela, dengan jumlah halamannya. Preview dan ekspor butuh halaman dibuka dari `ddreport -serve`; jika tidak, designer mengatakannya.

## Template siap pakai

Tiga belas template ada di `designer/templates/` (faktur, laporan penjualan, struk kasir, slip gaji, sertifikat, surat resmi, daftar stok, label produk, kuitansi, surat jalan, rapor siswa, daftar hadir, laporan laba rugi). Tiap file berisi `template` dan `data` contohnya. Di aplikasi standalone pilih lewat **Templates…**; `go test` mencetak semuanya ke setiap format.

## Menyumbang template

Galeri template di website terbuka untuk siapa saja. Untuk menambahkan template Anda:

1. Rancang di designer, lalu **Save template**. Buat file `designer/templates/<id>.json` berisi `name`, `description`, `template`, dan `data` contoh (lihat template yang sudah ada); `id` hanya huruf kecil, angka, dan tanda hubung.
2. Tambahkan entrinya di `designer/templates/index.json` dengan `author` (nama Anda) dan, bila mau, `url` (`https://`, misalnya profil GitHub Anda).
3. Kirim pull request. `go test ./...` mencetak template itu ke setiap format dengan data contohnya dan menolak placeholder yang tidak terisi, jadi CI langsung memeriksanya. Setelah digabung, template muncul di galeri website dan di menu **Templates…** aplikasi standalone.

Data contoh harus buatan, bukan data pelanggan sungguhan. Template yang disumbangkan berlisensi MIT seperti sisa repo.

## Template

Template adalah JSON. [`designer/template.schema.json`](designer/template.schema.json) adalah JSON Schema-nya (juga di `ddreport-designer/template.schema.json` dan di `/template.schema.json` pada `ddreport -serve`), untuk validasi dan pelengkapan otomatis di editor; properti yang tidak dikenal adalah kesalahan. [`llms.txt`](llms.txt) meringkas cara menulis template untuk asisten AI, dengan contoh yang dicek oleh test.

### Asisten AI (MCP)

`ddreport -mcp` menjadikan binary-nya server [MCP](https://modelcontextprotocol.io) lewat stdin/stdout, sehingga asisten AI (Claude, Cursor, dan lainnya) bisa membaca cara menulis template (`template_guide`: isi `llms.txt` dan JSON Schema), memeriksa template yang ditulisnya (`check_template`: properti yang salah ketik, yang diabaikan saat mencetak, lalu dicoba dicetak dengan data contoh), dan mencetaknya ke file (`render_template`). Contoh: minta asisten membuat template dari foto faktur lama Anda, lalu memeriksanya sendiri sampai valid.

```sh
claude mcp add ddreport -- /path/ke/ddreport -mcp     # Claude Code
```

Untuk aplikasi lain, daftarkan perintah `ddreport` dengan argumen `-mcp` di pengaturan server MCP-nya.

| Band | Muncul |
| --- | --- |
| `header`, `columnHeader`, `footer` | di setiap halaman; `"pages": "first"` hanya di halaman pertama, `"last"` (footer) hanya di halaman terakhir |
| `groupHeader`, `groupFooter` | sebelum dan sesudah setiap grup; tanpa `groupBy`, sekali di sekitar semua baris. Judul grup diulang di atas halaman tempat grupnya berlanjut |
| `detail` | untuk setiap baris data |
| `summary` | sekali, setelah baris terakhir |

`"newPage": true` pada `groupHeader`, `detail`, `groupFooter`, atau `summary` memulai band itu di halaman baru. Band yang lebih tinggi dari satu halaman (misalnya surat panjang dengan Wrap) berlanjut ke halaman berikutnya, baris demi baris; tidak ada teks yang terpotong. Footer yang ikut memanjang tetap di dalam margin bawah.

**Versi format**: designer menulis `"version": 1` di setiap template. Nomor ini hanya naik bila ddreport versi lama akan mencetak template baru dengan salah (misalnya melewatkan elemen yang belum dikenalnya). Ddreport menolak template yang versinya lebih baru dari yang bisa dibacanya, dengan pesan yang menyebut versinya, alih-alih mencetaknya tidak lengkap. Template tanpa `version` dibaca sebagai versi 1. Jadi perbarui backend (wrapper, binary, atau image Docker) bersamaan dengan atau sebelum designer.

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

**Grouping**: isi `groupBy` dengan nama field. Baris dengan nilai yang sama dikumpulkan menjadi satu grup, urut sesuai kemunculan pertama (atau sesuai `sortBy`). Sampai enam tingkat: `"region, city, store"`; tingkat kedua memakai band `groupHeader2` / `groupFooter2`, tingkat ketiga `groupHeader3` / `groupFooter3`, dan seterusnya sampai `groupHeader6` / `groupFooter6`, dan agregat di tiap band menghitung grup tingkatnya. Judul semua grup yang sedang berjalan diulang di atas halaman baru. Di designer, band tingkat berikutnya muncul begitu Group by berisi field sebanyak itu.

**Sheet per grup**: `"sheetPerGroup": true` membuat XLSX dan XLS berisi satu sheet untuk setiap grup tingkat pertama, dinamai sesuai grupnya (dengan aturan nama sheet Excel: maksimal 31 huruf, tanpa `[]:*?/\`, tidak kembar). Tiap sheet berisi judul kolom, baris, dan totalnya sendiri. `groupPageBreak` memulai setiap grup di halaman baru.

**Warna sesuai data**: `color`, `fill`, `textColor`, `stripe`, `headerFill`, `headerColor`, `footerFill`, `footerColor`, dan `fill` / `color` di `cellStyle` boleh berupa rumus, misalnya `"color": "{{if(total < 0, '#dc2626', '#000000')}}"` untuk angka negatif merah, atau `"fill": "{{if(stock < minimum, '#fee2e2', '')}}"`. Rumusnya dihitung per baris (di tabel data: per item) dan ikut ke XLSX dan XLS. Hasil yang bukan `#rrggbb` diabaikan, seperti warna yang salah tulis. Di designer: kolom **colour from data** dan **fill from data** di panel kanan (untuk tabel: warna teks isinya), ditulis tanpa kurung kurawal seperti *show when*: `if(total < 0, '#dc2626', '#000000')`. Mengosongkannya kembali ke warna dari ribbon.

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

**Crosstab (tabel silang)**: elemen `crosstab` dengan `rowField` (satu baris per nilai, misalnya `region`), `columnField` (satu kolom per nilai, misalnya `month`), dan `measure` (isi tiap sel, dihitung dari baris yang punya nilai baris dan kolom itu: `"sum(qty * price):0"`, `"avg(price):2"`, atau `"count()"`, default). Kolom dan baris `Total` (ubah dengan `totalLabel`) serta total keseluruhan ditambahkan otomatis, dan `text` mengisi sel kiri atas. Urutan baris dan kolom mengikuti data, jadi `sortBy` mengaturnya. Di band grup crosstab menghitung baris grup itu, di tempat lain semua baris, atau daftar di `data`. Hasilnya tabel biasa: garis, warna header dan total (`headerFill`, `footerFill`, ...), `stripe`, ganti halaman, dan ekspor XLSX/XLS/CSV berlaku sama. Setiap baris setinggi sepertiga kotak di designer, dan band-nya memanjang. Di designer: tab Insert > **Crosstab**.

**Sub-laporan**: elemen `subreport` dengan `data` (nama daftar, misalnya `lines`) dan `report` (template dengan band sendiri: `header`, `columnHeader`, `groupHeader`... sampai enam tingkat, `detail`, `summary`; `footer` tidak dipakai, dan `sortBy` / `groupBy` boleh diisi). Laporan itu dicetak untuk setiap item di dalam kotak elemen, dan band induknya ikut memanjang. Di band `detail` setiap baris mendapat sub-laporannya sendiri, misalnya setiap pesanan dengan rincian per jenisnya. Field induk tetap bisa dipakai di dalamnya (`{{number}}`), `{{index}}` adalah nomor item. Sub-laporan yang lebih panjang dari satu halaman berlanjut tanpa memotong band, dengan header dan judul kolomnya diulang. Di designer: tab Insert > **Sub-report**, lalu **Edit sub-report…** (atau klik ganda) membuka designer untuk band-bandnya, selebar kotaknya.

**CSV, XLSX, dan XLS**: elemen teks dan sel tabel di band `detail` menjadi kolom. Teks di band lain (judul kolom, subtotal, total) masuk ke kolom yang sejajar dengannya. Di XLSX dan XLS, teks bold tetap bold, `{{f:2}}` menjadi format angka, teks rata tengah atau kanan tetap begitu, sel tabel bergaris tetap bergaris, sel yang digabung ke kanan (`colSpan`) atau ke bawah (`rowSpan`) tetap tergabung, dan lebar kolom mengikuti desain. Sel hanya digabung bila kolom dan baris yang ditutupnya jelas, agar tidak ada nilai yang tersembunyi; CSV tidak mengenal gabungan. Di XLSX, gambar dan barcode/QR ikut sebagai gambar di baris dan kolom yang sejajar dengannya, dan baris itu setinggi gambarnya. Chart hanya ada di PDF dan HTML, dan XLS (format 2003) tidak bisa membawa gambar. `xls` ditulis sebagai XML Spreadsheet 2003; Excel menampilkan satu peringatan format saat membukanya. CSV diawali BOM UTF-8 agar Excel membaca huruf seperti `é` dengan benar; `"csvSeparator": ";"` untuk Excel berbahasa Indonesia atau Eropa yang memakai `;`. Di CSV, teks yang berawalan seperti formula (`=`, `@`, `+`, `-`) diberi tanda kutip tunggal di depan agar tidak dijalankan spreadsheet.

```json
{ "number": "INV-001", "customer": { "name": "Budi" }, "rows": [{ "name": "Kopi", "qty": 2 }] }
```

## PDF/A dan tanda tangan digital

**PDF/A** (untuk arsip jangka panjang): `"pdfA": true` di template, atau centang **PDF/A** di tab Layout. PDF-nya menjadi PDF/A-2b: semua font ada di dalam file (Helvetica, Times, dan Courier dicetak dengan Roboto dan Lora yang dibundel, karena font bawaan PDF tidak disematkan), warna dalam profil sRGB, serta metadata XMP dan ID dokumen yang diminta standar itu. Hasilnya, juga yang ditandatangani dan yang berisi teks Arab, lulus validasi [veraPDF](https://verapdf.org) untuk PDF/A-2b.

**Tanda tangan digital**: setiap PDF ditandatangani bila `DDREPORT_SIGN_CERT` dan `DDREPORT_SIGN_KEY` menunjuk sertifikat dan kunci privat (PEM; RSA atau ECDSA). `DDREPORT_SIGN_REASON` mengisi alasannya. Berlaku untuk CLI, `-serve`, Docker, dan wrapper Java/Node.js. Dari Go: `ddreport.SignPDF(pdf, cert, key, reason)`.

```sh
openssl pkcs12 -in identitas.p12 -nodes -out identitas.pem       # dari .p12 / .pfx
DDREPORT_SIGN_CERT=identitas.pem DDREPORT_SIGN_KEY=identitas.pem DDREPORT_SIGN_REASON="Faktur asli" ./ddreport -serve 127.0.0.1:8080
```

Tanda tangannya tidak terlihat di halaman (`adbe.pkcs7.detached`, SHA-256), ditambahkan sebagai pembaruan inkremental sehingga PDF/A tetap PDF/A; Adobe Reader menampilkannya di panel tanda tangan. Agar pembaca mempercayainya, sertifikat harus dari penerbit yang dipercaya pembaca itu (misalnya sertifikat elektronik dari PSrE untuk Indonesia).

## JasperReports

Template bisa dibawa ke dan dari JasperReports (format `.jrxml` versi 6, yang dibuka Jaspersoft Studio 6 dan 7):

```sh
ddreport -format jrxml < template.json > report.jrxml      # ke JasperReports
ddreport -from-jrxml < report.jrxml > template.json        # dari JasperReports; catatan ke stderr
```

Dari kode: `ddreport.ToJRXML(template)` dan `ddreport.FromJRXML(jrxml)` di Go, format `"jrxml"` di `render` Java/Node.js dan di `POST /render?format=jrxml`, serta `POST /import/jrxml` (balasannya `{"template": ..., "notes": [...]}`). Di aplikasi standalone: **Export > JRXML**, dan **Open…** menerima file `.jrxml`.

**Ke JasperReports**: ekspresi ditulis dalam Groovy (`language="groovy"`, jadi Groovy perlu ada di classpath JasperReports). Ketiga belas template siap pakai diuji di JasperReports 6.21: semuanya dikompilasi, diisi, dan diekspor ke PDF. Field baris menjadi `<field>` bertipe `java.lang.String` dengan path-nya sebagai field description, jadi `JsonDataSource` atas `rows` langsung mengisinya; nilai data di luar baris (`{{title}}`) menjadi parameter. Agregat menjadi variabel yang di-reset per grup, `{{page}}` / `{{pages}}` menjadi nomor halaman, format `:2` dan tanggal ikut, begitu juga teks, tabel (sel per sel), gambar, kotak, garis, barcode/QR (komponen `jr:QRCode` dan `jr:Code128`), grup, urutan, dan opsi band. Chart, sub-laporan, `terbilang`, dan baris berulang tabel dari data tidak punya padanan langsung: semuanya dicatat di komentar di awal file.

**Dari JasperReports**: band, grup (pada field, sampai enam), urutan, static text, text field, kotak, elips (menjadi kotak), garis, frame (isinya dan garisnya), barcode, dan gambar Base64 ikut, dengan font, warna, perataan, dan garis. Ekspresi text field ikut bila terdiri dari teks, field (`$F{}`, `$P{}`), nomor halaman, nomor baris, variabel Sum/Average/Lowest/Highest/Count, dan aritmetika; `pattern` menjadi format angka atau tanggal. Java lain disimpan apa adanya di dalam [kurung] dan dicatat, begitu juga chart, sub-laporan, crosstab, list, dan gambar dari file.

## Backend

**Go**

```go
import ddreport "github.com/gvjavx/DdDesignReport"

pdf, err := ddreport.Render(templateJSON, data, "pdf") // "pdf" | "html" | "xlsx" | "xls" | "csv"
```

Java, Kotlin, dan Node.js memanggil binary `ddreport`. Paketnya membawa binary untuk Windows, Linux, dan macOS (x64 dan arm64); `DDREPORT_BIN` menunjuk binary lain bila perlu. Binary-nya juga ada di setiap [GitHub Release](https://github.com/gvjavx/DdDesignReport/releases).

**Java** (Maven Central, `io.github.gvjavx:ddreport`): tambahkan wrapper-nya dan binary untuk sistem tempat aplikasi berjalan, dengan classifier `linux-x64`, `linux-arm64`, `windows-x64`, `windows-arm64`, `darwin-x64`, atau `darwin-arm64`.

```xml
<dependency><groupId>io.github.gvjavx</groupId><artifactId>ddreport</artifactId><version>0.4.0</version></dependency>
<dependency><groupId>io.github.gvjavx</groupId><artifactId>ddreport</artifactId><version>0.4.0</version><classifier>linux-x64</classifier></dependency>
```

```kotlin
// Gradle
implementation("io.github.gvjavx:ddreport:0.4.0")
runtimeOnly("io.github.gvjavx:ddreport:0.4.0:linux-x64")
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

**HTTP dan Docker** (PHP, Python, .NET, atau bahasa apa pun): `ddreport -serve` juga menerima `POST /render?format=pdf` dengan body `{"template": ..., "data": ...}`. Image Docker-nya ada di setiap rilis mulai 0.3.0:

```sh
docker run -p 8080:8080 -e DDREPORT_TOKEN=rahasia-panjang ghcr.io/gvjavx/ddreport
```

Dengan `DDREPORT_TOKEN`, setiap render harus mengirim `Authorization: Bearer <token>` (tanpa itu jawabannya `401`); designer di `/` tetap terbuka, dan preview-nya memakai token dari alamat halaman: `http://server:8080/#token=rahasia-panjang`. Tanpa token, siapa pun yang bisa menjangkau port itu bisa me-render, jadi `ddreport -serve` memberi peringatan bila alamatnya terbuka ke jaringan. Render berjalan paling banyak sejumlah prosesor sekaligus; permintaan lain menunggu giliran, dan bila sudah empat kali jumlah prosesor yang menunggu atau berjalan, render berikutnya dijawab `503` dengan `Retry-After: 5` agar tidak menunggu bermenit-menit. `GET /healthz` menjawab `ok` tanpa token, untuk load balancer atau pemantauan. Setiap render, impor, dan akses My templates dicatat satu baris di stderr: `POST /render?format=pdf 200 48213B 85ms` (tanpa token dan tanpa isi laporan). `DDREPORT_MAX_ROWS=50000` menolak laporan yang barisnya lebih dari itu dengan pesan error, alih-alih menghabiskan memori server (lihat [Performa](#performa)). Image Docker punya HEALTHCHECK bawaan: `ddreport -healthcheck 127.0.0.1:8080` meminta `/healthz` dan keluar dengan kode 1 bila server tidak menjawab `ok`. Permintaan dari halaman situs lain di browser (header `Origin` yang berbeda) ditolak; panggil dari backend Anda, bukan langsung dari browser. Tanpa token, server hanya melayani alamat `localhost` atau alamat IP (misalnya `http://192.168.1.5:8080`), agar situs lain yang mengarahkan namanya ke mesin Anda (DNS rebinding) tidak bisa membaca atau menghapus My templates; untuk memakai nama domain, isi `DDREPORT_TOKEN`.

PHP (juga di shared hosting yang tidak boleh menjalankan program, selama server ddreport ada di mesin lain):

```php
$ch = curl_init('http://localhost:8080/render?format=pdf');
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_HTTPHEADER => ['Content-Type: application/json', 'Authorization: Bearer ' . getenv('DDREPORT_TOKEN')],
    CURLOPT_POSTFIELDS => json_encode(['template' => $template, 'data' => $data]),
    CURLOPT_RETURNTRANSFER => true,
]);
$pdf = curl_exec($ch);
if (curl_getinfo($ch, CURLINFO_HTTP_CODE) !== 200) {
    throw new RuntimeException('ddreport: ' . $pdf); // pesan kesalahan template ada di body
}
```

Di Laravel: `Http::withToken(env('DDREPORT_TOKEN'))->post('http://localhost:8080/render?format=pdf', ['template' => $template, 'data' => $data])->throw()->body()`. Template boleh dibaca dengan `json_decode($json, true)`: `[]` yang ditulis PHP untuk objek kosong dibaca sebagai objek kosong.

Python:

```python
import os, requests

res = requests.post(
    "http://localhost:8080/render",
    params={"format": "pdf"},
    headers={"Authorization": "Bearer " + os.environ["DDREPORT_TOKEN"]},
    json={"template": template, "data": data},
    timeout=60,
)
res.raise_for_status()  # pesan kesalahan template ada di res.text
pdf = res.content
```

C# (.NET, `System.Net.Http`, `System.Net.Http.Headers`, `System.Text`, `System.Text.Json`):

```csharp
var http = new HttpClient { BaseAddress = new Uri("http://localhost:8080") };
http.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", Environment.GetEnvironmentVariable("DDREPORT_TOKEN"));

// template, data: objects, or JsonElement read with JsonDocument.Parse(...).RootElement
var body = new StringContent(JsonSerializer.Serialize(new { template, data }), Encoding.UTF8, "application/json");
var res = await http.PostAsync("/render?format=pdf", body);
if (!res.IsSuccessStatusCode)
    throw new Exception("ddreport: " + await res.Content.ReadAsStringAsync()); // pesan kesalahan template ada di body
byte[] pdf = await res.Content.ReadAsByteArrayAsync();
```

### Font

`Helvetica`, `Times`, dan `Courier` adalah font bawaan PDF. Selain itu ikut dibundel: **Roboto**, **Open Sans**, **Lato**, **Montserrat**, **Lora**, **Playfair Display**, dan **Great Vibes** (lisensi SIL OFL, di `fonts/`). Font bundel mencakup huruf Latin, Kiril, Yunani, dan Vietnam; di PDF dan HTML huruf yang dipakai ikut disematkan, jadi hasilnya sama di mana pun dibuka.

Font lain bisa didaftarkan dengan file `Nama-Regular.ttf`, `Nama-Bold.ttf`, `Nama-Italic.ttf`, `Nama-BoldItalic.ttf` (hanya Regular yang wajib):

```go
ddreport.AddFonts(os.DirFS("fonts-saya"))                       // atau:
ddreport.Families["Inter"] = &ddreport.Family{Regular: regular, Bold: bold}
```

Untuk binary (Java, Kotlin, Node.js, `-serve`): set `DDREPORT_FONTS` ke folder berisi file-file itu. Agar namanya muncul di designer, berikan opsi `fonts: ['Inter']`. Aplikasi standalone (`-serve`) melakukannya sendiri: semua font di `DDREPORT_FONTS` muncul di menu font dan tergambar di kanvas.

**Paket 100 font tambahan.** Sans, serif, display, tulisan tangan, dan monospace dari Google Fonts (lisensi SIL OFL atau Apache), misalnya Inter, Poppins, Merriweather, Oswald, Dancing Script, dan Roboto Mono. Paket ini tidak ikut di binary (sekitar 39 MB). Unduh sekali ke folder `ddreport-fonts`. Binary memakai folder itu dengan sendirinya bila dijalankan dari folder yang sama dan `DDREPORT_FONTS` tidak di-set:

```sh
node fontpack/download.mjs ddreport-fonts           # daftarnya ada di skrip itu
./ddreport -serve 127.0.0.1:8080                    # 100 font itu langsung ada di menu font
```

Folder lain: `DDREPORT_FONTS=/path/ke/folder`. Font yang baru diunduh muncul setelah `ddreport` dijalankan ulang.

**Memilih font di menu.** Tombol **⋯** di sebelah menu font membuka daftar semua font, masing-masing dengan centang. Hilangkan centangnya agar font itu tidak tampil di menu, centang lagi untuk memunculkannya kembali; ada kolom cari dan tombol **Semua** / **Tidak ada**. Pilihan disimpan di browser. Elemen yang memakai font tersembunyi tetap menampilkan fontnya.

Font hanya dibaca saat sebuah laporan memakainya, jadi folder berisi 100 font tidak memperlambat laporan yang tidak memakainya.

### Font non-Latin di PDF

Font bawaan PDF hanya mencakup huruf Latin. Untuk aksara lain, berikan font TrueType:

```go
ddreport.Font, _ = os.ReadFile("NotoSans-Regular.ttf")
ddreport.FontBold, _ = os.ReadFile("NotoSans-Bold.ttf") // opsional
```

Untuk binary (Java, Kotlin, Node.js, `-serve`): set `DDREPORT_FONT` dan `DDREPORT_FONT_BOLD` ke path file `.ttf`.

**Arab, Ibrani, dan Thai.** Di PDF, huruf Arab disambung sesuai posisinya (awal, tengah, akhir, sendiri, termasuk lam-alif), teks Arab dan Ibrani disusun dari kanan ke kiri dengan angka dan huruf Latin di dalamnya tetap dari kiri ke kanan, dan tanda vokal Thai duduk di atas atau di bawah hurufnya. Pakai font yang memuat bentuk sambung Arab, misalnya **Amiri** atau **Noto Naskh Arabic** dari paket 100 font, dan **Sarabun** untuk Thai; atur perataan teks Arab ke kanan. HTML diserahkan ke browser, yang menyusun teksnya sendiri. Penyusunan teks lain yang lebih rumit (Devanagari, aksara Jawa) belum didukung di PDF.

## Rilis

Tag versi menjalankan `.github/workflows/release.yml`: binary untuk setiap sistem dibangun dan dilampirkan ke GitHub Release, paketnya dicoba, lalu diterbitkan ke npm (`ddreport`, `@ddreport/<sistem>`, `ddreport-designer`) dan Maven Central (`io.github.gvjavx:ddreport`). Langkah terakhirnya mengirim image Docker `ghcr.io/gvjavx/ddreport`. GitHub membuat paket ghcr baru sebagai privat, jadi setelah rilis pertama yang membawanya, ubah sekali ke publik di halaman paketnya (*Package settings*, *Change visibility*).

```sh
git tag vX.Y.Z && git push origin vX.Y.Z
```

npm memakai *trusted publishing*: setiap paket di npmjs.com mencantumkan `gvjavx/DdDesignReport` dan `release.yml` sebagai trusted publisher, sehingga CI menerbitkan tanpa token. Secret `NPM_TOKEN` hanya dibutuhkan untuk versi pertama paket npm yang baru, karena trusted publisher baru bisa dipasang setelah paketnya ada; pasang sementara, lalu hapus lagi setelah trusted publisher-nya terpasang. Untuk Maven dibutuhkan secret `MAVEN_CENTRAL_USERNAME`, `MAVEN_CENTRAL_PASSWORD` (user token Central Portal), `MAVEN_GPG_PRIVATE_KEY`, dan `MAVEN_GPG_PASSPHRASE`. Registry yang secret-nya belum ada dilewati; versi yang sudah terbit tidak diterbitkan lagi. `node release/build.mjs 0.1.0` membangun semuanya di `dist/` secara lokal.

## Performa

Laporan penjualan siap pakai (berkelompok per kategori, dengan subtotal dan dua chart) dengan data buatan, di laptop Intel Core i7-7500U (2 inti, 2017). Waktunya naik lurus dengan jumlah baris.

| Baris | PDF | HTML | XLSX | XLS | CSV |
| --- | --- | --- | --- | --- | --- |
| 10 000 (±250 halaman) | ±1 dtk | ±0,8 dtk | 0,2 dtk | 0,2 dtk | 0,2 dtk |
| 100 000 (±2 500 halaman) | ±12 dtk, memori puncak ±1 GB | ±7 dtk | 2,2 dtk, ±220 MB | 1,8 dtk | 1,6 dtk |

Untuk PDF dan HTML, seluruh halaman disusun di memori sebelum ditulis, jadi laporan ratusan ribu baris butuh memori sebanding; pecah per periode atau per kelompok bila perlu, atau pakai XLSX/CSV untuk ekspor data mentah. Di server, `DDREPORT_MAX_ROWS` membatasi jumlah baris per laporan; dari Go, `ddreport.MaxRows`. Ukur sendiri:

```sh
go test -run '^$' -bench Large -benchmem                       # 10 000 baris, semua format
DDREPORT_BENCH_BIG=1 go test -run '^$' -bench Large -benchmem  # juga 100 000 baris
```

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

`.github/workflows/test.yml` menjalankan semua test Go, Node.js, dan Java di Linux dan Windows, serta test designer di Chromium, Firefox, dan WebKit, di setiap push. CI juga memeriksa kode Go dengan `govulncheck` dan gagal bila ada celah keamanan yang sudah diketahui; jalankan sendiri dengan `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...`.

## Pembuat

DdDesignReport dibuat oleh Naufal Hilmi Abdurrahman. Proyek lain dan jasa pembuatan website serta aplikasi (web, Android, desktop, AI): [aufal-tech.vercel.app](https://aufal-tech.vercel.app/).
