package ddreport

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// SignPDF signs a PDF this package made, so a reader shows who it is from and
// that it was not changed since: an invisible signature (adbe.pkcs7.detached,
// SHA-256) added as an incremental update, which keeps a PDF/A file PDF/A.
//
// certPEM holds the signer's certificate, then any intermediate certificates;
// keyPEM its private key (RSA or ECDSA, PKCS #8, PKCS #1 or SEC 1). A .p12 file
// converts with: openssl pkcs12 -in id.p12 -nodes -out id.pem. reason is shown
// with the signature and may be empty.
//
// The binary signs every PDF when DDREPORT_SIGN_CERT and DDREPORT_SIGN_KEY
// name such files (and DDREPORT_SIGN_REASON, if set, says why).
func SignPDF(pdf, certPEM, keyPEM []byte, reason string) ([]byte, error) {
	certs, key, err := signer(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return signPDF(pdf, certs, key, reason, time.Now())
}

func signer(certPEM, keyPEM []byte) ([]*x509.Certificate, crypto.Signer, error) {
	var certs []*x509.Certificate
	for rest := certPEM; ; {
		var b *pem.Block
		if b, rest = pem.Decode(rest); b == nil {
			break
		}
		if b.Type == "CERTIFICATE" {
			c, err := x509.ParseCertificate(b.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("ddreport: sign: certificate: %w", err)
			}
			certs = append(certs, c)
		}
	}
	if len(certs) == 0 {
		return nil, nil, errors.New("ddreport: sign: no CERTIFICATE in the certificate file")
	}
	var key crypto.Signer
	for rest := keyPEM; key == nil; {
		var b *pem.Block
		if b, rest = pem.Decode(rest); b == nil {
			return nil, nil, errors.New("ddreport: sign: no private key in the key file")
		}
		var k any
		var err error
		switch b.Type {
		case "PRIVATE KEY":
			k, err = x509.ParsePKCS8PrivateKey(b.Bytes)
		case "RSA PRIVATE KEY":
			k, err = x509.ParsePKCS1PrivateKey(b.Bytes)
		case "EC PRIVATE KEY":
			k, err = x509.ParseECPrivateKey(b.Bytes)
		default:
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("ddreport: sign: private key: %w", err)
		}
		switch k := k.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey:
			key = k.(crypto.Signer)
		default:
			return nil, nil, errors.New("ddreport: sign: only RSA and ECDSA keys can sign")
		}
	}
	// the key must be the first certificate's
	pub, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return nil, nil, err
	}
	if mine, _ := x509.MarshalPKIXPublicKey(certs[0].PublicKey); !bytes.Equal(pub, mine) {
		return nil, nil, errors.New("ddreport: sign: the key is not the first certificate's")
	}
	return certs, key, nil
}

const sigSpace = 16384 // bytes kept for the signature; a certificate chain of a few certificates fits

var (
	startXref = regexp.MustCompile(`startxref\s+(\d+)\s+%%EOF\s*$`)
	trailerRe = regexp.MustCompile(`(?s)trailer\s*<<(.*?)>>\s*startxref`)
	refRe     = func(key string) *regexp.Regexp { return regexp.MustCompile(`/` + key + `\s+(\d+)\s+0\s+R`) }
)

// object is the dictionary of object n, as written last.
func object(pdf []byte, n int) (string, error) {
	re := regexp.MustCompile(`(?s)(?:^|[\r\n])` + strconv.Itoa(n) + ` 0 obj\s*(<<.*?>>)\s*endobj`)
	all := re.FindAllSubmatch(pdf, -1)
	if all == nil {
		return "", fmt.Errorf("ddreport: sign: object %d not found", n)
	}
	return string(all[len(all)-1][1]), nil
}

// withEntry adds an entry before the closing >> of a dictionary.
func withEntry(dict, entry string) string {
	i := strings.LastIndex(dict, ">>")
	return dict[:i] + entry + "\n" + dict[i:]
}

