// Drag-and-drop report designer with a word-processor style ribbon.
// No dependencies, no build step:
//   import { createDesigner } from 'ddreport-designer'
//   const designer = createDesigner(element, { template, onChange, preview, lang, theme })
// The template it edits is the JSON the ddreport backend renders. 1px on screen = 1pt on paper.
// preview is optional: async template => HTML string (ask your backend to render it as "html").
// lang is 'en' (default) or 'id'. theme is 'light' (default), 'dark' or 'auto' (follows the system).
// fontBase is the URL the bundled fonts are served from (`ddreport -serve` has them at 'fonts/');
// without it the page shows them only where they are installed. fonts adds names from ddreport.Families.
// fields lists the data's field names for the Field menu; designer.setFields(list) changes it later.
// data is sample data (fields plus rows, or just rows) that charts are drawn from; designer.setData(data) changes it.

// the bands in print order; the second and third group levels show when Group by names that many fields
const BANDS = ['header', 'columnHeader', 'groupHeader', 'groupHeader2', 'groupHeader3', 'detail', 'groupFooter3', 'groupFooter2', 'groupFooter', 'summary', 'footer']
const HEIGHT = { header: 60, summary: 30, footer: 30 } // a new band's height; the others 20
const LEVEL = { groupHeader2: 2, groupFooter2: 2, groupHeader3: 3, groupFooter3: 3 }
const SIDES = ['marginTop', 'marginRight', 'marginBottom', 'marginLeft']
const PAPER = { A4: [595, 842], A5: [420, 595], Letter: [612, 792], Legal: [612, 1008], F4: [595, 935] }
const FONTS = {
  Helvetica: 'Helvetica,Arial,sans-serif', Times: "'Times New Roman',Times,serif", Courier: "'Courier New',Courier,monospace",
  // bundled with the renderer
  Roboto: 'Roboto,sans-serif', 'Open Sans': "'Open Sans',sans-serif", Lato: 'Lato,sans-serif', Montserrat: 'Montserrat,sans-serif',
  Lora: 'Lora,serif', 'Playfair Display': "'Playfair Display',serif", 'Great Vibes': "'Great Vibes',cursive",
}
const BUNDLED = Object.keys(FONTS).slice(3)
const FACES = [['Regular', 400, 'normal'], ['Bold', 700, 'normal'], ['Italic', 400, 'italic'], ['BoldItalic', 700, 'italic']]
const NEW = {
  text: { w: 160, h: 16, text: 'Text', fontSize: 10 },
  table: { w: 360, h: 40, border: 1, fontSize: 10, cells: [['', '', ''], ['', '', '']], cols: [1, 1, 1], aligns: [] },
  image: { w: 80, h: 80, src: '' },
  rect: { w: 120, h: 40, border: 1 },
  line: { w: 200, h: 1, border: 1 },
  chart: { w: 240, h: 140, chart: 'bar', label: '', value: '', color: '#4e79a7' },
  barcode: { w: 70, h: 70, text: '{{code}}', format: 'qr' },
  subreport: { w: 400, h: 80, data: 'items' },
}
// what the properties panel edits per type; fonts, colours and alignment live in the ribbon
const BASE = ['x', 'y', 'w', 'h']
const FIELDS = {
  text: [...BASE, 'text', 'showIf'],
  table: [...BASE, 'data', 'rows', 'columns', 'cols', 'heights', 'padding', 'border', 'grid', 'headerLine', 'footerLine', 'frame', 'color', 'textColor', 'stripe', 'header', 'headerFill', 'headerColor', 'footer', 'footerFill', 'footerColor', 'showIf'],
  image: [...BASE, 'src', 'dataSrc', 'showIf'],
  rect: [...BASE, 'border', 'color', 'showIf'],
  line: [...BASE, 'slope', 'border', 'color', 'showIf'],
  chart: [...BASE, 'chart', 'label', 'value', 'color', 'showIf'],
  barcode: [...BASE, 'format', 'text', 'showText', 'color', 'showIf'],
  subreport: [...BASE, 'data', 'showIf'],
}
// which ribbon properties apply to which type
const STYLE = {
  text: ['font', 'fontSize', 'lineHeight', 'bold', 'italic', 'underline', 'align', 'valign', 'color', 'fill', 'grow'],
  table: ['font', 'fontSize', 'lineHeight', 'bold', 'italic', 'underline', 'align', 'valign', 'color', 'fill', 'grow'],
  rect: ['fill'],
}
// a table's text colour is textColor: its color is the colour of its lines
const propOf = (e, k) => e.type === 'table' && k === 'color' ? 'textColor' : k
const DEFAULT = { font: 'Helvetica', fontSize: 10, lineHeight: 1.2, valign: 'top', color: '#000000', fill: '#ffffff', grow: false }
const NUMERIC = new Set(['x', 'y', 'w', 'h', 'border', 'rows', 'columns', 'height', 'headerLine', 'footerLine', 'frame'])
const SELECT = { chart: ['bar', 'hbar', 'line', 'pie', 'donut'], format: ['qr', 'code128'], grid: ['all', 'outer', 'rows'], slope: ['straight', 'down', 'up'] }
const BOOL = new Set(['header', 'footer', 'showText', 'newPage'])
const CLEARABLE = new Set(['textColor', 'stripe', 'headerFill', 'headerColor', 'footerFill', 'footerColor']) // colours that may be left empty
const LABEL = {
  x: 'from left', y: 'from top', w: 'width', h: 'height',
  cols: 'column widths', color: 'line colour', grid: 'lines', textColor: 'text colour', stripe: 'banded rows',
  header: 'header row', headerFill: 'header fill', headerColor: 'header text', footer: 'total row', footerFill: 'total row fill', footerColor: 'total row text',
  showIf: 'show when', dataSrc: 'picture from data', slope: 'direction', showText: 'show text', newPage: 'new page',
  data: 'rows from data', heights: 'row heights', padding: 'cell padding', headerLine: 'header line', footerLine: 'total line', frame: 'frame',
}
const PLACEHOLDER = { label: 'row field', value: 'field or qty * price', cols: '2,1,1', heights: '1,1,1', padding: '3', data: 'items', showIf: "paid = 'yes'", dataSrc: '{{photo}}' }
const TITLE = {
  data: 'A list in the data, such as items: the rows between the header and the total row repeat for each of its items. {{field}} is a field of the item, {{index}} its number.',
}
const FIELD_TOKENS = ['{{page}}', '{{pages}}', '{{index}}', '{{now:dd/MM/yyyy}}', '{{field:2}}', '{{field:dd MMMM yyyy}}', '{{qty * price:2}}', '{{sum(field):2}}', '{{avg(field):2}}', '{{min(field)}}', '{{max(field)}}', '{{count()}}', "{{if(field > 0, 'yes', 'no')}}", '{{terbilang(field)}}']

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
  Preview: 'Pratinjau', 'Render the report with sample data': 'Cetak laporan dengan data contoh', Close: 'Tutup', 'Report preview': 'Pratinjau laporan', 'Rendering…': 'Mencetak…', page: 'halaman', pages: 'halaman',
  'Band heights. Select an element to edit it; double-click a text or a table cell to type.': 'Tinggi band. Pilih elemen untuk mengubahnya; klik ganda teks atau sel tabel untuk mengetik.',
  'elements selected. Use the ribbon to format, align or move them together.': 'elemen dipilih. Pakai ribbon untuk memformat, meratakan, atau memindahkannya bersama.',
  'Drag to resize this band': 'Tarik untuk mengubah tinggi band ini', band: 'band', text: 'teks', rows: 'baris', columns: 'kolom', 'column widths': 'lebar kolom',
  border: 'garis', 'line colour': 'warna garis', src: 'gambar', chart: 'jenis chart', label: 'label', value: 'nilai', format: 'format', color: 'warna',
  'field or qty * price': 'field atau qty * price', 'Use <b>, <i> and <u> to style parts of the text': 'Pakai <b>, <i>, dan <u> untuk memformat sebagian teks',
  'element': 'elemen', Properties: 'Properti', selected: 'dipilih', 'Drag to resize': 'Tarik untuk mengubah ukuran', 'Delete this element': 'Hapus elemen ini', 'Drop here to delete': 'Lepas di sini untuk menghapus', 'Fit the page to the width': 'Paskan halaman ke lebar',
  lines: 'garis', 'text colour': 'warna teks', 'banded rows': 'baris selang-seling', 'header row': 'baris judul', 'header fill': 'isi judul', 'header text': 'teks judul',
  'total row': 'baris total', 'total row fill': 'isi baris total', 'total row text': 'teks baris total', Clear: 'Kosongkan',
  header: 'kepala', columnHeader: 'judul kolom', groupHeader: 'kepala grup', detail: 'detail', groupFooter: 'kaki grup', summary: 'ringkasan', footer: 'kaki halaman',
  groupHeader2: 'kepala grup 2', groupFooter2: 'kaki grup 2', groupHeader3: 'kepala grup 3', groupFooter3: 'kaki grup 3',
  'row field, or up to three: region, city': 'field baris, atau sampai tiga: region, city', 'A sheet per group': 'Sheet per grup',
  'Excel (xlsx, xls): a worksheet for each group of the first Group by field': 'Excel (xlsx, xls): satu sheet untuk setiap grup dari field Group by pertama',
  'header line': 'garis judul', 'total line': 'garis total', frame: 'bingkai',
  'Sub-report': 'Sub-laporan', 'Edit sub-report…': 'Ubah sub-laporan…', 'list in the data': 'daftar di data', Done: 'Selesai',
  'A report of its own for each item of a list in the data: header, column header, groups, detail and summary. Its footer is not used.':
    'Laporan tersendiri untuk setiap item daftar di data: kepala, judul kolom, grup, detail, dan ringkasan. Kaki halamannya tidak dipakai.',
  'show when': 'tampil jika', 'picture from data': 'gambar dari data', direction: 'arah', straight: 'lurus', down: 'turun', up: 'naik', 'show text': 'tampilkan teks',
  'new page': 'halaman baru', all: 'semua', first: 'pertama', last: 'terakhir', 'every page': 'setiap halaman', 'first page': 'halaman pertama', 'last page': 'halaman terakhir',
  'Justify': 'Rata kanan-kiri', 'Vertical alignment': 'Perataan vertikal', top: 'atas', middle: 'tengah', bottom: 'bawah',
  'Sort by': 'Urutkan', 'row field, -field for descending': 'field baris, -field untuk menurun', 'Dates in': 'Tanggal dalam', English: 'Inggris', Indonesian: 'Indonesia',
  'CSV separator': 'Pemisah CSV', Tab: 'Tab', 'No thousands separator': 'Tanpa pemisah ribuan',
  'rows from data': 'baris dari data', 'row heights': 'tinggi baris', 'cell padding': 'jarak tepi sel', Cell: 'Sel', 'cell fill': 'isi sel', 'cell text': 'teks sel',
  'cell align': 'rata sel', 'cell bold': 'tebal sel', 'cell italic': 'miring sel', 'as the row': 'seperti barisnya', yes: 'ya', no: 'tidak', justify: 'kanan-kiri', left: 'kiri', center: 'tengah', right: 'kanan',
  'Merge right': 'Gabung ke kanan', 'Merge down': 'Gabung ke bawah', Unmerge: 'Pisahkan',
  'Drag to change the column width': 'Seret untuk mengubah lebar kolom', 'Drag to change the row height': 'Seret untuk mengubah tinggi baris',
  'A list in the data, such as items: the rows between the header and the total row repeat for each of its items. {{field}} is a field of the item, {{index}} its number.':
    'Daftar di data, misalnya items: baris di antara judul dan baris total diulang untuk setiap itemnya. {{field}} adalah field item itu, {{index}} nomor urutnya.',
  Expressions: 'Ekspresi', 'Row above': 'Baris di atas', 'Row below': 'Baris di bawah', 'Delete row': 'Hapus baris', 'Column left': 'Kolom di kiri', 'Column right': 'Kolom di kanan', 'Delete column': 'Hapus kolom',
  'Click a cell first; rows and columns are added next to it': 'Klik sebuah sel dulu; baris dan kolom ditambahkan di sebelahnya',
  'Start here': 'Mulai di sini',
  'Click to put one on the page. Drag it to move it, double-click it to type.': 'Klik untuk menaruhnya di halaman. Geser untuk memindah, klik ganda untuk mengetik.',
  'from left': 'dari kiri', 'from top': 'dari atas', width: 'lebar', height: 'tinggi',
  table: 'tabel', image: 'gambar', rect: 'kotak', line: 'garis', barcode: 'barcode', subreport: 'sub-laporan',
}

// simple: true hides what a first report does not need, and names every button
const SIMPLE = ['Arrange', 'Insert.Report', 'Grouping', 'Margins', 'Layout.Data', 'Show', 'cut', 'valign', 'lineHeight',
  'x', 'y', 'showIf', 'cols', 'heights', 'padding', 'grid', 'headerLine', 'footerLine', 'frame', 'dataSrc']
// only a picture inside the template is drawn: a link elsewhere would be fetched by whoever opens the template
const isPicture = v => /^data:image\/[\w.+-]+;base64,[A-Za-z0-9+/=\s]*$/.test(v)

// icons: 24 x 24, drawn as strokes
const ICONS = {
  undo: 'M9 14 4 9l5-5M4 9h10a6 6 0 0 1 0 12h-3',
  redo: 'm15 14 5-5-5-5M20 9H10a6 6 0 0 0 0 12h3',
  cut: 'M6 9a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM6 21a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM8.2 7.8 20 20M8.2 16.2 20 4',
  copy: 'M9 9h11v11H9zM5 15H4V4h11v1',
  paste: 'M9 3h6v4H9zM15 5h4v16H5V5h4M9 12h6M9 16h6',
  duplicate: 'M8 8h12v12H8zM4 16V4h12M14 11v6M11 14h6',
  delete: 'M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13M10 11v5M14 11v5',
  noFill: 'M5 5h14v14H5zM5 5l14 14',
  alignLeft: 'M4 6h16M4 10h10M4 14h16M4 18h10',
  alignCenter: 'M4 6h16M7 10h10M4 14h16M7 18h10',
  alignRight: 'M4 6h16M10 10h10M4 14h16M10 18h10',
  alignJustify: 'M4 6h16M4 10h16M4 14h16M4 18h16',
  edgeLeft: 'M4 3v18M8 6h11v4H8zM8 14h6v4H8z',
  edgeCenter: 'M12 3v18M6 6h12v4H6zM8 14h8v4H8z',
  edgeRight: 'M20 3v18M5 6h11v4H5zM10 14h6v4h-6z',
  edgeTop: 'M3 4h18M6 8h4v11H6zM14 8h4v6h-4z',
  edgeMiddle: 'M3 12h18M6 6h4v12H6zM14 8h4v8h-4z',
  edgeBottom: 'M3 20h18M6 5h4v11H6zM14 10h4v6h-4z',
  distribute: 'M4 3v18M20 3v18M9 7h6v10H9z',
  front: 'M9 9h11v11H9zM4 15V4h11',
  back: 'M4 4h11v11H4zM9 20h11V9',
  text: 'M4 5h16v14H4zM9 9h6M12 9v7',
  table: 'M4 5h16v14H4zM4 10h16M4 15h16M10 5v14M15 5v14',
  image: 'M4 5h16v14H4zM4 16l5-5 4 4 3-3 4 4M8 9h.01',
  rect: 'M4 6h16v12H4z',
  line: 'M4 19 20 5',
  chart: 'M4 20h16M7 20v-7M12 20V7M17 20v-10',
  barcode: 'M4 5v14M7 5v14M11 5v14M14 5v14M17 5v14M20 5v14',
  subreport: 'M4 4h16v16H4zM8 8h8M8 12h8M8 16h5',
  preview: 'M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12zM12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6z',
  zoomOut: 'M5 12h14',
  zoomIn: 'M5 12h14M12 5v14',
  close: 'M6 6l12 12M18 6 6 18',
  fit: 'M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5',
}

