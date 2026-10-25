# ddreport-designer

A drag-and-drop report designer with a word-processor style ribbon, for [DdDesignReport](https://github.com/gvjavx/DdDesignReport). No dependencies, no build step. TypeScript types included.

```sh
npm install ddreport-designer
```

```js
import { createDesigner } from 'ddreport-designer'

const designer = createDesigner(document.getElementById('designer'), {
  template: savedTemplate,          // optional
  onChange: t => save(t),           // after every change
  lang: 'id',                       // optional: Indonesian interface (default 'en')
  theme: 'auto',                    // 'light' (default), 'dark' or 'auto'
  fields: ['name', 'qty', 'price'], // optional: the data's fields, for the Field menu
  data: sampleData,                 // optional: sample data the charts on the page are drawn from
  preview: async t => (await fetch('/report/preview', { method: 'POST', body: JSON.stringify(t) })).text(),
  simple: true,                     // optional: fewer tools, each button with its name
  hide: ['subreport', 'Layout.Data'], // optional: leave out tabs, groups, commands, settings
  messages: { 'Text box': 'Label' },  // optional: your own words, keyed by the English text
})
designer.getTemplate() // the JSON to render with the ddreport package (Node.js), Go, Java or Kotlin
designer.destroy()     // when the page removes the designer
```

The template it makes is rendered by [ddreport](https://www.npmjs.com/package/ddreport) on Node.js, by the Go library, or by the Java/Kotlin wrapper (`io.github.gvjavx:ddreport`).

## Ready-made templates

Thirteen templates come with the package, each with sample data: `invoice`, `sales-report`, `receipt`, `payslip`, `certificate`, `letter`, `stock`, `labels`, `kwitansi`, `surat-jalan`, `rapor`, `daftar-hadir`, `laba-rugi` (the list with names and descriptions is `templates/index.json`).

```js
import invoice from 'ddreport-designer/templates/invoice.json' // Vite, webpack and other bundlers read JSON

designer.setTemplate(invoice.template) // invoice.data is its sample data
designer.setData(invoice.data)
```

## In React or Vue

```jsx
// React
function ReportDesigner({ template, onChange }) {
  const ref = useRef(null)
  useEffect(() => {
    const d = createDesigner(ref.current, { template, onChange })
    return () => d.destroy()
  }, [])
  return <div ref={ref} style={{ height: 600 }} />
}
```

```js
// Vue
const el = ref(null); let d
onMounted(() => { d = createDesigner(el.value, { template, onChange }) })
onBeforeUnmount(() => d.destroy())
```

## Your look

The colours are CSS variables on the element; override them in your own style sheet:

```css
#designer.ddr { --a: #e11d48; --a-soft: #ffe4e6; --bg: #fff; --surface: #fff; --line: #e5e7eb; border-radius: 0 }
```

## The template format

`ddreport-designer/template.schema.json` is the JSON Schema of the templates, for validation and completion in your editor or before you store a template; unknown properties are mistakes. [llms.txt](https://github.com/gvjavx/DdDesignReport/blob/main/llms.txt) explains the format to AI assistants, with a tested example.

MIT licensed.
