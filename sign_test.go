package ddreport

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"regexp"
	"strconv"
	"testing"
	"time"
)

// testIdentity is a self-signed certificate and its key, as PEM.
func testIdentity(t *testing.T, key crypto.Signer) (certPEM, keyPEM []byte) {
	t.Helper()
	tpl := &x509.Certificate{SerialNumber: big.NewInt(42), Subject: pkix.Name{CommonName: "PT Kopi Nusantara"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	raw, err := x509.CreateCertificate(rand.Reader, tpl, tpl, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	k, _ := x509.MarshalPKCS8PrivateKey(key)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: k})
}

// checkSignature verifies a signed PDF the way a reader does: the byte range
// covers everything but the signature, the digest in the signed attributes is
// that of those bytes, and the signature of the attributes is the key's.
func checkSignature(t *testing.T, signed []byte, cert *x509.Certificate) {
	t.Helper()
	m := regexp.MustCompile(`/ByteRange \[0 (\d+) (\d+) (\d+)\]\s*/Contents <([0-9a-fA-F]+)>`).FindSubmatch(signed)
	if m == nil {
		t.Fatal("no signature dictionary")
	}
	a, _ := strconv.Atoi(string(m[1]))
	b, _ := strconv.Atoi(string(m[2]))
	c, _ := strconv.Atoi(string(m[3]))
	if signed[a] != '<' || signed[b-1] != '>' || b+c != len(signed) {
		t.Fatalf("byte range [0 %d %d %d] does not cover the file around the signature", a, b, c)
	}
	h := sha256.New()
	h.Write(signed[:a])
	h.Write(signed[b:])
	digest := h.Sum(nil)

	der, _ := hex.DecodeString(string(m[4]))
	var contentInfo struct {
		Type    asn1.ObjectIdentifier
		Content asn1.RawValue `asn1:"explicit,tag:0"`
	}
	if _, err := asn1.Unmarshal(der, &contentInfo); err != nil || !contentInfo.Type.Equal(oidSignedData) {
		t.Fatalf("not CMS signed data: %v", err)
	}
	var sd struct {
		Version          int
		DigestAlgorithms asn1.RawValue
		EncapContent     asn1.RawValue
		Certificates     asn1.RawValue `asn1:"tag:0"`
		SignerInfos      []struct {
			Version            int
			Sid                asn1.RawValue
			DigestAlgorithm    asn1.RawValue
			SignedAttrs        asn1.RawValue `asn1:"tag:0"`
			SignatureAlgorithm asn1.RawValue
			Signature          []byte
		} `asn1:"set"`
	}
	if _, err := asn1.Unmarshal(contentInfo.Content.Bytes, &sd); err != nil || len(sd.SignerInfos) != 1 {
		t.Fatalf("signed data: %v", err)
	}
	si := sd.SignerInfos[0]
	if !bytes.Contains(si.SignedAttrs.Bytes, digest) {
		t.Error("the signed message digest is not the digest of the byte range")
	}
	attrs := sha256.Sum256(tlv(0x31, si.SignedAttrs.Bytes)) // signed as a SET OF
	switch pub := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, attrs[:], si.Signature); err != nil {
			t.Errorf("RSA signature: %v", err)
		}
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(pub, attrs[:], si.Signature) {
			t.Error("ECDSA signature does not verify")
		}
	}
	if !bytes.Contains(sd.Certificates.FullBytes, cert.Raw) {
		t.Error("the certificate is not in the signature")
	}
}

func TestSignPDF(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	for _, tpl := range []string{
		`{"bands":{"detail":{"height":20,"elements":[{"type":"text","w":200,"h":14,"text":"{{name}}"}]}}}`,
		`{"pdfA":true,"bands":{"detail":{"height":20,"elements":[{"type":"text","w":200,"h":14,"text":"{{name}}"}]}}}`,
	} {
		pdf := render(t, []byte(tpl), []map[string]any{{"name": "Kopi"}}, "pdf")
		for _, key := range []crypto.Signer{rsaKey, ecKey} {
			certPEM, keyPEM := testIdentity(t, key)
			signed, err := SignPDF(pdf, certPEM, keyPEM, "Faktur asli")
			if err != nil {
				t.Fatal(err)
			}
			block, _ := pem.Decode(certPEM)
			cert, _ := x509.ParseCertificate(block.Bytes)
			checkSignature(t, signed, cert)
			if !bytes.HasPrefix(signed, pdf) {
				t.Error("an incremental update keeps the original bytes as they were")
			}
			for _, want := range []string{"/SubFilter /adbe.pkcs7.detached", "/Name (PT Kopi Nusantara)", "/Reason (Faktur asli)", "/AcroForm << /Fields [", "/SigFlags 3", "/Annots ["} {
				if !bytes.Contains(signed, []byte(want)) {
					t.Errorf("missing %s", want)
				}
			}
			// the new trailer points back to the old cross-reference table, and keeps the ID of a PDF/A file
			prev := regexp.MustCompile(`startxref\n(\d+)\n%%EOF\n$`).FindSubmatch(pdf)
			if !bytes.Contains(signed, []byte("/Prev "+string(prev[1])+" >>")) {
				t.Error("no /Prev to the original cross-reference table")
			}
			if bytes.Contains(pdf, []byte("/ID [")) && bytes.Count(signed, []byte("/ID [")) != 2 {
				t.Error("the update drops the document ID")
			}
		}
	}
	// a key that is not the certificate's, and files that are not PEM
	certPEM, _ := testIdentity(t, rsaKey)
	_, otherKey := testIdentity(t, ecKey)
	pdf := render(t, []byte(`{"bands":{"detail":{"height":20,"elements":[{"type":"text","w":20,"h":14,"text":"x"}]}}}`), []any{1}, "pdf")
	if _, err := SignPDF(pdf, certPEM, otherKey, ""); err == nil {
		t.Error("signed with a key that is not the certificate's")
	}
	if _, err := SignPDF(pdf, []byte("nothing"), otherKey, ""); err == nil {
		t.Error("signed without a certificate")
	}
	if _, err := SignPDF([]byte("not a pdf"), certPEM, otherKey, ""); err == nil {
		t.Error("signed what is not a PDF")
	}
}
