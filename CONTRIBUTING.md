# Berkontribusi

Issue dan pull request diterima dalam bahasa Indonesia atau Inggris.

1. `go test ./...` harus lulus. Perubahan di designer juga diuji dengan `node designer/run-tests.mjs` (butuh Playwright: `npm install --no-save playwright` lalu `npx playwright install`).
2. Template baru untuk galeri: lihat bagian galeri di [README.md](README.md).
3. Setiap commit ditandatangani dengan [Developer Certificate of Origin](https://developercertificate.org/): tambahkan `-s` saat commit.

   ```sh
   git commit -s -m "Pesan commit"
   ```

   Baris `Signed-off-by: Nama <email>` itu menyatakan Anda berhak mengirim kode tersebut di bawah lisensi MIT project ini. Tidak ada CLA yang perlu ditandatangani.

---

**English**: issues and pull requests are welcome in English. Run `go test ./...`, and sign off every commit with `git commit -s` ([DCO](https://developercertificate.org/)); contributions are accepted under the project's MIT license. No CLA.
