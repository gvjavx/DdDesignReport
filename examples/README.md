# Contoh

## Aplikasi lengkap (`app/`)

Fitur laporan di sebuah aplikasi Node.js, dari awal sampai akhir: designer di halaman, template per pelanggan, preview dengan data sungguhan, unduh PDF dan Excel, cetak struk ke printer jaringan atau lewat Web Serial. Penjelasan langkah demi langkahnya di [INTEGRASI.md](../INTEGRASI.md).

```sh
cd examples/app
npm install
node server.js                 # buka http://localhost:3000
PRINTER=192.168.1.50 node server.js   # dengan printer struk jaringan (Git Bash; PowerShell: $env:PRINTER = "...")
```

## Laporan penjualan

Laporan penjualan dengan grouping, subtotal, teks yang membungkus, dan chart.

- `sales.json`: template (bisa dibuka di designer lewat **Open…**)
- `data.json`: data contoh

Semua perintah dijalankan dari root repository. Perintahnya berlaku di PowerShell dan Git Bash, kecuali yang diberi catatan. Hasilnya masuk ke `examples/out/`.

## Go

```powershell
go run ./examples/go
```

## Binary (dipakai Node.js, Java, Kotlin, dan designer)

```powershell
go build -o examples/out/ddreport.exe ./cmd/ddreport
$env:DDREPORT_BIN = "$PWD\examples\out\ddreport.exe"
```

Di Git Bash, baris kedua diganti dengan:

```bash
export DDREPORT_BIN="$PWD/examples/out/ddreport.exe"
```

## Node.js

```powershell
node examples/node/example.js
```

## Java

```powershell
javac -d examples/out/classes java/io/ddreport/DdReport.java examples/java/Example.java
java -cp examples/out/classes Example
```

## Kotlin

Butuh class `DdReport` dari langkah Java di atas.

```powershell
kotlinc examples/kotlin/Example.kt -cp examples/out/classes -include-runtime -d examples/out/example.jar
java -cp "examples/out/example.jar;examples/out/classes" ExampleKt
```

## Designer dan preview

```powershell
examples/out/ddreport.exe -serve 127.0.0.1:8080
```

Buka http://127.0.0.1:8080, klik **Open…** dan pilih `examples/sales.json`. Tempel isi `examples/data.json` ke **Sample data for the preview**, lalu klik **Preview**.

## HTTP

Dengan `-serve` masih berjalan, di terminal lain (PowerShell):

```powershell
$body = '{"template":' + (Get-Content examples/sales.json -Raw) + ',"data":' + (Get-Content examples/data.json -Raw) + '}'
Invoke-WebRequest "http://127.0.0.1:8080/render?format=pdf" -Method Post -Body ([Text.Encoding]::UTF8.GetBytes($body)) -OutFile examples/out/http-sales.pdf
```
