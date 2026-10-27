package ddreport

import (
	"bytes"
	"crypto/md5"
	_ "embed"
	"fmt"
	"slices"
	"time"

	"codeberg.org/go-pdf/fpdf"
)

// sRGB2014.icc is the colour profile a PDF/A file says its colours are in,
// from the International Color Consortium (see icc/LICENSE.color.org).
//
//go:embed icc/sRGB2014.icc
var sRGB []byte

const pdfaProducer = "DdDesignReport"

// pdfaFont is the embedded font a PDF/A file prints a built-in PDF font with:
// PDF/A wants every font in the file, and Helvetica, Times and Courier are not.
// A font that is not in Families (or has no regular face) would fall back to
// Helvetica, so it prints with Roboto too.
func pdfaFont(name string) string {
	usable := func(n string) bool { f := Families[n]; return f != nil && f.load().Regular != nil }
	switch {
	case name == "Times":
		return "Lora"
	case name == "Courier" && usable("Roboto Mono"):
		return "Roboto Mono"
	case name != "" && name != "Helvetica" && name != "Courier" && usable(name):
		return name
	}
	return "Roboto"
}

// initPDFA makes the document PDF/A-2b: fixed dates the metadata repeats,
// an sRGB output intent, and XMP metadata saying which part it conforms to.
func (r *report) initPDFA() {
	now := r.t.now.Truncate(time.Second)
	r.pdf.SetCreationDate(now)
	r.pdf.SetModificationDate(now)
	r.pdf.SetProducer(pdfaProducer, false)
	r.pdf.AddOutputIntent(fpdf.OutputIntentType{
		SubtypeIdent:              fpdf.OutputIntent_GTS_PDFA1, // GTS_PDFA1 is the subtype for every part of PDF/A
		OutputConditionIdentifier: "sRGB IEC61966-2.1",
		Info:                      "sRGB IEC61966-2.1",
		ICCProfile:                sRGB,
	})
	date := now.Format("2006-01-02T15:04:05")
	r.pdf.SetXmpMetadata([]byte(`<?xpacket begin="` + "\xef\xbb\xbf" + `" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description rdf:about="" xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/" xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmlns:pdf="http://ns.adobe.com/pdf/1.3/">
<pdfaid:part>2</pdfaid:part>
<pdfaid:conformance>B</pdfaid:conformance>
<xmp:CreateDate>` + date + `</xmp:CreateDate>
<xmp:ModifyDate>` + date + `</xmp:ModifyDate>
<pdf:Producer>` + pdfaProducer + `</pdf:Producer>
</rdf:Description>
</rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`))
}

// withID gives a finished PDF the document ID PDF/A asks for, in its trailer.
// The trailer comes after the cross-reference table, so no offset moves.
func withID(pdf []byte) []byte {
	at := bytes.LastIndex(pdf, []byte("trailer\n<<\n"))
	if at < 0 {
		return pdf
	}
	at += len("trailer\n<<\n")
	id := fmt.Sprintf("%X", md5.Sum(pdf))
	return slices.Concat(pdf[:at], []byte("/ID [<"+id+"><"+id+">]\n"), pdf[at:])
}
