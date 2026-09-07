package pilot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, e := Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func testInventory() Inventory {
	return Inventory{PublicURL: "https://relay.example", ProbeURL: "https://relay.example/generate_204", DNSAddress: "1.1.1.1", EntryA: Node{Address: "192.0.2.10", Port: 443}, EntryB: Node{Address: "192.0.2.11", Port: 443, ServerName: "b.example", Certificate: "cert.pem", Key: "key.pem"}, Exit: Node{Address: "192.0.2.12", Port: 8443, ServerName: "exit.example", Certificate: "cert.pem", Key: "key.pem"}, RealityPublicKey: strings.Repeat("A", 43), RealityPrivateKey: strings.Repeat("B", 43), RealityShortID: "aabbccdd", RealityServerName: "relay.example", RealityHandshakePort: 443, Lab: true}
}

func TestCredentialLifecycle(t *testing.T) {
	s := testStore(t)
	now := time.Now()
	u, token, e := s.Add("owner", now.Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	api := &API{Store: s, Inventory: testInventory(), Now: func() time.Time { return now }}
	get := func(token string) int {
		r := httptest.NewRecorder()
		api.ServeHTTP(r, httptest.NewRequest("GET", "https://relay.example/sub/"+token, nil))
		if strings.Contains(r.Body.String(), token) {
			t.Error("token leaked in response")
		}
		if r.Header().Get("Cache-Control") != "no-store" {
			t.Error("secret response cacheable")
		}
		return r.Code
	}
	if get(token) != 200 {
		t.Fatal("fresh subscription unavailable")
	}
	newToken, e := s.Rotate(u.ID)
	if e != nil {
		t.Fatal(e)
	}
	if get(token) != 404 || get(newToken) != 200 {
		t.Fatal("rotation did not invalidate old URL")
	}
	newUser, e := s.ByToken(newToken, now)
	if e != nil {
		t.Fatal(e)
	}
	if newUser.EntryUUID == u.EntryUUID || newUser.ExitUUID == u.ExitUUID || newUser.HysteriaPassword == u.HysteriaPassword {
		t.Fatal("rotation retained proxy credentials")
	}
	if e = s.Revoke(u.ID); e != nil {
		t.Fatal(e)
	}
	if get(newToken) != 404 {
		t.Fatal("revoked subscription works")
	}
	active, _ := s.Users(true, now)
	if len(active) != 0 {
		t.Fatal("revoked user rendered to nodes")
	}
	_, token, e = s.Add("expires", now.Add(time.Minute))
	if e != nil {
		t.Fatal(e)
	}
	api.Now = func() time.Time { return now.Add(time.Minute) }
	if get(token) != 404 {
		t.Fatal("expiry boundary is not enforced")
	}
}

func TestConcurrentPilotLimit(t *testing.T) {
	s := testStore(t)
	var wg sync.WaitGroup
	for range 30 {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, _ = s.Add("test", time.Now().Add(time.Hour)) }()
	}
	wg.Wait()
	u, e := s.Users(true, time.Now())
	if e != nil || len(u) != 10 {
		t.Fatalf("limit failed: %d, %v", len(u), e)
	}
}

func TestBackupIncludesWALAndSecretsAreNotListed(t *testing.T) {
	s := testStore(t)
	u, tok, e := s.Add("owner", time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(u)
	for _, secret := range []string{tok, u.EntryUUID, u.ExitUUID, u.HysteriaPassword} {
		if strings.Contains(string(b), secret) {
			t.Fatal("list output contains a secret")
		}
	}
	path := filepath.Join(t.TempDir(), "backup.db")
	if e = s.Backup(path); e != nil {
		t.Fatal(e)
	}
	restored, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	if _, e = restored.ByToken(tok, time.Now()); e != nil {
		t.Fatal("backup lost committed user")
	}
	if e = s.Backup(path); e == nil {
		t.Fatal("backup overwrote an existing file")
	}
}

func TestTopologyAndSyncFailures(t *testing.T) {
	s := testStore(t)
	u, _, _ := s.Add("owner", time.Now().Add(time.Hour))
	i := testInventory()
	data, e := Client(i, u)
	if e != nil {
		t.Fatal(e)
	}
	var c M
	json.Unmarshal(data, &c)
	if strings.Contains(string(data), `"type": "direct"`) {
		t.Fatal("client can fall back to direct")
	}
	dns := c["dns"].(map[string]any)["servers"].([]any)[0].(map[string]any)
	if dns["detour"] != "relay-select" {
		t.Fatal("DNS bypasses tunnel")
	}
	out := c["outbounds"].([]any)
	if out[2].(map[string]any)["detour"] != "entry-a" || out[3].(map[string]any)["detour"] != "entry-b" {
		t.Fatal("two-hop chains missing")
	}
	var mu sync.Mutex
	applied := map[string][]byte{}
	e = Sync(context.Background(), s, i, func(_ context.Context, n Node, b []byte) error {
		mu.Lock()
		defer mu.Unlock()
		applied[n.Address] = b
		if n.Address == i.EntryB.Address {
			return errors.New("disconnected")
		}
		return nil
	})
	if e == nil || len(applied) != 3 {
		t.Fatal("partial node failure was reported as success")
	}
	if e = s.Revoke(u.ID); e != nil {
		t.Fatal(e)
	}
	e = Sync(context.Background(), s, i, func(_ context.Context, n Node, b []byte) error {
		if strings.Contains(string(b), u.EntryUUID) || strings.Contains(string(b), u.ExitUUID) || strings.Contains(string(b), u.HysteriaPassword) {
			t.Error("revoked credential remains on node")
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
}

// Emit the production profile contract for Dart tests when explicitly requested.
func TestExportContract(t *testing.T) {
	path := os.Getenv("RELAY_CONTRACT_OUT")
	if path == "" {
		t.Skip("contract export disabled")
	}
	u := User{EntryUUID: "11111111-1111-4111-8111-111111111111", ExitUUID: "22222222-2222-4222-8222-222222222222", HysteriaPassword: strings.Repeat("C", 43)}
	b, e := Client(testInventory(), u)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
