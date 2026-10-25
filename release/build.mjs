// Builds a release into dist/, for CI (.github/workflows/release.yml) or by hand:
//   node release/build.mjs 0.1.0
// dist/bin       the ddreport binary for each system, attached to the GitHub Release
// dist/npm       the npm packages: @ddreport/<system> with its binary, ddreport (the
//                wrapper, which depends on all of them and installs the one that fits),
//                and ddreport-designer
// dist/native    the binaries laid out for the Java jars with a classifier per system
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const version = process.argv[2]?.replace(/^v/, '')
if (!/^\d+\.\d+\.\d+(-[\w.]+)?$/.test(version ?? '')) {
  console.error('usage: node release/build.mjs <version>, e.g. 0.1.0')
  process.exit(2)
}
const dist = path.join(root, 'dist')
fs.rmSync(dist, { recursive: true, force: true })

// [Go GOOS, GOARCH] -> npm's os and cpu, and the classifier of the Java jar
const targets = [
  ['windows', 'amd64', 'win32', 'x64', 'windows-x64'],
  ['windows', 'arm64', 'win32', 'arm64', 'windows-arm64'],
  ['linux', 'amd64', 'linux', 'x64', 'linux-x64'],
  ['linux', 'arm64', 'linux', 'arm64', 'linux-arm64'],
  ['darwin', 'amd64', 'darwin', 'x64', 'darwin-x64'],
  ['darwin', 'arm64', 'darwin', 'arm64', 'darwin-arm64'],
]
const repository = { type: 'git', url: 'git+https://github.com/gvjavx/DdDesignReport.git' }
const common = { version, license: 'MIT', repository, homepage: 'https://github.com/gvjavx/DdDesignReport#readme', author: 'Naufal Hilmi Abdurrahman' }
const write = (file, body) => { fs.mkdirSync(path.dirname(file), { recursive: true }); fs.writeFileSync(file, body) }
const json = (file, obj) => write(file, JSON.stringify(obj, null, 2) + '\n')

const optional = {}
for (const [goos, goarch, os, cpu, system] of targets) {
  const exe = goos === 'windows' ? '.exe' : ''
  const out = path.join(dist, 'bin', `ddreport-${goos}-${goarch}${exe}`)
  // pure Go: every system builds on any machine; -s -w leaves out the debug tables
  execFileSync('go', ['build', '-trimpath', '-ldflags=-s -w', '-o', out, './cmd/ddreport'], {
    cwd: root, stdio: 'inherit', env: { ...process.env, GOOS: goos, GOARCH: goarch, CGO_ENABLED: '0' },
  })
  const name = `@ddreport/${os}-${cpu}` // its folder in dist/npm stays ddreport-<system>
  optional[name] = version
  const pkg = path.join(dist, 'npm', `ddreport-${os}-${cpu}`)
  fs.mkdirSync(path.join(pkg, 'bin'), { recursive: true })
  fs.copyFileSync(out, path.join(pkg, 'bin', `ddreport${exe}`))
  json(path.join(pkg, 'package.json'), {
    name, ...common, description: `The ddreport binary for ${os} ${cpu}, installed by the ddreport package`,
    os: [os], cpu: [cpu], files: ['bin'],
  })
  write(path.join(pkg, 'README.md'), `The \`ddreport\` binary for ${os} ${cpu}. Install [ddreport](https://www.npmjs.com/package/ddreport), which picks this package on ${os} ${cpu}.\n`)
  const native = path.join(dist, 'native', system, 'io', 'ddreport', 'native', system, `ddreport${exe}`)
  fs.mkdirSync(path.dirname(native), { recursive: true })
  fs.copyFileSync(out, native)
}

// the wrapper: its code, and every system's binary as an optional dependency (npm installs the one that fits)
const wrapper = path.join(dist, 'npm', 'ddreport')
fs.mkdirSync(wrapper, { recursive: true })
fs.copyFileSync(path.join(root, 'node', 'index.js'), path.join(wrapper, 'index.js'))
fs.copyFileSync(path.join(root, 'LICENSE'), path.join(wrapper, 'LICENSE'))
const node = JSON.parse(fs.readFileSync(path.join(root, 'node', 'package.json'), 'utf8'))
json(path.join(wrapper, 'package.json'), {
  ...node, ...common, files: ['index.js'], optionalDependencies: optional,
  keywords: ['report', 'pdf', 'xlsx', 'designer', 'reporting', 'invoice'], engines: { node: '>=18' }, scripts: undefined,
})
write(path.join(wrapper, 'README.md'), fs.readFileSync(path.join(root, 'release', 'npm-README.md'), 'utf8'))

// the designer
const designer = path.join(dist, 'npm', 'ddreport-designer')
fs.mkdirSync(designer, { recursive: true })
for (const f of ['designer.js', 'designer.d.ts', 'index.html', 'template.schema.json']) fs.copyFileSync(path.join(root, 'designer', f), path.join(designer, f))
fs.cpSync(path.join(root, 'designer', 'templates'), path.join(designer, 'templates'), { recursive: true }) // import 'ddreport-designer/templates/invoice.json'
fs.copyFileSync(path.join(root, 'LICENSE'), path.join(designer, 'LICENSE'))
const des = JSON.parse(fs.readFileSync(path.join(root, 'designer', 'package.json'), 'utf8'))
json(path.join(designer, 'package.json'), { ...des, ...common, keywords: ['report', 'designer', 'drag-and-drop', 'pdf', 'reporting'] })
write(path.join(designer, 'README.md'), fs.readFileSync(path.join(root, 'release', 'designer-README.md'), 'utf8'))

console.log(`built ${version} in ${dist}`)
