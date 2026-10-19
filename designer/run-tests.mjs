// Runs the designer tests (test.html) in Chromium, Firefox and WebKit, the
// engine of Safari, and fails when a check fails in any of them:
//   npm install --no-save playwright && npx playwright install --with-deps chromium firefox webkit
//   node designer/run-tests.mjs            (or name the browsers: node designer/run-tests.mjs firefox)
// BROWSER_PATH runs one installed browser instead, e.g. Edge for Chromium.
import http from 'node:http'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const { chromium, firefox, webkit } = await import('playwright')
const dir = path.dirname(fileURLToPath(import.meta.url))
const types = { '.html': 'text/html', '.js': 'text/javascript', '.json': 'application/json', '.ttf': 'font/ttf' }

// the designer folder, as a web server gives it (modules do not load from file://)
const server = http.createServer((req, res) => {
  const file = path.join(dir, decodeURIComponent(new URL(req.url, 'http://x').pathname))
  if (!file.startsWith(dir)) return res.writeHead(403).end()
  fs.readFile(file, (err, body) => {
    if (err) return res.writeHead(404).end()
    res.writeHead(200, { 'content-type': types[path.extname(file)] ?? 'application/octet-stream' }).end(body)
  })
})
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
const url = `http://127.0.0.1:${server.address().port}/test.html`

let failed = false
const names = process.argv.slice(2).length ? process.argv.slice(2) : ['chromium', 'firefox', 'webkit']
for (const name of names) {
  const browser = await { chromium, firefox, webkit }[name].launch(process.env.BROWSER_PATH ? { executablePath: process.env.BROWSER_PATH } : {})
  // a window as large as a desktop's: some checks drag elements across the page
  const page = await browser.newPage({ viewport: { width: 1600, height: 1000 } })
  page.on('pageerror', err => console.log(`${name}: page error: ${err.message}`))
  await page.goto(url)
  const result = await (await page.waitForFunction(() => window.result, null, { timeout: 120000 })).jsonValue()
  console.log(`${name}: ${result.total - result.failed} of ${result.total} checks passed`)
  for (const f of result.failures) console.log(`  ${f}`)
  failed ||= result.failed > 0
  await browser.close()
}
server.close()
process.exit(failed ? 1 : 0)
