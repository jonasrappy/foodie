// Foodie runs the household API and its maintenance commands without Node.js.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/jonasrappy/foodie/internal/auth"
	"github.com/jonasrappy/foodie/internal/config"
	"github.com/jonasrappy/foodie/internal/store"
	"github.com/jonasrappy/foodie/internal/web"
)

var version = "dev"

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("foodie stopped", "error", err)
		os.Exit(1)
	}
}
func run(args []string, logger *slog.Logger) error {
	command := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	configPath := flags.String("config", env("FOODIE_ENV_FILE", ".env"), "private .env file (or legacy JSON config)")
	dataDir := flags.String("data-dir", env("FOODIE_DATA_DIR", "./data"), "SQLite data directory")
	publicDir := flags.String("public-dir", env("FOODIE_PUBLIC_DIR", "./public"), "frontend assets directory")
	listen := flags.String("listen", env("FOODIE_LISTEN", "127.0.0.1:8082"), "loopback HTTP address")
	destination := flags.String("output", "", "backup destination; default: daily backup")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if command == "init" {
		target := *configPath
		if *destination != "" {
			target = *destination
		}
		if err := config.Generate(target); err != nil {
			return err
		}
		fmt.Printf("Created %s with private permissions. Read FOODIE_PASSWORD and FOODIE_BOT_TOKEN locally; do not commit this file.\n", target)
		return nil
	}
	settings, err := config.ReadEnvironment(*configPath)
	if err != nil {
		return err
	}
	supplied := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { supplied[f.Name] = true })
	if !supplied["data-dir"] {
		*dataDir = config.Value(settings, "FOODIE_DATA_DIR", *dataDir)
	}
	if !supplied["public-dir"] {
		*publicDir = config.Value(settings, "FOODIE_PUBLIC_DIR", *publicDir)
	}
	if !supplied["listen"] {
		*listen = config.Value(settings, "FOODIE_LISTEN", *listen)
	}
	switch command {
	case "version":
		fmt.Printf("foodie %s (%s)\n", version, runtime.Version())
		return nil
	case "backup":
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		daily := *destination == ""
		if daily {
			*destination = filepath.Join(*dataDir, "backups", "mad-"+time.Now().UTC().Format("2006-01-02")+".sqlite")
		}
		if err := store.Backup(ctx, filepath.Join(*dataDir, "mad.sqlite"), *destination); err != nil {
			return err
		}
		if daily {
			if err := store.PruneBackups(filepath.Dir(*destination), 14); err != nil {
				return err
			}
		}
		logger.Info("backup complete", "path", *destination)
		return nil
	case "change-password":
		for _, key := range []string{"FOODIE_PASSWORD", "FOODIE_PASSWORD_HASH", "FOODIE_PASSWORD_SALT", "FOODIE_SESSION_SECRET", "FOODIE_BOT_TOKEN", "FOODIE_BOT_TOKEN_HASH"} {
			if _, ok := os.LookupEnv(key); ok {
				return errors.New("credentials are overridden by process environment; update your environment provider instead of the file")
			}
		}
		c, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(os.Stdin, 4097))
		if err != nil {
			return err
		}
		if len(data) > 4096 {
			return errors.New("password too long")
		}
		password := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
		if length := auth.TextLength(password); length < 8 || length > 200 {
			return errors.New("password must contain 8 to 200 characters")
		}
		salt := make([]byte, 32)
		if _, err = rand.Read(salt); err != nil {
			return err
		}
		c.PasswordSalt = hex.EncodeToString(salt)
		if c.PasswordHash, err = auth.HashPassword(password, c.PasswordSalt); err != nil {
			return err
		}
		if err = config.Save(*configPath, c); err != nil {
			return err
		}
		logger.Info("password changed; restart Foodie to invalidate device sessions")
		return nil
	case "serve":
	default:
		return fmt.Errorf("unknown command %q; use init, serve, backup, change-password or version", command)
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("HTTP must listen on a loopback IP; use nginx for public access")
	}
	c, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	db, err := store.Open(filepath.Join(*dataDir, "mad.sqlite"))
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	signals, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	requestContext, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	server := &http.Server{Handler: web.New(db, c, *publicDir, logger), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 8192, BaseContext: func(net.Listener) context.Context { return requestContext }}
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	logger.Info("foodie listening", "address", listener.Addr().String(), "version", version)
	select {
	case err = <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-signals.Done():
		logger.Info("shutdown started")
		cancelRequests() // Ends SSE and rolls back any canceled database operations.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err = server.Shutdown(ctx); err != nil {
			_ = server.Close()
			return err
		}
		err = <-result
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		logger.Info("shutdown complete")
		return nil
	}
}
