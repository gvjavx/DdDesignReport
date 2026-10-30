// Types for ddreport-designer. The template is the JSON the ddreport backend renders;
// its full description is in the README (bands, elements, placeholders).

export interface Element {
  type: 'text' | 'table' | 'image' | 'rect' | 'line' | 'chart' | 'barcode' | 'subreport' | 'crosstab' | (string & {})
  x: number
  y: number
  w: number
  h: number
  [property: string]: unknown
}

export interface Band {
  height: number
  elements: Element[]
  pages?: '' | 'first' | 'last'
  newPage?: boolean
}

export interface Template {
  page: {
    width: number
    height: number
    marginTop: number
    marginRight: number
    marginBottom: number
    marginLeft: number
    background?: string
    backgroundImage?: string
    [property: string]: unknown
  }
  dataKey?: string
  groupBy?: string
  sortBy?: string
  bands: Record<string, Band>
  [property: string]: unknown
}

export interface DesignerOptions {
  /** the template to start with; an empty A4 page without it */
  template?: Template | object
  /** called with the template after every change */
  onChange?: (template: Template) => void
  /** renders the template as HTML (the backend's "html" format); shows the Preview button */
  preview?: (template: Template) => Promise<string> | string
  /** 'en' (default) or 'id' */
  lang?: 'en' | 'id'
  /** 'light' (default), 'dark', 'auto' to follow the system, or 'inherit' for the page's own theme (Bootstrap's --bs-* variables, its dark mode too) */
  theme?: 'light' | 'dark' | 'auto' | 'inherit'
  /** called whenever what a toolbar shows changes: for a toolbar of your own */
  onState?: (state: DesignerState) => void
  /** URL of the bundled fonts, e.g. '/fonts/' as `ddreport -serve` has them */
  fontBase?: string
  /** more font names, registered with the backend */
  fonts?: string[]
  /** the data's field names, for the Field menu; without it, those of the sample data */
  fields?: string[]
  /** sample data: charts on the page are drawn from it, the Field menu lists its fields, and the preview names fields it does not have */
  data?: unknown
  /** fewer tools, each button with its name: for a first report */
  simple?: boolean
  /**
   * What to leave out, by name: the whole ribbon ('ribbon', for a toolbar of your own), a tab ('Layout'), a ribbon group ('Arrange', or 'Layout.Data' for
   * one tab's), a command ('delete', 'preview'), an element to insert ('chart', 'subreport'),
   * or a setting in the ribbon or the properties panel ('sortBy', 'showIf'). A hidden command
   * does not run from its keyboard shortcut either.
   */
  hide?: string[]
  /** your own words for any text of the interface, keyed by its English text */
  messages?: Record<string, string>
}

export interface DesignerState {
  /** how many elements are selected, and the type when it is one */
  selected: number
  type: string | null
  bold: boolean
  italic: boolean
  underline: boolean
  align: 'left' | 'center' | 'right' | 'justify' | null
  undo: boolean
  redo: boolean
  paste: boolean
  /** 1 is 100% */
  zoom: number
}

export interface Designer {
  /** a copy of the template, to send to the backend */
  getTemplate(): Template
  /** replaces the template; Undo brings the previous one back */
  setTemplate(template?: Template | object): void
  /** changes the Field menu */
  setFields(fields: string[]): void
  /** changes the sample data */
  setData(data: unknown): void
  /**
   * does what a ribbon button does, for a toolbar of your own: 'bold', 'align:center', 'add:table',
   * 'undo', 'delete', 'zoom:fit'... A hidden command does nothing; an unknown one throws.
   */
  run(command: string): void
  /** the commands run takes (without their arguments), less the hidden ones */
  commands(): string[]
  /** what onState last said */
  getState(): DesignerState
  /** empties the element and stops listening to the page; call it when the designer goes away */
  destroy(): void
}

export function createDesigner(element: HTMLElement, options?: DesignerOptions): Designer
export function emptyTemplate(): Template
