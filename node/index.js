const { spawn, spawnSync } = require('node:child_process')

// the binary: DDREPORT_BIN, else the one installed for this system with the package
// (@ddreport/linux-x64, @ddreport/win32-x64, @ddreport/darwin-arm64...), else ddreport on the PATH
const bin = () => {
  if (process.env.DDREPORT_BIN) return process.env.DDREPORT_BIN
  try {
    return require.resolve(`@ddreport/${process.platform}-${process.arch}/bin/ddreport${process.platform === 'win32' ? '.exe' : ''}`)
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


/**
 * Keeps ddreport processes running and sends them one report after another, so
 * a busy server does not start a process per report. Reports wait their turn
 * while every process is busy. A process that stops or takes too long fails
 * only the report it was on, and a new one takes its place for the next.
 *   const renderer = createRenderer({ processes: 2 })
 *   const pdf = await renderer.render(template, data, 'pdf')
 *   renderer.close() // when the server stops
 * @param {{processes?: number, timeout?: number}} options how many processes (default 1), and the time limit per report in milliseconds (default 60000; 0 for none)
 * @returns {{render: (template: object, data: object|Array, format?: string) => Promise<Buffer>, close: () => void}}
 */
exports.createRenderer = ({ processes = 1, timeout = 60000 } = {}) => {
  const queue = []
  let workers = [], closed = false
  // a process keeps Node running only while it works on a report
  const hold = (w, on) => { for (const h of [w.p, w.p.stdin, w.p.stdout, w.p.stderr]) on ? h.ref?.() : h.unref?.() }
  const start = () => {
    const w = { p: spawn(bin(), ['-stdio']), job: null, buf: Buffer.alloc(0), err: [], gone: false }
    w.p.stdout.on('data', chunk => { w.buf = Buffer.concat([w.buf, chunk]); answer(w) })
    w.p.stderr.on('data', c => w.err.push(c))
    w.p.stdin.on('error', () => {}) // the process stopped; its exit says why
    w.p.on('error', e => stopped(w, hint(e)))
    w.p.on('exit', status => stopped(w, fail(status, Buffer.concat(w.err))))
    hold(w, false)
    return w
  }
  // answer takes "ok <length>\n<document>" or "error <length>\n<message>" once all of it has come
  const answer = w => {
    const nl = w.buf.indexOf(10)
    if (nl < 0 || !w.job) return
    const [status, length] = w.buf.subarray(0, nl).toString().split(' '), end = nl + 1 + Number(length)
    if (w.buf.length < end) return
    const body = Buffer.from(w.buf.subarray(nl + 1, end)), job = finish(w)
    w.buf = w.buf.subarray(end)
    status === 'ok' ? job.resolve(body) : job.reject(new Error(body.toString()))
    next()
  }
  const finish = w => { const job = w.job; clearTimeout(job.timer); w.job = null; hold(w, false); return job }
  // a process that stopped (or never started) fails its report and is left behind
  const stopped = (w, err) => {
    if (w.gone) return
    w.gone = true
    workers = workers.filter(x => x !== w)
    if (w.job) finish(w).reject(err)
    // with no process able to start, the waiting reports fail too instead of waiting for good
    if (w.p.pid === undefined && !workers.length) for (const job of queue.splice(0)) job.reject(err)
    next()
  }
  const next = () => {
    if (closed) return
    while (workers.length < Math.max(1, processes) && queue.length) workers.push(start())
    for (const w of workers) {
      if (w.job || w.gone || !queue.length) continue
      const job = w.job = queue.shift()
      hold(w, true)
      if (timeout) job.timer = setTimeout(() => { stopped(w, new Error(`ddreport took longer than ${timeout} ms`)); w.p.kill() }, timeout)
      const body = Buffer.from(JSON.stringify({ template: job.template, data: job.data ?? null }))
      w.p.stdin.write(`${job.format} ${body.length}\n`)
      w.p.stdin.write(body)
    }
  }
  return {
    render: (template, data, format = 'pdf') => new Promise((resolve, reject) => {
      if (closed) return reject(new Error('the renderer is closed'))
      // the format goes on the request line: a word, so it can never change where a request ends
      if (!/^\w+$/.test(format)) return reject(new Error(`ddreport: unknown format ${JSON.stringify(format)}`))
      queue.push({ template, data, format, resolve, reject })
      next()
    }),
    close: () => {
      closed = true
      for (const job of queue.splice(0)) job.reject(new Error('the renderer is closed'))
      for (const w of workers) w.p.stdin.end() // each finishes its report, then stops
      workers = []
    },
  }
}
