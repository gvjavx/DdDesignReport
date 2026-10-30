# Integrasi dalam 10 menit

Panduan memasang DdDesignReport sebagai fitur di aplikasi Anda: pelanggan mengubah sendiri tata letak faktur, struk, atau slip gajinya, dan aplikasi mencetaknya dengan data dari database Anda.

Contoh lengkapnya, yang bisa langsung dijalankan, ada di [`examples/app`](examples/app): satu server Node.js (±120 baris, tanpa framework) dan satu halaman.

```sh
cd examples/app
npm install
node server.js          # buka http://localhost:3000
```

## Gambaran

Ada empat bagian, dan hanya dua yang perlu Anda tulis sendiri:

```
 browser                               server Anda                          ddreport
┌─────────────────────┐  simpan     ┌──────────────────────────┐
│ designer            │ ──────────► │ tabel template:          │
│ (ddreport-designer) │  JSON       │ (pelanggan, laporan) →   │
│                     │             │ JSON template            │
│ tombol Preview      │ ──────────► │                          │  template + data   ┌──────────┐
│ tombol Cetak / PDF  │ ──────────► │ query data laporan  ─────┼──────────────────► │ render   │
└─────────────────────┘ ◄────────── │                          │ ◄──────────────────│ PDF, ... │
                         dokumen    └──────────────────────────┘     dokumen        └──────────┘
```

1. **Designer** di halaman: satu `createDesigner(...)`.
2. **Penyimpanan template**: satu baris per pelanggan dan laporan. Ini milik Anda.
3. **Data laporan**: query yang sudah Anda punya, dibentuk seperti yang dibaca template. Ini juga milik Anda.
4. **Render**: satu panggilan, dari Node.js, Java, Kotlin, Go, atau HTTP untuk PHP, Python, dan .NET.

Template disimpan di server Anda dan dikirim ke ddreport bersama datanya setiap kali dicetak; render sendiri tidak menyimpan apa pun.

## 1. Designer di halaman

```sh
npm install ddreport-designer
```

```js
import { createDesigner } from 'ddreport-designer'

const designer = createDesigner(document.getElementById('designer'), {
  lang: 'id',
  simple: true,                       // untuk admin dan staf: alat yang perlu saja, setiap tombol bernama
  preview: async template =>          // tombol Preview: server Anda mencetaknya sebagai HTML
    (await fetch('/api/preview/invoice', { method: 'POST', body: JSON.stringify(template) })).text(),
})

const { template, data } = await (await fetch('/api/templates/invoice')).json()
designer.setTemplate(template)
designer.setData(data)               // contoh data: chart di kanvas, menu Field, dan peringatan field yang salah ketik
```

Tanpa bundler, sajikan `node_modules/ddreport-designer/designer.js` sebagai file statis lalu `import { createDesigner } from '/designer.js'`, seperti di contoh. Untuk React dan Vue: [README paket designer](release/designer-README.md).

Di Laravel atau framework lain tanpa npm: unduh `https://cdn.jsdelivr.net/npm/ddreport-designer@0.4.3/designer.js` ke folder `public/`, lalu `import { createDesigner } from '/designer.js'`. Satu file, tanpa dependency.

## 2. Menyimpan template per pelanggan

Satu tabel cukup:

```sql
CREATE TABLE report_templates (
  customer_id BIGINT       NOT NULL,
  report      VARCHAR(50)  NOT NULL,   -- 'invoice', 'receipt', ...
  template    JSON         NOT NULL,
  updated_at  TIMESTAMP    NOT NULL,
  PRIMARY KEY (customer_id, report)
);
```

Selama pelanggan belum menyimpan tata letaknya sendiri, pakai template siap pakai (`ddreport-designer/templates/invoice.json`, atau buatan Anda). Saat menyimpan, coba render dulu dengan datanya, agar template yang tidak bisa dicetak ditolak dengan pesan yang jelas:

```js
await renderer.render(template, data, 'html') // melempar error dengan penjelasan bila template rusak
db.save(customerId, 'invoice', template)
```

