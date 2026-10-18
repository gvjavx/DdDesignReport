// node --test node/test.js
// Needs the ddreport binary: set DDREPORT_BIN or put it on PATH.
const test = require('node:test')
const assert = require('node:assert')
const { render } = require('.')

const template = {
  bands: {
    columnHeader: { height: 20, elements: [{ type: 'text', x: 0, y: 0, w: 100, h: 14, text: 'Name', bold: true }] },
    detail: { height: 20, elements: [{ type: 'text', x: 0, y: 0, w: 100, h: 14, text: '{{name}}' }] },
  },
}
const data = { rows: [{ name: 'Kopi' }, { name: 'Téh "manis"' }] }

test('renders every format', () => {
  assert.strictEqual(render(template, data, 'csv').toString(), 'Name\nKopi\n"Téh ""manis"""\n')
  assert.ok(render(template, data, 'pdf').subarray(0, 5).equals(Buffer.from('%PDF-')))
  assert.ok(render(template, data).subarray(0, 5).equals(Buffer.from('%PDF-')), 'pdf is the default')
  assert.match(render(template, data, 'html').toString(), /Téh &#34;manis&#34;/)
  assert.ok(render(template, data, 'xlsx').subarray(0, 2).equals(Buffer.from('PK')))
  assert.match(render(template, data, 'xls').toString(), /<Data ss:Type="String">Kopi<\/Data>/)
})

test('accepts an array of rows and no data at all', () => {
  assert.strictEqual(render(template, data.rows, 'csv').toString(), 'Name\nKopi\n"Téh ""manis"""\n')
  assert.strictEqual(render(template, undefined, 'csv').toString(), 'Name\n')
})

test('throws the message from the binary', () => {
  assert.throws(() => render(template, data, 'docx'), /unknown format "docx"/)
  assert.throws(() => render({ page: { width: 100, height: 100 }, bands: { detail: { height: 500 } } }, data, 'pdf'), /do not fit/)
})

test('throws when the binary is missing', () => {
  const bin = process.env.DDREPORT_BIN
  process.env.DDREPORT_BIN = 'no-such-ddreport-binary'
  try {
    assert.throws(() => render(template, data, 'csv'), /ENOENT/)
  } finally {
    if (bin === undefined) delete process.env.DDREPORT_BIN
    else process.env.DDREPORT_BIN = bin
  }
})

test('renderAsync gives the same output without blocking', async () => {
  const { renderAsync } = require('.')
  assert.strictEqual((await renderAsync(template, data, 'csv')).toString(), render(template, data, 'csv').toString())
  await assert.rejects(renderAsync(template, data, 'docx'), /unknown format "docx"/)
})
