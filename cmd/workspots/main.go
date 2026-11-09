// Command workspots serves the site (the default) or applies migrations.
//
//	workspots            # serve on $ADDR
//	workspots migrate    # apply migrations/ using $MIGRATE_DATABASE_URL, then exit
//	workspots grant-role <email> <role>   # make the first admin, using $MIGRATE_DATABASE_URL or $DATABASE_URL
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rydncn/workspots/internal/accounts"
	"github.com/rydncn/workspots/internal/config"
	"github.com/rydncn/workspots/internal/mail"
	"github.com/rydncn/workspots/internal/migrate"
	"github.com/rydncn/workspots/internal/obs"
	"github.com/rydncn/workspots/internal/web"
	"github.com/rydncn/workspots/migrations"
)

func main() {
	if err := run(os.Args[1:], os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, "workspots:", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	level := slog.LevelInfo
	if getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	log := obs.NewLogger(os.Stdout, level)

	if len(args) > 0 && args[0] == "migrate" {
		url := getenv("MIGRATE_DATABASE_URL")
		if url == "" {
			return errors.New("MIGRATE_DATABASE_URL is not set")
		}
		conn, err := pgx.Connect(ctx, url)
		if err != nil {
			return fmt.Errorf("connect for migrations: %w", err)
		}
		defer conn.Close(ctx)
		applied, err := migrate.Apply(ctx, conn, migrations.FS)
		for _, name := range applied {
			log.Info("migration applied", "name", name)
		}
		return err
	}

	if len(args) > 0 && args[0] == "grant-role" {
		if len(args) != 3 {
			return errors.New("usage: workspots grant-role <email> <role>")
		}
		url := getenv("MIGRATE_DATABASE_URL")
		if url == "" {
			url = getenv("DATABASE_URL")
		}
		if url == "" {
			return errors.New("MIGRATE_DATABASE_URL or DATABASE_URL must be set")
		}
		pool, err := pgxpool.New(ctx, url)
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		defer pool.Close()
		id, err := accounts.GrantRole(ctx, pool, args[1], args[2])
		if err != nil {
			return err
		}
		log.Info("role changed", "user_id", id, "role", args[2])
		return nil
	}

	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return errors.New("DATABASE_URL could not be parsed")
	}
	poolCfg.ConnConfig.Tracer = obs.QueryTracer{Logger: log}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("database pool: %w", err)
	}
	defer pool.Close()

	// Mail (ADR 0009): the dev outbox on stderr, separate from the structured
	// log on stdout; in production, until a provider is chosen, a log of the kind.
	var sender mail.Sender = &mail.Outbox{W: os.Stderr}
	if cfg.Production() {
		sender = mail.Logging{Logger: log}
	}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           web.New(web.Deps{DB: pool, Config: cfg, Logger: log, Mail: sender}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "addr", cfg.Addr, "env", cfg.Env)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}
