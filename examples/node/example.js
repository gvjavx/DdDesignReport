// Run from the repository root: node examples/node/example.js
// Needs the ddreport binary: set DDREPORT_BIN or put it on PATH.
const fs = require('node:fs')
const { render } = require('../../node')

const template = JSON.parse(fs.readFileSync('examples/sales.json', 'utf8'))
const data = JSON.parse(fs.readFileSync('examples/data.json', 'utf8'))

fs.mkdirSync('examples/out', { recursive: true })
for (const format of ['pdf', 'html', 'xlsx', 'csv']) {
  const name = `examples/out/node-sales.${format}`
  fs.writeFileSync(name, render(template, data, format))
  console.log('wrote', name)
}
