const { spawn, spawnSync } = require('node:child_process')

const bin = () => process.env.DDREPORT_BIN || 'ddreport'
const fail = (status, stderr) => new Error(stderr.toString().trim() || `ddreport exited with ${status}`)

/**
 * Renders a report through the `ddreport` binary. Blocks the event loop until
 * it is done; servers should use renderAsync.
 * @param {object} template template exported by the designer
 * @param {object|Array} data fields plus the row array, or just the rows
 * @param {'pdf'|'html'|'xlsx'|'xls'|'csv'} format
 * @returns {Buffer}
 */
exports.render = (template, data, format = 'pdf') => {
  const r = spawnSync(bin(), ['-format', format], {
    input: JSON.stringify({ template, data }),
    maxBuffer: Infinity,
  })
  if (r.error) throw r.error
  if (r.status !== 0) throw fail(r.status, r.stderr)
  return r.stdout
}

/**
 * Like render, without blocking the event loop.
 * @returns {Promise<Buffer>}
 */
exports.renderAsync = (template, data, format = 'pdf') =>
  new Promise((resolve, reject) => {
    const p = spawn(bin(), ['-format', format])
    const out = []
    const err = []
    p.stdout.on('data', c => out.push(c))
    p.stderr.on('data', c => err.push(c))
    p.stdin.on('error', () => {}) // a binary that exits early closes stdin; its exit code tells why
    p.on('error', reject)
    p.on('close', status => (status === 0 ? resolve(Buffer.concat(out)) : reject(fail(status, Buffer.concat(err)))))
    p.stdin.end(JSON.stringify({ template, data }))
  })
