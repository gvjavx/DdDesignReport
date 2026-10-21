# ddreport

Render report templates made in the [DdDesignReport](https://github.com/gvjavx/DdDesignReport) drag-and-drop designer to **PDF, HTML, XLSX, XLS or CSV** from Node.js.

```sh
npm install ddreport
```

The renderer for your system (Windows, Linux or macOS, x64 or arm64) comes with it as an optional dependency; nothing is downloaded at install time.

```js
const { render, renderAsync } = require('ddreport')

const pdf = await renderAsync(template, data, 'pdf') // Buffer; does not block the event loop
const xlsx = render(template, data, 'xlsx')            // the same, synchronous
```

- `template` is the JSON the designer exports ([ddreport-designer](https://www.npmjs.com/package/ddreport-designer)).
- `data` is an object with fields and a `rows` list, or just the list of rows.
- A render is stopped after 60 seconds; pass `{ timeout: ms }` as the fourth argument to change it.
- `DDREPORT_BIN` points to another `ddreport` binary, if you build your own.

On a busy server, keep the renderer running instead of starting it for every report (about five times faster for a small one):

```js
const { createRenderer } = require('ddreport')

const renderer = createRenderer({ processes: 2 }) // optional: { timeout: ms }, default 60000
const pdf = await renderer.render(template, data, 'pdf')
renderer.close() // when the server stops
```

Reports wait their turn while every process is busy. A process that stops or takes too long fails only its own report, and a new one takes its place. Idle processes do not keep Node.js running.

Templates support bands (header, column header, up to three group levels, detail, summary, footer), formulas and totals (`{{sum(qty * price):2}}`), dates, conditions, tables bound to lists in the data with merged cells, sub-reports, charts, barcodes and bundled fonts. See the [documentation](https://github.com/gvjavx/DdDesignReport#readme).

MIT licensed.
