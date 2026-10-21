// Types for ddreport-designer. The template is the JSON the ddreport backend renders;
// its full description is in the README (bands, elements, placeholders).

export interface Element {
  type: 'text' | 'table' | 'image' | 'rect' | 'line' | 'chart' | 'barcode' | 'subreport' | (string & {})
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
  /** 'light' (default), 'dark', or 'auto' to follow the system */
  theme?: 'light' | 'dark' | 'auto'
  /** URL of the bundled fonts, e.g. '/fonts/' as `ddreport -serve` has them */
  fontBase?: string
  /** more font names, registered with the backend */
  fonts?: string[]
  /** the data's field names, for the Field menu */
  fields?: string[]
  /** sample data that charts on the page are drawn from */
  data?: unknown
  /** fewer tools, each button with its name: for a first report */
  simple?: boolean
  /**
   * What to leave out, by name: a tab ('Layout'), a ribbon group ('Arrange', or 'Layout.Data' for
   * one tab's), a command ('delete', 'preview'), an element to insert ('chart', 'subreport'),
   * or a setting in the ribbon or the properties panel ('sortBy', 'showIf'). A hidden command
   * does not run from its keyboard shortcut either.
   */
  hide?: string[]
  /** your own words for any text of the interface, keyed by its English text */
  messages?: Record<string, string>
}

export interface Designer {
  /** a copy of the template, to send to the backend */
  getTemplate(): Template
  /** replaces the template; Undo brings the previous one back */
  setTemplate(template?: Template | object): void
  /** changes the Field menu */
  setFields(fields: string[]): void
  /** changes the sample data the charts are drawn from */
  setData(data: unknown): void
  /** empties the element and stops listening to the page; call it when the designer goes away */
  destroy(): void
}

export function createDesigner(element: HTMLElement, options?: DesignerOptions): Designer
export function emptyTemplate(): Template
