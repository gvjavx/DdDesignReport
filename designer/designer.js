// Drag-and-drop report designer with a word-processor style ribbon.
// No dependencies, no build step:
//   import { createDesigner } from 'ddreport-designer'
//   const designer = createDesigner(element, { template, onChange, preview, lang })
// The template it edits is the JSON the ddreport backend renders. 1px on screen = 1pt on paper.
// preview is optional: async template => HTML string (ask your backend to render it as "html").
// lang is 'en' (default) or 'id'.

const BANDS = ['header', 'columnHeader', 'groupHeader', 'detail', 'groupFooter', 'summary', 'footer']
const SIDES = ['marginTop', 'marginRight', 'marginBottom', 'marginLeft']
const PAPER = { A4: [595, 842], A5: [420, 595], Letter: [612, 792], Legal: [612, 1008], F4: [595, 935] }
const FONTS = { Helvetica: 'Helvetica,Arial,sans-serif', Times: "'Times New Roman',Times,serif", Courier: "'Courier New',Courier,monospace" }
const NEW = {
  text: { w: 160, h: 16, text: 'Text', fontSize: 10 },
  table: { w: 360, h: 40, border: 1, fontSize: 10, cells: [['', '', ''], ['', '', '']], cols: [1, 1, 1], aligns: [] },
  image: { w: 80, h: 80, src: '' },
  rect: { w: 120, h: 40, border: 1 },
  line: { w: 200, h: 1, border: 1 },
  chart: { w: 240, h: 140, chart: 'bar', label: '', value: '', color: '#4e79a7' },
  barcode: { w: 70, h: 70, text: '{{code}}', format: 'qr' },
}
// what the properties panel edits per type; fonts, colours and alignment live in the ribbon
const BASE = ['x', 'y', 'w', 'h']
const FIELDS = {
  text: [...BASE, 'text'],
  table: [...BASE, 'rows', 'columns', 'cols', 'border', 'color'],
  image: [...BASE, 'src'],
  rect: [...BASE, 'border', 'color'],
  line: [...BASE, 'border', 'color'],
  chart: [...BASE, 'chart', 'label', 'value', 'color'],
  barcode: [...BASE, 'format', 'text', 'color'],
}
// which ribbon properties apply to which type
const STYLE = {
  text: ['font', 'fontSize', 'lineHeight', 'bold', 'italic', 'underline', 'align', 'color', 'fill', 'grow'],
  table: ['font', 'fontSize', 'lineHeight', 'bold', 'italic', 'underline', 'align', 'fill', 'grow'],
  rect: ['fill'],
}
const DEFAULT = { font: 'Helvetica', fontSize: 10, lineHeight: 1.2, color: '#000000', fill: '#ffffff', grow: false }
const NUMERIC = new Set(['x', 'y', 'w', 'h', 'border', 'rows', 'columns', 'height'])
const SELECT = { chart: ['bar', 'hbar', 'line', 'pie', 'donut'], format: ['qr', 'code128'] }
const LABEL = { cols: 'column widths', color: 'line colour' }
const PLACEHOLDER = { label: 'row field', value: 'field or qty * price', cols: '2,1,1' }
const FIELD_TOKENS = ['{{page}}', '{{pages}}', '{{field}}', '{{field:2}}', '{{qty * price:2}}', '{{sum(field):2}}', '{{sum(qty * price):2}}', '{{avg(field):2}}', '{{min(field)}}', '{{max(field)}}', '{{count()}}']

// Indonesian interface. Anything missing here stays English.
const ID = {
  Home: 'Beranda', Insert: 'Sisipkan', Layout: 'Tata Letak', View: 'Tampilan',
  Undo: 'Urungkan', Clipboard: 'Papan Klip', Font: 'Font', Paragraph: 'Paragraf', Arrange: 'Atur', Text: 'Teks', Table: 'Tabel',
  Illustrations: 'Ilustrasi', Data: 'Data', Page: 'Halaman', Margins: 'Margin', Background: 'Latar', Grouping: 'Pengelompokan', Zoom: 'Zoom', Show: 'Tampilkan', Report: 'Laporan',
  'Undo (Ctrl+Z)': 'Urungkan (Ctrl+Z)', 'Redo (Ctrl+Y)': 'Ulangi (Ctrl+Y)',
  Cut: 'Potong', 'Cut (Ctrl+X)': 'Potong (Ctrl+X)', Copy: 'Salin', 'Copy (Ctrl+C)': 'Salin (Ctrl+C)', Paste: 'Tempel', 'Paste (Ctrl+V)': 'Tempel (Ctrl+V)',
  Duplicate: 'Gandakan', 'Duplicate (Ctrl+D)': 'Gandakan (Ctrl+D)', Delete: 'Hapus', 'Delete (Del)': 'Hapus (Del)',
  'Font size': 'Ukuran font', 'Bold (Ctrl+B)': 'Tebal (Ctrl+B)', 'Italic (Ctrl+I)': 'Miring (Ctrl+I)', 'Underline (Ctrl+U)': 'Garis bawah (Ctrl+U)',
  'Text colour': 'Warna teks', 'Highlight / fill colour': 'Warna sorotan / isi', Fill: 'Isi', 'No fill': 'Tanpa isi',
  Left: 'Kiri', Center: 'Tengah', Right: 'Kanan', 'Align text left': 'Rata kiri', 'Centre text': 'Rata tengah', 'Align text right': 'Rata kanan',
  'Line spacing': 'Spasi baris', Wrap: 'Bungkus', 'Wrap long text and let the band grow with it': 'Bungkus teks panjang dan biarkan band ikut memanjang',
  'Align left edges': 'Ratakan tepi kiri', 'Align centres': 'Ratakan tengah', 'Align right edges': 'Ratakan tepi kanan',
  'Align top edges': 'Ratakan tepi atas', 'Align middles': 'Ratakan tengah vertikal', 'Align bottom edges': 'Ratakan tepi bawah',
  Distribute: 'Sebar', 'Space selected elements evenly': 'Beri jarak yang sama antar elemen', Front: 'Depan', 'Bring to front': 'Bawa ke depan', Back: 'Belakang', 'Send to back': 'Kirim ke belakang',
  'Text box': 'Kotak teks', Rows: 'Baris', Columns: 'Kolom', 'Table rows': 'Baris tabel', 'Table columns': 'Kolom tabel',
  Picture: 'Gambar', 'Insert a picture from this device': 'Sisipkan gambar dari perangkat ini', Rectangle: 'Kotak', Line: 'Garis', Chart: 'Chart', Barcode: 'Barcode',
  'click to insert, or drag onto the page': 'klik untuk menyisipkan, atau seret ke halaman',
  'Field…': 'Field…', 'Insert field': 'Sisipkan field', 'Insert a field into the selected text': 'Sisipkan field ke teks terpilih',
  'Paper size': 'Ukuran kertas', Orientation: 'Orientasi', Portrait: 'Tegak', Landscape: 'Mendatar', Custom: 'Kustom',
  'Top margin': 'Margin atas', 'Right margin': 'Margin kanan', 'Bottom margin': 'Margin bawah', 'Left margin': 'Margin kiri',
  Colour: 'Warna', 'Page colour': 'Warna halaman', 'No page colour': 'Tanpa warna halaman',
  'Use a picture as the page background': 'Pakai gambar sebagai latar halaman', Remove: 'Hapus', 'Remove the background picture': 'Hapus gambar latar',
  'Group by': 'Kelompokkan', 'row field': 'field baris', 'New page per group': 'Halaman baru per kelompok', 'Rows key': 'Kunci baris',
  'Thousands separator': 'Pemisah ribuan', 'Decimal sign': 'Tanda desimal',
  'Zoom out': 'Perkecil', 'Zoom in': 'Perbesar', 'Reset zoom': 'Zoom 100%', Ruler: 'Penggaris', Snap: 'Tempel',
  'Snap to the edges of other elements while dragging (hold Alt to turn off)': 'Menempel ke tepi elemen lain saat digeser (tahan Alt untuk mematikan)',
  Preview: 'Pratinjau', 'Render the report with sample data': 'Cetak laporan dengan data contoh', Close: 'Tutup', 'Report preview': 'Pratinjau laporan',
  'Band heights. Select an element to edit it; double-click a text or a table cell to type.': 'Tinggi band. Pilih elemen untuk mengubahnya; klik ganda teks atau sel tabel untuk mengetik.',
  'elements selected. Use the ribbon to format, align or move them together.': 'elemen dipilih. Pakai ribbon untuk memformat, meratakan, atau memindahkannya bersama.',
  'Drag to resize this band': 'Tarik untuk mengubah tinggi band ini', band: 'band', text: 'teks', rows: 'baris', columns: 'kolom', 'column widths': 'lebar kolom',
  border: 'garis', 'line colour': 'warna garis', src: 'gambar', chart: 'jenis chart', label: 'label', value: 'nilai', format: 'format', color: 'warna',
  'field or qty * price': 'field atau qty * price', 'Use <b>, <i> and <u> to style parts of the text': 'Pakai <b>, <i>, dan <u> untuk memformat sebagian teks',
  'element': 'elemen',
}

