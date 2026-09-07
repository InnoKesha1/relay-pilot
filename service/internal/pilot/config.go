package pilot

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
)

const MaxUsers = 10
const SchemaVersion = 1

// Inventory contains deployment secrets. Keep it outside the repository, mode 0600.
type Inventory struct {
	PublicURL            string `json:"public_url"`
	ProbeURL             string `json:"probe_url"`
	DNSAddress           string `json:"dns_address"`
	EntryA               Node   `json:"entry_a"`
	EntryB               Node   `json:"entry_b"`
	Exit                 Node   `json:"exit"`
	RealityPublicKey     string `json:"reality_public_key"`
	RealityPrivateKey    string `json:"reality_private_key"`
	RealityShortID       string `json:"reality_short_id"`
	RealityServerName    string `json:"reality_server_name"`
	RealityHandshakePort int    `json:"reality_handshake_port"`
	SSHIdentity          string `json:"ssh_identity"`
	SSHKnownHosts        string `json:"ssh_known_hosts"`
	Lab                  bool   `json:"lab,omitempty"`
}

type Node struct {
	Address     string `json:"address"`
	Port        int    `json:"port"`
	ServerName  string `json:"server_name"`
	Certificate string `json:"certificate"`
	Key         string `json:"key"`
	SSHHost     string `json:"ssh_host"`
	SSHUser     string `json:"ssh_user"`
	SSHPort     int    `json:"ssh_port"`
	// A custom CA is for the isolated lab; production uses publicly trusted TLS.
	CA string `json:"ca,omitempty"`
}

var safeHost = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.:-]*$`)
var safeUser = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
var shortID = regexp.MustCompile(`^(?:[0-9a-fA-F]{2}){1,8}$`)

func LoadInventory(path string) (Inventory, error) {
	var i Inventory
	b, err := os.ReadFile(path)
	if err != nil {
		return i, err
	}
	if err = json.Unmarshal(b, &i); err != nil {
		return i, errors.New("invalid inventory JSON")
	}
	return i, i.Validate()
}

func (i Inventory) Validate() error {
	for name, raw := range map[string]string{"public_url": i.PublicURL, "probe_url": i.ProbeURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return fmt.Errorf("%s must be an HTTPS URL without credentials or fragment", name)
		}
		if name == "public_url" && (u.RawQuery != "" || (u.Path != "" && u.Path != "/")) {
			return errors.New("public_url must be an origin")
		}
	}
	if net.ParseIP(i.DNSAddress) == nil {
		return errors.New("dns_address must be an IP address (no bootstrap DNS)")
	}
	if i.EntryA.Address == i.EntryB.Address {
		return errors.New("entry nodes must have distinct IP addresses")
	}
	for name, n := range map[string]Node{"entry_a": i.EntryA, "entry_b": i.EntryB, "exit": i.Exit} {
		if net.ParseIP(n.Address) == nil || n.Port < 1 || n.Port > 65535 {
			return fmt.Errorf("%s needs a literal IP and valid port", name)
		}
		if n.Address == i.Exit.Address && name != "exit" {
			return errors.New("entry and exit must be distinct nodes")
		}
		if name != "entry_a" && (n.ServerName == "" || n.Certificate == "" || n.Key == "") {
			return fmt.Errorf("%s needs TLS name, certificate and key paths", name)
		}
		if !i.Lab && n.CA != "" {
			return errors.New("custom CAs are restricted to lab inventories")
		}
		if !i.Lab && (!safeHost.MatchString(n.SSHHost) || !safeUser.MatchString(n.SSHUser) || n.SSHPort < 1 || n.SSHPort > 65535) {
			return fmt.Errorf("invalid %s SSH destination", name)
		}
	}
	if i.RealityPublicKey == "" || i.RealityPrivateKey == "" || !shortID.MatchString(i.RealityShortID) || i.RealityServerName == "" || i.RealityHandshakePort < 1 || i.RealityHandshakePort > 65535 {
		return errors.New("incomplete Reality settings")
	}
	if !i.Lab && (i.SSHIdentity == "" || i.SSHKnownHosts == "") {
		return errors.New("SSH identity and verified known_hosts are required")
	}
	return nil
}
