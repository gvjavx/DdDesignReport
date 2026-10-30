// A report feature in a Node.js app, end to end: each customer edits their own
// layouts in the designer, the app keeps one template per customer, and prints
// it with data from its own database as PDF, Excel, or on a receipt printer.
//
//   npm install
//   node server.js          then open http://localhost:3000
//
// Only node:http, so the routes carry over to Express, Fastify or Nest as they are.
const http = require('node:http')
const fs = require('node:fs')
const path = require('node:path')
const net = require('node:net')
const { createRenderer } = require('ddreport')

// one ddreport process kept running, instead of starting one for every report
const renderer = createRenderer()
const designerDir = path.dirname(require.resolve('ddreport-designer'))
const saved = path.join(__dirname, 'saved') // in your app: a table, (customer, report) -> template JSON

// Who is asking. In your app: the customer of the logged-in user, from the session.
const customerOf = req => 'demo'

// The data a report prints. In your app: your database query, shaped as the
// template expects, fields plus a "rows" list. Here: the template's sample data.
async function reportData(id, customer) {
  return bundled(id).data
}

const bundled = id => JSON.parse(fs.readFileSync(path.join(designerDir, 'templates', id + '.json'), 'utf8'))
const file = (customer, id) => path.join(saved, customer, id + '.json')

// the customer's own layout, or the ready-made one until they save theirs
function template(customer, id) {
  try {
    return JSON.parse(fs.readFileSync(file(customer, id), 'utf8'))
  } catch {
    return bundled(id).template
  }
}

const types = { pdf: 'application/pdf', xlsx: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', html: 'text/html; charset=utf-8', escpos: 'application/octet-stream' }

async function handle(req, res) {
  const url = new URL(req.url, 'http://x')
  const customer = customerOf(req)
  const [, area, id, ext] = url.pathname.match(/^\/(api\/templates|api\/preview|reports|api\/print)\/([a-z0-9-]+)(?:\.(\w+))?$/) ?? []
  // ids name files: only the ready-made templates, nothing like ../
  if (id && !fs.existsSync(path.join(designerDir, 'templates', id + '.json'))) return send(res, 404, 'no such report')

  if (url.pathname === '/') return send(res, 200, fs.readFileSync(path.join(__dirname, 'index.html')), 'text/html; charset=utf-8')
  if (url.pathname === '/designer.js') return send(res, 200, fs.readFileSync(path.join(designerDir, 'designer.js')), 'text/javascript')
  if (url.pathname === '/api/templates') return send(res, 200, fs.readFileSync(path.join(designerDir, 'templates', 'index.json')), 'application/json')

  // the designer opens the customer's layout, with data to draw charts and fill the Field menu
  if (area === 'api/templates' && req.method === 'GET') {
    return send(res, 200, JSON.stringify({ template: template(customer, id), data: await reportData(id, customer) }), 'application/json')
  }
  // Save
  if (area === 'api/templates' && req.method === 'PUT') {
    const tpl = JSON.parse(await body(req))
    await renderer.render(tpl, await reportData(id, customer), 'html') // refuse a layout that does not print
    fs.mkdirSync(path.dirname(file(customer, id)), { recursive: true })
    fs.writeFileSync(file(customer, id), JSON.stringify(tpl))
    return send(res, 204)
  }
  // Preview in the designer: the layout being edited, not saved yet, with the real data
  if (area === 'api/preview' && req.method === 'POST') {
    const doc = await renderer.render(JSON.parse(await body(req)), await reportData(id, customer), 'html')
    return send(res, 200, doc, types.html)
  }
  // the report itself: /reports/invoice.pdf, /reports/invoice.xlsx, /reports/receipt.escpos
  if (area === 'reports' && types[ext]) {
    const doc = await renderer.render(template(customer, id), await reportData(id, customer), ext)
    res.setHeader('Content-Disposition', `${ext === 'pdf' ? 'inline' : 'attachment'}; filename="${id}.${ext}"`)
    return send(res, 200, doc, types[ext])
  }
  // a receipt to the printer on the shop's network (PRINTER=192.168.1.50), port 9100 like most of them
  if (area === 'api/print' && req.method === 'POST') {
    if (!process.env.PRINTER) return send(res, 400, 'set PRINTER to the address of the receipt printer')
    const doc = await renderer.render(template(customer, id), await reportData(id, customer), 'escpos')
    await new Promise((ok, fail) => {
      const s = net.connect({ host: process.env.PRINTER, port: 9100, timeout: 5000 }, () => s.end(doc, ok))
      s.on('error', fail).on('timeout', () => s.destroy(new Error('the printer does not answer')))
    })
    return send(res, 204)
  }
  send(res, 404, 'not found')
}

function send(res, status, content = '', type = 'text/plain; charset=utf-8') {
  res.writeHead(status, { 'Content-Type': type }).end(content)
}

// a request body, at most 10 MB
async function body(req) {
  let size = 0
  const parts = []
  for await (const part of req) {
    if ((size += part.length) > 10 << 20) throw new Error('too large')
    parts.push(part)
  }
  return Buffer.concat(parts).toString()
}

http.createServer((req, res) => handle(req, res).catch(err => {
  // a template mistake comes back as a message the designer shows
  if (!res.headersSent) send(res, 400, err.message)
})).listen(process.env.PORT ?? 3000, '127.0.0.1', () => console.log('http://localhost:' + (process.env.PORT ?? 3000)))