const CSS = `
.ddr{display:flex;flex-direction:column;font:13px system-ui,sans-serif;color:#222;background:#f3f3f3;border:1px solid #c8c8c8}
.ddr button,.ddr select,.ddr input,.ddr textarea{font:inherit}
.ddr-tabs{display:flex;background:#185abd}
.ddr-tabs button{border:0;background:none;color:#fff;padding:6px 16px;cursor:pointer}
.ddr-tabs button[aria-selected=true]{background:#f3f3f3;color:#185abd;font-weight:600}
.ddr-panel{display:flex;flex-wrap:wrap;gap:4px 0;padding:6px 4px;border-bottom:1px solid #c8c8c8;min-height:58px}
.ddr-panel[hidden]{display:none}
.ddr-group{display:flex;flex-direction:column;justify-content:space-between;gap:4px;padding:0 10px;border-right:1px solid #d6d6d6}
.ddr-group>div{display:flex;flex-wrap:wrap;gap:4px;align-items:center}
.ddr-group>span{font-size:11px;color:#666;text-align:center}
.ddr-group button{min-width:28px;height:26px;padding:0 7px;border:1px solid transparent;border-radius:3px;background:none;color:inherit;cursor:pointer}
.ddr-group button:hover:not(:disabled){background:#dde6f5;border-color:#a9c0e8}
.ddr-group button[aria-pressed=true]{background:#c5d8f5;border-color:#185abd}
.ddr-group button:disabled{opacity:.4;cursor:default}
.ddr-group button[draggable]{cursor:grab}
.ddr-group input[type=number]{width:52px}
.ddr-group input[type=text]{width:84px}
.ddr-group input[type=color]{width:26px;height:22px;padding:0;border:1px solid #aaa;vertical-align:middle}
.ddr-group label{display:inline-flex;align-items:center;gap:3px}
.ddr-work{display:flex;align-items:flex-start;min-height:0}
.ddr-stage{flex:1;min-width:0;overflow:auto;padding:16px;background:#e4e4e4;align-self:stretch}
.ddr-zoom{transform-origin:0 0;width:max-content}
.ddr-ruler{position:relative;height:18px;margin-bottom:4px;font-size:9px;color:#555;background-color:#fff;box-shadow:0 0 0 1px #bbb}
.ddr-ruler span{position:absolute;top:0;padding-left:2px}
.ddr-page{display:flex;flex-direction:column;box-sizing:border-box;color:#000;background:#fff center/100% 100% no-repeat;box-shadow:0 1px 4px #0005;outline:none}
.ddr-band{position:relative;flex:none;box-sizing:border-box;border-bottom:1px dashed #9ab}
.ddr-band[data-name=footer]{margin-top:auto;border-top:1px dashed #9ab}
.ddr-band::before{content:attr(data-name);position:absolute;right:2px;top:0;font-size:9px;color:#9ab;pointer-events:none}
.ddr-grip{position:absolute;left:0;right:0;bottom:-3px;height:6px;cursor:ns-resize;z-index:1}
.ddr-el{position:absolute;box-sizing:border-box;overflow:hidden;resize:both;cursor:move;outline:1px dotted #ccd;white-space:pre-wrap;overflow-wrap:anywhere;font-family:Helvetica,Arial,sans-serif;line-height:1.2;touch-action:none;user-select:none}
.ddr-el[data-type=line]{resize:none;overflow:visible}
.ddr-el[data-type=line]::after{content:'';position:absolute;inset:-4px}
.ddr-el[data-type=image]{background:#eee center/100% 100% no-repeat}
.ddr-el:is([data-type=chart],[data-type=barcode],[data-unknown]){background:#eef3f8;font-size:10px;padding:4px}
.ddr-el[data-type=table]{display:grid}
.ddr-cell{box-sizing:border-box;overflow:hidden;padding:2px 3px;min-width:0}
.ddr-el.ddr-sel{outline:2px solid #185abd}
.ddr-marquee{position:fixed;z-index:10;border:1px solid #185abd;background:#185abd22;pointer-events:none}
.ddr [contenteditable]:not([contenteditable=false]){user-select:text;cursor:text;outline:2px solid #e8a200}
.ddr-props{flex:none;display:flex;flex-direction:column;gap:4px;width:250px;padding:10px;box-sizing:border-box}
.ddr-props label{display:flex;justify-content:space-between;align-items:center;gap:8px;overflow-wrap:anywhere}
.ddr-props :is(input:not([type=checkbox]),select,textarea){flex:none;width:104px;box-sizing:border-box}
.ddr-props textarea{height:70px}
.ddr-props p{margin:0 0 4px;color:#666}
.ddr-preview{padding:8px}
.ddr-preview iframe{display:block;width:min(90vw,900px);height:80vh;border:1px solid #bbb;background:#fff;margin-top:8px}
`

