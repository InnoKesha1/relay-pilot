package pilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type ApplyFunc func(context.Context, Node, []byte) error

func SSHApply(i Inventory) ApplyFunc {
	return func(ctx context.Context, n Node, config []byte) error {
		// Neither credentials nor JSON appear in the process argument list.
		cmd := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "ConnectTimeout=8", "-o", "UserKnownHostsFile="+i.SSHKnownHosts, "-i", i.SSHIdentity, "-p", strconv.Itoa(n.SSHPort), n.SSHUser+"@"+n.SSHHost, "sudo -n /usr/local/sbin/relaypilot-node apply")
		cmd.Stdin = bytes.NewReader(config)
		// Do not expose remote parser errors: they can contain a credential or URL.
		if err := cmd.Run(); err != nil {
			return errors.New("node apply failed; inspect restricted node status")
		}
		return nil
	}
}

func Sync(ctx context.Context, s *Store, i Inventory, apply ApplyFunc) error {
	users, generation, deadline, err := s.Snapshot(ctx)
	if err != nil {
		return errors.New("cannot read active participants")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for role, node := range map[string]Node{"entry-a": i.EntryA, "entry-b": i.EntryB, "exit": i.Exit} {
		config, err := Server(i, role, users)
		if err != nil {
			return err
		}
		config, err = json.Marshal(M{"generation": generation, "valid_until": deadline, "config": json.RawMessage(config)})
		if err != nil {
			return err
		}
		wg.Add(1)
		go func(role string, n Node, b []byte) {
			defer wg.Done()
			c, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			if err := apply(c, n, b); err != nil {
				errs <- fmt.Errorf("%s: application not confirmed", role)
			}
		}(role, node, config)
	}
	wg.Wait()
	close(errs)
	var all []error
	for err := range errs {
		all = append(all, err)
	}
	return errors.Join(all...)
}