const CSS = `
.ddr{--a:#2563eb;--a-soft:#e7eefd;--a-text:#fff;--bg:#f6f7f9;--surface:#fff;--text:#1f2937;--muted:#6b7280;--line:#e2e5ea;--stage:#eceef2;--hover:#eef2f7;
  container-type:inline-size;display:flex;flex-direction:column;min-height:560px;font:13px/1.4 system-ui,-apple-system,"Segoe UI",sans-serif;color:var(--text);background:var(--bg);border:1px solid var(--line);border-radius:10px;overflow:hidden;color-scheme:light}
.ddr[data-theme=dark]{--a:#7aa2ff;--a-soft:#1e2a44;--a-text:#0b1220;--bg:#161a22;--surface:#1d222c;--text:#e5e9f0;--muted:#98a2b3;--line:#303745;--stage:#11141a;--hover:#272e3b;color-scheme:dark}
@media (prefers-color-scheme:dark){.ddr[data-theme=auto]{--a:#7aa2ff;--a-soft:#1e2a44;--a-text:#0b1220;--bg:#161a22;--surface:#1d222c;--text:#e5e9f0;--muted:#98a2b3;--line:#303745;--stage:#11141a;--hover:#272e3b;color-scheme:dark}}
.ddr *{box-sizing:border-box}
.ddr [hidden]{display:none!important}
.ddr[data-simple] .ddr-panel .ddr-b:not(.ddr-tile){flex-direction:column;gap:1px;height:46px;min-width:50px;padding:2px 5px;font-size:10.5px}
.ddr[data-simple] .ddr-panel .ddr-b:not(.ddr-tile)::after{content:attr(data-label);font:10.5px/1.1 system-ui,sans-serif}
.ddr[data-simple] .ddr-band::before{font-size:10px;line-height:14px}
.ddr[data-simple] .ddr-panel .ddr-swatch{flex-direction:column;gap:3px;width:auto;min-width:50px;height:46px;padding:4px 5px 2px}
.ddr[data-simple] .ddr-panel .ddr-swatch input{position:static;width:22px}
.ddr[data-simple] .ddr-panel .ddr-swatch span{margin:0}
.ddr[data-simple] .ddr-panel .ddr-swatch::after{content:attr(data-label);font:400 10.5px/1.1 system-ui,sans-serif}
.ddr button,.ddr select,.ddr input,.ddr textarea{font:inherit;color:inherit}
.ddr :is(select,input:not([type=checkbox],[type=color],[type=range]),textarea){height:30px;padding:0 8px;border:1px solid var(--line);border-radius:6px;background:var(--surface)}
.ddr textarea{height:76px;padding:6px 8px;resize:vertical}
.ddr :is(button,select,input,textarea,summary,[tabindex]):focus-visible{outline:2px solid var(--a);outline-offset:1px}
.ddr-i{width:18px;height:18px;flex:none;fill:none;stroke:currentColor;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round}

.ddr-tabs{display:flex;gap:2px;padding:6px 8px 0;background:var(--surface);border-bottom:1px solid var(--line);overflow-x:auto;scrollbar-width:none}
.ddr-tabs button{border:0;background:none;padding:7px 14px 8px;border-bottom:2px solid transparent;margin-bottom:-1px;color:var(--muted);font-weight:600;cursor:pointer;white-space:nowrap;border-radius:6px 6px 0 0}
.ddr-tabs button:hover{color:var(--text);background:var(--hover)}
.ddr-tabs button[aria-selected=true]{color:var(--a);border-color:var(--a)}
.ddr-panel{display:flex;gap:0;padding:8px 6px;background:var(--surface);border-bottom:1px solid var(--line);overflow-x:auto;scrollbar-width:thin;min-height:70px}
.ddr-panel[hidden]{display:none}
.ddr-group{display:flex;flex-direction:column;justify-content:space-between;gap:4px;padding:0 10px;border-right:1px solid var(--line);flex:none}
.ddr-group:last-child{border-right:0}
.ddr-group>div{display:flex;gap:3px;align-items:center}
.ddr-group>span{font-size:11px;color:var(--muted);text-align:center;white-space:nowrap}
.ddr-b{display:inline-flex;align-items:center;justify-content:center;gap:6px;min-width:32px;height:32px;padding:0 7px;border:1px solid transparent;border-radius:6px;background:none;cursor:pointer;white-space:nowrap}
.ddr-b:hover:not(:disabled){background:var(--hover)}
.ddr-b[aria-pressed=true]{background:var(--a-soft);color:var(--a);border-color:var(--a)}
.ddr-b:disabled{opacity:.35;cursor:default}
.ddr-b[draggable]{cursor:grab}
.ddr-tile{flex-direction:column;gap:2px;height:46px;min-width:58px;padding:4px 8px;font-size:11.5px}
.ddr-tile .ddr-i{width:20px;height:20px;color:var(--a)}
.ddr-letter{font:600 15px Georgia,"Times New Roman",serif}
.ddr-group input[type=number]{width:58px}
.ddr-group input[type=text]{width:92px}
.ddr-swatch{position:relative;display:inline-flex;align-items:center;justify-content:center;width:32px;height:32px;border-radius:6px;font-weight:700;cursor:pointer}
.ddr-swatch:hover{background:var(--hover)}
.ddr-swatch input{position:absolute;left:6px;right:6px;bottom:3px;width:20px;height:6px;padding:0;border:0;background:none;cursor:pointer}
.ddr-swatch input::-webkit-color-swatch-wrapper{padding:0}
.ddr-swatch input::-webkit-color-swatch{border:1px solid var(--line);border-radius:2px}
.ddr-swatch span{margin-top:-6px;font-size:13px}
.ddr-group label{display:inline-flex;align-items:center;gap:5px;white-space:nowrap}
.ddr-group input[type=checkbox],.ddr-props input[type=checkbox]{width:15px;height:15px;accent-color:var(--a)}

.ddr-work{position:relative;display:flex;flex:1;min-height:0}
.ddr-stage{flex:1;min-width:0;overflow:auto;padding:16px;touch-action:pan-x pan-y;background:var(--stage) radial-gradient(circle,color-mix(in srgb,var(--muted) 28%,transparent) 1px,transparent 1.2px) 0 0/16px 16px}
.ddr-sizer{margin:0} /* its left margin centres the page on a whole pixel, see center() */
.ddr-zoom{transform-origin:0 0;width:max-content}
.ddr-ruler{position:relative;height:18px;margin-bottom:6px;font-size:9px;color:#555;background-color:#fff;border-radius:3px;box-shadow:0 0 0 1px #0002}
.ddr-ruler span{position:absolute;top:1px;padding-left:3px}
.ddr-page{position:relative;display:flex;flex-direction:column;color:#000;background:#fff center/100% 100% no-repeat;box-shadow:0 1px 3px #0003,0 8px 24px #0002;outline:none}
.ddr-band{position:relative;flex:none;border-bottom:1px dashed #b6c2d2}
.ddr-band:hover{background:#2563eb08}
.ddr-band[data-name=footer]{margin-top:auto;border-top:1px dashed #b6c2d2}
.ddr-band[data-thin]::before{display:none}
.ddr-band::before{content:attr(data-label);position:absolute;right:0;top:0;padding:0 5px;font:600 8.5px/13px system-ui,sans-serif;color:#64748b;background:#f1f5f9;border-radius:0 0 0 4px;pointer-events:none;opacity:.85}
.ddr-grip{position:absolute;left:0;right:0;bottom:-4px;height:8px;cursor:ns-resize;z-index:1}
.ddr-grip:hover,.ddr-grip:active{background:#2563eb55}
.ddr-hint{position:absolute;left:50%;top:38%;transform:translate(-50%,-50%);width:min(360px,80%);padding:14px 16px;border:1.5px dashed #94a3b8;border-radius:10px;text-align:center;font:13px/1.45 system-ui,sans-serif;color:#475569;background:#fffc;pointer-events:none}
.ddr-hint b{display:block;margin-bottom:2px;color:#1f2937}
.ddr-hint div{display:flex;flex-wrap:wrap;justify-content:center;gap:6px;margin:8px 0}
.ddr-hint .ddr-b{pointer-events:auto;border-color:#cbd5e1;background:#fff;color:#1f2937}
.ddr-hint .ddr-b:hover{background:#eef2f7}
.ddr-el{position:absolute;overflow:hidden;cursor:move;outline:1px dashed #cbd5e1;white-space:pre-wrap;overflow-wrap:anywhere;font-family:Helvetica,Arial,sans-serif;line-height:1.2;touch-action:none;user-select:none}
.ddr-el:hover{outline:1px solid #93b4f5}
.ddr-el[data-type=line]{overflow:visible}
.ddr-el[data-type=line]::after{content:'';position:absolute;inset:-5px}
.ddr-el[data-type=image]{background:#eef2f7 center/100% 100% no-repeat}
.ddr-el:is([data-type=chart],[data-type=barcode]){background:#fff}
.ddr-el:is([data-type=subreport],[data-unknown]){background:#eef4fb;color:#334155;font:10px/1.3 system-ui,sans-serif;padding:4px}
.ddr-el[data-type=table]{display:grid}
.ddr-cell{overflow:hidden;padding:2px 3px;min-width:0}
.ddr-tgrip{position:absolute;z-index:2;display:none}
.ddr-el.ddr-sel .ddr-tgrip{display:block}
.ddr-tgrip[data-kind=col]{cursor:col-resize}
.ddr-tgrip[data-kind=row]{cursor:row-resize}
.ddr-tgrip:hover,.ddr-tgrip:active{background:#2563eb55}
.ddr-el.ddr-sel .ddr-cell[data-active]{outline:2px solid #f59e0b;outline-offset:-2px}
.ddr-cell[data-body]{box-shadow:inset 3px 0 #93c5fd}
.ddr-el[data-repeat]::after{content:"\\21BB  " attr(data-repeat);position:absolute;right:2px;bottom:1px;padding:0 4px;border-radius:3px;font:600 9px/14px system-ui,sans-serif;color:#1d4ed8;background:#dbeafe;pointer-events:none}
.ddr-props .ddr-celltitle{margin:8px 0 2px;font-weight:700;color:var(--text)}
.ddr-el[data-cond]{outline:1px dashed #f59e0b}
.ddr-el.ddr-sel{outline:2px solid #2563eb}
.ddr-handle{position:absolute;z-index:3;width:11px;height:11px;margin:-6px 0 0 -6px;background:#2563eb;border:2px solid #fff;border-radius:3px;box-shadow:0 0 0 1px #2563eb;cursor:nwse-resize;touch-action:none}
@media (pointer:coarse){.ddr-handle{width:20px;height:20px;margin:-10px 0 0 -10px;border-radius:50%}.ddr-b{min-width:38px;height:38px}}
.ddr .ddr-x{position:absolute;z-index:3;display:grid;place-items:center;width:18px;height:18px;margin:-9px 0 0 -9px;padding:0;border:2px solid #fff;border-radius:50%;background:#dc2626;color:#fff;box-shadow:0 0 0 1px #dc2626;cursor:pointer;touch-action:none}
.ddr-x .ddr-i{width:10px;height:10px;stroke-width:3.2}
.ddr-x:hover{background:#b91c1c}
.ddr-trash{position:absolute;z-index:5;left:50%;top:12px;transform:translateX(-50%);display:flex;align-items:center;gap:8px;padding:9px 16px;border:2px dashed #dc2626;border-radius:999px;background:var(--surface);color:#dc2626;font-weight:600;box-shadow:0 6px 18px #0003;pointer-events:none;white-space:nowrap}
.ddr-trash[hidden]{display:none}
.ddr-trash.over{background:#dc2626;color:#fff;border-style:solid;transform:translateX(-50%) scale(1.06)}
@media (pointer:coarse){.ddr-x{width:26px;height:26px;margin:-13px 0 0 -13px}.ddr-x .ddr-i{width:13px;height:13px}}
.ddr-marquee{position:fixed;z-index:10;border:1px solid #2563eb;background:#2563eb22;pointer-events:none}
.ddr [contenteditable]:not([contenteditable=false]){user-select:text;cursor:text;outline:2px solid #f59e0b}

.ddr-side{flex:none;width:264px;overflow:auto;background:var(--surface);border-left:1px solid var(--line)}
.ddr-side>summary{position:sticky;top:0;z-index:1;padding:10px 12px;font-weight:700;cursor:pointer;background:var(--surface);border-bottom:1px solid var(--line);list-style-position:inside}
.ddr-props{display:flex;flex-direction:column;gap:6px;padding:12px}
.ddr-props label{display:flex;justify-content:space-between;align-items:center;gap:8px;color:var(--muted)}
.ddr-props :is(input:not([type=checkbox]),select,textarea){flex:none;width:122px;color:var(--text)}
.ddr-props input[type=color]{height:30px;padding:2px;border:1px solid var(--line);border-radius:6px;background:var(--surface)}
.ddr-props input[type=file]{height:auto;padding:4px;font-size:11px}
.ddr-props p{margin:0 0 4px;color:var(--muted)}
.ddr-props span{display:flex;gap:4px;flex:none}
.ddr-props span input{width:92px!important}
.ddr-tablebar{display:grid;grid-template-columns:1fr 1fr 1fr;gap:4px;margin-top:4px}
.ddr-tablebar button{min-height:28px;padding:2px 4px;font-size:11.5px;border:1px solid var(--line);border-radius:6px;background:var(--surface);cursor:pointer}
.ddr-tablebar button:hover{background:var(--hover)}
.ddr-props span button{width:26px;border:1px solid var(--line);border-radius:6px;background:var(--surface);cursor:pointer}

.ddr-status{display:flex;align-items:center;gap:8px;padding:4px 8px 4px 12px;background:var(--surface);border-top:1px solid var(--line);font-size:12px;color:var(--muted)}
.ddr-status [data-id=status]{flex:1;min-width:0;overflow:hidden;white-space:nowrap;text-overflow:ellipsis}
.ddr-status .ddr-b{min-width:28px;height:28px;padding:0 5px}
.ddr-status input[type=range]{width:110px;accent-color:var(--a)}
.ddr-status output{min-width:40px;text-align:right;font-variant-numeric:tabular-nums}

.ddr-preview{padding:10px;border:1px solid var(--line);border-radius:10px;background:var(--surface);color:var(--text)}
.ddr-preview::backdrop,.ddr-subdialog::backdrop{background:#0f172a88}
.ddr-subdialog{width:min(96vw,1200px);height:90vh;padding:0;border:1px solid var(--line);border-radius:10px;background:var(--bg);color:var(--text);overflow:hidden}
.ddr-subdialog[open]{display:flex;flex-direction:column}
.ddr-subdialog form{display:flex;align-items:center;gap:8px;padding:8px 12px;border-bottom:1px solid var(--line);background:var(--surface)}
.ddr-subdialog form span{flex:1}
.ddr-subdialog form button,.ddr-subedit{height:30px;padding:0 12px;border:1px solid var(--line);border-radius:6px;background:var(--surface);color:var(--text);cursor:pointer}
.ddr-subdialog .ddr-primary{background:var(--a);border-color:var(--a);color:var(--a-text);font-weight:600}
.ddr-subdialog>div{flex:1;min-height:0;border:0;border-radius:0}
.ddr-preview form{display:flex;align-items:center;gap:12px}
.ddr-preview [role=status]{color:var(--muted)}
.ddr-sheet{position:relative;display:grid;place-items:center;width:min(92vw,900px);height:80vh;margin-top:8px;border:1px solid var(--line);border-radius:6px;overflow:hidden;background:#e5e7eb}
.ddr-sheet p{margin:0;padding:16px;max-width:60ch;color:#475569;text-align:center;white-space:pre-wrap}
.ddr-sheet p.err{color:#dc2626}
.ddr-sheet iframe{position:absolute;inset:0;width:100%;height:100%;border:0;visibility:hidden}
.ddr-sheet iframe.ready{visibility:visible}

@container (max-width:760px){
  .ddr-work{flex-direction:column}
  .ddr-side{width:auto;max-height:42%;border-left:0;border-top:1px solid var(--line)}
  .ddr-props{display:grid;grid-template-columns:repeat(auto-fill,minmax(230px,1fr));gap:6px 14px}
  .ddr-props p{grid-column:1/-1}
  .ddr-props :is(input:not([type=checkbox]),select,textarea){width:92px}
  .ddr-group>span{display:none}
  .ddr-panel{min-height:0}
  .ddr-stage{padding:10px}
  .ddr-status input[type=range]{display:none}
}
`

