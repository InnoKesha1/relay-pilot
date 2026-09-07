package pilot

import (
	"encoding/json"
	"errors"
	"strings"
)

type M = map[string]any

func tlsClient(n Node) M {
	t := M{"enabled": true, "server_name": n.ServerName}
	if n.CA != "" {
		t["certificate"] = strings.Split(strings.TrimSpace(n.CA), "\n")
	}
	return t
}
func encode(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }

// Client returns a complete sing-box configuration. Never feed it through a
// subscription converter: doing so can expose entry nodes as single-hop routes.
func Client(i Inventory, u User) ([]byte, error) {
	entryA := M{"type": "vless", "tag": "entry-a", "server": i.EntryA.Address, "server_port": i.EntryA.Port, "uuid": u.EntryUUID,
		"tls": M{"enabled": true, "server_name": i.RealityServerName, "utls": M{"enabled": true, "fingerprint": "chrome"}, "reality": M{"enabled": true, "public_key": i.RealityPublicKey, "short_id": i.RealityShortID}}}
	entryB := M{"type": "hysteria2", "tag": "entry-b", "server": i.EntryB.Address, "server_port": i.EntryB.Port, "password": u.HysteriaPassword, "tls": tlsClient(i.EntryB)}
	exit := func(tag, via string) M {
		// Keep the outer TLS/QUIC transport alive when an individual TCP stream
		// closes. Without smux, short HTTP responses over Hysteria2 can be lost.
		return M{"type": "vless", "tag": tag, "server": i.Exit.Address, "server_port": i.Exit.Port, "uuid": u.ExitUUID, "tls": tlsClient(i.Exit), "detour": via, "packet_encoding": "xudp", "multiplex": M{"enabled": true, "protocol": "smux"}}
	}
	return encode(M{
		"log":      M{"disabled": true},
		"dns":      M{"servers": []M{{"type": "https", "tag": "relay-dns", "server": i.DNSAddress, "server_port": 443, "path": "/dns-query", "tls": M{"enabled": true}, "detour": "relay-select"}}, "final": "relay-dns", "strategy": "prefer_ipv4"},
		"inbounds": []M{{"type": "tun", "tag": "relay-tun", "address": []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"}, "mtu": 1400, "auto_route": true, "strict_route": true, "stack": "mixed"}},
		"outbounds": []M{
			{"type": "selector", "tag": "relay-select", "outbounds": []string{"relay-auto", "route-a", "route-b"}, "default": "relay-auto", "interrupt_exist_connections": true},
			{"type": "urltest", "tag": "relay-auto", "outbounds": []string{"route-a", "route-b"}, "url": i.ProbeURL, "interval": "15s", "tolerance": 100, "idle_timeout": "24h", "interrupt_exist_connections": true},
			exit("route-a", "entry-a"), exit("route-b", "entry-b"), entryA, entryB,
		},
		"route": M{"rules": []M{{"action": "sniff"}, {"protocol": "dns", "action": "hijack-dns"}}, "final": "relay-select", "auto_detect_interface": true, "default_domain_resolver": "relay-dns"},
	})
}

func Server(i Inventory, role string, users []User) ([]byte, error) {
	var inbound M
	var rules []M
	tlsServer := func(n Node) M {
		return M{"enabled": true, "server_name": n.ServerName, "certificate_path": n.Certificate, "key_path": n.Key}
	}
	switch role {
	case "entry-a":
		members := []M{}
		for _, u := range users {
			members = append(members, M{"name": u.ID, "uuid": u.EntryUUID})
		}
		inbound = M{"type": "vless", "tag": "ingress", "listen": "0.0.0.0", "listen_port": i.EntryA.Port, "users": members,
			"tls": M{"enabled": true, "server_name": i.RealityServerName, "reality": M{"enabled": true, "handshake": M{"server": i.RealityServerName, "server_port": i.RealityHandshakePort}, "private_key": i.RealityPrivateKey, "short_id": []string{i.RealityShortID}}}}
	case "entry-b":
		members := []M{}
		for _, u := range users {
			members = append(members, M{"name": u.ID, "password": u.HysteriaPassword})
		}
		inbound = M{"type": "hysteria2", "tag": "ingress", "listen": "0.0.0.0", "listen_port": i.EntryB.Port, "users": members, "tls": tlsServer(i.EntryB)}
	case "exit":
		members := []M{}
		for _, u := range users {
			members = append(members, M{"name": u.ID, "uuid": u.ExitUUID})
		}
		inbound = M{"type": "vless", "tag": "ingress", "listen": "0.0.0.0", "listen_port": i.Exit.Port, "users": members, "tls": tlsServer(i.Exit), "multiplex": M{"enabled": true}}
	default:
		return nil, errors.New("unknown node role")
	}
	if role != "exit" {
		rules = []M{{"ip_cidr": []string{i.Exit.Address + cidrSuffix(i.Exit.Address)}, "port": []int{i.Exit.Port}, "action": "route", "outbound": "egress"}, {"action": "reject"}}
	} else if !i.Lab {
		rules = []M{{"ip_is_private": true, "action": "reject"}}
	} else {
		rules = []M{}
	}
	return encode(M{"log": M{"disabled": true}, "dns": M{"servers": []M{{"type": "https", "tag": "exit-dns", "server": i.DNSAddress, "tls": M{"enabled": true}}}, "final": "exit-dns"}, "inbounds": []M{inbound}, "outbounds": []M{{"type": "direct", "tag": "egress"}}, "route": M{"rules": rules, "final": "egress", "default_domain_resolver": "exit-dns"}})
}
func cidrSuffix(ip string) string {
	if strings.Contains(ip, ":") {
		return "/128"
	}
	return "/32"
}
