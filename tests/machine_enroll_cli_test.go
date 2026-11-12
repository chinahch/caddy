package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func enrollmentArgs(t *testing.T, panel string) ([]string, string, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yml")
	creds := filepath.Join(dir, "credentials.env")
	return []string{"--panel-url", panel, "--machine-id", "7", "--config", cfg, "--credentials", creds, "--meta", filepath.Join(dir, "install-meta.json"), "--install-root", dir}, cfg, creds
}
func TestEnrollmentPersistsCandidateBeforeRequestAndRetries(t *testing.T) {
	code := strings.Repeat("a", 43)
	t.Setenv("CADDY_ENROLLMENT_CODE", code)
	var first string
	calls := 0
	var state string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/server/enroll" || r.Method != "POST" {
			t.Errorf("wrong enrollment route: %s", r.URL)
			w.WriteHeader(400)
			return
		}
		var body struct {
			MachineID   int `json:"machine_id"`
			Code, Token string
		}
		json.NewDecoder(r.Body).Decode(&body)
		calls++
		if body.Code != code || body.MachineID != 7 || len(body.Token) != 43 {
			t.Error("invalid enrollment request")
		}
		raw, e := os.ReadFile(state)
		if e != nil || !strings.Contains(string(raw), body.Token) || strings.Contains(string(raw), code) {
			t.Error("candidate not privately persisted before request")
		}
		info, _ := os.Stat(state)
		if runtime.GOOS != "windows" && info != nil && info.Mode().Perm()&0077 != 0 {
			t.Error("state is not private")
		}
		if first == "" {
			first = body.Token
			h, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("no hijack")
			}
			c, _, _ := h.Hijack()
			c.Close()
			return
		}
		if body.Token != first {
			t.Error("retry changed candidate")
		}
		fmt.Fprint(w, `{"ok":true,"machine_id":7}`)
	}))
	defer srv.Close()
	args, cfg, creds := enrollmentArgs(t, srv.URL)
	state = filepath.Join(filepath.Dir(cfg), ".machine-enrollment.json")
	if err := runMachineEnroll(args); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("claims=%d", calls)
	}
	raw, _ := os.ReadFile(creds)
	if !strings.Contains(string(raw), first) {
		t.Fatal("claimed token not installed")
	}
	raw, _ = os.ReadFile(cfg)
	if strings.Contains(string(raw), first) || !strings.Contains(string(raw), "machine_id: 7") {
		t.Fatal("invalid generated config")
	}
}

