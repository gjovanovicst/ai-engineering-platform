package samlmeta

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "idp"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	cert := base64.StdEncoding.EncodeToString(der)
	raw := []byte(`<EntityDescriptor entityID="https://idp.example.com"><IDPSSODescriptor><SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/><KeyDescriptor use="signing"><KeyInfo><X509Data><X509Certificate>` + cert + `</X509Certificate></X509Data></KeyInfo></KeyDescriptor></IDPSSODescriptor></EntityDescriptor>`)
	meta, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if meta.EntityID != "https://idp.example.com" || !strings.Contains(meta.Certificate, "BEGIN CERTIFICATE") || meta.SSOURL != "https://idp.example.com/sso" {
		t.Fatalf("meta = %+v", meta)
	}
	if _, err := Parse([]byte(`<EntityDescriptor entityID="x"/>`)); err == nil {
		t.Fatal("missing certificate must fail")
	}
}

func TestCheckURLBlocksPrivate(t *testing.T) {
	if err := CheckURL("http://127.0.0.1/metadata", false); err == nil {
		t.Fatal("loopback must be blocked")
	}
	if err := CheckURL("https://8.8.8.8/metadata", false); err != nil {
		t.Fatalf("public IP should pass: %v", err)
	}
	if err := CheckURL("ftp://example.com/x", false); err == nil {
		t.Fatal("non-http scheme must be blocked")
	}
	if err := CheckURL("http://127.0.0.1/metadata", true); err != nil {
		t.Fatalf("allowPrivate should pass: %v", err)
	}
}
