package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/chinahch/caddy/connector/internal/agent"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

func main() {
	if e := run(); e != nil {
		slog.Error(e.Error())
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 3 || os.Args[1] != "--config" {
		return errors.New("usage: luma-connect --config /etc/caddy/connect.json")
	}
	if runtime.GOOS != "linux" {
		return errors.New("connector supervisor deployment requires Linux")
	}
	data, e := os.ReadFile(os.Args[2])
	if e != nil {
		return errors.New("cannot read worker configuration")
	}
	var cfg agent.Config
	if e = json.Unmarshal(data, &cfg); e != nil {
		return errors.New("invalid worker JSON")
	}
	if cfg.BinDir == "" {
		cfg.BinDir = "/usr/local/lib/caddy-connect"
	}
	if cfg.StateDir == "" {
		cfg.StateDir = "/var/lib/caddy-connect"
	}
	if !filepath.IsAbs(cfg.BinDir) || !filepath.IsAbs(cfg.StateDir) {
		return errors.New("worker binary and state directories must be absolute paths")
	}
	if cfg.TokenEnv == "" {
		cfg.TokenEnv = "LUMA_NODE_TOKEN"
	}
	client, e := agent.NewClient(cfg, os.Getenv(cfg.TokenEnv))
	if e != nil {
		return e
	}
	if e = os.MkdirAll(cfg.StateDir, 0700); e != nil {
		return e
	}
	release, e := agent.LockState(cfg.StateDir)
	if e != nil {
		return e
	}
	defer release()
	queue, e := agent.OpenSpool(filepath.Join(cfg.StateDir, "queue"))
	if e != nil {
		return fmt.Errorf("cannot initialize traffic queue: %w", e)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return agent.NewSupervisor(cfg.BinDir, cfg.StateDir, queue).Run(ctx, client)
}
