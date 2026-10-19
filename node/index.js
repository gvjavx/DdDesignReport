const { spawn, spawnSync } = require('node:child_process')

// the binary: DDREPORT_BIN, else the one installed for this system with the package
// (ddreport-linux-x64, ddreport-win32-x64, ddreport-darwin-arm64...), else ddreport on the PATH
const bin = () => {
  if (process.env.DDREPORT_BIN) return process.env.DDREPORT_BIN
  try {
    return require.resolve(`ddreport-${process.platform}-${process.arch}/bin/ddreport${process.platform === 'win32' ? '.exe' : ''}`)
  } catch {
    return 'ddreport'
  }
}
// a binary that cannot be found says how to get one
const hint = err => {
  if (err.code === 'ENOENT') err.message += `: no ddreport binary for ${process.platform}-${process.arch}; reinstall ddreport with its optional dependencies, or set DDREPORT_BIN`
  return err
}
const fail = (status, stderr) => new Error(stderr.toString().trim() || `ddreport exited with ${status}`)

/**
 * Renders a report through the `ddreport` binary. Blocks the event loop until
 * it is done; servers should use renderAsync.
 * @param {object} template template exported by the designer
 * @param {object|Array} data fields plus the row array, or just the rows
 * @param {'pdf'|'html'|'xlsx'|'xls'|'csv'} format
 * @param {{timeout?: number}} options timeout in milliseconds, default 60000; 0 for none
 * @returns {Buffer}
 */
exports.render = (template, data, format = 'pdf', { timeout = 60000 } = {}) => {
  const r = spawnSync(bin(), ['-format', format], {
    input: JSON.stringify({ template, data }),
    maxBuffer: Infinity,
    timeout: timeout || undefined,
  })
  if (r.error?.code === 'ETIMEDOUT') throw new Error(`ddreport took longer than ${timeout} ms`)
  if (r.error) throw hint(r.error)
  if (r.status !== 0) throw fail(r.status, r.stderr)
  return r.stdout
}

/**
 * Like render, without blocking the event loop.
 * @returns {Promise<Buffer>}
 */
exports.renderAsync = (template, data, format = 'pdf', { timeout = 60000 } = {}) =>
  new Promise((resolve, reject) => {
    const p = spawn(bin(), ['-format', format])
    const out = []
    const err = []
    // a render that hangs is stopped, so it cannot hold a request open for good
    const timer = timeout && setTimeout(() => { p.kill(); reject(new Error(`ddreport took longer than ${timeout} ms`)) }, timeout)
    p.stdout.on('data', c => out.push(c))
    p.stderr.on('data', c => err.push(c))
    p.stdin.on('error', () => {}) // a binary that exits early closes stdin; its exit code tells why
    p.on('error', e => { clearTimeout(timer); reject(hint(e)) })
    p.on('close', status => {
      clearTimeout(timer)
      status === 0 ? resolve(Buffer.concat(out)) : reject(fail(status, Buffer.concat(err)))
    })
    p.stdin.end(JSON.stringify({ template, data }))
  })
