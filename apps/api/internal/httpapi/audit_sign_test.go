package httpapi

import "testing"

func TestAuditSignature(t *testing.T) {
	if auditSignature(nil, []byte("x")) != "" {
		t.Fatal("no key must mean no signature")
	}
	a := auditSignature([]byte("k"), []byte("body"))
	b := auditSignature([]byte("k"), []byte("body"))
	if a == "" || a != b {
		t.Fatalf("signature = %q / %q", a, b)
	}
	if auditSignature([]byte("k"), []byte("other")) == a {
		t.Fatal("body must affect the signature")
	}
}
