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

const bom = String.fromCharCode(0xfeff) // CSV starts with the byte order mark Excel needs

test('renders every format', () => {
  assert.strictEqual(render(template, data, 'csv').toString(), bom + 'Name\nKopi\n"Téh ""manis"""\n')
  assert.ok(render(template, data, 'pdf').subarray(0, 5).equals(Buffer.from('%PDF-')))
  assert.ok(render(template, data).subarray(0, 5).equals(Buffer.from('%PDF-')), 'pdf is the default')
  assert.match(render(template, data, 'html').toString(), /Téh &#34;manis&#34;/)
  assert.ok(render(template, data, 'xlsx').subarray(0, 2).equals(Buffer.from('PK')))
  assert.match(render(template, data, 'xls').toString(), /<Data ss:Type="String">Kopi<\/Data>/)
})

test('accepts an array of rows and no data at all', () => {
  assert.strictEqual(render(template, data.rows, 'csv').toString(), bom + 'Name\nKopi\n"Téh ""manis"""\n')
  assert.strictEqual(render(template, undefined, 'csv').toString(), bom + 'Name\n')
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

test('a render that takes too long is stopped', async () => {
  // fifty thousand rows take seconds; a limit of 100 ms stops them, on a fast machine too
  const { renderAsync } = require('.')
  const many = Array.from({ length: 50000 }, () => ({ name: 'x' }))
  assert.throws(() => render(template, many, 'pdf', { timeout: 100 }), /longer than 100 ms/)
  await assert.rejects(renderAsync(template, many, 'pdf', { timeout: 100 }), /longer than 100 ms/)
})

test('createRenderer keeps processes running and gives the same output', async () => {
  const { createRenderer } = require('.')
  const renderer = createRenderer({ processes: 2 })
  try {
    const want = render(template, data, 'csv').toString()
    // more reports than processes: they wait their turn, and come back in the right order
    const all = await Promise.all(Array.from({ length: 5 }, (_, i) => renderer.render(template, { rows: [{ name: 'n' + i }] }, 'csv')))
    all.forEach((b, i) => assert.strictEqual(b.toString(), bom + 'Name\nn' + i + '\n'))
    assert.strictEqual((await renderer.render(template, data, 'csv')).toString(), want)
    assert.ok((await renderer.render(template, data)).subarray(0, 5).equals(Buffer.from('%PDF-')), 'pdf is the default')
    // an error fails only its own report
    await assert.rejects(renderer.render(template, data, 'docx'), /unknown format "docx"/)
    await assert.rejects(renderer.render(template, data, 'csv 2\n{}'), /unknown format/) // cannot pass for a request line
    assert.strictEqual((await renderer.render(template, data, 'csv')).toString(), want)
  } finally {
    renderer.close()
  }
  await assert.rejects(renderer.render(template, data, 'csv'), /closed/)
})

test('createRenderer replaces a process that takes too long', async () => {
  const { createRenderer } = require('.')
  const renderer = createRenderer({ timeout: 100 })
  try {
    const many = Array.from({ length: 50000 }, () => ({ name: 'x' }))
    await assert.rejects(renderer.render(template, many, 'pdf'), /longer than 100 ms/)
    assert.strictEqual((await renderer.render(template, data, 'csv')).toString(), bom + 'Name\nKopi\n"Téh ""manis"""\n')
  } finally {
    renderer.close()
  }
})

test('createRenderer says when the binary is missing', async () => {
  const { createRenderer } = require('.')
  const bin = process.env.DDREPORT_BIN
  process.env.DDREPORT_BIN = 'no-such-ddreport-binary'
  try {
    const renderer = createRenderer()
    await assert.rejects(Promise.all([renderer.render(template, data, 'csv'), renderer.render(template, data, 'csv')]), /ENOENT/)
    renderer.close()
  } finally {
    if (bin === undefined) delete process.env.DDREPORT_BIN
    else process.env.DDREPORT_BIN = bin
  }
})