export const emptyTemplate = () => ({
  page: { width: 595, height: 842, margin: 0, marginTop: 36, marginRight: 36, marginBottom: 36, marginLeft: 36, background: '', backgroundImage: '' }, // A4
  dataKey: 'rows',
  groupBy: '',
  groupPageBreak: false,
  bands: Object.fromEntries(BANDS.map(b => [b, { height: HEIGHT[b] ?? 20, elements: [] }])),
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
      elements: (Array.isArray(band.elements) ? band.elements : []).filter(isObject).map(cleanElement),
    }
  }
  for (const k of ['background', 'backgroundImage']) out.page[k] = String(out.page[k] ?? '')
  return out
}

// cleanElement repairs one element, from a template or from the clipboard, so drawing it cannot fail
function cleanElement(e) {
  const def = NEW[e.type] ?? { w: 80, h: 20 }
  e = { ...e, type: String(e.type ?? 'text'), x: Math.max(0, num(e.x)), y: Math.max(0, num(e.y)), w: Math.max(0, num(e.w, def.w)), h: Math.max(0, num(e.h, def.h)) }
  for (const k of ['text', 'src', 'label', 'value']) if (e[k] != null && typeof e[k] !== 'string') e[k] = String(e[k])
  if (e.type === 'text' || e.type === 'barcode') e.text = String(e.text ?? '')
  if (e.type === 'subreport' && !isObject(e.report)) e.report = { bands: {} } // its bands are cleaned when it is opened
  if (e.type === 'table') {
    const rows = (Array.isArray(e.cells) ? e.cells : []).filter(Array.isArray), width = Math.max(1, ...rows.map(r => r.length))
    e.cells = (rows.length ? rows : [[]]).map(r => Array.from({ length: width }, (_, c) => String(r[c] ?? '')))
    e.cols = Array.from({ length: width }, (_, c) => num(e.cols?.[c]) > 0 ? num(e.cols[c]) : 1)
    e.heights = Array.from({ length: e.cells.length }, (_, r) => num(e.heights?.[r]) > 0 ? num(e.heights[r]) : 1)
    e.aligns = Array.isArray(e.aligns) ? e.aligns : []
    // per-cell styles: only those of cells that exist, and only objects
    e.cellStyle = Object.fromEntries(Object.entries(isObject(e.cellStyle) ? e.cellStyle : {}).filter(([k, v]) => {
      const [r, c] = k.split(',').map(Number)
      return isObject(v) && r >= 0 && c >= 0 && r < e.cells.length && c < width
    }))
  }
  return e
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

// ---- drawings on the canvas: charts from the sample data, barcodes -------------
// They show what will print; the renderer draws the real thing (an exact,
// scannable code; the chart from the real data with its value scale).

// capture keeps a drag's pointer events coming to el. A browser that refuses
// (Firefox, for a pointer it does not know) leaves the drag working on the
// events it gets anyway, instead of stopping it.
const capture = (el, ev) => { try { el.setPointerCapture(ev.pointerId) } catch {} }

const PALETTE = ['#4e79a7', '#f28e2b', '#e15759', '#76b7b2', '#59a14f', '#edc948', '#b07aa1', '#9c755f']
const SVG = 'http://www.w3.org/2000/svg'
const svgEl = (name, attrs = {}) => {
  const el = document.createElementNS(SVG, name)
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v)
  return el
}

// value works out a chart value for a row: a field, a number, or + - * / and brackets
// over them, as the renderer does ("qty * price"); NaN when it is not a number
function value(expr, row) {
  let i = 0
  const src = String(expr ?? ''), peek = () => { while (src[i] === ' ') i++; return src[i] }
  const field = path => path.split('.').reduce((o, k) => o?.[k], row)
  const factor = () => {
    const c = peek()
    if (c === '(') { i++; const v = sum(); peek(); i++; return v }
    if (c === '-') { i++; return -factor() }
    if (c === '[') { const end = src.indexOf(']', i); const k = src.slice(i + 1, end); i = end + 1; return +row?.[k] }
    const word = /^[\p{L}\p{N}_.]+/u.exec(src.slice(i))?.[0] ?? ''
    i += word.length
    return /^[\d.]/.test(word) ? +word : +field(word)
  }
  const product = () => { let v = factor(); for (let c = peek(); c === '*' || c === '/'; c = peek()) { i++; v = c === '*' ? v * factor() : v / factor() } return v }
  const sum = () => { let v = product(); for (let c = peek(); c === '+' || c === '-'; c = peek()) { i++; v = c === '+' ? v + product() : v - product() } return v }
  const own = row?.[src]
  return own !== undefined ? +own : sum()
}

// chartSketch draws a chart into an SVG of the element's size, laid out as the
// renderer lays it out (value scale, labels, legend), from rows of the sample
// data, or from example values when there are none to draw from. number writes
// a value as the report does; other names the slice of what does not fit.
function chartSketch(e, rows, number, other) {
  const sums = new Map()
  for (const row of rows) {
    const label = String(e.label ? (e.label.split('.').reduce((o, k) => o?.[k], row) ?? '') : ''), v = value(e.value, row)
    if (Number.isFinite(v)) sums.set(label, (sums.get(label) ?? 0) + v)
  }
  if (!e.label || !e.value || !sums.size) [['A', 3], ['B', 5], ['C', 2], ['D', 4]].forEach(([l, v]) => sums.set(l, v)) // an example, for the shape
  const w = e.w, h = e.h, color = e.color || PALETTE[0]
  const svg = svgEl('svg', { width: '100%', height: '100%', viewBox: `0 0 ${w} ${h}`, preserveAspectRatio: 'none', style: 'display:block' })
  const text = (x, y, s, anchor = 'middle') => { const t = svgEl('text', { x, y, 'font-size': 7, 'text-anchor': anchor, fill: '#334155' }); t.textContent = s; svg.append(t) }
  const line = (x1, y1, x2, y2, stroke) => svg.append(svgEl('line', { x1, y1, x2, y2, stroke, 'stroke-width': 0.5 }))
  const num = v => number(v, v === Math.trunc(v) ? 0 : 2)
  if (e.chart === 'pie' || e.chart === 'donut') {
    // as printed: no slices of nothing in the legend, and the smallest that do not fit summed into one
    if ([...sums.values()].some(v => v)) for (const [l, v] of sums) if (!v) sums.delete(l)
    const fit = Math.max(Math.floor(h / 11), 1)
    if (sums.size > fit) {
      const keep = new Set([...sums.keys()].sort((a, b) => sums.get(b) - sums.get(a)).slice(0, fit - 1))
      let rest = 0
      for (const [l, v] of sums) if (!keep.has(l)) { rest += v; sums.delete(l) }
      sums.set(other, (sums.get(other) ?? 0) + rest)
    }
    const labels = [...sums.keys()], vals = labels.map(l => sums.get(l))
    const total = vals.reduce((a, v) => a + Math.max(v, 0), 0) || 1, r = Math.min(h, w * 0.55) / 2
    let a = -Math.PI / 2
    labels.forEach((l, i) => {
      const sweep = 2 * Math.PI * Math.max(vals[i], 0) / total, c = PALETTE[i % PALETTE.length]
      const p = (rad, ang) => `${r + rad * Math.cos(ang)} ${r + rad * Math.sin(ang)}`
      const big = sweep > Math.PI ? 1 : 0, inner = e.chart === 'donut' ? r * 0.55 : 0
      const d = sweep >= 2 * Math.PI - 1e-9
        ? `M${p(r, a)}A${r} ${r} 0 1 1 ${p(r, a + Math.PI)}A${r} ${r} 0 1 1 ${p(r, a)}Z`
        : `M${p(inner, a)}L${p(r, a)}A${r} ${r} 0 ${big} 1 ${p(r, a + sweep)}L${p(inner, a + sweep)}` + (inner ? `A${inner} ${inner} 0 ${big} 0 ${p(inner, a)}` : '') + 'Z'
      if (sweep > 0) svg.append(svgEl('path', { d, fill: c }))
      svg.append(svgEl('rect', { x: 2 * r + 8, y: i * 11 + 1, width: 7, height: 7, fill: c }))
      text(2 * r + 18, i * 11 + 7.5, `${l}: ${num(vals[i])}`, 'start')
      a += sweep
    })
    return svg
  }
  const labels = [...sums.keys()], vals = labels.map(l => sums.get(l)), n = labels.length
  // the value scale: round steps from below the lowest value (or zero) to above the highest
  let lo = Math.min(0, ...vals), hi = Math.max(0, ...vals)
  const raw = (hi - lo) / 4, mag = raw > 0 ? 10 ** Math.floor(Math.log10(raw)) : 1
  const step = raw > 0 ? ([1, 2, 5].map(m => m * mag).find(s => s >= raw) ?? 10 * mag) : 1
  lo = Math.floor(lo / step) * step; hi = Math.ceil(hi / step) * step
  if (hi === lo) hi = lo + step
  const ticks = Array.from({ length: Math.round((hi - lo) / step) + 1 }, (_, i) => lo + i * step)
  const wide = s => [...s].length * 3.9
  if (e.chart === 'hbar') { // label, bar, value on each line; the scale along the bottom
    const labelW = Math.min(w * 0.3, 90), plotW = w - labelW - 50, slot = (h - 10) / n, x = v => labelW + plotW * (v - lo) / (hi - lo)
    for (const v of ticks) { line(x(v), 0, x(v), h - 10, '#dddddd'); text(x(v), h - 2.5, num(v)) }
    const every = Math.ceil(9 / slot)
    labels.forEach((l, i) => {
      const v = vals[i], y = slot * i
      svg.append(svgEl('rect', { x: x(Math.min(v, 0)), y: y + slot * 0.15, width: x(Math.max(v, 0)) - x(Math.min(v, 0)), height: slot * 0.7, fill: color }))
      if (i % every === 0) { text(labelW - 4, y + slot / 2 + 2.5, l, 'end'); text(x(Math.max(v, 0)) + 3, y + slot / 2 + 2.5, num(v), 'start') }
    })
    line(x(0), 0, x(0), h - 10, '#888888')
    return svg
  }
  const axisW = Math.max(...ticks.map(v => wide(num(v)) + 4)), plot = h - 20, slot = (w - axisW) / n
  const y = v => 10 + plot * (hi - v) / (hi - lo)
  for (const v of ticks) { line(axisW, y(v), w, y(v), '#dddddd'); text(axisW - 4, y(v) + 2.5, num(v), 'end') }
  const every = Math.ceil((Math.max(...labels.map((l, i) => Math.max(wide(l), wide(num(vals[i]))))) + 2) / slot)
  const pts = []
  labels.forEach((l, i) => {
    const v = vals[i], x = axisW + slot * i
    let at = y(v)
    if (e.chart === 'line') pts.push([x + slot / 2, y(v)])
    else { at = y(Math.max(v, 0)); svg.append(svgEl('rect', { x: x + slot * 0.15, y: at, width: slot * 0.7, height: y(Math.min(v, 0)) - at, fill: color })) }
    if (i % every === 0) { text(x + slot / 2, at - 2.5, num(v)); text(x + slot / 2, h - 2.5, l) }
  })
  if (pts.length) {
    svg.append(svgEl('polyline', { points: pts.map(p => p.join(',')).join(' '), fill: 'none', stroke: color, 'stroke-width': 1.5 }))
    for (const [x, yy] of pts) svg.append(svgEl('rect', { x: x - 2, y: yy - 2, width: 4, height: 4, fill: color }))
  }
  line(axisW, y(0), w, y(0), '#888888')
  return svg
}

// ---- barcodes, encoded the way the renderer encodes them -------------------
// The renderer uses github.com/boombuler/barcode: a QR code at error correction
// level M, in the first of numeric, alphanumeric and byte mode that takes the
// text, or Code 128. These are its steps, so the code on the page is the
// printed one, module for module, and scans.

// QR versions 1 to 40 at level M: error correction codewords per block, then
// the blocks and data codewords per block of its two groups
const QR_M = [[10,1,16,0,0],[16,1,28,0,0],[26,1,44,0,0],[18,2,32,0,0],[24,2,43,0,0],[16,4,27,0,0],[18,4,31,0,0],[22,2,38,2,39],[22,3,36,2,37],[26,4,43,1,44],[30,1,50,4,51],[22,6,36,2,37],[22,8,37,1,38],[24,4,40,5,41],[24,5,41,5,42],[28,7,45,3,46],[28,10,46,1,47],[26,9,43,4,44],[26,3,44,11,45],[26,3,41,13,42],[26,17,42,0,0],[28,17,46,0,0],[28,4,47,14,48],[28,6,45,14,46],[28,8,47,13,48],[28,19,46,4,47],[28,22,45,3,46],[28,3,45,23,46],[28,21,45,7,46],[28,19,47,10,48],[28,2,46,29,47],[28,10,46,23,47],[28,14,46,21,47],[28,14,46,23,47],[28,12,47,26,48],[28,6,47,34,48],[28,29,46,14,47],[28,13,46,32,47],[28,40,47,7,48],[28,18,47,31,48]]
const EXP = [], LOG = []
for (let i = 0, x = 1; i < 255; i++) {
  EXP[i] = x
  LOG[x] = i
  x <<= 1
  if (x & 256) x ^= 285
}
const gmul = (a, b) => a && b ? EXP[(LOG[a] + LOG[b]) % 255] : 0
// ecc is the Reed-Solomon error correction of a block's data codewords
function ecc(data, n) {
  let g = [1]
  for (let i = 0; i < n; i++) {
    const next = new Array(g.length + 1).fill(0)
    g.forEach((c, j) => { next[j] ^= c; next[j + 1] ^= gmul(c, EXP[i]) })
    g = next
  }
  const rest = [...data, ...new Array(n).fill(0)]
  data.forEach((_, i) => { const f = rest[i]; if (f) g.forEach((c, j) => { rest[i + j] ^= gmul(c, f) }) })
  return rest.slice(data.length)
}
// bch adds the check bits of format and version information
function bch(value, poly, bits) {
  const deg = p => 31 - Math.clz32(p)
  let r = value << bits
  while (r && deg(r) >= bits) r ^= poly << (deg(r) - bits)
  return value << bits | r
}
// the eight masks, x the column and y the row
const MASKS = [
  (x, y) => (y + x) % 2 === 0, (x, y) => y % 2 === 0, (x, y) => x % 3 === 0, (x, y) => (y + x) % 3 === 0,
  (x, y) => (Math.floor(y / 2) + Math.floor(x / 3)) % 2 === 0, (x, y) => y * x % 2 + y * x % 3 === 0,
  (x, y) => (y * x % 2 + y * x % 3) % 2 === 0, (x, y) => ((y + x) % 2 + y * x % 3) % 2 === 0,
]
// where the alignment patterns go, worked out as the renderer does
function aligns(v) {
  if (v === 1) return []
  const first = 6, last = 17 + 4 * v - 7, count = Math.ceil((last - first) / 28) + 1
  let step = Math.ceil((last - first) / (count - 1))
  if (count > 2 && step % 2 === 1) {
    const frac = (last - first) / (count - 1), round = frac % 1 >= 0.5 ? Math.ceil(frac) : Math.floor(frac)
    step += round % 2 === 0 ? -1 : 1
  }
  return [first, ...Array.from({ length: count - 2 }, (_, i) => last - step * (count - 2 - i)), last]
}

