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

// touch drives the standalone designer on a phone-sized touch screen with real
// touch input, as the browser delivers it: taps everywhere, and in Chromium,
// whose touch input can be sent finger by finger, drags and a pinch as well.
async function touch(browser, name) {
  const context = await browser.newContext({ viewport: { width: 390, height: 800 }, hasTouch: true, isMobile: name !== 'firefox' })
  const page = await context.newPage(), checks = []
  const check = (what, ok, detail = '') => checks.push([what, ok, detail])
  page.on('pageerror', err => check('no page error', false, err.message))
  await page.goto(url.replace('test.html', 'index.html'))
  await page.waitForSelector('.ddr-hint')
  const els = () => page.locator('.ddr-el').count()
  check('fits the phone: no sideways scrolling', await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
  check('properties folded away, the page first', await page.evaluate(() => !document.querySelector('.ddr-side').open))
  await page.tap('.ddr-hint [data-cmd="add:text"]')
  check('tap a start button: a text box', await els() === 1)
  await page.tap('[data-tab=Insert]')
  check('tap a tab: it opens', await page.locator('[data-panel=Insert]').isVisible())
  if (name === 'chromium') {
    const cdp = await context.newCDPSession(page)
    const fingers = async (type, points) => cdp.send('Input.dispatchTouchEvent', { type, touchPoints: points.map(([x, y], id) => ({ x, y, id })) })
    const drag = async (from, by, steps = 8) => {
      await fingers('touchStart', [from])
      for (let i = 1; i <= steps; i++) await fingers('touchMove', [[from[0] + by[0] * i / steps, from[1] + by[1] * i / steps]])
      await fingers('touchEnd', [])
    }
    const box = async s => page.locator(s).first().boundingBox()
    const style = s => page.locator(s).first().evaluate(n => [parseFloat(n.style.left), parseFloat(n.style.width)])
    let b = await box('.ddr-el'), before = await style('.ddr-el')
    await drag([b.x + 10, b.y + b.height / 2], [60, 30])
    let after = await style('.ddr-el')
    check('drag with a finger: the element moves', after[0] > before[0] + 20, `${before[0]} -> ${after[0]}`)
    b = await box('.ddr-handle')
    await drag([b.x + b.width / 2, b.y + b.height / 2], [50, 0])
    const wider = await style('.ddr-el')
    check('drag the handle: the element grows', wider[1] > after[1] + 20, `${after[1]} -> ${wider[1]}`)
    const zoom = () => page.locator('[data-id=zoom]').textContent()
    const z0 = await zoom(), s = await box('.ddr-stage'), cx = s.x + s.width / 2, cy = s.y + s.height / 2
    await fingers('touchStart', [[cx - 30, cy], [cx + 30, cy]])
    for (let i = 1; i <= 8; i++) await fingers('touchMove', [[cx - 30 - i * 10, cy], [cx + 30 + i * 10, cy]])
    await fingers('touchEnd', [])
    check('pinch with two fingers: it zooms in', parseInt(await zoom()) > parseInt(z0), `${z0} -> ${await zoom()}`)
    check('a pinch moves nothing', (await style('.ddr-el'))[0] === wider[0])
  }
  await page.locator('.ddr-x').tap({ timeout: 3000 }).catch(() => check('the selected element still has its red cross', false))
  check('tap the red cross: the element goes', await els() === 0)
  await context.close()
  return checks
}

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
  if (name !== 'firefox') { // Firefox in Playwright has no phone emulation
    const checks = await touch(browser, name), bad = checks.filter(c => !c[1])
    console.log(`${name}, touch screen: ${checks.length - bad.length} of ${checks.length} checks passed`)
    for (const [what, , detail] of bad) console.log(`  ✗ ${what}${detail ? ' — ' + detail : ''}`)
    failed ||= bad.length > 0
  }
  await browser.close()
}
server.close()
process.exit(failed ? 1 : 0)
