<img src="logo.svg" alt="" width="72">

# DdDesignReport

A reporting library with a drag-and-drop designer. Templates are designed in the browser, saved as JSON, and rendered by one Go core to **PDF, HTML, XLSX, XLS or CSV** from Go, Java, Kotlin, Node.js, or any language over HTTP. Templates also go to and from JasperReports (`.jrxml`). MIT licensed.

The full documentation is in Indonesian: [README.md](README.md). This page covers what you need to start; the template format is described by its [JSON Schema](designer/template.schema.json) and summarised, with a tested example, in [llms.txt](llms.txt). What is new in each version: [CHANGELOG.md](CHANGELOG.md) (Indonesian).

## The designer

A standalone app with preview and export: download the binary for your system from the [latest GitHub Release](https://github.com/gvjavx/DdDesignReport/releases/latest) (`ddreport-linux-amd64`, `ddreport-darwin-arm64`, `ddreport-windows-amd64.exe`, ...), rename it to `ddreport`, then:

```sh
./ddreport -serve 127.0.0.1:8080      # open http://127.0.0.1:8080
```

The standalone app keeps your own templates on the server (**My templates**, in the `ddreport-templates` folder or `-templates`, with the last 20 versions of each), shows every font of `DDREPORT_FONTS` in its font menu, and opens and exports JasperReports `.jrxml` files.

Or embedded in your own web app (`npm install ddreport-designer`), with no dependencies and TypeScript types:

```js
import { createDesigner } from 'ddreport-designer'

const designer = createDesigner(document.getElementById('designer'), {
  template: savedTemplate,            // optional
  onChange: t => save(t),             // after every change
  fields: ['name', 'qty', 'price'],   // optional: the data fields offered in the Field menu
  data: sampleData,                   // optional: sample data the canvas draws charts and barcodes from
  simple: true,                       // optional: easy mode for first-time users
  lang: 'id',                         // optional: Indonesian interface (default 'en')
})
designer.getTemplate()                // the JSON to send to your backend
```

It comes with 13 ready-made templates (invoice, sales report, receipt, payslip, certificate, letter, stock list, labels, receipt of payment, delivery note, school report, attendance list, profit and loss): `import invoice from 'ddreport-designer/templates/invoice.json'`. Options to hide tools, rename any text and change the colours are in the [package README](release/designer-README.md). `theme: 'inherit'` takes Bootstrap's colours, font and corners (dark mode too), and `hide: ['ribbon']` with `designer.run('bold')` and `onState` lets your own toolbar drive the designer; a full Bootstrap page is in [designer/bootstrap.html](designer/bootstrap.html).

## Rendering

**Go**

```go
import ddreport "github.com/gvjavx/DdDesignReport"

pdf, err := ddreport.Render(templateJSON, data, "pdf") // "pdf" | "html" | "xlsx" | "xls" | "csv" | "jrxml"
```

**Node.js** (`npm install ddreport`; the binary for your system comes with it)

```js
const { createRenderer } = require('ddreport')
const renderer = createRenderer({ processes: 2 }) // keeps the renderer running between reports
const pdf = await renderer.render(template, data, 'pdf')
```

**Java and Kotlin** (Maven Central `io.github.gvjavx:ddreport`, plus the binary for your system with a classifier: `linux-x64`, `linux-arm64`, `windows-x64`, `windows-arm64`, `darwin-x64`, `darwin-arm64`)

```java
try (DdReport.Renderer renderer = new DdReport.Renderer()) {
    byte[] pdf = renderer.render(templateJson, dataJson, "pdf");
}
```

**Any language over HTTP** (PHP, Python, .NET, ...): run the render server, from the binary or as a Docker image (in every release from 0.3.0):

```sh
docker run -p 8080:8080 -e DDREPORT_TOKEN=a-long-secret ghcr.io/gvjavx/ddreport
```

```sh
curl -X POST "http://localhost:8080/render?format=pdf" \
  -H "Authorization: Bearer a-long-secret" \
  -d '{"template": {...}, "data": {...}}' -o report.pdf
```

The answer is the document, `400` with the error message, or `401` without the right token. Without `DDREPORT_TOKEN` anyone who can reach the port can render, so keep such a server on `127.0.0.1`; it then only answers to `localhost` and IP addresses, so a site that points its own name at your machine cannot use it. To reach it by a domain name, set the token. The designer on `/` takes the token from its address for preview and export: `http://server:8080/#token=a-long-secret`. PHP, Python and C# examples are in the [README](README.md#backend); PHP templates read with `json_decode($json, true)` work as they are.

## Templates in short

- Sizes are in points; A4 is 595 × 842.
- Bands: `header` and `columnHeader` on every page, `groupHeader` / `groupFooter` around each group (`groupBy`, up to six levels), `detail` for each data row, `summary` after the last row, `footer` on every page.
- Placeholders: `{{field}}`, `{{customer.name}}`, `{{qty * price:2}}`, `{{sum(total)}}`, `{{count()}}`, `{{page}}` / `{{pages}}`, `{{date:dd MMMM yyyy}}`, `{{if(paid, 'PAID', 'DUE')}}`, `{{terbilang(total)}}` (Indonesian number words).
- Elements: text, table (including tables that repeat a row for each item of a list in the data), image, box, line, chart (bar, horizontal bar, line, pie, donut), QR code and Code 128, sub-report, and crosstab (`rowField`, `columnField`, `measure` such as `sum(total):0`, with totals).
- Colours can follow the data: `"color": "{{if(total < 0, '#dc2626', '#000000')}}"`; in the designer, *colour from data* in the properties panel.
- JasperReports: `ddreport -format jrxml` writes a template as a JasperReports 6 `.jrxml` (Groovy expressions, fields filled by a `JsonDataSource`), and `ddreport -from-jrxml` reads one back as a template, noting what did not come over. Also `ToJRXML` / `FromJRXML` in Go, `"jrxml"` in `render`, and `POST /import/jrxml`.
- In XLSX and XLS, bold text, number formats, alignment, cell colours, borders and merged cells (across and down) carry over; one sheet per group is an option. XLSX also carries pictures and barcodes; charts are in PDF and HTML only.
- Bundled fonts (Roboto, Open Sans, Lato, Montserrat, Lora, Playfair Display, Great Vibes) are embedded in PDF and HTML; your own TrueType fonts can be added (`DDREPORT_FONTS`), and `node fontpack/download.mjs` fetches 100 more from Google Fonts. They are read only when a report uses them.
- Arabic and Hebrew print right to left in PDF, Arabic letters joined (use a font such as Amiri from the font pack), and Thai with its marks.
- PDF/A-2b with `"pdfA": true`. Every PDF is digitally signed when `DDREPORT_SIGN_CERT` and `DDREPORT_SIGN_KEY` name a PEM certificate and key (`ddreport.SignPDF` in Go).
- Large reports: 100 000 rows print to PDF in about 12 seconds on a 2017 laptop (`go test -bench Large`).
- Templates from the community are welcome: see "Menyumbang template" in the [README](README.md#menyumbang-template).

## Tests

```sh
go test ./...                 # core, binary, templates, schema, regression tests
node designer/run-tests.mjs   # the designer in Chromium, Firefox and WebKit (needs Playwright)
```

## Author

DdDesignReport is made by Naufal Hilmi Abdurrahman. Other projects, and websites and apps built to order (web, Android, desktop, AI): [aufal-tech.vercel.app](https://aufal-tech.vercel.app/).