`customerId` selalu diambil dari sesi login di server, bukan dari permintaan browser, agar pelanggan tidak bisa membuka template pelanggan lain.

## 3. Bentuk data

Template membaca satu objek: field biasa untuk kop dan total, ditambah daftar `rows` yang diulang di band **detail**.

```json
{
  "number": "INV-0042",
  "customer": { "name": "Toko Maju", "city": "Sleman" },
  "items": [ { "name": "Kopi", "qty": 2, "price": 18000 } ],
  "rows":  [ { "name": "Kopi", "qty": 2, "price": 18000 } ]
}
```

| Di template | Dibaca dari |
| --- | --- |
| `{{number}}`, `{{customer.city}}` | field dan field bersarang |
| band detail | setiap item `rows` (nama lain: `"dataKey"`) |
| tabel atau sub-laporan dengan **Data** `items` | setiap item daftar `items` |
| `{{sum(qty * price)}}`, `{{count()}}` | dihitung dari baris yang sedang dicetak |

Faktur biasanya dua query: kepala faktur menjadi field, baris barangnya menjadi `rows`. Angka yang dikirim database sebagai teks (`"18000.00"`) tetap dihitung sebagai angka.

### Field yang salah ketik

Placeholder yang field-nya tidak ada di data tercetak kosong, tanpa error. Ada tiga cara menangkapnya:

- **Preview di designer** menyebutnya: "Tidak ada di data contoh, tercetak kosong: nmae", selama designer diberi contoh data (`data`) atau daftar field (`fields`).
- **`DDREPORT_STRICT=1`** di lingkungan binary atau server membuat render gagal dengan nama field-nya, alih-alih mencetak kosong. Cocok untuk test dan staging.
- Dari Go: `ddreport.Missing(template, data)` mengembalikan daftarnya.

## 4. Render

Node.js (`npm install ddreport`, binary untuk sistemnya ikut terpasang):

```js
const { createRenderer } = require('ddreport')
const renderer = createRenderer()   // satu proses yang terus berjalan

app.get('/reports/invoice/:id.pdf', async (req, res) => {
  const template = await db.template(req.user.customerId, 'invoice')
  const data = await invoiceData(req.params.id)          // query Anda
  res.type('pdf').send(await renderer.render(template, data, 'pdf'))
})
```

Format lain: `'xlsx'`, `'xls'`, `'csv'`, `'html'`, `'escpos'` (printer struk), `'txt'`.

Laravel, lewat server ddreport (Docker) di mesin yang sama atau lain:

```sh
docker run -d -p 8080:8080 -e DDREPORT_TOKEN=rahasia-panjang ghcr.io/gvjavx/ddreport
```

```php
// routes/web.php
Route::get('/reports/invoice/{invoice}.pdf', function (Invoice $invoice) {
    $template = ReportTemplate::for(auth()->user()->customer_id, 'invoice');   // JSON tersimpan
    $data = [
        'number'   => $invoice->number,
        'customer' => ['name' => $invoice->customer->name],
        'rows'     => $invoice->items->map->only(['name', 'qty', 'price']),
    ];
    $pdf = Http::withToken(env('DDREPORT_TOKEN'))
        ->post('http://localhost:8080/render?format=pdf', ['template' => json_decode($template), 'data' => $data])
        ->throw()->body();
    return response($pdf)->header('Content-Type', 'application/pdf');
});

// Preview dari designer: template yang sedang diubah, dengan data sungguhan
Route::post('/api/preview/invoice', fn (Request $r) => Http::withToken(env('DDREPORT_TOKEN'))
    ->post('http://localhost:8080/render?format=html', ['template' => $r->json()->all(), 'data' => /* data contoh */ []])
    ->throw()->body());
```

