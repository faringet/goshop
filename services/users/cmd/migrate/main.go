package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"github.com/pressly/goose/v3"
	"goshop/services/users/config"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dir := flag.String("dir", "services/users/migrations", "path to migrations directory")
	timeout := flag.Duration("timeout", 10*time.Second, "DB connect timeout")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: migrate [-dir path] <command> [args]\n")
		fmt.Fprintf(os.Stderr, "commands: up | down | status | up-to <version> | down-to <version> | redo | reset | version | create <name> sql\n")
		os.Exit(2)
	}
	cmd := flag.Arg(0)
	args := flag.Args()[1:]

	cfg := config.NewConfig()

	dsn := cfg.Postgres.DSN()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fail("open db", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fail("ping db", err)
	}

	migrationsDir := *dir
	goose.SetTableName("schema_migrations_users")

	if err := gooseRun(db, migrationsDir, cmd, args...); err != nil {
		fail("goose "+cmd, err)
	}
}

func gooseRun(db *sql.DB, dir, cmd string, args ...string) error {
	switch cmd {
	case "up":
		return goose.Up(db, dir)
	case "down":
		return goose.Down(db, dir)
	case "status":
		return goose.Status(db, dir)
	case "redo":
		return goose.Redo(db, dir)
	case "reset":
		return goose.Reset(db, dir)
	case "version":
		return goose.Version(db, dir)
	case "up-to":
		if len(args) != 1 {
			return fmt.Errorf("usage: up-to <version>")
		}
		return goose.UpTo(db, dir, mustParseInt(args[0]))
	case "down-to":
		if len(args) != 1 {
			return fmt.Errorf("usage: down-to <version>")
		}
		return goose.DownTo(db, dir, mustParseInt(args[0]))
	case "create":
		if len(args) != 2 {
			return fmt.Errorf("usage: create <name> <sql|go>")
		}
		err := goose.Create(db, dir, args[0], args[1])
		return err
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func mustParseInt(s string) int64 {
	var v int64
	_, err := fmt.Sscan(s, &v)
	if err != nil {
		fail("parse version", err)
	}
	return v
}

func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "migrate: %s: %v\n", what, err)
	os.Exit(1)
}