func signPDF(pdf []byte, certs []*x509.Certificate, key crypto.Signer, reason string, at time.Time) ([]byte, error) {
	m := startXref.FindSubmatch(pdf)
	tr := trailerRe.FindAllSubmatch(pdf, -1)
	if m == nil || tr == nil {
		return nil, errors.New("ddreport: sign: not a PDF with a cross-reference table")
	}
	prev, trailer := string(m[1]), string(tr[len(tr)-1][1])
	num := func(s string) int { n, _ := strconv.Atoi(s); return n }
	sizeM := regexp.MustCompile(`/Size\s+(\d+)`).FindStringSubmatch(trailer)
	rootM, infoM := refRe("Root").FindStringSubmatch(trailer), refRe("Info").FindStringSubmatch(trailer)
	if sizeM == nil || rootM == nil {
		return nil, errors.New("ddreport: sign: the trailer has no /Size or /Root")
	}
	size, root := num(sizeM[1]), num(rootM[1])
	catalog, err := object(pdf, root)
	if err != nil {
		return nil, err
	}
	pagesM := refRe("Pages").FindStringSubmatch(catalog)
	if pagesM == nil {
		return nil, errors.New("ddreport: sign: the catalog has no /Pages")
	}
	pages, err := object(pdf, num(pagesM[1]))
	if err != nil {
		return nil, err
	}
	kid := regexp.MustCompile(`/Kids\s*\[\s*(\d+)\s+0\s+R`).FindStringSubmatch(pages)
	if kid == nil {
		return nil, errors.New("ddreport: sign: no pages")
	}
	first := num(kid[1])
	page, err := object(pdf, first)
	if err != nil {
		return nil, err
	}
	if strings.Contains(catalog, "/AcroForm") || strings.Contains(page, "/Annots") {
		return nil, errors.New("ddreport: sign: the PDF has forms or annotations already")
	}

	sig, widget := size, size+1
	placeholder := "/ByteRange [0 ********** ********** **********]"
	text := func(s string) string { // a PDF text string
		return "(" + strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`).Replace(s) + ")"
	}
	name := certs[0].Subject.CommonName
	objects := []struct {
		n    int
		dict string
	}{
		{sig, "<< /Type /Sig /Filter /Adobe.PPKLite /SubFilter /adbe.pkcs7.detached " + placeholder +
			" /Contents <" + strings.Repeat("0", 2*sigSpace) + "> /M " + text("D:"+at.UTC().Format("20060102150405")+"Z") +
			" /Name " + text(name) + " /Reason " + text(reason) + " >>"},
		{widget, fmt.Sprintf("<< /Type /Annot /Subtype /Widget /FT /Sig /Rect [0 0 0 0] /F 132 /T (Signature1) /V %d 0 R /P %d 0 R >>", sig, first)},
		{first, withEntry(page, fmt.Sprintf("/Annots [%d 0 R]", widget))},
		{root, withEntry(catalog, fmt.Sprintf("/AcroForm << /Fields [%d 0 R] /SigFlags 3 >>", widget))},
	}
	out := bytes.NewBuffer(slices.Clone(pdf))
	if !bytes.HasSuffix(pdf, []byte("\n")) {
		out.WriteString("\n")
	}
	offsets := map[int]int{}
	for _, o := range objects {
		offsets[o.n] = out.Len()
		fmt.Fprintf(out, "%d 0 obj\n%s\nendobj\n", o.n, o.dict)
	}
	xref := out.Len()
	out.WriteString("xref\n0 1\n0000000000 65535 f \n")
	for _, n := range slices.Sorted(func(yield func(int) bool) {
		for n := range offsets {
			if !yield(n) {
				return
			}
		}
	}) {
		fmt.Fprintf(out, "%d 1\n%010d 00000 n \n", n, offsets[n])
	}
	info := ""
	if infoM != nil {
		info = " /Info " + infoM[1] + " 0 R"
	}
	id := ""
	if idM := regexp.MustCompile(`/ID\s*\[[^\]]*\]`).FindString(trailer); idM != "" {
		id = " " + idM
	}
	fmt.Fprintf(out, "trailer\n<< /Size %d /Root %d 0 R%s%s /Prev %s >>\nstartxref\n%d\n%%%%EOF\n", widget+1, root, info, id, prev, xref)
	doc := out.Bytes()

	// what is signed: everything but the signature's own <...>
	from := bytes.Index(doc[offsets[sig]:], []byte("/Contents <")) + offsets[sig] + len("/Contents ")
	to := from + 2 + 2*sigSpace
	ranges := fmt.Sprintf("/ByteRange [0 %d %d %d]", from, to, len(doc)-to)
	if len(ranges) > len(placeholder) {
		return nil, errors.New("ddreport: sign: the PDF is too large")
	}
	at0 := bytes.Index(doc[offsets[sig]:], []byte(placeholder)) + offsets[sig]
	copy(doc[at0:], ranges+strings.Repeat(" ", len(placeholder)-len(ranges)))
	digest := sha256.New()
	digest.Write(doc[:from])
	digest.Write(doc[to:])
	cms, err := signedData(digest.Sum(nil), certs, key, at)
	if err != nil {
		return nil, err
	}
	if len(cms) > sigSpace {
		return nil, errors.New("ddreport: sign: the certificate chain is too long for the space kept for it")
	}
	hex.Encode(doc[from+1:], cms)
	return doc, nil
}

// DER writing: a tag, its length, its content.
func tlv(tag byte, parts ...[]byte) []byte {
	content := slices.Concat(parts...)
	n := len(content)
	if n < 128 {
		return slices.Concat([]byte{tag, byte(n)}, content)
	}
	var l []byte
	for ; n > 0; n >>= 8 {
		l = append([]byte{byte(n)}, l...)
	}
	return slices.Concat([]byte{tag, 0x80 | byte(len(l))}, l, content)
}

func der(v any) []byte {
	b, err := asn1.Marshal(v)
	if err != nil {
		panic(err) // only fixed values of known types are written
	}
	return b
}

var (
	oidData          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	oidSignedData    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidSHA256        = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidContentType   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}
	oidMessageDigest = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
	oidSigningTime   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 5}
	oidRSA           = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	oidECDSASHA256   = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}
)

// signedData is a CMS SignedData (RFC 5652) over a digest of content kept
// elsewhere: the signer's certificates, and its signature of the signed
// attributes (content type, the digest, the signing time).
func signedData(digest []byte, certs []*x509.Certificate, key crypto.Signer, at time.Time) ([]byte, error) {
	const seq, set, octets = 0x30, 0x31, 0x04
	null := []byte{0x05, 0x00}
	algorithm := tlv(seq, der(oidSHA256), null)
	attrs := [][]byte{
		tlv(seq, der(oidContentType), tlv(set, der(oidData))),
		tlv(seq, der(oidMessageDigest), tlv(set, tlv(octets, digest))),
		tlv(seq, der(oidSigningTime), tlv(set, der(at.UTC()))),
	}
	slices.SortFunc(attrs, bytes.Compare) // a DER SET OF is in order
	h := sha256.Sum256(tlv(set, attrs...))
	signature, err := key.Sign(rand.Reader, h[:], crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("ddreport: sign: %w", err)
	}
	signatureAlgorithm := tlv(seq, der(oidRSA), null)
	if _, ok := key.(*ecdsa.PrivateKey); ok {
		signatureAlgorithm = tlv(seq, der(oidECDSASHA256))
	}
	signerInfo := tlv(seq,
		der(1),
		tlv(seq, certs[0].RawIssuer, der(certs[0].SerialNumber)),
		algorithm,
		tlv(0xA0, attrs...), // [0] IMPLICIT signed attributes
		signatureAlgorithm,
		tlv(octets, signature))
	var raw [][]byte
	for _, c := range certs {
		raw = append(raw, c.Raw)
	}
	body := tlv(seq,
		der(1),
		tlv(set, algorithm),
		tlv(seq, der(oidData)),
		tlv(0xA0, raw...), // [0] IMPLICIT certificates
		tlv(set, signerInfo))
	return tlv(seq, der(oidSignedData), tlv(0xA0, body)), nil
}
