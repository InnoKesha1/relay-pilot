package pilot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type API struct {
	Store       *Store
	Inventory   Inventory
	Requests    atomic.Uint64
	Now         func() time.Time
	LastSync    atomic.Int64
	SyncErrors  atomic.Uint64
	MonitorSync bool
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	switch r.URL.Path {
	case "/healthz":
		if a.MonitorSync && time.Now().Unix()-a.LastSync.Load() > 45 {
			http.Error(w, "nodes unavailable", 503)
			return
		}
		if err := a.Store.DB.PingContext(r.Context()); err != nil {
			http.Error(w, "unavailable", 503)
			return
		}
		w.WriteHeader(204)
		return
	case "/metrics":
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "relaypilot_last_successful_sync_seconds %d\nrelaypilot_sync_errors_total %d\n", a.LastSync.Load(), a.SyncErrors.Load())
		return
	case "/generate_204":
		w.WriteHeader(204)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/sub/") {
		http.NotFound(w, r)
		return
	}
	a.Requests.Add(1)
	token := strings.TrimPrefix(r.URL.Path, "/sub/")
	now := time.Now()
	if a.Now != nil {
		now = a.Now()
	}
	u, err := a.Store.ByToken(token, now)
	if err != nil {
		http.Error(w, "access unavailable", 404)
		return
	}
	b, err := Client(a.Inventory, u)
	if err != nil {
		http.Error(w, "profile unavailable", 503)
		return
	}
	sum := sha256.Sum256(b)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-RelayPilot-Profile", "1")
	w.Header().Set("Profile-Title", "Relay Pilot")
	w.Header().Set("Profile-Update-Interval", "24")
	w.Header().Set("Subscription-Userinfo", fmt.Sprintf("upload=0; download=0; total=0; expire=%d", u.Expires))
	w.Header().Set("ETag", `"`+hex.EncodeToString(sum[:])+`"`)
	w.WriteHeader(200)
	_, _ = w.Write(b)
}
