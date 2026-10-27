package ddreport

import (
	"embed"
	"io/fs"
	"regexp"
	"strings"
	"sync"
)

// Family is a TrueType font family. Only Regular is required: a missing bold or
// italic face falls back to the closest one there is.
type Family struct {
	Regular, Bold, Italic, BoldItalic []byte

	// the files AddFonts found, read the first time the family is used, so a
	// folder of a hundred fonts costs nothing until a report names one
	mu      sync.Mutex
	pending [4]fontFileRef
}

type fontFileRef struct {
	fsys fs.FS
	path string
}

var faceNames = [4]string{"Regular", "Bold", "Italic", "BoldItalic"}

func (f *Family) slots() [4]*[]byte { return [4]*[]byte{&f.Regular, &f.Bold, &f.Italic, &f.BoldItalic} }

// load reads the faces AddFonts found and nothing has read yet. A file that
// cannot be read leaves its face out, so the closest other face is used.
func (f *Family) load() *Family {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, s := range f.slots() {
		if p := f.pending[i]; p.fsys != nil {
			*s, _ = fs.ReadFile(p.fsys, p.path)
			f.pending[i] = fontFileRef{}
		}
	}
	return f
}

// Faces names the faces the family has, of Regular, Bold, Italic and BoldItalic,
// without reading the files AddFonts found.
func (f *Family) Faces() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var faces []string
	for i, s := range f.slots() {
		if *s != nil || f.pending[i].fsys != nil {
			faces = append(faces, faceNames[i])
		}
	}
	return faces
}

func (f *Family) face(bold, italic bool) []byte {
	f.load()
	try := [][]byte{f.Regular}
	switch {
	case bold && italic:
		try = [][]byte{f.BoldItalic, f.Bold, f.Italic, f.Regular}
	case bold:
		try = [][]byte{f.Bold, f.Regular}
	case italic:
		try = [][]byte{f.Italic, f.Regular}
	}
	for _, b := range try {
		if b != nil {
			return b
		}
	}
	return nil
}

// Families are the fonts a template can name besides the built-in Helvetica,
// Times and Courier. It starts with the bundled ones (see BundledFonts); add
// your own at startup:
//
//	ddreport.Families["Inter"] = &ddreport.Family{Regular: regular, Bold: bold}
var Families = map[string]*Family{}

// BundledFonts holds the bundled TrueType files, named Family-Face.ttf
// (Face is Regular, Bold, Italic or BoldItalic), with their licences.
// `ddreport -serve` hands them to the designer.
//
//go:embed fonts
var BundledFonts embed.FS

var (
	fontFile  = regexp.MustCompile(`^fonts/(\w+)-(Regular|Bold|Italic|BoldItalic)\.ttf$`)
	wordStart = regexp.MustCompile(`([a-z])([A-Z])`)
	safeName  = regexp.MustCompile(`^[\w -]+$`) // a family name that can go into CSS as is
)

func init() {
	if err := AddFonts(BundledFonts); err != nil {
		panic(err)
	}
}

// AddFonts adds every Family-Face.ttf file of fsys (at its root or under fonts/)
// to Families. "OpenSans-Bold.ttf" is the bold face of "Open Sans".
func AddFonts(fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		m := fontFile.FindStringSubmatch("fonts/" + strings.TrimPrefix(path, "fonts/"))
		if m == nil {
			return nil
		}
		// it is read when first used; a file that cannot be read is said now, not then
		file, err := fsys.Open(path)
		if err != nil {
			return err
		}
		file.Close()
		name := wordStart.ReplaceAllString(m[1], "$1 $2")
		f := Families[name]
		if f == nil {
			f = &Family{}
			Families[name] = f
		}
		i := 0
		for faceNames[i] != m[2] {
			i++
		}
		f.mu.Lock()
		*f.slots()[i], f.pending[i] = nil, fontFileRef{fsys, path}
		f.mu.Unlock()
		return nil
	})
}
