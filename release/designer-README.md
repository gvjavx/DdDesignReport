# ddreport-designer

A drag-and-drop report designer with a word-processor style ribbon, for [DdDesignReport](https://github.com/gvjavx/DdDesignReport). No dependencies, no build step.

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
})
designer.getTemplate() // the JSON to render with the ddreport package (Node.js), Go, Java or Kotlin
```

The template it makes is rendered by [ddreport](https://www.npmjs.com/package/ddreport) on Node.js, by the Go library, or by the Java/Kotlin wrapper (`io.github.gvjavx:ddreport`).

MIT licensed.
