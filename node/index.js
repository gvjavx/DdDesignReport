const { spawnSync } = require('node:child_process')

/**
 * Renders a report through the `ddreport` binary.
 * @param {object} template template exported by the designer
 * @param {object|Array} data fields plus the row array, or just the rows
 * @param {'pdf'|'html'|'xlsx'|'csv'} format
 * @returns {Buffer}
 */
exports.render = (template, data, format = 'pdf') => {
  // ponytail: spawnSync blocks the event loop while rendering. Switch to async spawn if reports get big.
  const r = spawnSync(process.env.DDREPORT_BIN || 'ddreport', ['-format', format], {
    input: JSON.stringify({ template, data }),
    maxBuffer: Infinity,
  })
  if (r.error) throw r.error
  if (r.status !== 0) throw new Error(r.stderr.toString().trim() || `ddreport exited with ${r.status}`)
  return r.stdout
}