func TestEnrollmentRefreshesExpiredPendingCommandWithoutChangingCandidate(t *testing.T) {
	oldCode := strings.Repeat("a", 43)
	newCode := strings.Repeat("b", 43)
	t.Setenv("CADDY_ENROLLMENT_CODE", oldCode)
	candidate := ""
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Code, Token string }
		json.NewDecoder(r.Body).Decode(&body)
		calls++
		if candidate == "" {
			candidate = body.Token
		}
		if candidate != body.Token {
			t.Error("replacement command changed pending candidate")
		}
		if body.Code != newCode {
			w.WriteHeader(401)
			return
		}
		fmt.Fprint(w, `{"ok":true,"machine_id":7}`)
	}))
	defer srv.Close()
	args, _, creds := enrollmentArgs(t, srv.URL)
	if err := runMachineEnroll(args); err == nil {
		t.Fatal("expired command accepted")
	}
	t.Setenv("CADDY_ENROLLMENT_CODE", newCode)
	if err := runMachineEnroll(args); err != nil {
		t.Fatal("new command cannot recover pending enrollment:", err)
	}
	raw, _ := os.ReadFile(creds)
	if !strings.Contains(string(raw), candidate) || calls != 2 {
		t.Fatal("pending candidate not safely recovered")
	}
}
func TestEnrollmentFailurePreservesConfigAndReusesCandidate(t *testing.T) {
	code := strings.Repeat("b", 43)
	t.Setenv("CADDY_ENROLLMENT_CODE", code)
	allow := false
	candidate := ""
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var b struct{ Token string }
		json.NewDecoder(r.Body).Decode(&b)
		if candidate == "" {
			candidate = b.Token
		}
		if b.Token != candidate {
			t.Error("process retry changed candidate")
		}
		if !allow {
			w.WriteHeader(401)
			fmt.Fprint(w, code+candidate)
			return
		}
		fmt.Fprint(w, `{"ok":true,"machine_id":7}`)
	}))
	defer srv.Close()
	args, cfg, creds := enrollmentArgs(t, srv.URL)
	original := "instances:\n  - panel:\n      url: " + srv.URL + "\n    machine:\n      machine_id: 7\n      token_env: PRIVATE_TOKEN\n    kernel:\n      custom_config_xray: /private/keep.json\n"
	os.WriteFile(cfg, []byte(original), 0600)
	os.WriteFile(creds, []byte("PRIVATE_TOKEN=old\nOTHER=keep\n"), 0600)
	err := runMachineEnroll(args)
	if err == nil || strings.Contains(err.Error(), code) || strings.Contains(err.Error(), candidate) {
		t.Fatal("claim failure missing or leaked secret")
	}
	raw, _ := os.ReadFile(cfg)
	if string(raw) != original {
		t.Fatal("failed claim changed config")
	}
	raw, _ = os.ReadFile(creds)
	if string(raw) != "PRIVATE_TOKEN=old\nOTHER=keep\n" {
		t.Fatal("failed claim changed credentials")
	}
	allow = true
	if err = runMachineEnroll(args); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(cfg)
	if string(raw) != original {
		t.Fatal("reconnect rewrote config")
	}
	raw, _ = os.ReadFile(creds)
	if !strings.Contains(string(raw), candidate) || !strings.Contains(string(raw), "OTHER=keep") {
		t.Fatal("reconnect lost credentials")
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
	candidate = ""
	t.Setenv("CADDY_ENROLLMENT_CODE", strings.Repeat("c", 43))
	if err = runMachineEnroll(args); err != nil {
		t.Fatal("replacement enrollment command should work:", err)
	}
}
func TestEnrollmentRejectsUnsafeExistingIdentityBeforeClaim(t *testing.T) {
	t.Setenv("CADDY_ENROLLMENT_CODE", strings.Repeat("d", 43))
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(500) }))
	defer srv.Close()
	for _, extra := range []string{"inline", "shared", "id", "panel", "symlink"} {
		t.Run(extra, func(t *testing.T) {
			args, cfg, creds := enrollmentArgs(t, srv.URL)
			id := 7
			panel := srv.URL
			if extra == "id" {
				id = 8
			}
			if extra == "panel" {
				panel = "http://other.invalid"
			}
			source := fmt.Sprintf("instances:\n  - panel:\n      url: %s\n    machine:\n      machine_id: %d\n      token_env: PRIVATE_TOKEN\n", panel, id)
			if extra == "inline" {
				source += "      token: overrides-env\n"
			}
			if extra == "shared" {
				source += "  - panel:\n      url: http://other.invalid\n      token_env: PRIVATE_TOKEN\n      node_id: 9\n"
			}
			os.WriteFile(cfg, []byte(source), 0600)
			os.WriteFile(creds, []byte("PRIVATE_TOKEN=old\n"), 0600)
			if extra == "symlink" {
				os.Remove(creds)
				if e := os.Symlink(cfg, creds); e != nil {
					t.Skip("symlink unsupported")
				}
			}
			if err := runMachineEnroll(args); err == nil {
				t.Fatal("unsafe binding accepted")
			}
			if calls != 0 {
				t.Fatal("unsafe binding claimed before validation")
			}
		})
	}
}
func TestEnrollmentRejectsPendingIdentityChangesAndRedirects(t *testing.T) {
	t.Setenv("CADDY_ENROLLMENT_CODE", strings.Repeat("e", 43))
	leaked := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer srv.Close()
	args, cfg, _ := enrollmentArgs(t, srv.URL)
	if err := runMachineEnroll(args); err == nil {
		t.Fatal("redirect accepted")
	}
	if leaked {
		t.Fatal("redirect leaked capability")
	}
	t.Setenv("CADDY_ENROLLMENT_CODE", strings.Repeat("f", 43))
	if err := runMachineEnroll(args); err == nil {
		t.Fatal("pending code identity changed")
	}
	args[3] = "8"
	if err := runMachineEnroll(args); err == nil {
		t.Fatal("pending machine identity changed")
	}
	raw, _ := os.ReadFile(filepath.Join(filepath.Dir(cfg), ".machine-enrollment.json"))
	hash := sha256.Sum256([]byte(strings.Repeat("f", 43)))
	if !strings.Contains(string(raw), hex.EncodeToString(hash[:])) {
		t.Fatal("renewed request capability not recorded")
	}
}
func TestEnrollmentRetriesTruncatedSuccess(t *testing.T) {
	t.Setenv("CADDY_ENROLLMENT_CODE", strings.Repeat("h", 43))
	calls := 0
	candidate := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var b struct{ Token string }
		json.NewDecoder(r.Body).Decode(&b)
		if candidate == "" {
			candidate = b.Token
		}
		if b.Token != candidate {
			t.Error("response retry changed candidate")
		}
		if calls == 1 {
			fmt.Fprint(w, `{"ok":`)
			return
		}
		fmt.Fprint(w, `{"ok":true,"machine_id":7}`)
	}))
	defer srv.Close()
	args, _, _ := enrollmentArgs(t, srv.URL)
	if err := runMachineEnroll(args); err != nil {
		t.Fatal("truncated successful response must retry with same candidate:", err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}
