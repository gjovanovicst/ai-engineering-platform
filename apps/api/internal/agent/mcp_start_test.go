package agent

import "testing"

func TestParseMCPServers(t *testing.T) {
	servers, err := ParseMCPServers(`[{"name":"fs","command":"mcp-fs","args":["--root","/workspace"]}]`)
	if err != nil || len(servers) != 1 || servers[0].Name != "fs" {
		t.Fatalf("servers=%v err=%v", servers, err)
	}
	if _, err := ParseMCPServers(`[{"name":"fs"}]`); err == nil {
		t.Fatal("expected a validation error")
	}
	if _, err := ParseMCPServers(`not json`); err == nil {
		t.Fatal("expected a JSON error")
	}
}