// qr encodes text as a QR code: its size in modules and which are dark; null when it cannot be encoded
function qr(text) {
  if (!text) return null
  const bytes = new TextEncoder().encode(text), ALNUM = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:'
  const dataBytes = v => { const [, b1, d1, b2, d2] = QR_M[v - 1]; return b1 * d1 + b2 * d2 }
  const countBits = (mode, v) => ({ 1: [10, 12, 14], 2: [9, 11, 13], 4: [8, 16, 16] })[mode][v < 10 ? 0 : v < 27 ? 1 : 2]
  const thirds = text.match(/[\s\S]{1,3}/g), pairs = text.match(/[\s\S]{1,2}/g)
  // the renderer's modes in its order: mode, its character count, whether it takes the text, its bits, how to write them
  const modes = [
    [1, text.length, thirds.every(c => /^[+-]?\d+$/.test(c) && +c >= 0), Math.floor(text.length / 3) * 10 + [0, 4, 7][text.length % 3],
      add => thirds.forEach(c => add(+c, [10, 4, 7][c.length % 3]))],
    [2, text.length, [...text].every(c => ALNUM.includes(c)), Math.floor(text.length / 2) * 11 + text.length % 2 * 6,
      add => pairs.forEach(p => p.length === 2 ? add(ALNUM.indexOf(p[0]) * 45 + ALNUM.indexOf(p[1]), 11) : add(ALNUM.indexOf(p), 6))],
    [4, bytes.length, true, bytes.length * 8, add => bytes.forEach(b => add(b, 8))],
  ]
  for (const [mode, length, takes, count, write] of modes) {
    if (!takes) continue
    let v = 1
    while (v <= 40 && dataBytes(v) * 8 < 4 + count + countBits(mode, v)) v++
    if (v <= 40) return qrMatrix(v, mode, length, write, dataBytes(v))
  }
  return null
}

function qrMatrix(v, mode, length, write, total) {
  // the bits: mode, character count, the text, then the end and padding
  const bits = [], add = (value, n) => { for (let i = n - 1; i >= 0; i--) bits.push(value >> i & 1) }
  add(mode, 4)
  add(length, ({ 1: [10, 12, 14], 2: [9, 11, 13], 4: [8, 16, 16] })[mode][v < 10 ? 0 : v < 27 ? 1 : 2])
  write(add)
  for (let i = 0; i < 4 && bits.length < total * 8; i++) bits.push(0)
  while (bits.length % 8) bits.push(0)
  for (let i = 0; bits.length < total * 8; i++) add(i % 2 ? 17 : 236, 8)
  // the codewords in blocks, each with its error correction, interleaved
  const [ecn, b1, d1, b2, d2] = QR_M[v - 1], words = [], blocks = []
  for (let i = 0; i < bits.length; i += 8) words.push(bits.slice(i, i + 8).reduce((a, b) => a << 1 | b, 0))
  let at = 0
  for (const [n, d] of [[b1, d1], [b2, d2]]) for (let b = 0; b < n; b++) { const data = words.slice(at, at += d); blocks.push([data, ecc(data, ecn)]) }
  const stream = []
  for (let i = 0; i < Math.max(d1, d2); i++) for (const [data] of blocks) if (i < data.length) stream.push(data[i])
  for (let i = 0; i < ecn; i++) for (const [, e] of blocks) stream.push(e[i])

  // the modules, one grid per mask; x * dim + y as in the renderer
  const dim = 17 + 4 * v, occupied = new Uint8Array(dim * dim), grids = MASKS.map(() => new Uint8Array(dim * dim))
  const setAll = (x, y, on) => { occupied[x * dim + y] = 1; for (const g of grids) g[x * dim + y] = on ? 1 : 0 }
  for (const [ox, oy] of [[0, 0], [0, dim - 7], [dim - 7, 0]]) { // finder patterns, with their white border
    for (let x = -1; x < 8; x++) for (let y = -1; y < 8; y++) {
      const on = (x === 0 || x === 6 || y === 0 || y === 6 || (x > 1 && x < 5 && y > 1 && y < 5)) && x <= 6 && y <= 6 && x >= 0 && y >= 0
      if (x + ox >= 0 && x + ox < dim && y + oy >= 0 && y + oy < dim) setAll(x + ox, y + oy, on)
    }
  }
  for (const x of aligns(v)) for (const y of aligns(v)) {
    if (occupied[x * dim + y]) continue
    for (let i = -2; i <= 2; i++) for (let j = -2; j <= 2; j++) setAll(x + i, y + j, i === -2 || i === 2 || j === -2 || j === 2 || (i === 0 && j === 0))
  }
  for (let i = 0; i < dim; i++) { // timing patterns
    if (!occupied[i * dim + 6]) setAll(i, 6, i % 2 === 0)
    if (!occupied[6 * dim + i]) setAll(6, i, i % 2 === 0)
  }
  setAll(8, dim - 8, true) // the dark module
  if (v >= 7) {
    const info = bch(v, 0x1F25, 12)
    for (let i = 0; i < 18; i++) { const x = dim - 11 + i % 3, y = Math.floor(i / 3); setAll(x, y, info >> i & 1); setAll(y, x, info >> i & 1) }
  }
  // format information: level M and the mask, in two copies
  const FORMAT = [[0, 8], [1, 8], [2, 8], [3, 8], [4, 8], [5, 8], [7, 8], [8, 8], [8, 7], [8, 5], [8, 4], [8, 3], [8, 2], [8, 1], [8, 0],
    [8, dim - 1], [8, dim - 2], [8, dim - 3], [8, dim - 4], [8, dim - 5], [8, dim - 6], [8, dim - 7], [dim - 8, 8], [dim - 7, 8], [dim - 6, 8], [dim - 5, 8], [dim - 4, 8], [dim - 3, 8], [dim - 2, 8], [dim - 1, 8]]
  for (const [x, y] of FORMAT) occupied[x * dim + y] = 1
  grids.forEach((g, m) => {
    const info = bch(m, 0x537, 10) ^ 0x5412
    FORMAT.forEach(([x, y], k) => { g[x * dim + y] = info >> (14 - k % 15) & 1 })
  })
  // the data, two columns at a time, up and down from the bottom right, round the timing column
  let bit = 0, x = dim - 1, y = dim - 1, up = true
  const put = (px, py) => {
    if (occupied[px * dim + py]) return
    const on = bit < stream.length * 8 ? stream[bit >> 3] >> (7 - bit % 8) & 1 : 0
    bit++
    grids.forEach((g, m) => { g[px * dim + py] = on ^ (MASKS[m](px, py) ? 1 : 0) })
  }
  for (;;) {
    put(x, y)
    put(x - 1, y)
    y += up ? -1 : 1
    if (y >= 0 && y < dim) continue
    y = up ? 0 : dim - 1
    x -= 2
    if (x === 6) x--
    if (x < 0) break
    up = !up
  }
  // the mask the renderer picks: the lowest penalty, the first of equals
  const penalty = g => {
    const get = (x, y) => g[x * dim + y]
    let p = 0
    for (let x = 0; x < dim; x++) {
      let cx = 0, cy = 0, sx = 0, sy = 0
      for (let y = 0; y < dim; y++) {
        if (get(x, y) === sx) cx++
        else { sx ^= 1; if (cx >= 5) p += cx - 2; cx = 1 }
        if (get(y, x) === sy) cy++
        else { sy ^= 1; if (cy >= 5) p += cy - 2; cy = 1 }
      }
      if (cx >= 5) p += cx - 2
      if (cy >= 5) p += cy - 2
    }
    for (let x = 0; x < dim - 1; x++) for (let y = 0; y < dim - 1; y++) {
      const c = get(x, y)
      if (get(x, y + 1) === c && get(x + 1, y) === c && get(x + 1, y + 1) === c) p += 3
    }
    const p1 = [1, 0, 1, 1, 1, 0, 1, 0, 0, 0, 0], p2 = [0, 0, 0, 0, 1, 0, 1, 1, 1, 0, 1]
    for (let x = 0; x <= dim - 11; x++) for (let y = 0; y < dim; y++) {
      let a = true, b = true, c = true, d = true
      for (let i = 0; i < 11; i++) {
        const across = get(x + i, y), down = get(y, x + i)
        a &&= across === p1[i]; b &&= across === p2[i]; c &&= down === p1[i]; d &&= down === p2[i]
      }
      if (a || b) p += 40
      if (c || d) p += 40
    }
    const dark = g.reduce((s, m) => s + m, 0) * 100 / g.length
    return p + Math.min(Math.abs(Math.floor(dark / 5) - 10), Math.abs(Math.ceil(dark / 5) - 10)) * 10
  }
  const scores = grids.map(penalty), best = grids[scores.indexOf(Math.min(...scores))]
  return { size: dim, dark: (x, y) => best[x * dim + y] === 1 }
}

// Code 128: the bars of each symbol, and the symbols as the renderer picks them (sets A, B and C)
const C128 = "11011001100 11001101100 11001100110 10010011000 10010001100 10001001100 10011001000 10011000100 10001100100 11001001000 11001000100 11000100100 10110011100 10011011100 10011001110 10111001100 10011101100 10011100110 11001110010 11001011100 11001001110 11011100100 11001110100 11101101110 11101001100 11100101100 11100100110 11101100100 11100110100 11100110010 11011011000 11011000110 11000110110 10100011000 10001011000 10001000110 10110001000 10001101000 10001100010 11010001000 11000101000 11000100010 10110111000 10110001110 10001101110 10111011000 10111000110 10001110110 11101110110 11010001110 11000101110 11011101000 11011100010 11011101110 11101011000 11101000110 11100010110 11101101000 11101100010 11100011010 11101111010 11001000010 11110001010 10100110000 10100001100 10010110000 10010000110 10000101100 10000100110 10110010000 10110000100 10011010000 10011000010 10000110100 10000110010 11000010010 11001010000 11110111010 11000010100 10001111010 10100111100 10010111100 10010011110 10111100100 10011110100 10011110010 11110100100 11110010100 11110010010 11011011110 11011110110 11110110110 10101111000 10100011110 10001011110 10111101000 10111100010 11110101000 11110100010 10111011110 10111101110 11101011110 11110101110 11010000100 11010010000 11010011100 1100011101011".split(' ')
function code128(text) {
  const runes = [...text]
  if (!runes.length || runes.length > 80) return null
  const FNC = ['ñ', 'ò', 'ó', 'ô'] // FNC1 to FNC4, as the renderer reads them
  const AB = ' !"#$%&\'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_', B = AB + '`abcdefghijklmnopqrstuvwxyz{|}~\x7f'
  const AONLY = Array.from({ length: 32 }, (_, i) => String.fromCharCode(i)).join(''), A = AB + AONLY
  const has = (table, r) => table.includes(r) || FNC.includes(r)
  const useC = (next, cur) => {
    let need = cur === 105 ? 2 : 4
    if (next.length < need) return false
    for (let i = 0; i < need; i++) {
      if (i % 2 === 0 && next[i] === FNC[0]) { need++; if (next.length < need) return false; continue }
      if (!(next[i] >= '0' && next[i] <= '9')) return false
    }
    return true
  }
  const useA = (next, cur) => {
    const r = next[0]
    if (!has(B, r) || cur === 103) return has(A, r)
    if (cur === 0) for (const x of next) { if (has(AB, x)) continue; if (AONLY.includes(x)) return true; break }
    return false
  }
  const codes = []
  let cur = 0
  for (let i = 0; i < runes.length; i++) {
    const r = runes[i]
    if (useC(runes.slice(i), cur)) {
      if (cur !== 105) { codes.push(cur ? 99 : 105); cur = 105 }
      codes.push(r === FNC[0] ? 102 : (r.charCodeAt(0) - 48) * 10 + runes[++i].charCodeAt(0) - 48)
      continue
    }
    const a = useA(runes.slice(i), cur), set = a ? 103 : 104
    if (cur !== set) { codes.push(cur ? (a ? 101 : 100) : set); cur = set }
    const idx = [102, 97, 96, a ? 101 : 100][FNC.indexOf(r)] ?? (a ? A : B).indexOf(r)
    if (idx < 0) return null
    codes.push(idx)
  }
  const sum = codes.reduce((s, c, i) => s + (i ? i * c : c), 0) % 103
  return [...codes, sum, 106].map(c => C128[c]).join('')
}

// codeSketch draws a barcode in its box as it prints: a QR code in a quiet zone
// two modules wide at the top left, or Code 128 bars over the whole width, and
// the text under it when asked. Text that cannot be encoded draws no code.
const codes = new Map() // text -> its code, so redrawing the page does not encode it again
function codeSketch(e, text) {
  const key = e.format + ':' + text
  if (!codes.has(key)) {
    if (codes.size > 500) codes.clear()
    codes.set(key, e.format === 'code128' ? code128(text) : qr(text))
  }
  const code = codes.get(key), color = e.color || '#000', h = e.showText ? Math.max(e.h - 10, 1) : e.h
  const svg = svgEl('svg', { width: '100%', height: '100%', viewBox: `0 0 ${e.w} ${e.h}`, preserveAspectRatio: 'none', style: 'display:block', 'shape-rendering': 'crispEdges' })
  const run = (x, y, w, hh) => svg.append(svgEl('rect', { x, y, width: w, height: hh, fill: color }))
  let textW = e.w
  if (typeof code === 'string') { // Code 128: runs of dark modules
    const mw = e.w / code.length
    for (let i = 0; i < code.length; i++) {
      if (code[i] !== '1') continue
      const start = i
      while (code[i + 1] === '1') i++
      run(start * mw, 0, (i - start + 1) * mw, h)
    }
  } else if (code) {
    const mw = Math.min(e.w, h) / (code.size + 4)
    textW = mw * (code.size + 4)
    for (let y = 0; y < code.size; y++) for (let x = 0; x < code.size; x++) {
      if (!code.dark(x, y)) continue
      const start = x
      while (x + 1 < code.size && code.dark(x + 1, y)) x++
      run(2 * mw + start * mw, 2 * mw + y * mw, (x - start + 1) * mw, mw)
    }
  }
  if (e.showText) {
    const t = svgEl('text', { x: textW / 2, y: h + 8, 'font-size': 7, 'text-anchor': 'middle', fill: color })
    t.textContent = text
    svg.append(t)
  }
  return svg
}

