//go:build integration

package pilot

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type labProcess struct {
	cmd  *exec.Cmd
	log  bytes.Buffer
	done chan error
}

func (p *labProcess) stop() {
	if p == nil || p.cmd.Process == nil {
		return
	}
	select {
	case <-p.done:
		return
	default:
	}
	_ = p.cmd.Process.Kill()
	<-p.done
}
func labPort(t *testing.T) int {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// Real sing-box processes, real TLS/Reality/Hysteria2 and an owned loopback probe.
// This verifies transport behaviour; device TUN and mobile-network QA are separate.
func TestNetworkChainsAndFailover(t *testing.T) {
	bin := os.Getenv("SING_BOX_BIN")
	if bin == "" {
		t.Fatal("SING_BOX_BIN must name sing-box 1.13.0")
	}
	dir := t.TempDir()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub := &key.PublicKey
	certTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, e := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, pub, key)
	if e != nil {
		t.Fatal(e)
	}
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	os.WriteFile(certPath, certPEM, 0600)
	os.WriteFile(keyPath, keyPEM, 0600)
	var originMu sync.Mutex
	var dnsOrigins []string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dns-query" {
			b, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
			if len(b) < 17 {
				http.Error(w, "bad DNS", 400)
				return
			}
			end := 12
			for end < len(b) && b[end] != 0 {
				end += int(b[end]) + 1
			}
			end += 5
			if end > len(b) {
				http.Error(w, "bad DNS", 400)
				return
			}
			answer := append([]byte{}, b[:end]...)
			answer[2] = 0x81
			answer[3] = 0x80
			for k := 6; k < 12; k++ {
				answer[k] = 0
			}
			if binary.BigEndian.Uint16(b[end-4:end-2]) == 1 {
				answer[7] = 1
				answer = append(answer, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 30, 0, 4, 127, 0, 0, 1)
			}
			// Reality's own localhost bootstrap is control traffic; track the
			// injected user query specifically, not that server-side lookup.
			if bytes.Contains(b, []byte("probe")) {
				originMu.Lock()
				dnsOrigins = append(dnsOrigins, r.RemoteAddr)
				originMu.Unlock()
			}
			w.Header().Set("Content-Type", "application/dns-message")
			w.Write(answer)
			return
		}
		if r.URL.Path == "/generate_204" {
			w.WriteHeader(204)
			return
		}
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		fmt.Fprint(w, host)
	})
	plainListener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	plainPort := plainListener.Addr().(*net.TCPAddr).Port
	plainServer := &http.Server{Handler: handler}
	go plainServer.Serve(plainListener)
	defer plainServer.Close()
	tlsListener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	tlsPort := tlsListener.Addr().(*net.TCPAddr).Port
	tlsServer := &http.Server{Handler: handler, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13}}
	go tlsServer.ServeTLS(tlsListener, certPath, keyPath)
	defer tlsServer.Close()
	realityKey, _ := ecdh.X25519().GenerateKey(rand.Reader)
	i := testInventory()
	i.RealityServerName = "localhost"
	i.RealityHandshakePort = tlsPort
	i.RealityPublicKey = base64.RawURLEncoding.EncodeToString(realityKey.PublicKey().Bytes())
	i.RealityPrivateKey = base64.RawURLEncoding.EncodeToString(realityKey.Bytes())
	i.EntryA.Address = "127.0.0.2"
	i.EntryA.Port = labPort(t)
	i.EntryB.Address = "127.0.0.3"
	i.EntryB.Port = labPort(t)
	i.Exit.Address = "127.0.0.4"
	i.Exit.Port = labPort(t)
	for _, n := range []*Node{&i.EntryB, &i.Exit} {
		n.ServerName = "localhost"
		n.Certificate = certPath
		n.Key = keyPath
		n.CA = string(certPEM)
	}
	s := testStore(t)
	u, _, e := s.Add("lab", time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	doh := func(detour string) M {
		m := M{"type": "https", "tag": "relay-dns", "server": "127.0.0.1", "server_port": tlsPort, "path": "/dns-query", "tls": M{"enabled": true, "server_name": "localhost", "certificate": strings.Split(strings.TrimSpace(string(certPEM)), "\n")}}
		if detour != "" {
			m["detour"] = detour
		}
		return m
	}
	processes := map[string]*labProcess{}
	defer func() {
		for name, p := range processes {
			p.stop()
			if t.Failed() {
				t.Log(name, p.log.String())
			}
		}
	}()
	start := func(name string, c M) {
		c["log"] = M{"level": "debug"}
		if old := processes[name]; old != nil {
			old.stop()
		}
		b, _ := encode(c)
		path := filepath.Join(dir, name+".json")
		os.WriteFile(path, b, 0600)
		if out, e := exec.Command(bin, "check", "-c", path).CombinedOutput(); e != nil {
			t.Fatalf("%s config rejected: %s", name, out)
		}
		p := &labProcess{done: make(chan error, 1)}
		p.cmd = exec.Command(bin, "run", "-c", path)
		p.cmd.Stdout = &p.log
		p.cmd.Stderr = &p.log
		if e := p.cmd.Start(); e != nil {
			t.Fatal(e)
		}
		processes[name] = p
		go func() { p.done <- p.cmd.Wait() }()
		time.Sleep(350 * time.Millisecond)
	}
	server := func(role string, n Node, users []User) {
		b, e := Server(i, role, users)
		if e != nil {
			t.Fatal(e)
		}
		var c M
		json.Unmarshal(b, &c)

		c["inbounds"].([]any)[0].(map[string]any)["listen"] = n.Address
		c["outbounds"].([]any)[0].(map[string]any)["inet4_bind_address"] = n.Address
		c["dns"] = M{"servers": []M{doh("")}, "final": "relay-dns"}
		c["route"].(map[string]any)["default_domain_resolver"] = "relay-dns"
		start(role, c)
	}
	server("exit", i.Exit, []User{u})
	server("entry-a", i.EntryA, []User{u})
	server("entry-b", i.EntryB, []User{u})
	b, _ := Client(i, u)
	var client M
	json.Unmarshal(b, &client)
	proxyPort := labPort(t)
	apiPort := labPort(t)
	dnsPort := labPort(t)
	client["inbounds"] = []M{{"type": "mixed", "tag": "lab-proxy", "listen": "127.0.0.1", "listen_port": proxyPort}, {"type": "direct", "tag": "lab-dns-in", "listen": "127.0.0.1", "listen_port": dnsPort, "network": "udp"}}
	client["experimental"] = M{"clash_api": M{"external_controller": fmt.Sprintf("127.0.0.1:%d", apiPort), "secret": "isolated-lab"}}
	client["dns"] = M{"servers": []M{doh("relay-select")}, "final": "relay-dns"}
	client["outbounds"].([]any)[1].(map[string]any)["url"] = fmt.Sprintf("http://127.0.0.1:%d/generate_204", plainPort)
	start("client", client)
	apiURL := fmt.Sprintf("http://127.0.0.1:%d", apiPort)
	selectRoute := func(name string) {
		r, _ := http.NewRequest("PUT", apiURL+"/proxies/relay-select", strings.NewReader(`{"name":"`+name+`"}`))
		r.Header.Set("Authorization", "Bearer isolated-lab")
		resp, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		resp.Body.Close()
		if resp.StatusCode != 204 {
			t.Fatalf("select route: %d", resp.StatusCode)
		}
	}
	proxyURL, _ := url.Parse(fmt.Sprintf("socks5://127.0.0.1:%d", proxyPort))
	request := func(closeConnection bool) error {
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: closeConnection}
		defer transport.CloseIdleConnections()
		h := &http.Client{Transport: transport, Timeout: 5 * time.Second}
		resp, e := h.Get(fmt.Sprintf("http://127.0.0.1:%d/echo", plainPort))
		if e != nil {
			return e
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 || string(body) != i.Exit.Address {
			return fmt.Errorf("unexpected egress (status %d, body %s)", resp.StatusCode, body)
		}
		return nil
	}
	for _, route := range []string{"route-a", "route-b"} {
		selectRoute(route)
		if e := request(false); e != nil {
			t.Fatalf("%s keep-alive: %v", route, e)
		}
		for attempt := 0; attempt < 100; attempt++ {
			if e := request(true); e != nil {
				t.Fatalf("%s Connection: close request %d: %v", route, attempt+1, e)
			}
		}
		t.Log(route, "verified: exit egress, keep-alive and 100 Connection: close responses")
	}
	if os.Getenv("RELAY_LAB_CONNECTION_CLOSE") == "1" {
		return
	}
	// A DNS query injected into the client must reach DoH through the final exit.
	conn, e := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", dnsPort))
	if e != nil {
		t.Fatal(e)
	}
	conn.SetDeadline(time.Now().Add(6 * time.Second))
	q := []byte{0x12, 0x34, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 5, 'p', 'r', 'o', 'b', 'e', 4, 't', 'e', 's', 't', 0, 0, 1, 0, 1}
	conn.Write(q)
	buf := make([]byte, 512)
	n, e := conn.Read(buf)
	conn.Close()
	if e != nil || n < 12 || buf[7] != 1 {
		t.Fatalf("client DNS failed: %v", e)
	}
	originMu.Lock()
	origins := append([]string{}, dnsOrigins...)
	originMu.Unlock()
	if len(origins) == 0 {
		t.Fatal("no DoH request observed")
	}
	for _, origin := range origins {
		host, _, _ := net.SplitHostPort(origin)
		if host != i.Exit.Address {
			t.Fatalf("DNS escaped exit: %s", origin)
		}
	}
	t.Log("DNS verified through exit")
	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for range 5 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- request(false) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	selectRoute("relay-auto")
	// Each entry fails in turn. Losing the UDP listener also simulates unavailable Hysteria2.
	for _, role := range []string{"entry-a", "entry-b"} {
		processes[role].stop()
		delete(processes, role)
		began := time.Now()
		var last error
		for time.Since(began) < 60*time.Second {
			last = request(false)
			if last == nil {
				break
			}
			time.Sleep(time.Second)
		}
		if last != nil {
			t.Fatalf("failover after %s exceeded 60s: %v", role, last)
		}
		t.Logf("%s failure recovered in %s", role, time.Since(began).Round(time.Millisecond))
		if role == "entry-a" {
			server(role, i.EntryA, []User{u})
		} else {
			server(role, i.EntryB, []User{u})
		}
	}
	// A real exit outage is tested while credentials are still valid.
	processes["exit"].stop()
	delete(processes, "exit")
	for _, route := range []string{"route-a", "route-b", "relay-auto"} {
		selectRoute(route)
		if e := request(false); e == nil {
			t.Fatal("exit outage fell back to direct")
		}
	}
	server("exit", i.Exit, []User{u})
	selectRoute("route-a")
	if e := request(false); e != nil {
		t.Fatalf("exit did not recover: %v", e)
	}
	// Cached client credentials must fail after the server-side revoke is applied.
	s.Revoke(u.ID)
	active, _ := s.Users(true, time.Now())
	server("entry-a", i.EntryA, active)
	server("entry-b", i.EntryB, active)
	server("exit", i.Exit, active)
	for _, route := range []string{"route-a", "route-b"} {
		selectRoute(route)
		if e := request(false); e == nil {
			t.Fatal("revoked cached profile still works")
		}
	}
	processes["exit"].stop()
	delete(processes, "exit")
	selectRoute("relay-auto")
	if e := request(false); e == nil {
		t.Fatal("exit failure fell back to direct")
	}
	t.Log("revocation and exit failure fail closed")
}
