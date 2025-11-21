package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"goshop/services/users/config"
	"goshop/services/users/migrations"
)

func main() {
	dirFlag := flag.String("dir", "services/users/migrations", "path to migrations directory (used for create)")
	timeout := flag.Duration("timeout", 10*time.Second, "DB connect timeout")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: migrate [-dir path] <command> [args]\n")
		fmt.Fprintf(os.Stderr, "commands: up | down | status | up-to <v> | down-to <v> | redo | reset | version | create <name> sql\n")
		os.Exit(2)
	}
	cmd := flag.Arg(0)
	args := flag.Args()[1:]

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		cfg := config.New()
		dsn = cfg.Postgres.DSN()
	}

	db := mustOpenWithRetry(dsn, *timeout, 60, 2*time.Second)
	defer db.Close()

	goose.SetTableName("schema_migrations_users")
	if err := goose.SetDialect("postgres"); err != nil {
		fail("goose dialect", err)
	}
	goose.SetBaseFS(migrations.FS)

	ctx := context.Background()
	const lockKey = "users_migrations"
	if _, err := db.ExecContext(ctx, `SELECT pg_advisory_lock(1, hashtext($1))`, lockKey); err != nil {
		fail("advisory_lock", err)
	}
	defer db.ExecContext(ctx, `SELECT pg_advisory_unlock(1, hashtext($1))`, lockKey)

	start := time.Now()
	if err := gooseRun(db, ".", cmd, args, *dirFlag); err != nil {
		fail("goose "+cmd, err)
	}
	log.Printf("migrations: command=%s done in %s", cmd, time.Since(start).Round(time.Millisecond))
}

func gooseRun(db *sql.DB, dirInEmbed, cmd string, args []string, diskDir string) error {
	switch cmd {
	case "up":
		return goose.Up(db, dirInEmbed)
	case "down":
		return goose.Down(db, dirInEmbed)
	case "status":
		return goose.Status(db, dirInEmbed)
	case "redo":
		return goose.Redo(db, dirInEmbed)
	case "reset":
		return goose.Reset(db, dirInEmbed)
	case "version":
		return goose.Version(db, dirInEmbed)
	case "up-to":
		if len(args) != 1 {
			return fmt.Errorf("usage: up-to <version>")
		}
		return goose.UpTo(db, dirInEmbed, mustParseInt(args[0]))
	case "down-to":
		if len(args) != 1 {
			return fmt.Errorf("usage: down-to <version>")
		}
		return goose.DownTo(db, dirInEmbed, mustParseInt(args[0]))
	case "create":
		if len(args) != 2 {
			return fmt.Errorf("usage: create <name> <sql|go>")
		}
		return goose.Create(db, diskDir, args[0], args[1])
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func mustOpenWithRetry(dsn string, pingTimeout time.Duration, retries int, sleep time.Duration) *sql.DB {
	var db *sql.DB
	var err error
	ctx := context.Background()

	for i := 1; i <= retries; i++ {
		if db != nil {
			_ = db.Close()
		}
		db, err = sql.Open("pgx", dsn)
		if err == nil {
			cctx, cancel := context.WithTimeout(ctx, pingTimeout)
			err = db.PingContext(cctx)
			cancel()
		}
		if err == nil {
			return db
		}
		log.Printf("db not ready (try %d/%d): %v; sleep %s", i, retries, err, sleep)
		time.Sleep(sleep)
	}
	fail("open/ping db", err)
	return nil
}

func mustParseInt(s string) int64 {
	var v int64
	if _, err := fmt.Sscan(s, &v); err != nil {
		fail("parse version", err)
	}
	return v
}

func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "migrate: %s: %v\n", what, err)
	os.Exit(1)
}