export function createDesigner(root, { template, onChange, preview, lang = 'en', theme = 'light', fonts = [], fontBase, fields = [], data, simple = false, hide = [], messages = {} } = {}) {
  let tpl, sel = [], editing = null, cancelled = false, clip = [], zoom = 1, snap = true, activeCol = 0, activeRow = 0
  let history = [], at = -1
  const images = [] // data URIs, kept out of the undo snapshots
  const nodes = new Map() // element -> its DOM node
  // messages: the host's own words for any text, keyed by its English; then Indonesian with lang 'id'
  const L = s => messages[s] ?? ((lang === 'id' && ID[s]) || s)
  // what is left out of the interface: tabs, groups (or Tab.Group), commands, fields and properties, by name
  const hidden = new Set([...(simple ? SIMPLE : []), ...hide])
  const off = [] // undoes what reaches outside the designer, for destroy()
  const listen = (target, type, fn) => { target.addEventListener(type, fn); off.push(() => target.removeEventListener(type, fn)) }
  const fontList = { ...FONTS, ...Object.fromEntries(fonts.map(f => [f, `'${f}',sans-serif`])) }
  // Great Vibes comes with a regular face only
  const faces = fontBase ? BUNDLED.flatMap(f => FACES.slice(0, f === 'Great Vibes' ? 1 : 4).map(([face, weight, style]) =>
    `@font-face{font-family:'${f}';font-weight:${weight};font-style:${style};src:url("${fontBase}${f.replaceAll(' ', '')}-${face}.ttf")}`)).join('') : ''

  const icon = name => `<svg class="ddr-i" viewBox="0 0 24 24" aria-hidden="true"><path d="${ICONS[name]}"/></svg>`
  // three kinds of ribbon button: a picture, a letter (B, I, U), and a tile with a picture above its name
  // data-label is the name without its shortcut, shown under the picture in simple mode
  const named = title => `title="${L(title)}" aria-label="${L(title)}" data-label="${L(title).replace(/ \(.*\)$/, '')}"`
  const ib = (cmd, pic, title) => `<button type="button" class="ddr-b" data-cmd="${cmd}" ${named(title)}>${icon(pic)}</button>`
  const lb = (cmd, letter, title) => `<button type="button" class="ddr-b ddr-letter" data-cmd="${cmd}" ${named(title)}>${letter}</button>`
  const tile = (cmd, pic, label, title = label, more = '') => `<button type="button" class="ddr-b ddr-tile" data-cmd="${cmd}" title="${L(title)}" aria-label="${L(label)}" ${more}>${icon(pic)}<span>${L(label)}</span></button>`
  const insert = (type, pic, label) => tile('add:' + type, pic, label, `${L(label)}: ${L('click to insert, or drag onto the page')}`, `draggable="true" data-type="${type}"`)
  const swatch = (prop, letter, title) => `<label class="ddr-swatch" title="${L(title)}" data-label="${L(title).split(" / ")[0]}"><span>${letter}</span><input data-prop="${prop}" type="color" aria-label="${L(title)}"></label>`
  const group = (name, html) => `<div class="ddr-group" data-group="${name}"><div>${html}</div><span>${L(name)}</span></div>`
  const options = (list, translate) => list.map(v => `<option value="${v}">${translate ? L(v) : v}</option>`).join('')
  const sideLetters = lang === 'id' ? ['A', 'Ka', 'B', 'Ki'] : ['T', 'R', 'B', 'L']
  const RIBBON = {
    Home:
      group('Undo', ib('undo', 'undo', 'Undo (Ctrl+Z)') + ib('redo', 'redo', 'Redo (Ctrl+Y)')) +
      group('Clipboard', ib('cut', 'cut', 'Cut (Ctrl+X)') + ib('copy', 'copy', 'Copy (Ctrl+C)') + ib('paste', 'paste', 'Paste (Ctrl+V)') + ib('duplicate', 'duplicate', 'Duplicate (Ctrl+D)') + ib('delete', 'delete', 'Delete (Del)')) +
      group('Font',
        `<select data-prop="font" title="${L('Font')}" aria-label="${L('Font')}">${Object.entries(fontList).map(([f, css]) => `<option value="${f}" style="font-family:${css.replaceAll('"', '&quot;')}">${f}</option>`).join('')}</select>` +
        `<input data-prop="fontSize" type="number" min="4" max="200" title="${L('Font size')}" aria-label="${L('Font size')}">` +
        lb('bold', '<b>B</b>', 'Bold (Ctrl+B)') + lb('italic', '<i>I</i>', 'Italic (Ctrl+I)') + lb('underline', '<u>U</u>', 'Underline (Ctrl+U)') +
        swatch('color', 'A', 'Text colour') + swatch('fill', '▰', 'Highlight / fill colour') + ib('noFill', 'noFill', 'No fill')) +
      group('Paragraph',
        ib('align:left', 'alignLeft', 'Align text left') + ib('align:center', 'alignCenter', 'Centre text') + ib('align:right', 'alignRight', 'Align text right') + ib('align:justify', 'alignJustify', 'Justify') +
        `<select data-prop="valign" title="${L('Vertical alignment')}" aria-label="${L('Vertical alignment')}">${options(['top', 'middle', 'bottom'], true)}</select>` +
        `<select data-prop="lineHeight" title="${L('Line spacing')}" aria-label="${L('Line spacing')}">${options([1, 1.2, 1.5, 2])}</select>` +
        `<label title="${L('Wrap long text and let the band grow with it')}"><input data-prop="grow" type="checkbox">${L('Wrap')}</label>`) +
      group('Arrange',
        ib('arrange:x:0', 'edgeLeft', 'Align left edges') + ib('arrange:x:0.5', 'edgeCenter', 'Align centres') + ib('arrange:x:1', 'edgeRight', 'Align right edges') +
        ib('arrange:y:0', 'edgeTop', 'Align top edges') + ib('arrange:y:0.5', 'edgeMiddle', 'Align middles') + ib('arrange:y:1', 'edgeBottom', 'Align bottom edges') +
        ib('distribute', 'distribute', 'Space selected elements evenly') + ib('front', 'front', 'Bring to front') + ib('back', 'back', 'Send to back')),
    Insert:
      group('Text', insert('text', 'text', 'Text box')) +
      group('Table', `<input data-id="rows" type="number" min="1" max="50" value="2" title="${L('Rows')}" aria-label="${L('Table rows')}">×<input data-id="cols" type="number" min="1" max="20" value="3" title="${L('Columns')}" aria-label="${L('Table columns')}">` + insert('table', 'table', 'Table')) +
      group('Illustrations', tile('picture', 'image', 'Picture', 'Insert a picture from this device') + insert('rect', 'rect', 'Rectangle') + insert('line', 'line', 'Line') + insert('chart', 'chart', 'Chart') + insert('barcode', 'barcode', 'Barcode')) +
      group('Report', insert('subreport', 'subreport', 'Sub-report')) +
      group('Data', `<select data-id="field" title="${L('Insert a field into the selected text')}" aria-label="${L('Insert field')}"></select>`),
    Layout:
      group('Page',
        `<select data-id="paper" title="${L('Paper size')}" aria-label="${L('Paper size')}">${options([...Object.keys(PAPER), 'Custom'], true)}</select>` +
        `<select data-id="orientation" title="${L('Orientation')}" aria-label="${L('Orientation')}">${options(['Portrait', 'Landscape'], true)}</select>` +
        `<label>W<input data-page="width" type="number" min="50"></label><label>H<input data-page="height" type="number" min="50"></label>`) +
      group('Margins', ['Top', 'Right', 'Bottom', 'Left'].map((side, i) => `<label>${sideLetters[i]}<input data-page="${SIDES[i]}" type="number" min="0" title="${L(side + ' margin')}" aria-label="${L(side + ' margin')}"></label>`).join('')) +
      group('Background',
        swatch('', '▰', 'Page colour').replace('data-prop=""', 'data-page="background"') + ib('noBackground', 'noFill', 'No page colour') +
        tile('backgroundImage', 'image', 'Picture', 'Use a picture as the page background') + ib('noBackgroundImage', 'delete', 'Remove the background picture')) +
      group('Grouping', `<label>${L('Sort by')}<input data-tpl="sortBy" type="text" placeholder="${L('row field, -field for descending')}" title="${L('row field, -field for descending')}"></label>` +
        `<label>${L('Group by')}<input data-tpl="groupBy" type="text" placeholder="${L('row field, or up to three: region, city')}" title="${L('row field, or up to three: region, city')}"></label><label><input data-tpl="groupPageBreak" type="checkbox">${L('New page per group')}</label>` +
        `<label title="${L('Excel (xlsx, xls): a worksheet for each group of the first Group by field')}"><input data-tpl="sheetPerGroup" type="checkbox">${L('A sheet per group')}</label>`) +
      group('Data', `<label>${L('Rows key')}<input data-tpl="dataKey" type="text"></label><label title="${L('Thousands separator')}">1<input data-tpl="thousands" type="text" style="width:34px" placeholder="," aria-label="${L('Thousands separator')}">000</label>` +
        `<label title="${L('No thousands separator')}"><input data-id="nothousands" type="checkbox">${L('No thousands separator')}</label>` +
        `<label title="${L('Decimal sign')}">0<input data-tpl="decimal" type="text" style="width:34px" placeholder="." aria-label="${L('Decimal sign')}">00</label>` +
        `<label>${L('Dates in')}<select data-tpl="locale"><option value="">${L('English')}</option><option value="id">${L('Indonesian')}</option></select></label>` +
        `<label>${L('CSV separator')}<select data-tpl="csvSeparator"><option value="">,</option><option value=";">;</option><option value="&#9;">${L('Tab')}</option></select></label>`),
    View:
      group('Show', `<label><input data-id="ruler" type="checkbox" checked>${L('Ruler')}</label><label title="${L('Snap to the edges of other elements while dragging (hold Alt to turn off)')}"><input data-id="snap" type="checkbox" checked>${L('Snap')}</label>`),
  }

  root.classList.add('ddr')
  root.dataset.theme = theme
  root.toggleAttribute('data-simple', simple)
  const tabs = Object.keys(RIBBON)
  root.innerHTML = `<style>${CSS}${faces}</style>
    <div class="ddr-tabs" role="tablist">${tabs.map((t, i) => `<button type="button" role="tab" aria-selected="${i === 0}" tabindex="${i ? -1 : 0}" data-tab="${t}">${L(t)}</button>`).join('')}</div>
    ${tabs.map((t, i) => `<div class="ddr-panel" role="tabpanel" data-panel="${t}" ${i ? 'hidden' : ''}>${RIBBON[t]}</div>`).join('')}
    <div class="ddr-work">
      <div class="ddr-trash" hidden>${icon('delete')}<span>${L('Drop here to delete')}</span></div>
      <div class="ddr-stage"><div class="ddr-sizer"><div class="ddr-zoom"><div class="ddr-ruler"></div><div class="ddr-page" tabindex="-1"></div></div></div></div>
      <details class="ddr-side" open><summary data-id="side"></summary><div class="ddr-props"></div></details>
    </div>
    <div class="ddr-status"><span data-id="status" role="status"></span>
      ${ib('zoom:-1', 'zoomOut', 'Zoom out')}<input data-id="zoomrange" type="range" min="25" max="300" step="5" value="100" aria-label="${L('Zoom')}"><button type="button" class="ddr-b" data-cmd="zoom:0" title="${L('Reset zoom')}" aria-label="${L('Reset zoom')}"><output data-id="zoom">100%</output></button>${ib('zoom:1', 'zoomIn', 'Zoom in')}${ib('zoom:fit', 'fit', 'Fit the page to the width')}
    </div>`
  const $ = s => root.querySelector(s), $$ = s => [...root.querySelectorAll(s)]
  const page = $('.ddr-page'), props = $('.ddr-props'), ruler = $('.ddr-ruler'), zoomer = $('.ddr-zoom'), sizer = $('.ddr-sizer'), stage = $('.ddr-stage')

  // the rows charts are drawn from: those of the sample data, under the rows key or as the list itself
  const sampleRows = () => Array.isArray(data) ? data : Array.isArray(data?.[tpl.dataKey || 'rows']) ? data[tpl.dataKey || 'rows'] : []
  // a number as the report writes it, with the template's separators
  const number = (v, dec) => {
    const sep = tpl.thousands === 'none' ? '' : tpl.thousands || (tpl.decimal ? '' : ','), [int, frac] = Math.abs(v).toFixed(dec).split('.')
    return (v < 0 ? '-' : '') + int.replace(/\B(?=(\d{3})+(?!\d))/g, sep) + (frac ? (tpl.decimal || '.') + frac : '')
  }
  // a text with its plain {{fields}} filled in from the sample data, as it will print; the rest stays as written
  const filled = text => String(text ?? '').replace(/\{\{\s*([\p{L}\p{N}_.]+)\s*\}\}/gu, (all, key) => {
    const v = [sampleRows()[0], Array.isArray(data) ? null : data].map(o => key.split('.').reduce((x, k) => x?.[k], o)).find(x => x != null)
    return v == null || typeof v === 'object' ? all : String(v)
  })

  // editSubreport opens a designer of its own for the bands of a sub-report, as
  // wide as its box. Done keeps the changes; Close or Escape leaves them.
  function editSubreport(e) {
    const dialog = root.appendChild(document.createElement('dialog')), host = document.createElement('div')
    dialog.className = 'ddr-subdialog'
    // what happens in the designer inside stays there: no click, key or change reaches this one
    for (const type of ['click', 'mousedown', 'pointerdown', 'keydown', 'change', 'input', 'dragstart', 'dragover', 'drop', 'wheel']) dialog.addEventListener(type, ev => ev.stopPropagation())
    dialog.innerHTML = `<form method="dialog"><b></b><span></span><button value="cancel">${L('Close')}</button><button value="done" class="ddr-primary">${L('Done')}</button></form>`
    dialog.querySelector('b').textContent = `${L('Sub-report')}: ${e.data || '?'}`
    dialog.append(host)
    // its sample rows: the list in the sample data, in the report's data or in its first row
    const list = [Array.isArray(data) ? data[0] : data, sampleRows()[0]].map(o => o?.[e.data]).find(Array.isArray) ?? []
    const page = { width: e.w, height: 1000, margin: 0, marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0 }
    const inner = createDesigner(host, { template: { ...(e.report ?? {}), page }, lang, theme, fonts, fontBase, fields, data: list, simple, hide, messages })
    dialog.onclose = () => {
      if (dialog.returnValue === 'done') {
        const t = inner.getTemplate()
        delete t.page // the box of the element is its page
        e.report = t
        redraw()
      }
      inner.destroy()
      dialog.remove()
      keepFocus()
    }
    dialog.showModal()
    return inner
  }

  // the Field menu: the data's own fields first, so a name is picked rather than typed (and mistyped)
  function fillFields() {
    const opt = v => `<option value="${v.replaceAll('&', '&amp;').replaceAll('"', '&quot;')}">${v.replaceAll('&', '&amp;').replaceAll('<', '&lt;')}</option>`
    $('[data-id=field]').innerHTML = `<option value="">${L('Field…')}</option>` +
      (fields.length ? `<optgroup label="${L('Data')}">${fields.map(f => opt(`{{${f}}}`)).join('')}</optgroup>` : '') +
      `<optgroup label="${L('Expressions')}">${FIELD_TOKENS.map(opt).join('')}</optgroup>`
  }
  fillFields()

  const px = v => v + 'px'
  const bandOf = e => BANDS.find(b => tpl.bands[b].elements.includes(e))
  const levels = () => String(tpl.groupBy ?? '').split(',').filter(f => f.trim()).length
  // group bands show once Group by names their level; in simple mode the first level too
  const shown = () => BANDS.filter(b => tpl.bands[b].elements.length || levels() >= (LEVEL[b] ?? (simple && /^group/.test(b) ? 1 : 0)))
  const contentW = () => tpl.page.width - tpl.page.marginLeft - tpl.page.marginRight
  const target = () => (sel[0] && bandOf(sel[0])) || 'header'
  const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, Math.round(+v) || lo))
  // the keyboard only reaches the designer while something inside it has the focus
  const keepFocus = () => (nodes.get(sel[0]) ?? page).focus({ preventScroll: true })

  // ---- history -----------------------------------------------------------
  const pack = () => JSON.stringify(tpl, (k, v) => typeof v === 'string' && v.startsWith('data:') ? '\u0000' + (images.includes(v) ? images.indexOf(v) : images.push(v) - 1) : v)
  const unpack = s => JSON.parse(s, (k, v) => typeof v === 'string' && v[0] === '\u0000' ? images[+v.slice(1)] : v)
  // edit names a run of typing in one field: its keystrokes are one undo step, not one each
  let lastEdit = null
  function remember(edit) {
    if (edit && edit === lastEdit && at === history.length - 1) history[at] = pack()
    else {
      history.splice(++at, Infinity, pack())
      if (history.length > 100) { history.shift(); at-- }
    }
    lastEdit = edit ?? null
    sync()
  }
  function changed(edit) {
    remember(edit)
    onChange?.(tpl)
  }
  function travel(step) {
    lastEdit = null
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
    grab()
    sync()
  }

  // one handle on the bottom-right corner of the selected element resizes it;
  // unlike the browser's own resize grip it also works by touch and at any zoom
  const handle = Object.assign(document.createElement('div'), { className: 'ddr-handle', title: L('Drag to resize') })
  // ... and a small red button on its top-right corner deletes it
  const cross = Object.assign(document.createElement('button'), { type: 'button', className: 'ddr-x', title: L('Delete this element'), innerHTML: icon('close') })
  cross.setAttribute('aria-label', L('Delete this element'))
  cross.onpointerdown = ev => ev.stopPropagation()
  cross.onclick = () => { moveTo(sel); keepFocus() } // Safari does not focus a clicked button, so say where the keyboard goes
  function grab() {
    const e = sel.length === 1 && !editing ? sel[0] : null, band = e && page.querySelector(`[data-name="${bandOf(e)}"]`)
    if (!band) return void (handle.remove(), cross.remove())
    handle.style.left = px(e.x + e.w); handle.style.top = px(e.y + e.h)
    cross.style.left = px(e.x + e.w); cross.style.top = px(e.y)
    if (handle.parentNode !== band) band.append(handle, cross)
  }

  // while an element is being dragged a bin shows at the top; dropping on it deletes the selection
  const trash = $('.ddr-trash')
  const overTrash = ev => {
    const r = trash.getBoundingClientRect()
    return !trash.hidden && ev.clientX >= r.left && ev.clientX <= r.right && ev.clientY >= r.top && ev.clientY <= r.bottom
  }
  handle.onpointerdown = ev => {
    const e = sel[0], w0 = e.w, h0 = e.h, n = nodes.get(e)
    const straight = e.type === 'line' && e.slope !== 'down' && e.slope !== 'up' // a straight line only changes its length
    const flat = straight && e.w >= e.h, tall = straight && !flat
    ev.stopPropagation()
    handle.onpointermove = mv => {
      if (!tall) e.w = Math.max(4, Math.round(w0 + (mv.clientX - ev.clientX) / zoom))
      if (!flat) e.h = Math.max(4, Math.round(h0 + (mv.clientY - ev.clientY) / zoom))
      n.style.width = px(e.w); n.style.height = px(e.h)
      grab()
    }
    handle.onpointerup = () => { handle.onpointermove = handle.onpointerup = null; redraw() }
    capture(handle, ev)
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
    if (type === 'subreport' && !e.report) e.report = { bands: { ...emptyTemplate().bands, header: { height: 0, elements: [] }, footer: { height: 0, elements: [] } } }
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
    // what comes from the clipboard may be anything: only elements that can be drawn get in
    sel = list.flatMap(([band, json]) => {
      let e
      try { e = JSON.parse(json) } catch {}
      if (!isObject(e)) return []
      e = cleanElement(e)
      e.x += 10; e.y += 10
      tpl.bands[band].elements.push(e)
      return [e]
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
      else e[propOf(e, key)] = value
    }
    redraw()
  }

  // tableEdit inserts or deletes a row or a column next to the cell last clicked;
  // rows keep their height, so the table grows or shrinks with them
  function tableEdit(e, op) {
    const r = Math.min(activeRow, e.cells.length - 1), c = Math.min(activeCol, e.cells[0].length - 1)
    const heights = rowWeights(e), unit = e.h / heights.reduce((a, b) => a + b, 0) // points per height unit
    e.aligns = e.cells[0].map((_, i) => e.aligns?.[i] || 'left')
    e.heights = heights
    if (op === 'rowAbove' || op === 'rowBelow') {
      const at = r + (op === 'rowBelow')
      e.cells.splice(at, 0, e.cells[0].map(() => ''))
      e.heights.splice(at, 0, heights[r])
      shiftStyles(e, 0, at, 1)
    } else if (op === 'rowDelete' && e.cells.length > 1) {
      e.cells.splice(r, 1)
      e.heights.splice(r, 1)
      shiftStyles(e, 0, r, -1)
    } else if (op === 'colLeft' || op === 'colRight') {
      const at = c + (op === 'colRight')
      for (const row of e.cells) row.splice(at, 0, '')
      e.cols.splice(at, 0, 1)
      e.aligns.splice(at, 0, 'left')
      shiftStyles(e, 1, at, 1)
    } else if (op === 'colDelete' && e.cells[0].length > 1) {
      for (const row of e.cells) row.splice(c, 1)
      e.cols.splice(c, 1)
      e.aligns.splice(c, 1)
      shiftStyles(e, 1, c, -1)
    } else if (op === 'mergeRight' || op === 'mergeDown') {
      // the cell takes in its neighbour, and the neighbour's words, so nothing goes missing
      const st = (e.cellStyle ??= {})[`${r},${c}`] ??= {}, cs = st.colSpan || 1, rs = st.rowSpan || 1
      const taken = op === 'mergeRight'
        ? c + cs < e.cells[0].length && Array.from({ length: rs }, (_, i) => [r + i, c + cs])
        : r + rs < e.cells.length && Array.from({ length: cs }, (_, j) => [r + rs, c + j])
      if (!taken) return
      for (const [tr, tc] of taken) {
        if (e.cells[tr]?.[tc]) e.cells[r][c] = [e.cells[r][c], e.cells[tr][tc]].filter(Boolean).join(' ')
        if (e.cells[tr]) e.cells[tr][tc] = ''
        delete e.cellStyle[`${tr},${tc}`]
      }
      if (op === 'mergeRight') st.colSpan = cs + 1
      else st.rowSpan = rs + 1
    } else if (op === 'unmerge') {
      const st = e.cellStyle?.[`${r},${c}`]
      if (st) { delete st.colSpan; delete st.rowSpan }
    }
    e.h = Math.round(unit * e.heights.reduce((a, b) => a + b, 0)) // rows keep their height
    redraw()
  }

  // pasteCells fills a table with cells copied from a spreadsheet (tab-separated
  // lines), from the cell last clicked, adding rows and columns as needed
  function pasteCells(e, text) {
    const grid = text.replace(/\r/g, '').replace(/\n$/, '').split('\n').map(line => line.split('\t'))
    const r0 = Math.min(activeRow, e.cells.length - 1), c0 = Math.min(activeCol, e.cells[0].length - 1)
    const unit = e.h / rowWeights(e).reduce((a, b) => a + b, 0)
    tableSize(e, Math.max(e.cells.length, r0 + grid.length), Math.max(e.cells[0].length, c0 + Math.max(...grid.map(l => l.length))))
    grid.forEach((line, i) => line.forEach((v, j) => {
      if (e.cells[r0 + i]?.[c0 + j] != null) e.cells[r0 + i][c0 + j] = v.replace(/<(\/?[biu]>)/g, '&lt;$1')
    }))
    e.h = Math.round(unit * rowWeights(e).reduce((a, b) => a + b, 0)) // new rows as tall as the others
    redraw()
  }

  // rowWeights are a table's relative row heights, one per row
  const rowWeights = e => e.cells.map((_, i) => e.heights?.[i] > 0 ? e.heights[i] : 1)

  // shiftStyles keeps the per-cell styles with their cells when a row (axis 0)
  // or a column (axis 1) is inserted at `at` (by 1) or deleted there (by -1)
  function shiftStyles(e, axis, at, by) {
    const moved = {}
    for (const [k, v] of Object.entries(e.cellStyle ?? {})) {
      const rc = k.split(',').map(Number), i = rc[axis]
      if (by < 0 && i === at) continue // the row or column deleted
      if (by > 0 ? i >= at : i > at) rc[axis] += by
      moved[rc.join(',')] = v
    }
    e.cellStyle = moved
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
    handle.remove()
    cross.remove()
    if (rich) node.dataset.rich = 1
    try { node.contentEditable = rich ? 'true' : 'plaintext-only' } catch { node.contentEditable = 'true' } // older Firefox has no plaintext-only
    node.focus()
    getSelection().selectAllChildren(node)
    node.onpaste = ev => { ev.preventDefault(); document.execCommand('insertText', false, ev.clipboardData.getData('text/plain')) }
    // finishing is the same whether the typing ends by a click elsewhere (blur) or by a key
    finish = ev => {
      node.onblur = finish = null
      node.contentEditable = 'false'
      editing = null
      if (cancelled) draw()
      else save()
      if (!ev?.relatedTarget) keepFocus() // unless the user went on to another control
    }
    node.onblur = finish
  }
  let finish = null // ends the typing under way

  function tableSize(e, rows, cols) {
    rows = clamp(rows, 1, 50); cols = clamp(cols, 1, 20)
    e.cells = Array.from({ length: rows }, (_, r) => Array.from({ length: cols }, (_, c) => e.cells[r]?.[c] ?? ''))
    e.cols = Array.from({ length: cols }, (_, c) => e.cols?.[c] ?? 1)
    e.heights = Array.from({ length: rows }, (_, r) => e.heights?.[r] ?? 1)
    // styles of cells that are gone go with them
    e.cellStyle = Object.fromEntries(Object.entries(e.cellStyle ?? {}).filter(([k]) => { const [r, c] = k.split(',').map(Number); return r < rows && c < cols }))
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
    $('[data-id=zoomrange]').value = Math.round(zoom * 100)
    center()
  }
  // center puts the page in the middle of the stage on a whole pixel: lines stay
  // sharp, and a pointer lands on the same point of the page in every browser
  function center() {
    const pad = parseFloat(getComputedStyle(stage).paddingLeft) || 0
    sizer.style.marginLeft = px(Math.max(0, Math.floor((stage.clientWidth - 2 * pad - sizer.offsetWidth) / 2)))
  }
  const resizes = new ResizeObserver(center)
  resizes.observe(stage)
  off.push(() => resizes.disconnect())
  const fitZoom = () => (stage.clientWidth - 34) / tpl.page.width || 1
  // zoomAt keeps the point of the page under (cx, cy) where it is
  function zoomAt(z, cx, cy) {
    const before = zoomer.getBoundingClientRect(), x = (cx - before.left) / zoom, y = (cy - before.top) / zoom
    setZoom(z)
    const after = zoomer.getBoundingClientRect()
    stage.scrollLeft += after.left + x * zoom - cx
    stage.scrollTop += after.top + y * zoom - cy
  }
  // Ctrl + wheel zooms; so does a trackpad pinch, which browsers send as one
  stage.addEventListener('wheel', ev => {
    if (!ev.ctrlKey && !ev.metaKey) return
    ev.preventDefault()
    zoomAt(zoom * Math.exp(-ev.deltaY * (ev.deltaMode === 1 ? 16 : 1) * 0.002), ev.clientX, ev.clientY)
  }, { passive: false })
  // two fingers on a touch screen pinch-zoom; they neither drag an element nor draw a selection box
  const touches = new Map()
  let pinch = null, pinched = false // pinched: the touch under way became a pinch
  const spread = () => { const [a, b] = touches.values(); return [Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY), (a.clientX + b.clientX) / 2, (a.clientY + b.clientY) / 2] }
  stage.addEventListener('pointerdown', ev => {
    if (ev.pointerType !== 'touch') return
    touches.set(ev.pointerId, ev)
    if (touches.size === 1) pinched = false
    if (touches.size === 2) { pinch = { d: spread()[0], z: zoom }; pinched = true; ev.stopPropagation() }
  }, true)
  stage.addEventListener('pointermove', ev => {
    if (!touches.has(ev.pointerId)) return
    touches.set(ev.pointerId, ev)
    if (!pinch || touches.size < 2) return
    ev.stopPropagation()
    const [d, cx, cy] = spread()
    if (pinch.d) zoomAt(pinch.z * d / pinch.d, cx, cy)
  }, true)
  const lift = ev => { touches.delete(ev.pointerId); if (touches.size < 2) pinch = null }
  stage.addEventListener('pointerup', lift, true)
  stage.addEventListener('pointercancel', lift, true)

  // ---- commands (ribbon buttons and shortcuts) ------------------------------
  const commands = {
    undo: () => travel(-1),
    redo: () => travel(1),
    copy: () => { clip = copied(); sync(); toSystem('copy') },
    cut: () => { if (sel.length) { clip = copied(); toSystem('copy'); moveTo(sel) } },
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
    zoom: step => setZoom(step === 'fit' ? fitZoom() : +step ? zoom + step * 0.1 : 1),
  }
  // a command the host hid does not run from a shortcut either
  const run = cmd => { const [name, ...args] = cmd.split(':'); if (!hidden.has(name) && !(name === 'add' && hidden.has(args[0]))) commands[name](...args) }

  // The system clipboard: Ctrl+C / Ctrl+X put the selected elements on it as
  // JSON, so they can be pasted into another designer (another tab, another
  // report); Ctrl+V takes them back, or turns plain text into a text box. The
  // ribbon's Copy and Cut fill it too; its Paste uses what was copied here.
  let copying = false
  // the focus is in this designer, and not in one opened inside it (a sub-report's)
  const ours = ev => copying || !editing && !ev.target.closest?.('input,select,textarea') && document.activeElement?.closest('.ddr') === root
  function toSystem(how) {
    copying = true
    try { document.execCommand(how) } catch {} finally { copying = false }
  }
  for (const how of ['copy', 'cut']) {
    listen(document, how, ev => {
      if (!ours(ev) || !sel.length) return
      ev.preventDefault()
      clip = copied()
      ev.clipboardData.setData('text/plain', JSON.stringify({ ddreport: clip }))
      if (how === 'cut' && !copying) moveTo(sel)
      else sync()
    })
  }
  listen(document, 'paste', ev => {
    if (!ours(ev)) return
    ev.preventDefault()
    const text = ev.clipboardData.getData('text/plain')
    let list
    try { list = JSON.parse(text).ddreport } catch {}
    if (Array.isArray(list) && list.length && list.every(i => BANDS.includes(i?.[0]) && typeof i[1] === 'string')) clip = paste(list)
    else if (sel.length === 1 && sel[0].type === 'table' && /[\t\n]/.test(text.trim())) pasteCells(sel[0], text)
    else if (text.trim()) add('text', target(), 0, 0, { text: text.replace(/<(\/?[biu]>)/g, '&lt;$1') })
    else if (clip.length) clip = paste(clip)
  })

  // keep the text selection and keyboard focus where they are when a ribbon button is pressed
  root.addEventListener('mousedown', ev => { if (ev.target.closest('.ddr-panel button:not([draggable])')) ev.preventDefault() })
  root.addEventListener('click', ev => {
    const tab = ev.target.closest('[data-tab]'), button = ev.target.closest('[data-cmd]')
    if (tab) {
      for (const t of $$('[data-tab]')) { t.setAttribute('aria-selected', t === tab); t.tabIndex = t === tab ? 0 : -1 }
      for (const p of $$('[data-panel]')) p.hidden = p.dataset.panel !== tab.dataset.tab
    }
    if (button) run(button.dataset.cmd)
  })
  $('[data-id=zoomrange]').oninput = ev => setZoom(ev.target.value / 100)
  // arrow keys walk the tabs
  $('.ddr-tabs').onkeydown = ev => {
    const all = $$('[data-tab]'), step = { ArrowRight: 1, ArrowLeft: -1 }[ev.key]
    if (!step) return
    const next = all[(all.indexOf(ev.target) + step + all.length) % all.length]
    next.focus()
    next.click()
  }
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
    else if (d.id === 'nothousands') tpl.thousands = value ? 'none' : ''
    else return
    redraw()
  })

  root.addEventListener('keydown', ev => {
    if (editing) {
      const inCell = editing.closest('.ddr-cell')
      if (ev.key === 'Escape') { cancelled = true; finish() }
      // in a table cell Enter keeps the text and Shift+Enter starts a new line; in a text box Enter is a new line
      if (ev.key === 'Enter') { ev.preventDefault(); inCell && !ev.shiftKey ? finish() : document.execCommand('insertLineBreak') }
      if (ev.key === 'Tab' && inCell) { // on to the next cell, or back with Shift
        ev.preventDefault()
        const all = [...inCell.parentNode.querySelectorAll('.ddr-cell')], next = all[all.indexOf(inCell) + (ev.shiftKey ? -1 : 1)]
        finish()
        if (next) {
          activeCol = +next.dataset.col; activeRow = +next.dataset.row
          next.dispatchEvent(new MouseEvent('dblclick', { bubbles: true }))
        }
      }
      return
    }
    if (ev.target.closest('input,select,textarea,button')) return
    const key = ev.key.toLowerCase(), big = ev.shiftKey ? 10 : 1
    const cmd = ev.ctrlKey || ev.metaKey
      // Ctrl+C, X and V are left to the browser, which fires the clipboard events above
      ? { z: ev.shiftKey ? 'redo' : 'undo', y: 'redo', d: 'duplicate', a: 'all', b: 'bold', i: 'italic', u: 'underline', '=': 'zoom:1', '+': 'zoom:1', '-': 'zoom:-1', 0: 'zoom:0' }[key]
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
    document.body.append(box) // not inside the designer: its container query would make it the box's frame in Firefox and Safari
    page.onpointermove = mv => {
      s.left = px(Math.min(ev.clientX, mv.clientX)); s.top = px(Math.min(ev.clientY, mv.clientY))
      s.width = px(Math.abs(mv.clientX - ev.clientX)); s.height = px(Math.abs(mv.clientY - ev.clientY))
    }
    page.onpointerup = () => {
      page.onpointermove = page.onpointerup = null
      const b = box.getBoundingClientRect()
      box.remove()
      if (pinched) return // a second finger made it a pinch: the zoom changed, the selection did not
      sel = [...nodes].filter(([, n]) => {
        const r = n.getBoundingClientRect()
        return r.left < b.right && r.right > b.left && r.top < b.bottom && r.bottom > b.top
      }).map(([e]) => e)
      mark()
      renderProps()
      keepFocus() // so Delete, Ctrl+C and the arrow keys act on what was just selected
    }
    capture(page, ev)
  }

  if (preview) {
    const dialog = document.createElement('dialog'), box = document.createElement('div')
    dialog.className = 'ddr-preview'
    // the frame sits over a note that says what is going on, and only shows once its document has
    // loaded: while rendering, or after an error, the preview never looks like an empty page
    dialog.innerHTML = `<form method="dialog"><button>${L('Close')}</button><span role="status"></span></form>` +
      `<div class="ddr-sheet"><p role="alert"></p><iframe sandbox title="${L('Report preview')}"></iframe></div>`
    box.innerHTML = group('Report', tile('preview', 'preview', 'Preview', 'Render the report with sample data'))
    const frame = dialog.querySelector('iframe'), status = dialog.querySelector('[role=status]'), note = dialog.querySelector('[role=alert]')
    let runs = 0
    frame.onload = () => { if (frame.srcdoc) frame.classList.add('ready') }
    commands.preview = async () => {
      const run = ++runs // a later click wins over a slow earlier render
      frame.classList.remove('ready')
      status.textContent = ''
      note.textContent = L('Rendering…')
      note.classList.remove('err')
      if (!dialog.open) dialog.showModal() // open at once, so the click visibly did something
      try {
        const html = String(await preview(structuredClone(tpl)))
        if (run !== runs) return
        // pages as sheets on a grey desk, so the page breaks show, scaled to fit the frame. The frame is
        // sandboxed and cannot be reached from here, so this goes into the document; printing it is unchanged.
        // The run number keeps the document new even when nothing changed, so the frame always reloads and shows.
        const fit = Math.min(1, (frame.clientWidth - 40) / (tpl.page.width * 4 / 3)), pages = html.split('<div class="p">').length - 1
        frame.srcdoc = html.replace('</head>', `<style>@media screen{html{background:#e5e7eb}body{padding:12px 0;zoom:${fit}}.p{margin:0 auto 12px;background:#fff;box-shadow:0 1px 4px #0003}}</style><!--${run}--></head>`)
        status.textContent = pages ? `${pages} ${L(pages === 1 ? 'page' : 'pages')}` : ''
      } catch (err) {
        if (run !== runs) return
        note.textContent = err.message || String(err)
        note.classList.add('err')
      }
    }
    $('[data-panel=View]').append(box.firstChild)
    // and next to the zoom, where it is always in reach
    box.innerHTML = ib('preview', 'preview', 'Render the report with sample data')
    $('[data-id=status]').after(box.firstChild)
    root.append(dialog)
  }

  // ---- drawing --------------------------------------------------------------
  // sync shows the selection and the page settings in the ribbon
  function sync() {
    const p = tpl.page
    for (const c of $$('[data-prop]')) {
      const key = c.dataset.prop, e = sel.find(s => STYLE[s.type]?.includes(key))
      c.disabled = !e
      const v = (e && e[propOf(e, key)]) || DEFAULT[key]
      if (c.type === 'checkbox') c.checked = !!v
      else c.value = v
    }
    const styled = key => sel.filter(e => STYLE[e.type]?.includes(key))
    for (const key of ['bold', 'italic', 'underline']) $(`[data-cmd=${key}]`).setAttribute('aria-pressed', styled(key).some(e => e[key]))
    const first = styled('align')[0], align = first && (first.type === 'table' ? first.aligns?.[activeCol] : first.align) || 'left'
    for (const b of $$('[data-cmd^="align:"]')) b.setAttribute('aria-pressed', !!first && b.dataset.cmd === 'align:' + align)
    for (const c of $$('[data-page]')) c.value = p[c.dataset.page] || (c.type === 'color' ? '#ffffff' : 0)
    for (const c of $$('[data-tpl]')) c.type === 'checkbox' ? c.checked = !!tpl[c.dataset.tpl] : c.value = tpl[c.dataset.tpl] ?? ''
    // "none" is no thousands separator: the box is ticked and the separator field stays empty
    const none = tpl.thousands === 'none'
    $('[data-id=nothousands]').checked = none
    Object.assign($('[data-tpl=thousands]'), { disabled: none, value: none ? '' : tpl.thousands ?? '' })
    const [short, long] = [p.width, p.height].sort((a, b) => a - b)
    $('[data-id=paper]').value = Object.keys(PAPER).find(k => PAPER[k][0] === short && PAPER[k][1] === long) ?? 'Custom'
    $('[data-id=orientation]').value = p.width > p.height ? 'Landscape' : 'Portrait'
    $('[data-cmd=undo]').disabled = at <= 0
    $('[data-cmd=redo]').disabled = at >= history.length - 1
    for (const c of ['cut', 'copy', 'duplicate', 'delete', 'front', 'back']) $(`[data-cmd=${c}]`).disabled = !sel.length
    $('[data-cmd=paste]').disabled = !clip.length
    $('[data-cmd=distribute]').disabled = sel.length < 3
    const paper = Object.keys(PAPER).find(k => PAPER[k][0] === short && PAPER[k][1] === long) ?? L('Custom')
    $('[data-id=status]').textContent = `${paper} · ${p.width} × ${p.height} pt` + (sel.length === 1 ? ` · ${L(sel[0].type)} ${sel[0].w} × ${sel[0].h}` : sel.length ? ` · ${sel.length} ${L('selected')}` : '')
    $('[data-id=side]').textContent = L('Properties') + (sel.length === 1 ? ' · ' + L(sel[0].type) : '')
  }

  function draw() {
    const refocus = page.contains(document.activeElement), p = tpl.page
    for (const b of BANDS) for (const e of tpl.bands[b].elements) fit(e, tpl.bands[b])
    nodes.clear()
    Object.assign(page.style, {
      width: px(p.width), minHeight: px(p.height), padding: SIDES.map(s => px(p[s])).join(' '),
      backgroundColor: p.background || '#fff', backgroundImage: isPicture(p.backgroundImage) ? `url("${p.backgroundImage}")` : '',
    })
    page.replaceChildren(...shown().map(name => {
      const band = tpl.bands[name], el = document.createElement('div'), grip = document.createElement('div')
      el.className = 'ddr-band'
      el.dataset.name = name
      el.dataset.label = L(name)
      el.style.height = px(band.height)
      el.toggleAttribute('data-thin', band.height < 12) // no room for the name
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
        grip.onpointermove = mv => { band.height = Math.max(0, Math.round(h0 + (mv.clientY - y0) / zoom)); el.style.height = px(band.height); el.toggleAttribute('data-thin', band.height < 12) }
        grip.onpointerup = () => { grip.onpointermove = grip.onpointerup = null; redraw() } // a band never ends up shorter than what is in it
        capture(grip, ev)
      }
      el.append(...band.elements.map(node), grip)
      return el
    }))
    if (!BANDS.some(b => tpl.bands[b].elements.length)) { // an empty page says how to begin, with buttons to begin with
      const hint = page.appendChild(document.createElement('div')), starters = [['add:text', 'text', 'Text box'], ['add:table', 'table', 'Table'], ['picture', 'image', 'Picture'], ['add:chart', 'chart', 'Chart']]
      hint.className = 'ddr-hint'
      hint.innerHTML = `<b>${L('Start here')}</b><div>${starters.filter(([cmd]) => !hidden.has(cmd.replace('add:', ''))).map(([cmd, pic, label]) => tile(cmd, pic, label)).join('')}</div>`
      hint.append(L('Click to put one on the page. Drag it to move it, double-click it to type.'))
    }
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
    grab()
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

  function node(e) {
    const n = document.createElement('div'), s = n.style, stroke = e.color || '#000'
    nodes.set(e, n)
    n.className = 'ddr-el'
    n.dataset.type = e.type
    n.tabIndex = 0
    n.setAttribute('role', 'group')
    n.setAttribute('aria-label', `${L('element')}: ${e.type}`)
    s.left = px(e.x); s.top = px(e.y); s.width = px(e.w); s.height = px(e.h)
    if (e.type === 'text' || e.type === 'table') {
      s.fontFamily = fontList[e.font] || ''
      s.fontSize = px(e.fontSize || 10); s.lineHeight = e.lineHeight || ''
      s.fontWeight = e.bold ? 'bold' : ''; s.fontStyle = e.italic ? 'italic' : ''; s.textDecoration = e.underline ? 'underline' : ''
      s.backgroundColor = e.fill || ''
    }
    if (e.showIf) { // printed only when its condition holds: marked, and the condition in the tooltip
      n.dataset.cond = ''
      n.title = `${L('show when')}: ${e.showIf}`
    }
    if (e.type === 'text') {
      // the words sit in an inner box, so the element can align them up and down
      const words = n.appendChild(document.createElement('div'))
      fill(words, e.text)
      s.textAlign = e.align || 'left'; s.color = stroke
      if (e.valign === 'middle' || e.valign === 'bottom') Object.assign(s, { display: 'flex', flexDirection: 'column', justifyContent: e.valign === 'middle' ? 'center' : 'flex-end' })
      n.ondblclick = () => edit(words, true, () => { e.text = markup(words).replace(/\n$/, ''); redraw() })
    } else if (e.type === 'table') {
      const line = e.border > 0 ? `${e.border}px solid ${stroke}` : '1px dotted #ccd'
      const grid = e.border > 0 ? e.grid || 'all' : 'all', rows = e.cells.length // without a border, guides show every cell
      // look is what the renderer does: header and total row bold with their own colours, body rows banded
      const look = r => e.header && r === 0 ? [e.headerFill, e.headerColor, true]
        : e.footer && r === rows - 1 && (r > 0 || !e.header) ? [e.footerFill, e.footerColor, true]
        : [(e.header ? r % 2 === 0 : r % 2 === 1) ? e.stripe : '', e.textColor, e.bold]
      const heights = rowWeights(e), nc = e.cols.length
      s.gridTemplateColumns = e.cols.map(c => `minmax(0,${c}fr)`).join(' ')
      s.gridTemplateRows = heights.map(h => `minmax(0,${h}fr)`).join(' ')
      if (grid === 'outer') s.border = line
      else s.borderTop = line
      if (grid === 'all') s.borderLeft = line
      if (e.data) n.dataset.repeat = e.data // a data table: its body rows repeat for each item of that list
      // a merged cell covers its neighbours, inside its part of the table (header, body, total row), as when printed
      const part = r => e.header && r === 0 ? 0 : e.footer && r === rows - 1 && (r > 0 || !e.header) ? 2 : 1
      const style = (r, c) => e.cellStyle?.[`${r},${c}`] ?? {}
      const span = (r, c) => {
        let rs = 1
        while (rs < (style(r, c).rowSpan || 1) && r + rs < rows && part(r + rs) === part(r)) rs++
        return [Math.min(style(r, c).colSpan || 1, nc - c), rs]
      }
      const covered = new Set()
      e.cells.forEach((row, r) => row.forEach((_, c) => {
        if (covered.has(`${r},${c}`)) return
        const [cs, rs] = span(r, c)
        for (let i = 0; i < rs; i++) for (let j = 0; j < cs; j++) if (i || j) covered.add(`${r + i},${c + j}`)
      }))
      const pad = e.padding != null && e.padding !== '' ? `${e.padding}px` : '2px 3px'
      e.cells.forEach((row, r) => row.forEach((text, c) => {
        if (covered.has(`${r},${c}`)) return
        const cell = document.createElement('div'), words = cell.appendChild(document.createElement('div')), st = style(r, c), [cs, rs] = span(r, c)
        const [bg, ink, bold] = look(r)
        cell.className = 'ddr-cell'
        fill(words, text)
        cell.dataset.col = c
        cell.dataset.row = r
        if (r === activeRow && c === activeCol) cell.dataset.active = ''
        if (e.data && part(r) === 1 && c === 0) cell.dataset.body = ''
        if (grid !== 'outer') cell.style.borderBottom = line
        if (grid === 'all') cell.style.borderRight = line
        Object.assign(cell.style, {
          gridColumn: `${c + 1} / span ${cs}`, gridRow: `${r + 1} / span ${rs}`, padding: pad,
          textAlign: st.align || e.aligns?.[c] || 'left', backgroundColor: st.fill || bg || '', color: st.color || ink || '',
          fontWeight: (st.bold ?? bold) ? 'bold' : 'normal', fontStyle: (st.italic ?? e.italic) ? 'italic' : 'normal',
        })
        if (e.valign === 'middle' || e.valign === 'bottom') Object.assign(cell.style, { display: 'flex', flexDirection: 'column', justifyContent: e.valign === 'middle' ? 'center' : 'flex-end' })
        cell.ondblclick = () => edit(words, true, () => { e.cells[r][c] = markup(words).replace(/\n$/, ''); changed() })
        n.append(cell)
      }))
      // grips on the inner column and row edges: drag to share the room between two neighbours
      const grip = (kind, i, at) => {
        const g = n.appendChild(document.createElement('div'))
        g.className = 'ddr-tgrip'
        g.dataset.kind = kind
        g.title = L(kind === 'col' ? 'Drag to change the column width' : 'Drag to change the row height')
        Object.assign(g.style, kind === 'col' ? { left: px(at - 3), top: 0, bottom: 0, width: '6px' } : { top: px(at - 3), left: 0, right: 0, height: '6px' })
        g.onpointerdown = ev => {
          ev.stopPropagation()
          const total = kind === 'col' ? e.w : e.h, list = kind === 'col' ? e.cols : heights, sum = list.reduce((a, b) => a + b, 0)
          const size = list.map(v => v / sum * total), start = ev[kind === 'col' ? 'clientX' : 'clientY']
          g.onpointermove = mv => {
            const d = (mv[kind === 'col' ? 'clientX' : 'clientY'] - start) / zoom, pair = size[i - 1] + size[i]
            const a = Math.min(Math.max(6, size[i - 1] + d), pair - 6), now = [...size]
            now.splice(i - 1, 2, Math.round(a), Math.round(pair - a))
            if (kind === 'col') { e.cols = now; s.gridTemplateColumns = now.map(v => `minmax(0,${v}fr)`).join(' ') }
            else { e.heights = now; s.gridTemplateRows = now.map(v => `minmax(0,${v}fr)`).join(' ') }
          }
          g.onpointerup = () => { g.onpointermove = g.onpointerup = null; redraw() }
          capture(g, ev)
        }
      }
      let x = 0, y = 0
      const colSum = e.cols.reduce((a, b) => a + b, 0), rowSum = heights.reduce((a, b) => a + b, 0)
      e.cols.slice(0, -1).forEach((w, i) => { x += w / colSum * e.w; grip('col', i + 1, x) })
      heights.slice(0, -1).forEach((h, i) => { y += h / rowSum * e.h; grip('row', i + 1, y) })
    } else if (e.type === 'rect') {
      s.backgroundColor = e.fill || ''
      if (e.border > 0) s.border = `${e.border}px solid ${stroke}`
    } else if (e.type === 'line' && (e.slope === 'down' || e.slope === 'up')) {
      const svg = n.appendChild(document.createElementNS('http://www.w3.org/2000/svg', 'svg')), l = svg.appendChild(document.createElementNS('http://www.w3.org/2000/svg', 'line'))
      Object.entries({ width: '100%', height: '100%', viewBox: `0 0 ${e.w || 1} ${e.h || 1}`, preserveAspectRatio: 'none', style: 'display:block;overflow:visible' }).forEach(([k, v]) => svg.setAttribute(k, v))
      Object.entries({ x1: 0, x2: e.w, y1: e.slope === 'up' ? e.h : 0, y2: e.slope === 'up' ? 0 : e.h, 'stroke-width': e.border || 1, 'vector-effect': 'non-scaling-stroke' }).forEach(([k, v]) => l.setAttribute(k, v))
      l.style.stroke = stroke
    } else if (e.type === 'line') {
      s[e.h > e.w ? 'borderLeft' : 'borderTop'] = `${e.border || 1}px solid ${stroke}`
    } else if (e.type === 'image') {
      if (isPicture(e.src)) s.backgroundImage = `url("${e.src}")`
      else if (e.src) n.textContent = e.src // a picture from the data: its placeholder
    } else if (e.type === 'chart') {
      n.append(chartSketch(e, sampleRows(), number, tpl.locale === 'id' ? 'Lainnya' : 'Other'))
      n.title = `${e.chart}: ${e.value || '?'} / ${e.label || '?'}`
    } else if (e.type === 'barcode') {
      n.append(codeSketch(e, filled(e.text)))
      n.title = `${e.format === 'code128' ? 'Code 128' : 'QR code'}: ${e.text}`
    } else if (e.type === 'subreport') { // what its bands hold; double-click to change them
      const bands = Object.entries(e.report?.bands ?? {}).filter(([, b]) => b?.elements?.length).map(([k, b]) => `${L(k)} (${b.elements.length})`)
      n.textContent = `${L('Sub-report')}: ${e.data || '?'}\n${bands.join(' · ') || L('Edit sub-report…')}`
      n.ondblclick = () => editSubreport(e)
    } else { // a type this designer does not know: keep it, show what it is
      n.dataset.unknown = 1
      n.textContent = e.type
    }

    let dragged = false
    // reaching another table with the keyboard starts at its first cell, not at the last table's
    n.onfocus = () => { if (!sel.includes(e)) { if (e.type === 'table') activeCol = activeRow = 0; select(e) } }
    n.onpointerdown = ev => {
      if (editing && n.contains(editing)) return
      const hit = ev.target.closest?.('[data-col]')
      if (hit) { // the cell the cell options and the row and column buttons work on
        activeCol = +hit.dataset.col; activeRow = +hit.dataset.row
        for (const c of n.querySelectorAll('[data-active]')) c.removeAttribute('data-active')
        hit.dataset.active = ''
        if (sel.length === 1 && sel[0] === e) renderProps()
      }
      if (ev.shiftKey) return select(e, true) // shift-click adds to / removes from the selection
      if (!sel.includes(e)) select(e)
      else sync()
      const start = sel.map(o => [o, o.x, o.y]), mine = start.find(o => o[0] === e)
      n.onpointermove = mv => {
        let dx = (mv.clientX - ev.clientX) / zoom, dy = (mv.clientY - ev.clientY) / zoom
        if (snap && !mv.altKey) {
          const [ox, oy] = snapOffset(e, mine[1] + dx, mine[2] + dy)
          dx += ox; dy += oy
        }
        for (const [o, x, y] of start) { o.x = Math.max(0, Math.round(x + dx)); o.y = Math.round(y + dy); place(o) }
        dragged = true
        trash.hidden = false
        trash.classList.toggle('over', overTrash(mv))
      }
      capture(n, ev)
    }
    n.onpointerup = ev => {
      if (!n.onpointermove) return
      n.onpointermove = null
      const binned = overTrash(ev)
      trash.hidden = true
      trash.classList.remove('over')
      if (!dragged) return
      dragged = false
      if (binned) return moveTo(sel) // undo brings them back
      // one element dropped over another band: it now belongs to that band
      const top = n.getBoundingClientRect().top
      const over = [...page.children].map(b => [b.dataset.name, b.getBoundingClientRect()]).find(([, r]) => top >= r.top && top < r.bottom)
      if (sel.length === 1 && over && over[0] !== bandOf(e)) {
        e.y = Math.round((top - over[1].top) / zoom)
        return moveTo([e], over[0])
      }
      redraw()
    }
    return n
  }

  // ---- properties panel -------------------------------------------------------
  // table fields that are derived from its cells
  const VIRTUAL = {
    rows: { get: e => e.cells.length, set: (e, v) => tableSize(e, v, e.cells[0].length) },
    columns: { get: e => e.cells[0].length, set: (e, v) => tableSize(e, e.cells.length, v) },
    cols: { get: e => e.cols.join(','), set: (e, v) => { e.cols = e.cells[0].map((_, i) => +v.split(',')[i] || 1) } },
    // turning the total row on adds a row for it at the bottom, and turning it off takes that row away;
    // a one-row table (say, the totals in the summary band under a data table) becomes the total row itself
    footer: {
      get: e => !!e.footer,
      set: (e, v) => {
        if (v === !!e.footer || e.cells.length < (v ? 2 : 3)) return void (e.footer = v)
        const unit = e.h / rowWeights(e).reduce((a, b) => a + b, 0)
        tableSize(e, e.cells.length + (v ? 1 : -1), e.cells[0].length)
        e.h = Math.round(unit * rowWeights(e).reduce((a, b) => a + b, 0)) // the rows keep their height
        e.footer = v
      },
    },
    // a straight line is flat or upright by its shape; a slanted one gets room to slant in
    slope: {
      get: e => e.slope || 'straight',
      set: (e, v) => {
        e.slope = v === 'straight' ? '' : v
        if (e.slope && Math.min(e.w, e.h) < 10) e.h = e.w = Math.max(e.w, e.h, 60)
      },
    },
    // a picture from the data is a placeholder in src, such as {{photo}}
    dataSrc: { get: e => e.src?.startsWith('data:') ? '' : e.src || '', set: (e, v) => { e.src = v } },
    // relative row heights, like the column widths: "2,1,1" makes the first row twice as tall
    heights: { get: e => rowWeights(e).map(h => Math.round(h * 100) / 100).join(','), set: (e, v) => { e.heights = e.cells.map((_, i) => +v.split(',')[i] || 1) } },
    // empty is the default room (3pt across, 2pt down); a number is the same room on every side
    padding: { get: e => e.padding ?? '', set: (e, v) => { if (v.trim() === '' || isNaN(+v)) delete e.padding; else e.padding = Math.max(0, +v) } },
  }

  function renderProps() {
    props.replaceChildren()
    const note = text => props.append(Object.assign(document.createElement('p'), { textContent: text }))
    if (sel.length > 1) return note(`${sel.length} ${L('elements selected. Use the ribbon to format, align or move them together.')}`)
    const e = sel[0]
    if (!e) {
      note(L('Band heights. Select an element to edit it; double-click a text or a table cell to type.'))
      for (const b of shown()) {
        const band = tpl.bands[b]
        field(band, 'height', L(b))
        if (b === 'header' || b === 'columnHeader' || b === 'footer') { // every page, or the first (or last) only
          const pages = choice(b === 'footer' ? ['all', 'first', 'last'] : ['all', 'first'], band.pages || 'all')
          pages.onchange = () => { band.pages = pages.value === 'all' ? '' : pages.value; changed() }
          row(`${L(b)} · ${L('pages')}`, pages)
        } else if (b !== 'groupHeader') field(band, 'newPage', `${L(b)} · ${L('new page')}`)
      }
      return
    }
    // keyboard way to move an element to another band
    const band = choice(shown(), bandOf(e))
    band.onchange = () => moveTo([e], band.value)
    row(L('band'), band)
    // the colours of the header and the total row only show while that row is on
    for (const k of FIELDS[e.type] ?? BASE) if (!/^(header|footer)./.test(k) || e[k.slice(0, 6)]) field(e, k, L(LABEL[k] ?? k))
    if (e.type === 'subreport') {
      note(L('A report of its own for each item of a list in the data: header, column header, groups, detail and summary. Its footer is not used.'))
      const b = props.appendChild(Object.assign(document.createElement('button'), { type: 'button', textContent: L('Edit sub-report…'), className: 'ddr-subedit' }))
      b.onclick = () => editSubreport(e)
    }
    if (e.type === 'table') { // rows and columns next to the cell last clicked
      const bar = props.appendChild(document.createElement('div'))
      bar.className = 'ddr-tablebar'
      bar.title = L('Click a cell first; rows and columns are added next to it')
      for (const [op, label] of [['rowAbove', 'Row above'], ['rowBelow', 'Row below'], ['rowDelete', 'Delete row'], ['colLeft', 'Column left'], ['colRight', 'Column right'], ['colDelete', 'Delete column']]) {
        const b = bar.appendChild(Object.assign(document.createElement('button'), { type: 'button', textContent: L(label) }))
        b.dataset.table = op
        b.onclick = () => tableEdit(e, op)
      }
      cellProps(e)
    }
  }

  // cellProps edits the cell last clicked: its own colours, alignment and weight
  // over those of its row and column, and merging it with its neighbours
  function cellProps(e) {
    const r = Math.min(activeRow, e.cells.length - 1), c = Math.min(activeCol, e.cells[0].length - 1), key = `${r},${c}`
    const st = () => (e.cellStyle ??= {})[key] ??= {}
    const tidy = () => { if (!Object.keys(e.cellStyle?.[key] ?? {}).length) delete e.cellStyle?.[key] } // no empty styles left behind
    props.append(Object.assign(document.createElement('p'), { className: 'ddr-celltitle', textContent: `${L('Cell')} ${r + 1}, ${c + 1}` }))
    const set = (k, v) => { if (v === '' || v == null) delete st()[k]; else st()[k] = v; tidy(); draw(); changed() }
    for (const [k, label, fallback] of [['fill', 'cell fill', '#ffffff'], ['color', 'cell text', '#000000']]) {
      const box = document.createElement('span'), input = Object.assign(document.createElement('input'), { type: 'color', value: e.cellStyle?.[key]?.[k] || fallback })
      input.style.opacity = e.cellStyle?.[key]?.[k] ? '' : '.35'
      input.dataset.cell = k
      input.oninput = () => { input.style.opacity = ''; set(k, input.value) }
      const clear = Object.assign(document.createElement('button'), { type: 'button', textContent: '×', title: L('Clear') })
      clear.onclick = () => { set(k, ''); renderProps() }
      box.append(input, clear)
      row(L(label), box)
    }
    const align = choice(['', 'left', 'center', 'right', 'justify'], e.cellStyle?.[key]?.align ?? '')
    align.options[0].text = L('as the row')
    align.dataset.cell = 'align'
    align.onchange = () => set('align', align.value)
    row(L('cell align'), align)
    for (const [k, label] of [['bold', 'cell bold'], ['italic', 'cell italic']]) {
      const v = e.cellStyle?.[key]?.[k], pick = choice(['', 'yes', 'no'], v === true ? 'yes' : v === false ? 'no' : '')
      pick.options[0].text = L('as the row')
      pick.dataset.cell = k
      pick.onchange = () => set(k, pick.value === '' ? '' : pick.value === 'yes')
      row(L(label), pick)
    }
    const bar = props.appendChild(document.createElement('div'))
    bar.className = 'ddr-tablebar'
    for (const [op, label] of [['mergeRight', 'Merge right'], ['mergeDown', 'Merge down'], ['unmerge', 'Unmerge']]) {
      const b = bar.appendChild(Object.assign(document.createElement('button'), { type: 'button', textContent: L(label) }))
      b.dataset.table = op
      b.onclick = () => tableEdit(e, op)
    }
  }

  function choice(values, value) {
    const input = document.createElement('select')
    input.append(...values.map(v => new Option(L(v), v)))
    input.value = value
    return input
  }

  function row(label, input) {
    const el = document.createElement('label')
    el.append(label, input)
    props.append(el)
  }

  function field(obj, key, label = key) {
    if (hidden.has(key)) return
    let input
    const virtual = VIRTUAL[key], typing = {} // names this field's run of edits for the undo history
    if (SELECT[key]) {
      input = choice(SELECT[key], obj[key] || SELECT[key][0])
    } else if (key === 'text') {
      input = document.createElement('textarea')
      input.value = obj.text ?? ''
      if (obj.type === 'text') input.title = L('Use <b>, <i> and <u> to style parts of the text')
    } else {
      input = document.createElement('input')
      input.type = key === 'src' ? 'file' : key === 'color' || CLEARABLE.has(key) ? 'color' : BOOL.has(key) ? 'checkbox' : NUMERIC.has(key) ? 'number' : 'text'
      if (input.type === 'number') input.min = 0
      if (input.type === 'file') input.accept = 'image/png,image/jpeg,image/gif'
      else if (input.type === 'checkbox') input.checked = !!(virtual ? virtual.get(obj) : obj[key])
      else {
        const v = virtual ? virtual.get(obj) : obj[key] // 0 is a value: x and y are often 0
        input.value = input.type !== 'color' ? v ?? '' : v || (/Fill|stripe/.test(key) ? '#ffffff' : '#000000')
      }
      input.placeholder = L(PLACEHOLDER[key] ?? '')
      if (TITLE[key]) input.title = L(TITLE[key])
    }
    input.oninput = () => {
      if (input.type === 'file') {
        const reader = new FileReader()
        reader.onload = () => { obj.src = reader.result; draw(); changed() }
        if (input.files[0]) reader.readAsDataURL(input.files[0])
        return
      }
      input.style.opacity = ''
      const value = input.type === 'number' ? Math.max(0, +input.value || 0) : input.type === 'checkbox' ? input.checked : input.value
      if (virtual) virtual.set(obj, value)
      else obj[key] = value
      draw()
      changed(typing) // the keystrokes of one field are one undo step
      if (input.type === 'checkbox' || key === 'slope') renderProps() // shows or hides the colours of that row; a new size
    }
    // typing a position outside the page is corrected when the field is left, not mid-keystroke
    input.onchange = () => { if (!virtual && input.type === 'number' && +input.value !== obj[key]) input.value = obj[key] }
    if (!CLEARABLE.has(key)) return row(label, input)
    // an optional colour: until one is picked it shows faded, and × takes it away again
    const box = document.createElement('span'), clear = Object.assign(document.createElement('button'), { type: 'button', textContent: '×', title: L('Clear') })
    input.style.opacity = obj[key] ? '' : '.35'
    clear.onclick = () => { obj[key] = ''; draw(); changed(); renderProps() }
    box.append(input, clear)
    row(label, box)
  }

  // leave out what the host hid; a name matches a tab, a group (Tab.Group for one tab's), a command or a setting
  for (const name of hidden) {
    const [a, b] = name.split('.').map(s => globalThis.CSS.escape(s))
    const found = b ? `[data-panel="${a}"] [data-group="${b}"]`
      : ['data-tab', 'data-panel', 'data-group', 'data-cmd', 'data-prop', 'data-tpl', 'data-page'].map(k => `[${k}="${a}"]`).join() + `,[data-cmd="add:${a}"]`
    for (const el of root.querySelectorAll(found)) (el.closest('label') ?? el).hidden = true
  }
  // a ribbon group with nothing left in it goes too, and the first tab still shown opens
  for (const g of $$('.ddr-group')) if (![...g.firstChild.children].some(c => !c.hidden)) g.hidden = true
  if ($('[data-tab][aria-selected=true]').hidden) $$('[data-tab]').find(t => !t.hidden)?.click()

  load(template, true)
  requestAnimationFrame(() => {
    if (!stage.clientWidth) return
    if (root.clientWidth <= 760) $('.ddr-side').open = false // small screen: the page comes first, properties one tap away
    if (stage.clientWidth < tpl.page.width + 34) setZoom(fitZoom())
  })
  return {
    getTemplate: () => structuredClone(tpl),
    setTemplate: t => load(t),
    setFields: list => { fields = Array.isArray(list) ? list.map(String) : []; fillFields() },
    setData: d => { data = d; draw() },
    // destroy empties the element and lets go of the document, for a page that removes the designer
    destroy: () => {
      for (const undo of off.splice(0)) undo()
      root.replaceChildren()
      root.classList.remove('ddr')
    },
  }
}