Java dan Kotlin: `DdReport.render(templateJson, dataJson, "pdf")`, atau `new DdReport.Renderer()` di server. Go: `ddreport.Render(template, data, "pdf")`. Selengkapnya di [README](README.md#backend).

## 5. Cetak ke printer struk

`escpos` menghasilkan perintah printer, bukan gambar. Browser tidak bisa mengirimnya ke printer begitu saja; pilih jalur yang cocok dengan printernya:

| Printer | Jalur |
| --- | --- |
| Printer jaringan (LAN/Wi-Fi), hampir semua printer kasir punya port 9100 | Server mengirim byte-nya ke `ip-printer:9100`. Contoh: route `/api/print` di `examples/app/server.js` (`net.connect`). Di PHP: `fsockopen($ip, 9100)` lalu `fwrite`. |
| Bluetooth, atau USB yang muncul sebagai port serial | Browser memakai Web Serial (Chrome dan Edge di komputer). Contoh: tombol **Cetak struk (USB/Bluetooth)** di `examples/app/index.html`. |
| USB yang terpasang sebagai printer Windows | Bagikan printernya (Sharing), lalu dari komputer kasir: `copy /b struk.escpos \\localhost\NamaPrinter`, atau pakai agen cetak lokal. Browser tidak bisa menjangkaunya langsung. |
| Android (Flutter, Kotlin) | Kirim byte `escpos` dari server ke aplikasi, lalu tulis ke printer lewat Bluetooth seperti perintah ESC/POS lainnya. |

Lebar baris mengikuti lebar kertas di template (Struk 58 mm = 32 karakter, 80 mm = 48); ubah dengan **Karakter per baris** bila printer Anda berbeda.

## Masalah yang sering muncul

| Gejala | Sebab dan jalan keluar |
| --- | --- |
| Field tercetak kosong | Nama field salah ketik atau tidak ada di hasil query. Lihat peringatan di Preview, atau jalankan dengan `DDREPORT_STRICT=1`. |
| Node.js: `no ddreport binary for linux-x64` | Paket binary untuk sistem itu tidak ikut terpasang: `npm install` dijalankan dengan `--omit=optional`, atau `node_modules` disalin dari sistem lain (Windows ke server Linux, atau ke image Docker). Jalankan `npm install` di sistem tujuan, atau set `DDREPORT_BIN`. |
| Java: `Add the dependency io.github.gvjavx:ddreport with the classifier of this system` | Tambahkan dependency kedua dengan classifier sistem **server** (`linux-x64`), bukan laptop Anda. Untuk beberapa sistem, tambahkan beberapa classifier. Nama package Java-nya `io.ddreport` walau groupId-nya `io.github.gvjavx`. |
| HTTP `401 a valid token is needed` | Server dijalankan dengan `DDREPORT_TOKEN`: kirim `Authorization: Bearer <token>`. |
| HTTP `403 requests from other sites are refused` | Render dipanggil langsung dari JavaScript browser. Panggil dari backend Anda; token tidak boleh sampai ke browser. |
| HTTP `403 set DDREPORT_TOKEN to use this server by a name other than localhost` | Server tanpa token hanya melayani `localhost` dan alamat IP. Untuk nama domain, isi `DDREPORT_TOKEN`. |
| HTTP `503` dengan `Retry-After` | Antrean render penuh (empat kali jumlah prosesor). Ulangi sebentar lagi, atau tambah server. |
| Font di kanvas berbeda dari PDF | Font tambahan harus terdaftar di dua tempat: folder `DDREPORT_FONTS` di server, dan `fonts` + `fontBase` di designer. Lihat [Font](README.md#font). |
| Serverless (Vercel, Netlify, Lambda) | Menjalankan binary sering dibatasi atau lambat saat mulai. Jalankan server ddreport (Docker) di tempat lain dan panggil lewat HTTP, atau pakai Go langsung. |
| Laporan besar memakan memori | PDF 100 000 baris butuh ±500 MB. Batasi dengan `DDREPORT_MAX_ROWS`, bagi per periode, atau pakai XLSX/CSV untuk data mentah. |

Butuh dibantu memasangnya? Lihat [jasa](https://gvjavx.github.io/DdDesignReport-website/#jasa).
