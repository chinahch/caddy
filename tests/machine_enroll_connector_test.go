package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnrollmentRefusesMatchingConnectorBeforeClaim(t *testing.T) {
	t.Setenv("CADDY_ENROLLMENT_CODE", strings.Repeat("g", 43))
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer srv.Close()
	args, cfg, _ := enrollmentArgs(t, srv.URL)
	connector := filepath.Join(filepath.Dir(cfg), "connect.json")
	os.WriteFile(connector, []byte(`{"panel_url":"`+srv.URL+`","machine_id":7,"token_env":"LUMA_NODE_TOKEN"}`), 0600)
	if err := runMachineEnroll(args); err == nil || !strings.Contains(err.Error(), "connector") {
		t.Fatal("matching connector was not refused before credential rotation")
	}
	if calls != 0 {
		t.Fatal("connector credential revoked before migration")
	}
}
