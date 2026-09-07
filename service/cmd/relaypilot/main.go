package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"relaypilot.local/service/internal/pilot"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "relaypilot:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	fs := flag.NewFlagSet("relaypilot", flag.ContinueOnError)
	db := fs.String("db", "/var/lib/relaypilot/pilot.db", "SQLite database")
	inv := fs.String("inventory", "/etc/relaypilot/inventory.json", "private inventory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()
	if len(args) == 0 {
		return errors.New("commands: init, serve, user add|list|rotate|revoke|expire, sync, render, backup")
	}
	s, err := pilot.Open(*db)
	if err != nil {
		return err
	}
	defer s.Close()
	if args[0] == "init" {
		fmt.Println("Database ready (no participants).")
		return nil
	}
	if args[0] == "backup" {
		if len(args) != 2 {
			return errors.New("backup PATH")
		}
		return s.Backup(args[1])
	}
	i, err := pilot.LoadInventory(*inv)
	if err != nil {
		return err
	}
	switch args[0] {
	case "user":
		if len(args) < 2 {
			return errors.New("user add|list|rotate|revoke|expire")
		}
		switch args[1] {
		case "add":
			if len(args) != 4 {
				return errors.New("user add NAME EXPIRES_RFC3339")
			}
			expiry, e := time.Parse(time.RFC3339, args[3])
			if e != nil {
				return errors.New("expiry must be RFC3339")
			}
			u, t, e := s.Add(args[2], expiry)
			if e != nil {
				return e
			}
			fmt.Println("ID:", u.ID)
			fmt.Println("Subscription (secret):", strings.TrimRight(i.PublicURL, "/")+"/sub/"+t)
		case "list":
			u, e := s.Users(false, time.Now())
			if e != nil {
				return e
			}
			return json.NewEncoder(os.Stdout).Encode(u)
		case "rotate":
			if len(args) != 3 {
				return errors.New("user rotate ID")
			}
			t, e := s.Rotate(args[2])
			if e != nil {
				return e
			}
			fmt.Println("Subscription (secret):", strings.TrimRight(i.PublicURL, "/")+"/sub/"+t)
		case "revoke":
			if len(args) != 3 {
				return errors.New("user revoke ID")
			}
			if e := s.Revoke(args[2]); e != nil {
				return e
			}
		case "expire":
			if len(args) != 4 {
				return errors.New("user expire ID EXPIRES_RFC3339")
			}
			expiry, e := time.Parse(time.RFC3339, args[3])
			if e != nil {
				return errors.New("expiry must be RFC3339")
			}
			if e = s.Expire(args[2], expiry); e != nil {
				return e
			}
		default:
			return errors.New("unknown user command")
		}
		fmt.Println("Database updated; synchronizing all three nodes.")
		if err := pilot.Sync(context.Background(), s, i, pilot.SSHApply(i)); err != nil {
			return fmt.Errorf("change saved, node propagation pending: %w", err)
		}
		fmt.Println("All nodes confirmed the current configuration.")
		return nil
	case "sync":
		return pilot.Sync(context.Background(), s, i, pilot.SSHApply(i))
	case "render":
		if len(args) != 2 {
			return errors.New("render OUTPUT_DIRECTORY")
		}
		if e := os.MkdirAll(args[1], 0700); e != nil {
			return e
		}
		users, e := s.Users(true, time.Now())
		if e != nil {
			return e
		}
		for _, role := range []string{"entry-a", "entry-b", "exit"} {
			b, e := pilot.Server(i, role, users)
			if e != nil {
				return e
			}
			if e = os.WriteFile(filepath.Join(args[1], role+".json"), b, 0600); e != nil {
				return e
			}
		}
		return nil
	case "serve":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		logger := log.New(os.Stderr, "relaypilot ", log.LstdFlags|log.LUTC)
		api := &pilot.API{Store: s, Inventory: i, MonitorSync: true}
		go func() {
			timer := time.NewTicker(15 * time.Second)
			defer timer.Stop()
			for {
				err := pilot.Sync(ctx, s, i, pilot.SSHApply(i))
				if err != nil {
					api.SyncErrors.Add(1)
					logger.Print("node sync incomplete")
				} else {
					api.LastSync.Store(time.Now().Unix())
				}
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				}
			}
		}()
		srv := &http.Server{Addr: "127.0.0.1:8090", Handler: api, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
		go func() {
			<-ctx.Done()
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdown)
		}()
		logger.Print("subscription service started on loopback")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	default:
		return errors.New("unknown command")
	}
}
