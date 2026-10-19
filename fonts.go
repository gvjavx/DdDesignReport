package ddreport

import (
	"embed"
	"io/fs"
	"regexp"
	"strings"
)

// Family is a TrueType font family. Only Regular is required: a missing bold or
// italic face falls back to the closest one there is.
type Family struct{ Regular, Bold, Italic, BoldItalic []byte }

func (f *Family) face(bold, italic bool) []byte {
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
		b, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		name := wordStart.ReplaceAllString(m[1], "$1 $2")
		f := Families[name]
		if f == nil {
			f = &Family{}
			Families[name] = f
		}
		*map[string]*[]byte{"Regular": &f.Regular, "Bold": &f.Bold, "Italic": &f.Italic, "BoldItalic": &f.BoldItalic}[m[2]] = b
		return nil
	})
}
