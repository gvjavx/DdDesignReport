package ddreport

import (
	"unicode"
)

// The PDF draws the glyph of each character as it comes, left to right. That
// is right for Latin, Cyrillic, Greek and Thai (whose marks have no width and
// sit on the letter before them), but Arabic letters take another shape by
// how they join their neighbours, and Arabic and Hebrew read right to left.
// visual turns a word into what is drawn: Arabic letters in their joined forms
// (the presentation forms most Arabic fonts carry, such as Amiri and Noto
// Naskh Arabic), lam-alef as one letter, and right-to-left text reversed, with
// the numbers and Latin in it kept left to right. HTML needs none of this:
// browsers shape and order the text themselves.

// arabicForms are a letter's isolated, final, initial and medial forms; a
// letter that joins only the one before it has no initial or medial form.
var arabicForms = map[rune][4]rune{
	0x0621: {0xFE80}, 0x0622: {0xFE81, 0xFE82}, 0x0623: {0xFE83, 0xFE84}, 0x0624: {0xFE85, 0xFE86}, 0x0625: {0xFE87, 0xFE88},
	0x0626: {0xFE89, 0xFE8A, 0xFE8B, 0xFE8C}, 0x0627: {0xFE8D, 0xFE8E}, 0x0628: {0xFE8F, 0xFE90, 0xFE91, 0xFE92},
	0x0629: {0xFE93, 0xFE94}, 0x062A: {0xFE95, 0xFE96, 0xFE97, 0xFE98}, 0x062B: {0xFE99, 0xFE9A, 0xFE9B, 0xFE9C},
	0x062C: {0xFE9D, 0xFE9E, 0xFE9F, 0xFEA0}, 0x062D: {0xFEA1, 0xFEA2, 0xFEA3, 0xFEA4}, 0x062E: {0xFEA5, 0xFEA6, 0xFEA7, 0xFEA8},
	0x062F: {0xFEA9, 0xFEAA}, 0x0630: {0xFEAB, 0xFEAC}, 0x0631: {0xFEAD, 0xFEAE}, 0x0632: {0xFEAF, 0xFEB0},
	0x0633: {0xFEB1, 0xFEB2, 0xFEB3, 0xFEB4}, 0x0634: {0xFEB5, 0xFEB6, 0xFEB7, 0xFEB8}, 0x0635: {0xFEB9, 0xFEBA, 0xFEBB, 0xFEBC},
	0x0636: {0xFEBD, 0xFEBE, 0xFEBF, 0xFEC0}, 0x0637: {0xFEC1, 0xFEC2, 0xFEC3, 0xFEC4}, 0x0638: {0xFEC5, 0xFEC6, 0xFEC7, 0xFEC8},
	0x0639: {0xFEC9, 0xFECA, 0xFECB, 0xFECC}, 0x063A: {0xFECD, 0xFECE, 0xFECF, 0xFED0}, 0x0641: {0xFED1, 0xFED2, 0xFED3, 0xFED4},
	0x0642: {0xFED5, 0xFED6, 0xFED7, 0xFED8}, 0x0643: {0xFED9, 0xFEDA, 0xFEDB, 0xFEDC}, 0x0644: {0xFEDD, 0xFEDE, 0xFEDF, 0xFEE0},
	0x0645: {0xFEE1, 0xFEE2, 0xFEE3, 0xFEE4}, 0x0646: {0xFEE5, 0xFEE6, 0xFEE7, 0xFEE8}, 0x0647: {0xFEE9, 0xFEEA, 0xFEEB, 0xFEEC},
	0x0648: {0xFEED, 0xFEEE}, 0x0649: {0xFEEF, 0xFEF0}, 0x064A: {0xFEF1, 0xFEF2, 0xFEF3, 0xFEF4},
	// Persian and Urdu letters
	0x067E: {0xFB56, 0xFB57, 0xFB58, 0xFB59}, 0x0686: {0xFB7A, 0xFB7B, 0xFB7C, 0xFB7D}, 0x0698: {0xFB8A, 0xFB8B},
	0x06A9: {0xFB8E, 0xFB8F, 0xFB90, 0xFB91}, 0x06AF: {0xFB92, 0xFB93, 0xFB94, 0xFB95}, 0x06CC: {0xFBFC, 0xFBFD, 0xFBFE, 0xFBFF},
}

// lamAlef are the isolated and final forms of lam followed by each alef.
var lamAlef = map[rune][2]rune{0x0622: {0xFEF5, 0xFEF6}, 0x0623: {0xFEF7, 0xFEF8}, 0x0625: {0xFEF9, 0xFEFA}, 0x0627: {0xFEFB, 0xFEFC}}

const tatweel = 0x0640

// mark is a sign drawn on the letter before it (harakat, Hebrew points), which does not break joining.
func mark(r rune) bool { return unicode.Is(unicode.Mn, r) }

