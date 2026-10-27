// Downloads 100 more font families from Google Fonts (SIL OFL or Apache licensed)
// into one folder, as the Family-Face.ttf files ddreport reads:
//   node fontpack/download.mjs [folder]          (default: ddreport-fonts)
//   DDREPORT_FONTS=ddreport-fonts ddreport -serve 127.0.0.1:8080
// Each family gets the faces it has of Regular, Bold, Italic and BoldItalic.
// Files already there are kept, so running it again only fetches what is missing.
import fs from 'node:fs'
import path from 'node:path'

// none of the 7 bundled ones; each name written without its spaces reads back
// as itself ("Open Sans" -> OpenSans), so no capitals in a row and no digits
const FAMILIES = [
  // sans
  'Inter', 'Poppins', 'Raleway', 'Nunito', 'Nunito Sans', 'Work Sans', 'Rubik', 'Mulish', 'Karla', 'Barlow',
  'Manrope', 'Quicksand', 'Cabin', 'Arimo', 'Hind', 'Heebo', 'Assistant', 'Titillium Web', 'Fira Sans', 'Josefin Sans',
  'Kanit', 'Oxygen', 'Dosis', 'Exo', 'Archivo', 'Overpass', 'Maven Pro', 'Varela Round', 'Questrial', 'Mukta',
  'Libre Franklin', 'Jost', 'Outfit', 'Lexend', 'Figtree', 'Plus Jakarta Sans', 'Public Sans', 'Red Hat Display', 'Urbanist',
  'Space Grotesk', 'Signika', 'Asap', 'Catamaran', 'Prompt', 'Sarabun', 'Chivo', 'Noto Sans', 'Alegreya Sans',
  // serif
  'Merriweather', 'Libre Baskerville', 'Crimson Text', 'Cormorant Garamond', 'Bitter', 'Arvo', 'Noto Serif', 'Spectral', 'Cardo', 'Domine',
  'Zilla Slab', 'Roboto Slab', 'Alegreya', 'Vollkorn', 'Gelasio', 'Frank Ruhl Libre', 'Noticia Text', 'Rokkitt', 'Crete Round', 'Prata',
  'Cinzel', 'Literata', 'Newsreader', 'Fraunces', 'Josefin Slab',
  // narrow and display
  'Roboto Condensed', 'Oswald', 'Bebas Neue', 'Anton', 'Barlow Condensed', 'Archivo Narrow', 'Comfortaa',
  'Abril Fatface', 'Alfa Slab One', 'Lilita One',
  // Arabic, joined in the PDF by ddreport, and Thai
  'Amiri', 'Noto Naskh Arabic',
  // handwriting and script
  'Dancing Script', 'Pacifico', 'Lobster', 'Caveat', 'Satisfy', 'Kaushan Script', 'Sacramento', 'Allura', 'Parisienne',
  'Permanent Marker', 'Indie Flower',
  // monospace
  'Roboto Mono', 'Source Code Pro', 'Inconsolata', 'Space Mono',
]
const FACE = { '400 normal': 'Regular', '700 normal': 'Bold', '400 italic': 'Italic', '700 italic': 'BoldItalic' }

const dir = path.resolve(process.argv[2] || 'ddreport-fonts')
fs.mkdirSync(dir, { recursive: true })
const failed = []
for (const family of FAMILIES) {
  const file = family.replaceAll(' ', '')
  if (file.replace(/([a-z])([A-Z])/g, '$1 $2') !== family) throw new Error(`${family} would not read back as itself`)
  try {
    // Google answers a request without a browser's User-Agent with plain TrueType files.
    // A family without italics or a bold refuses the full request, so ask for less.
    let css
    for (const axes of [':ital,wght@0,400;0,700;1,400;1,700', ':wght@400;700', '']) {
      const res = await fetch(`https://fonts.googleapis.com/css2?family=${family.replaceAll(' ', '+')}${axes}`)
      if (res.ok) { css = await res.text(); break }
    }
    if (!css) throw new Error('not on Google Fonts')
    let got = 0
    for (const [, style, weight, url] of css.matchAll(/font-style: (\w+);\s*font-weight: (\d+);[\s\S]*?src: url\((\S+?\.ttf)\)/g)) {
      const face = FACE[`${weight} ${style}`], out = path.join(dir, `${file}-${face}.ttf`)
      if (!face) continue
      got++
      if (fs.existsSync(out)) continue
      const res = await fetch(url)
      if (!res.ok) throw new Error(`${url}: ${res.status}`)
      fs.writeFileSync(out, Buffer.from(await res.arrayBuffer()))
    }
    if (!got) throw new Error('no TrueType face')
    console.log(`${family}: ${got} face${got > 1 ? 's' : ''}`)
  } catch (err) {
    failed.push(family)
    console.error(`${family}: ${err.message}`)
  }
}
console.log(`\n${FAMILIES.length - failed.length} of ${FAMILIES.length} families in ${dir}`)
console.log(`use them: DDREPORT_FONTS=${dir} ddreport -serve 127.0.0.1:8080`)
process.exit(failed.length ? 1 : 0)