export const emptyTemplate = () => ({
  page: { width: 595, height: 842, margin: 0, marginTop: 36, marginRight: 36, marginBottom: 36, marginLeft: 36, background: '', backgroundImage: '' }, // A4
  dataKey: 'rows',
  groupBy: '',
  groupPageBreak: false,
  bands: Object.fromEntries(BANDS.map((b, i) => [b, { height: [60, 20, 20, 20, 20, 30, 30][i], elements: [] }])),
})

const num = (v, fallback = 0) => Number.isFinite(+v) && v !== null && v !== '' ? +v : fallback
const isObject = v => v !== null && typeof v === 'object' && !Array.isArray(v)

// clean turns whatever was loaded into a template the designer can draw:
// missing parts get defaults, values of the wrong kind are repaired.
function clean(t) {
  const d = emptyTemplate(), src = isObject(t) ? structuredClone(t) : {}, page = isObject(src.page) ? src.page : {}
  const out = { ...d, ...src, page: { ...d.page, ...page }, bands: {} }
  for (const k of ['width', 'height']) out.page[k] = num(page[k]) > 0 ? num(page[k]) : d.page[k]
  for (const s of SIDES) out.page[s] = Math.max(0, num(page[s]) || num(page.margin ?? d.page[s])) // older templates have one margin
  out.page.margin = 0
  for (const b of BANDS) {
    const band = isObject(src.bands) && isObject(src.bands[b]) ? src.bands[b] : {}
    out.bands[b] = {
      ...band,
      height: Math.max(0, num(band.height, d.bands[b].height)),
      elements: (Array.isArray(band.elements) ? band.elements : []).filter(isObject).map(e => {
        const def = NEW[e.type] ?? { w: 80, h: 20 }
        e = { ...e, type: String(e.type ?? 'text'), x: Math.max(0, num(e.x)), y: Math.max(0, num(e.y)), w: Math.max(0, num(e.w, def.w)), h: Math.max(0, num(e.h, def.h)) }
        if (e.type === 'text' || e.type === 'barcode') e.text = String(e.text ?? '')
        if (e.type === 'table') {
          const rows = (Array.isArray(e.cells) ? e.cells : []).filter(Array.isArray), width = Math.max(1, ...rows.map(r => r.length))
          e.cells = (rows.length ? rows : [[]]).map(r => Array.from({ length: width }, (_, c) => String(r[c] ?? '')))
          e.cols = Array.from({ length: width }, (_, c) => num(e.cols?.[c]) > 0 ? num(e.cols[c]) : 1)
          e.aligns = Array.isArray(e.aligns) ? e.aligns : []
        }
        return e
      }),
    }
  }
  return out
}

// markup <-> DOM for text with <b>, <i>, <u>. A literal "<b>" is written "&lt;b>".
function fill(node, text) {
  node.replaceChildren()
  let cur = node
  for (const part of String(text ?? '').split(/(<\/?[biu]>)/)) {
    if (/^<[biu]>$/.test(part)) cur = cur.appendChild(document.createElement(part[1]))
    else if (/^<\/[biu]>$/.test(part)) cur = cur === node ? cur : cur.parentNode
    else if (part) cur.append(part.replace(/&lt;(\/?[biu]>)/g, '<$1'))
  }
}
const markup = node => [...node.childNodes].map(c => {
  if (c.nodeType === 3) return c.nodeValue.replace(/<(\/?[biu]>)/g, '&lt;$1')
  if (c.nodeName === 'BR') return '\n'
  const tag = { B: 'b', STRONG: 'b', I: 'i', EM: 'i', U: 'u' }[c.nodeName]
  return tag ? `<${tag}>${markup(c)}</${tag}>` : markup(c)
}).join('')

