package main

import "testing"

func TestParseRoleMap(t *testing.T) {
	m, err := parseRoleMap("aip-admins:admin, aip-viewers:viewer")
	if err != nil || m["aip-admins"] != "admin" || m["aip-viewers"] != "viewer" {
		t.Fatalf("m=%v err=%v", m, err)
	}
	if _, err := parseRoleMap("broken"); err == nil {
		t.Fatal("entry without colon must fail")
	}
	if _, err := parseRoleMap("g:owner"); err == nil {
		t.Fatal("owner mapping must fail")
	}
}