func rtl(r rune) bool {
	return unicode.In(r, unicode.Arabic, unicode.Hebrew) && !mark(r) && !unicode.IsDigit(r)
}

func hasRTL(s string) bool {
	for _, c := range s {
		if rtl(c) {
			return true
		}
	}
	return false
}

// joinsNext says whether a letter connects to the one after it.
func joinsNext(r rune) bool { f, ok := arabicForms[r]; return r == tatweel || ok && f[2] != 0 }

// joinsPrev says whether a letter connects to the one before it.
func joinsPrev(r rune) bool {
	_, ok := arabicForms[r]
	return (r == tatweel || ok) && r != 0x0621
}

// shapeArabic gives each Arabic letter the form its neighbours ask for.
func shapeArabic(s string) []rune {
	in := []rune(s)
	// the letter before and after position i, past any marks
	neighbour := func(i, step int) rune {
		for j := i + step; j >= 0 && j < len(in); j += step {
			if !mark(in[j]) {
				return in[j]
			}
		}
		return 0
	}
	var out []rune
	for i := 0; i < len(in); i++ {
		c := in[i]
		forms, ok := arabicForms[c]
		if !ok {
			out = append(out, c)
			continue
		}
		before := joinsNext(neighbour(i, -1))
		after := neighbour(i, 1)
		if c == 0x0644 { // lam then alef: one letter
			if lig, ok := lamAlef[after]; ok {
				form := lig[0]
				if before {
					form = lig[1]
				}
				out = append(out, form)
				// the marks between them go after it; the alef is in it
				for i++; i < len(in) && in[i] != after; i++ {
					out = append(out, in[i])
				}
				continue
			}
		}
		next := joinsPrev(after) && forms[2] != 0
		switch {
		case before && next:
			out = append(out, forms[3])
		case before:
			out = append(out, forms[1])
		case next:
			out = append(out, forms[2])
		default:
			out = append(out, forms[0])
		}
	}
	return out
}

// visual is a word as drawn left to right; words without right-to-left letters stay as they are.
func visual(s string) string {
	if !hasRTL(s) {
		return s
	}
	runes := shapeArabic(s)
	// clusters: a letter with the marks on it, which stay after it
	var clusters [][]rune
	for _, c := range runes {
		if mark(c) && len(clusters) > 0 {
			clusters[len(clusters)-1] = append(clusters[len(clusters)-1], c)
			continue
		}
		clusters = append(clusters, []rune{c})
	}
	// runs of other characters (numbers, Latin) keep their order inside the reversed word
	var out []rune
	for i := len(clusters) - 1; i >= 0; {
		if rtl(clusters[i][0]) || !unicode.IsLetter(clusters[i][0]) && !unicode.IsDigit(clusters[i][0]) {
			out = append(out, clusters[i]...)
			i--
			continue
		}
		j := i
		for j > 0 && !rtl(clusters[j-1][0]) && (unicode.IsLetter(clusters[j-1][0]) || unicode.IsDigit(clusters[j-1][0]) || clusters[j-1][0] == '.' || clusters[j-1][0] == ',') {
			j--
		}
		for k := j; k <= i; k++ {
			out = append(out, clusters[k]...)
		}
		i = j - 1
	}
	return string(out)
}

// bidiOrder puts the words of a right-to-left line in the order they are
// drawn left to right: the line reversed, but a run of left-to-right words in
// it (Order 123) kept as it reads. Spaces between two such words belong to the run.
func bidiOrder(spans []span) []span {
	kind := func(s string) int { // 1 right to left, 2 left to right, 0 neither (spaces, signs)
		for _, c := range s {
			switch {
			case rtl(c):
				return 1
			case unicode.IsLetter(c) || unicode.IsDigit(c):
				return 2
			}
		}
		return 0
	}
	kinds := make([]int, len(spans))
	for i, s := range spans {
		kinds[i] = kind(s.text)
	}
	for i, k := range kinds { // neither: left to right only between two left-to-right words
		if k != 0 {
			continue
		}
		before, after := 0, 0
		for j := i - 1; j >= 0 && before == 0; j-- {
			before = kinds[j]
		}
		for j := i + 1; j < len(kinds) && after == 0; j++ {
			after = kinds[j]
		}
		kinds[i] = 1
		if before == 2 && after == 2 {
			kinds[i] = 2
		}
	}
	var out []span
	for i := len(spans) - 1; i >= 0; {
		if kinds[i] != 2 {
			out = append(out, spans[i])
			i--
			continue
		}
		j := i
		for j > 0 && kinds[j-1] == 2 {
			j--
		}
		out = append(out, spans[j:i+1]...)
		i = j - 1
	}
	return out
}

// rightToLeft says whether a text reads right to left: its first letter that has a direction does.
func rightToLeft(s string) bool {
	for _, c := range s {
		if rtl(c) {
			return true
		}
		if unicode.IsLetter(c) {
			return false
		}
	}
	return false
}