export function createDesigner(root, { template, onChange, preview, lang = 'en' } = {}) {
  let tpl, sel = [], editing = null, cancelled = false, clip = [], zoom = 1, snap = true, activeCol = 0
  let history = [], at = -1
  const images = [] // data URIs, kept out of the undo snapshots
  const nodes = new Map() // element -> its DOM node
  const L = s => (lang === 'id' && ID[s]) || s

  const btn = (cmd, label, title = label, more = '') => `<button type="button" data-cmd="${cmd}" title="${L(title)}" aria-label="${L(title)}" ${more}>${L(label)}</button>`
  const group = (name, html) => `<div class="ddr-group"><div>${html}</div><span>${L(name)}</span></div>`
  const options = (list, translate) => list.map(v => `<option value="${v}">${translate ? L(v) : v}</option>`).join('')
  const insert = (type, label) => btn('add:' + type, label, `${L(label)}: ${L('click to insert, or drag onto the page')}`, `draggable="true" data-type="${type}"`)
  const sideLetters = lang === 'id' ? ['A', 'Ka', 'B', 'Ki'] : ['T', 'R', 'B', 'L']
  const RIBBON = {
    Home:
      group('Undo', btn('undo', '↶', 'Undo (Ctrl+Z)') + btn('redo', '↷', 'Redo (Ctrl+Y)')) +
      group('Clipboard', btn('cut', 'Cut', 'Cut (Ctrl+X)') + btn('copy', 'Copy', 'Copy (Ctrl+C)') + btn('paste', 'Paste', 'Paste (Ctrl+V)') + btn('duplicate', 'Duplicate', 'Duplicate (Ctrl+D)') + btn('delete', 'Delete', 'Delete (Del)')) +
      group('Font',
        `<select data-prop="font" title="${L('Font')}" aria-label="${L('Font')}">${options(Object.keys(FONTS))}</select>` +
        `<input data-prop="fontSize" type="number" min="4" max="200" title="${L('Font size')}" aria-label="${L('Font size')}">` +
        btn('bold', '<b>B</b>', 'Bold (Ctrl+B)') + btn('italic', '<i>I</i>', 'Italic (Ctrl+I)') + btn('underline', '<u>U</u>', 'Underline (Ctrl+U)') +
        `<label title="${L('Text colour')}">A<input data-prop="color" type="color" aria-label="${L('Text colour')}"></label>` +
        `<label title="${L('Highlight / fill colour')}">${L('Fill')}<input data-prop="fill" type="color" aria-label="${L('Highlight / fill colour')}"></label>` + btn('noFill', '✕', 'No fill')) +
      group('Paragraph',
        btn('align:left', 'Left', 'Align text left') + btn('align:center', 'Center', 'Centre text') + btn('align:right', 'Right', 'Align text right') +
        `<select data-prop="lineHeight" title="${L('Line spacing')}" aria-label="${L('Line spacing')}">${options([1, 1.2, 1.5, 2])}</select>` +
        `<label title="${L('Wrap long text and let the band grow with it')}"><input data-prop="grow" type="checkbox">${L('Wrap')}</label>`) +
      group('Arrange',
        btn('arrange:x:0', '⇤', 'Align left edges') + btn('arrange:x:0.5', '↔', 'Align centres') + btn('arrange:x:1', '⇥', 'Align right edges') +
        btn('arrange:y:0', '⤒', 'Align top edges') + btn('arrange:y:0.5', '↕', 'Align middles') + btn('arrange:y:1', '⤓', 'Align bottom edges') +
        btn('distribute', 'Distribute', 'Space selected elements evenly') + btn('front', 'Front', 'Bring to front') + btn('back', 'Back', 'Send to back')),
    Insert:
      group('Text', insert('text', 'Text box')) +
      group('Table', `<input data-id="rows" type="number" min="1" max="50" value="2" title="${L('Rows')}" aria-label="${L('Table rows')}">×<input data-id="cols" type="number" min="1" max="20" value="3" title="${L('Columns')}" aria-label="${L('Table columns')}">` + insert('table', 'Table')) +
      group('Illustrations', btn('picture', 'Picture', 'Insert a picture from this device') + insert('rect', 'Rectangle') + insert('line', 'Line') + insert('chart', 'Chart') + insert('barcode', 'Barcode')) +
      group('Data', `<select data-id="field" title="${L('Insert a field into the selected text')}" aria-label="${L('Insert field')}"><option value="">${L('Field…')}</option>${options(FIELD_TOKENS)}</select>`),
    Layout:
      group('Page',
        `<select data-id="paper" title="${L('Paper size')}" aria-label="${L('Paper size')}">${options([...Object.keys(PAPER), 'Custom'], true)}</select>` +
        `<select data-id="orientation" title="${L('Orientation')}" aria-label="${L('Orientation')}">${options(['Portrait', 'Landscape'], true)}</select>` +
        `<label>W<input data-page="width" type="number" min="50"></label><label>H<input data-page="height" type="number" min="50"></label>`) +
      group('Margins', ['Top', 'Right', 'Bottom', 'Left'].map((side, i) => `<label>${sideLetters[i]}<input data-page="${SIDES[i]}" type="number" min="0" title="${L(side + ' margin')}" aria-label="${L(side + ' margin')}"></label>`).join('')) +
      group('Background',
        `<label>${L('Colour')}<input data-page="background" type="color" aria-label="${L('Page colour')}"></label>` + btn('noBackground', '✕', 'No page colour') +
        btn('backgroundImage', 'Picture', 'Use a picture as the page background') + btn('noBackgroundImage', 'Remove', 'Remove the background picture')) +
      group('Grouping', `<label>${L('Group by')}<input data-tpl="groupBy" type="text" placeholder="${L('row field')}"></label><label><input data-tpl="groupPageBreak" type="checkbox">${L('New page per group')}</label>`) +
      group('Data', `<label>${L('Rows key')}<input data-tpl="dataKey" type="text"></label><label title="${L('Thousands separator')}">1<input data-tpl="thousands" type="text" style="width:24px" placeholder="," aria-label="${L('Thousands separator')}">000</label><label title="${L('Decimal sign')}">0<input data-tpl="decimal" type="text" style="width:24px" placeholder="." aria-label="${L('Decimal sign')}">00</label>`),
    View:
      group('Zoom', btn('zoom:-1', '−', 'Zoom out') + `<output data-id="zoom">100%</output>` + btn('zoom:1', '+', 'Zoom in') + btn('zoom:0', '100%', 'Reset zoom')) +
      group('Show', `<label><input data-id="ruler" type="checkbox" checked>${L('Ruler')}</label><label title="${L('Snap to the edges of other elements while dragging (hold Alt to turn off)')}"><input data-id="snap" type="checkbox" checked>${L('Snap')}</label>`),
  }

  root.classList.add('ddr')
  const tabs = Object.keys(RIBBON)
  root.innerHTML = `<style>${CSS}</style>
    <div class="ddr-tabs" role="tablist">${tabs.map((t, i) => `<button type="button" role="tab" aria-selected="${i === 0}" data-tab="${t}">${L(t)}</button>`).join('')}</div>
    ${tabs.map((t, i) => `<div class="ddr-panel" role="tabpanel" data-panel="${t}" ${i ? 'hidden' : ''}>${RIBBON[t]}</div>`).join('')}
    <div class="ddr-work"><div class="ddr-stage"><div class="ddr-sizer"><div class="ddr-zoom"><div class="ddr-ruler"></div><div class="ddr-page" tabindex="-1"></div></div></div></div><div class="ddr-props"></div></div>`
  const $ = s => root.querySelector(s), $$ = s => [...root.querySelectorAll(s)]
  const page = $('.ddr-page'), props = $('.ddr-props'), ruler = $('.ddr-ruler'), zoomer = $('.ddr-zoom'), sizer = $('.ddr-sizer')

  const px = v => v + 'px'
  const bandOf = e => BANDS.find(b => tpl.bands[b].elements.includes(e))
  const contentW = () => tpl.page.width - tpl.page.marginLeft - tpl.page.marginRight
  const target = () => (sel[0] && bandOf(sel[0])) || 'header'
  const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, Math.round(+v) || lo))
  // the keyboard only reaches the designer while something inside it has the focus
  const keepFocus = () => (nodes.get(sel[0]) ?? page).focus({ preventScroll: true })

  // ---- history -----------------------------------------------------------
  const pack = () => JSON.stringify(tpl, (k, v) => typeof v === 'string' && v.startsWith('data:') ? '\u0000' + (images.includes(v) ? images.indexOf(v) : images.push(v) - 1) : v)
  const unpack = s => JSON.parse(s, (k, v) => typeof v === 'string' && v[0] === '\u0000' ? images[+v.slice(1)] : v)
  function remember() {
    history.splice(++at, Infinity, pack())
    if (history.length > 100) { history.shift(); at-- }
    sync()
  }
  function changed() {
    remember()
    onChange?.(tpl)
  }
  function travel(step) {
    if (!history[at + step]) return
    at += step
    tpl = unpack(history[at])
    sel = []
    draw()
    renderProps()
    onChange?.(tpl)
  }

  // load replaces the template. Except for the very first one this is a step
  // in the history like any other, so starting over can be undone.
  function load(t, first) {
    tpl = clean(t)
    sel = []
    if (first) { history = []; at = -1 }
    draw()
    renderProps()
    remember()
  }

  // ---- selection and editing ----------------------------------------------
  function select(e, toggle) {
    if (!e) sel = []
    else if (!toggle) sel = [e]
    else sel = sel.includes(e) ? sel.filter(s => s !== e) : [...sel, e]
    mark()
    renderProps()
  }

  function mark() {
    for (const [e, n] of nodes) n.classList.toggle('ddr-sel', sel.includes(e))
    sync()
  }

  // fit keeps an element on the paper: inside the left and right margins, and
  // inside its band, which grows when the element reaches below it.
  function fit(e, band) {
    e.x = Math.max(0, Math.min(Math.round(e.x), Math.max(0, Math.round(contentW() - e.w))))
    e.y = Math.max(0, Math.round(e.y))
    if (e.y + e.h > band.height) band.height = Math.ceil(e.y + e.h)
  }

  function add(type, band, x, y, extra) {
    const e = { type, x: Math.max(0, Math.round(x)), y: Math.max(0, Math.round(y)), ...structuredClone(NEW[type]), ...extra }
    if (type === 'table') tableSize(e, $('[data-id=rows]').value, $('[data-id=cols]').value)
    // do not hide a new element exactly on top of another one
    for (let i = 0; i < 20 && tpl.bands[band].elements.some(o => o.x === e.x && o.y === e.y); i++) { e.x += 10; e.y += 10 }
    tpl.bands[band].elements.push(e)
    sel = [e]
    redraw()
  }

  function redraw() {
    draw()
    renderProps()
    changed()
  }

  // moveTo puts the elements in another band; with no band it deletes them
  function moveTo(list, band) {
    for (const b of BANDS) tpl.bands[b].elements = tpl.bands[b].elements.filter(x => !list.includes(x))
    if (band) tpl.bands[band].elements.push(...list)
    else sel = sel.filter(s => !list.includes(s))
    redraw()
  }

  function paste(list) {
    sel = list.map(([band, json]) => {
      const e = JSON.parse(json)
      e.x += 10; e.y += 10
      tpl.bands[band].elements.push(e)
      return e
    })
    redraw()
    return sel.map(e => [bandOf(e), JSON.stringify(e)]) // pasting again steps a little further
  }
  const copied = () => sel.map(e => [bandOf(e), JSON.stringify(e)])

  // apply sets a ribbon property on every selected element that has it
  function apply(key, value) {
    for (const e of sel) {
      if (!STYLE[e.type]?.includes(key)) continue
      if (key === 'align' && e.type === 'table') e.aligns = Object.assign(e.cells[0].map((_, c) => e.aligns?.[c] || 'left'), { [Math.min(activeCol, e.cells[0].length - 1)]: value })
      else e[key] = value
    }
    redraw()
  }

  // toggle is bold / italic / underline: on the selected words while typing, on whole elements otherwise
  function toggle(key) {
    if (editing?.dataset.rich) return document.execCommand(key)
    apply(key, !sel.filter(e => STYLE[e.type]?.includes(key)).every(e => e[key]))
  }

  // edit lets the user type in a text or a table cell. Enter (in a cell) or a
  // click elsewhere keeps the change, Escape throws it away.
  function edit(node, rich, save) {
    editing = node
    cancelled = false
    if (rich) node.dataset.rich = 1
    try { node.contentEditable = rich ? 'true' : 'plaintext-only' } catch { node.contentEditable = 'true' } // older Firefox has no plaintext-only
    node.focus()
    getSelection().selectAllChildren(node)
    node.onpaste = ev => { ev.preventDefault(); document.execCommand('insertText', false, ev.clipboardData.getData('text/plain')) }
    node.onblur = ev => {
      node.onblur = null
      node.contentEditable = 'false'
      editing = null
      if (cancelled) draw()
      else save()
      if (!ev.relatedTarget) keepFocus() // unless the user went on to another control
    }
  }

  function tableSize(e, rows, cols) {
    rows = clamp(rows, 1, 50); cols = clamp(cols, 1, 20)
    e.cells = Array.from({ length: rows }, (_, r) => Array.from({ length: cols }, (_, c) => e.cells[r]?.[c] ?? ''))
    e.cols = Array.from({ length: cols }, (_, c) => e.cols?.[c] ?? 1)
  }

  function arrange(axis, mode) {
    if (!sel.length) return
    const size = axis === 'x' ? 'w' : 'h'
    let lo = 0, hi = axis === 'x' ? contentW() : tpl.bands[bandOf(sel[0])].height // one element: against its band
    if (sel.length > 1) { lo = Math.min(...sel.map(s => s[axis])); hi = Math.max(...sel.map(s => s[axis] + s[size])) }
    for (const s of sel) s[axis] = Math.round(lo + (hi - lo - s[size]) * mode)
    redraw()
  }

  function distribute() {
    if (sel.length < 3) return
    const list = [...sel].sort((a, b) => a.x - b.x), last = list.at(-1)
    const gap = (last.x + last.w - list[0].x - list.reduce((sum, e) => sum + e.w, 0)) / (list.length - 1)
    let x = list[0].x
    for (const e of list) { e.x = Math.round(x); x += e.w + gap }
    redraw()
  }

  function order(front) {
    for (const e of sel) {
      const list = tpl.bands[bandOf(e)].elements
      list.splice(list.indexOf(e), 1)
      front ? list.push(e) : list.unshift(e)
    }
    redraw()
  }

  function pickImage(done) {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = 'image/png,image/jpeg,image/gif' // what the PDF renderer reads
    input.onchange = () => {
      const reader = new FileReader()
      reader.onload = () => done(reader.result)
      if (input.files[0]) reader.readAsDataURL(input.files[0])
    }
    input.click()
  }

  function setZoom(z) {
    zoom = Math.min(3, Math.max(0.25, z))
    zoomer.style.transform = `scale(${zoom})`
    sizer.style.width = px(zoomer.offsetWidth * zoom)
    sizer.style.height = px(zoomer.offsetHeight * zoom)
    $('[data-id=zoom]').value = Math.round(zoom * 100) + '%'
  }

  // ---- commands (ribbon buttons and shortcuts) ------------------------------
  const commands = {
    undo: () => travel(-1),
    redo: () => travel(1),
    copy: () => { clip = copied(); sync() },
    cut: () => { if (sel.length) { clip = copied(); moveTo(sel) } },
    paste: () => { if (clip.length) clip = paste(clip) },
    duplicate: () => { if (sel.length) paste(copied()) },
    delete: () => { if (sel.length) moveTo(sel) },
    all: () => { sel = BANDS.flatMap(b => tpl.bands[b].elements); mark(); renderProps() },
    none: () => select(null),
    bold: () => toggle('bold'),
    italic: () => toggle('italic'),
    underline: () => toggle('underline'),
    align: v => apply('align', v),
    noFill: () => apply('fill', ''),
    arrange: (axis, mode) => arrange(axis, +mode),
    distribute,
    front: () => order(true),
    back: () => order(false),
    add: type => add(type, target(), 0, 0),
    picture: () => pickImage(src => {
      const img = new Image()
      img.onload = () => {
        const w = Math.min(200, img.naturalWidth)
        add('image', target(), 0, 0, { src, w, h: Math.round(w * img.naturalHeight / img.naturalWidth) })
      }
      img.src = src
    }),
    noBackground: () => { tpl.page.background = ''; redraw() },
    backgroundImage: () => pickImage(src => { tpl.page.backgroundImage = src; redraw() }),
    noBackgroundImage: () => { tpl.page.backgroundImage = ''; redraw() },
    zoom: step => setZoom(+step ? zoom + step * 0.1 : 1),
  }
  const run = cmd => { const [name, ...args] = cmd.split(':'); commands[name](...args) }

  // keep the text selection and keyboard focus where they are when a ribbon button is pressed
  root.addEventListener('mousedown', ev => { if (ev.target.closest('.ddr-panel button:not([draggable])')) ev.preventDefault() })
  root.addEventListener('click', ev => {
    const tab = ev.target.closest('[data-tab]'), button = ev.target.closest('[data-cmd]')
    if (tab) {
      for (const t of $$('[data-tab]')) t.setAttribute('aria-selected', t === tab)
      for (const p of $$('[data-panel]')) p.hidden = p.dataset.panel !== tab.dataset.tab
    }
    if (button) run(button.dataset.cmd)
  })
  root.addEventListener('dragstart', ev => { if (ev.target.dataset?.type) ev.dataTransfer.setData('text/plain', ev.target.dataset.type) })

  root.addEventListener('change', ev => {
    const c = ev.target, d = c.dataset, p = tpl.page
    const value = c.type === 'checkbox' ? c.checked : c.type === 'number' || d.prop === 'lineHeight' ? +c.value || 0 : c.value
    if (d.prop) return apply(d.prop, value)
    if (d.page) p[d.page] = d.page === 'width' || d.page === 'height' ? Math.max(50, value) : value
    else if (d.tpl) tpl[d.tpl] = value
    else if (d.id === 'paper' && PAPER[value]) [p.width, p.height] = p.width > p.height ? [...PAPER[value]].reverse() : PAPER[value]
    else if (d.id === 'orientation') { if ((value === 'Landscape') !== p.width > p.height) [p.width, p.height] = [p.height, p.width] }
    else if (d.id === 'field' && value) {
      c.value = ''
      const text = sel.find(e => e.type === 'text')
      if (!text) return add('text', target(), 0, 0, { text: value })
      text.text += value
    }
    else if (d.id === 'ruler') return void (ruler.hidden = !value)
    else if (d.id === 'snap') return void (snap = value)
    else return
    redraw()
  })

  root.addEventListener('keydown', ev => {
    if (editing) {
      if (ev.key === 'Escape') { cancelled = true; editing.blur() }
      if (ev.key === 'Enter') { ev.preventDefault(); editing.dataset.rich ? document.execCommand('insertLineBreak') : editing.blur() }
      return
    }
    if (ev.target.closest('input,select,textarea,button')) return
    const key = ev.key.toLowerCase(), big = ev.shiftKey ? 10 : 1
    const cmd = ev.ctrlKey || ev.metaKey
      ? { z: ev.shiftKey ? 'redo' : 'undo', y: 'redo', c: 'copy', x: 'cut', v: 'paste', d: 'duplicate', a: 'all', b: 'bold', i: 'italic', u: 'underline' }[key]
      : { delete: 'delete', backspace: 'delete', escape: 'none' }[key]
    const move = { arrowleft: [-1, 0], arrowright: [1, 0], arrowup: [0, -1], arrowdown: [0, 1] }[key]
    if (cmd) run(cmd)
    else if (move && sel.length) {
      for (const e of sel) { e.x += move[0] * big; e.y += move[1] * big }
      redraw()
    }
    else return
    ev.preventDefault()
  })

  // drag over empty paper to select every element the box touches; a plain click selects nothing
  page.onpointerdown = ev => {
    if (ev.target !== page && !ev.target.classList.contains('ddr-band')) return
    const box = document.createElement('div'), s = box.style
    box.className = 'ddr-marquee'
    s.left = px(ev.clientX); s.top = px(ev.clientY)
    root.append(box)
    page.onpointermove = mv => {
      s.left = px(Math.min(ev.clientX, mv.clientX)); s.top = px(Math.min(ev.clientY, mv.clientY))
      s.width = px(Math.abs(mv.clientX - ev.clientX)); s.height = px(Math.abs(mv.clientY - ev.clientY))
    }
    page.onpointerup = () => {
      page.onpointermove = page.onpointerup = null
      const b = box.getBoundingClientRect()
      box.remove()
      sel = [...nodes].filter(([, n]) => {
        const r = n.getBoundingClientRect()
        return r.left < b.right && r.right > b.left && r.top < b.bottom && r.bottom > b.top
      }).map(([e]) => e)
      mark()
      renderProps()
      keepFocus() // so Delete, Ctrl+C and the arrow keys act on what was just selected
    }
    page.setPointerCapture(ev.pointerId)
  }

  if (preview) {
    const dialog = document.createElement('dialog'), box = document.createElement('div')
    dialog.className = 'ddr-preview'
    dialog.innerHTML = `<form method="dialog"><button>${L('Close')}</button> <span role="alert"></span></form><iframe sandbox title="${L('Report preview')}"></iframe>`
    box.innerHTML = group('Report', btn('preview', 'Preview', 'Render the report with sample data'))
    commands.preview = async () => {
      const frame = dialog.querySelector('iframe'), alert = dialog.querySelector('[role=alert]')
      frame.srcdoc = alert.textContent = ''
      try { frame.srcdoc = await preview(structuredClone(tpl)) } catch (err) { alert.textContent = err.message }
      if (!dialog.open) dialog.showModal()
    }
    $('[data-panel=View]').append(box.firstChild)
    root.append(dialog)
  }

  // ---- drawing --------------------------------------------------------------
  // sync shows the selection and the page settings in the ribbon
  function sync() {
    const p = tpl.page
    for (const c of $$('[data-prop]')) {
      const key = c.dataset.prop, e = sel.find(s => STYLE[s.type]?.includes(key))
      c.disabled = !e
      const v = e?.[key] || DEFAULT[key]
      if (c.type === 'checkbox') c.checked = !!v
      else c.value = v
    }
    const styled = key => sel.filter(e => STYLE[e.type]?.includes(key))
    for (const key of ['bold', 'italic', 'underline']) $(`[data-cmd=${key}]`).setAttribute('aria-pressed', styled(key).some(e => e[key]))
    const first = styled('align')[0], align = first && (first.type === 'table' ? first.aligns?.[activeCol] : first.align) || 'left'
    for (const b of $$('[data-cmd^="align:"]')) b.setAttribute('aria-pressed', !!first && b.dataset.cmd === 'align:' + align)
    for (const c of $$('[data-page]')) c.value = p[c.dataset.page] || (c.type === 'color' ? '#ffffff' : 0)
    for (const c of $$('[data-tpl]')) c.type === 'checkbox' ? c.checked = !!tpl[c.dataset.tpl] : c.value = tpl[c.dataset.tpl] ?? ''
    const [short, long] = [p.width, p.height].sort((a, b) => a - b)
    $('[data-id=paper]').value = Object.keys(PAPER).find(k => PAPER[k][0] === short && PAPER[k][1] === long) ?? 'Custom'
    $('[data-id=orientation]').value = p.width > p.height ? 'Landscape' : 'Portrait'
    $('[data-cmd=undo]').disabled = at <= 0
    $('[data-cmd=redo]').disabled = at >= history.length - 1
    for (const c of ['cut', 'copy', 'duplicate', 'delete', 'front', 'back']) $(`[data-cmd=${c}]`).disabled = !sel.length
    $('[data-cmd=paste]').disabled = !clip.length
    $('[data-cmd=distribute]').disabled = sel.length < 3
  }

  function draw() {
    const refocus = page.contains(document.activeElement), p = tpl.page
    for (const b of BANDS) for (const e of tpl.bands[b].elements) fit(e, tpl.bands[b])
    nodes.clear()
    Object.assign(page.style, {
      width: px(p.width), minHeight: px(p.height), padding: SIDES.map(s => px(p[s])).join(' '),
      backgroundColor: p.background || '#fff', backgroundImage: p.backgroundImage ? `url("${p.backgroundImage}")` : '',
    })
    page.replaceChildren(...BANDS.map(name => {
      const band = tpl.bands[name], el = document.createElement('div'), grip = document.createElement('div')
      el.className = 'ddr-band'
      el.dataset.name = name
      el.style.height = px(band.height)
      el.ondragover = ev => ev.preventDefault()
      el.ondrop = ev => {
        ev.preventDefault()
        const type = ev.dataTransfer.getData('text/plain'), r = el.getBoundingClientRect()
        if (NEW[type]) add(type, name, (ev.clientX - r.left) / zoom, (ev.clientY - r.top) / zoom)
      }
      // drag the bottom edge to change the band height
      grip.className = 'ddr-grip'
      grip.title = L('Drag to resize this band')
      grip.onpointerdown = ev => {
        const y0 = ev.clientY, h0 = band.height
        grip.onpointermove = mv => { band.height = Math.max(0, Math.round(h0 + (mv.clientY - y0) / zoom)); el.style.height = px(band.height) }
        grip.onpointerup = () => { grip.onpointermove = grip.onpointerup = null; redraw() } // a band never ends up shorter than what is in it
        grip.setPointerCapture(ev.pointerId)
      }
      el.append(...band.elements.map(e => node(e, band, el)), grip)
      return el
    }))
    // ruler: shaded margins, a tick every 50pt, a number every 100pt
    ruler.style.width = px(p.width)
    ruler.style.backgroundImage = `linear-gradient(90deg,#0002 ${p.marginLeft}px,#0000 ${p.marginLeft}px ${p.width - p.marginRight}px,#0002 ${p.width - p.marginRight}px),repeating-linear-gradient(90deg,#888 0 1px,#0000 1px 50px)`
    ruler.replaceChildren(...Array.from({ length: Math.floor(p.width / 100) + 1 }, (_, i) => Object.assign(document.createElement('span'), { textContent: i * 100, style: `left:${i * 100}px` })))
    setZoom(zoom)
    mark()
    if (refocus) keepFocus()
  }

  function place(e) {
    const s = nodes.get(e).style
    s.left = px(e.x); s.top = px(e.y)
  }

  // snapOffset nudges a dragged element onto the edges of its neighbours and of its band
  function snapOffset(e, x, y) {
    const band = tpl.bands[bandOf(e)], others = band.elements.filter(o => !sel.includes(o))
    const near = (v, size, lines) => {
      for (const line of lines) for (const edge of [0, size]) if (Math.abs(v + edge - line) < 5) return line - edge - v
      return 0
    }
    return [
      near(x, e.w, [0, contentW(), ...others.flatMap(o => [o.x, o.x + o.w])]),
      near(y, e.h, [0, band.height, ...others.flatMap(o => [o.y, o.y + o.h])]),
    ]
  }

  function node(e, band, bandEl) {
    const n = document.createElement('div'), s = n.style, stroke = e.color || '#000'
    nodes.set(e, n)
    n.className = 'ddr-el'
    n.dataset.type = e.type
    n.tabIndex = 0
    n.setAttribute('role', 'group')
    n.setAttribute('aria-label', `${L('element')}: ${e.type}`)
    s.left = px(e.x); s.top = px(e.y); s.width = px(e.w); s.height = px(e.h)
    if (e.type === 'text' || e.type === 'table') {
      s.fontFamily = FONTS[e.font] || ''
      s.fontSize = px(e.fontSize || 10); s.lineHeight = e.lineHeight || ''
      s.fontWeight = e.bold ? 'bold' : ''; s.fontStyle = e.italic ? 'italic' : ''; s.textDecoration = e.underline ? 'underline' : ''
      s.background = e.fill || ''
    }
    if (e.type === 'text') {
      fill(n, e.text)
      s.textAlign = e.align || 'left'; s.color = stroke
      n.ondblclick = () => edit(n, true, () => { e.text = markup(n).replace(/\n$/, ''); redraw() })
    } else if (e.type === 'table') {
      const line = e.border > 0 ? `${e.border}px solid ${stroke}` : '1px dotted #ccd'
      s.gridTemplateColumns = e.cols.map(c => `minmax(0,${c}fr)`).join(' ')
      s.gridTemplateRows = `repeat(${e.cells.length},minmax(0,1fr))`
      s.borderLeft = s.borderTop = line
      e.cells.forEach((row, r) => row.forEach((text, c) => {
        const cell = document.createElement('div')
        cell.className = 'ddr-cell'
        cell.textContent = text
        cell.dataset.col = c
        cell.style.borderRight = cell.style.borderBottom = line
        cell.style.textAlign = e.aligns?.[c] || 'left'
        cell.ondblclick = () => edit(cell, false, () => { e.cells[r][c] = cell.textContent; changed() })
        n.append(cell)
      }))
    } else if (e.type === 'rect') {
      s.background = e.fill || ''
      if (e.border > 0) s.border = `${e.border}px solid ${stroke}`
    } else if (e.type === 'line') {
      s[e.h > e.w ? 'borderLeft' : 'borderTop'] = `${e.border || 1}px solid ${stroke}`
    } else if (e.type === 'image') {
      if (e.src) s.backgroundImage = `url("${e.src}")`
    } else if (e.type === 'chart') {
      n.textContent = `${e.chart} chart\n${e.value || 'value?'} per ${e.label || 'label?'}`
    } else if (e.type === 'barcode') {
      n.textContent = `${e.format === 'code128' ? 'Code 128' : 'QR code'}\n${e.text}`
    } else { // a type this designer does not know: keep it, show what it is
      n.dataset.unknown = 1
      n.textContent = e.type
    }

    let dragged = false
    n.onfocus = () => { if (!sel.includes(e)) select(e) }
    n.onpointerdown = ev => {
      if (editing && n.contains(editing)) return
      if (ev.target.dataset.col) activeCol = +ev.target.dataset.col
      if (ev.shiftKey) return select(e, true) // shift-click adds to / removes from the selection
      if (!sel.includes(e)) select(e)
      else sync()
      const r = n.getBoundingClientRect()
      // bottom-right corner is the browser's own resize grip (CSS resize: both)
      if (e.type !== 'line' && ev.clientX > r.right - 16 * zoom && ev.clientY > r.bottom - 16 * zoom) return
      const start = sel.map(o => [o, o.x, o.y]), mine = start.find(o => o[0] === e)
      n.onpointermove = mv => {
        let dx = (mv.clientX - ev.clientX) / zoom, dy = (mv.clientY - ev.clientY) / zoom
        if (snap && !mv.altKey) {
          const [ox, oy] = snapOffset(e, mine[1] + dx, mine[2] + dy)
          dx += ox; dy += oy
        }
        for (const [o, x, y] of start) { o.x = Math.max(0, Math.round(x + dx)); o.y = Math.round(y + dy); place(o) }
        dragged = true
      }
      n.setPointerCapture(ev.pointerId)
    }
    n.onpointerup = () => {
      if (!n.onpointermove) return
      n.onpointermove = null
      if (!dragged) return
      dragged = false
      // one element dropped over another band: it now belongs to that band
      const top = n.getBoundingClientRect().top
      const over = [...page.children].map(b => [b.dataset.name, b.getBoundingClientRect()]).find(([, r]) => top >= r.top && top < r.bottom)
      if (sel.length === 1 && over && over[0] !== bandOf(e)) {
        e.y = Math.round((top - over[1].top) / zoom)
        return moveTo([e], over[0])
      }
      redraw()
    }
    // picks up the size after a drag on the native resize grip
    if (e.type !== 'line') new ResizeObserver(() => {
      if (!n.offsetWidth || (Math.abs(e.w - n.offsetWidth) < 1 && Math.abs(e.h - n.offsetHeight) < 1)) return
      e.w = n.offsetWidth; e.h = n.offsetHeight
      fit(e, band) // the band grows with it; no redraw here, the resize drag is still going on
      bandEl.style.height = px(band.height)
      place(e)
      setZoom(zoom)
      if (sel.includes(e)) renderProps()
      changed()
    }).observe(n)
    return n
  }

  // ---- properties panel -------------------------------------------------------
  // table fields that are derived from its cells
  const VIRTUAL = {
    rows: { get: e => e.cells.length, set: (e, v) => tableSize(e, v, e.cells[0].length) },
    columns: { get: e => e.cells[0].length, set: (e, v) => tableSize(e, e.cells.length, v) },
    cols: { get: e => e.cols.join(','), set: (e, v) => { e.cols = e.cells[0].map((_, i) => +v.split(',')[i] || 1) } },
  }

  function renderProps() {
    props.replaceChildren()
    const note = text => props.append(Object.assign(document.createElement('p'), { textContent: text }))
    if (sel.length > 1) return note(`${sel.length} ${L('elements selected. Use the ribbon to format, align or move them together.')}`)
    const e = sel[0]
    if (!e) {
      note(L('Band heights. Select an element to edit it; double-click a text or a table cell to type.'))
      for (const b of BANDS) field(tpl.bands[b], 'height', b)
      return
    }
    // keyboard way to move an element to another band
    const band = choice(BANDS, bandOf(e))
    band.onchange = () => moveTo([e], band.value)
    row(L('band'), band)
    for (const k of FIELDS[e.type] ?? BASE) field(e, k, L(LABEL[k] ?? k))
  }

  function choice(values, value) {
    const input = document.createElement('select')
    input.append(...values.map(v => new Option(v)))
    input.value = value
    return input
  }

  function row(label, input) {
    const el = document.createElement('label')
    el.append(label, input)
    props.append(el)
  }

  function field(obj, key, label = key) {
    let input
    const virtual = VIRTUAL[key]
    if (SELECT[key]) {
      input = choice(SELECT[key], obj[key] || SELECT[key][0])
    } else if (key === 'text') {
      input = document.createElement('textarea')
      input.value = obj.text ?? ''
      if (obj.type === 'text') input.title = L('Use <b>, <i> and <u> to style parts of the text')
    } else {
      input = document.createElement('input')
      input.type = key === 'src' ? 'file' : key === 'color' ? 'color' : NUMERIC.has(key) ? 'number' : 'text'
      if (input.type === 'number') input.min = 0
      if (input.type === 'file') input.accept = 'image/png,image/jpeg,image/gif'
      else input.value = virtual ? virtual.get(obj) : obj[key] ?? (key === 'color' ? '#000000' : '')
      input.placeholder = L(PLACEHOLDER[key] ?? '')
    }
    input.oninput = () => {
      if (input.type === 'file') {
        const reader = new FileReader()
        reader.onload = () => { obj.src = reader.result; draw(); changed() }
        if (input.files[0]) reader.readAsDataURL(input.files[0])
        return
      }
      const value = input.type === 'number' ? Math.max(0, +input.value || 0) : input.value
      if (virtual) virtual.set(obj, value)
      else obj[key] = value
      draw()
      changed()
    }
    // typing a position outside the page is corrected when the field is left, not mid-keystroke
    input.onchange = () => { if (!virtual && input.type === 'number' && +input.value !== obj[key]) input.value = obj[key] }
    row(label, input)
  }

  load(template, true)
  return {
    getTemplate: () => structuredClone(tpl),
    setTemplate: t => load(t),
  }
}
