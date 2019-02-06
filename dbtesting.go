package dbtesting

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	dsnEnvVar             = "DBTESTING_DSN"
	defaultSetUpTimeout   = 10 * time.Second
	defaultCleanUpTimeout = 3 * time.Second
	defaultLogPrefix      = "dbtesting"
)

type T struct {
	*testing.T
	Tx *sql.Tx
}

type Config struct {
	ConnectFunc    func() (*sql.DB, error)
	SkipFunc       func() bool
	SetUpFunc      func(context.Context, *sql.DB) error
	CleanUpFunc    func(context.Context, *sql.DB) error
	SetUpTimeout   time.Duration
	CleanUpTimeout time.Duration
	Logger         interface {
		Printf(format string, v ...interface{})
	}
}

var state = struct {
	Skip bool
	DB   *sql.DB
}{}

func RunTests(m *testing.M, cfg Config) int {
	if !flag.Parsed() {
		// we might rely on flags having been parsed, and this is idempotent anyway
		flag.Parse()
	}

	if cfg.SetUpTimeout == 0 {
		cfg.SetUpTimeout = defaultSetUpTimeout
	}
	if cfg.CleanUpTimeout == 0 {
		cfg.CleanUpTimeout = defaultCleanUpTimeout
	}
	if cfg.ConnectFunc == nil {
		cfg.ConnectFunc = defaultConnect
	}
	if cfg.SetUpFunc == nil {
		cfg.SetUpFunc = defaultSetUp
	}
	if cfg.SkipFunc == nil {
		cfg.SkipFunc = testing.Short
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(os.Stderr, defaultLogPrefix, log.LstdFlags)
	}

	return runTests(m, cfg)
}

func Inject(f func(*T)) func(t *testing.T) {
	return func(t *testing.T) {
		if state.Skip {
			t.Skip()
		}

		tx, err := state.DB.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatalf("db.BeginTX: %v", err)
		}
		defer func() {
			if p := recover(); p != nil {
				if err := tx.Rollback(); err != nil {
					t.Logf("tx.Rollback during panic: %v", err)
				}
				panic(p)
			}
			if err := tx.Rollback(); err != nil {
				t.Logf("tx.Rollback on test complete: %v", err)
			}
		}()
		f(&T{t, tx})
	}
}

func SQL(query string) func(context.Context, *sql.DB) error {
	return func(ctx context.Context, db *sql.DB) error {
		_, err := db.ExecContext(ctx, query)
		return err
	}
}

func runTests(m interface{ Run() int }, cfg Config) int {
	if state.Skip = cfg.SkipFunc(); state.Skip {
		return m.Run()
	}

	db, err := cfg.ConnectFunc()
	if err != nil {
		log.Printf("unable to connect: %v", err)
		return 1
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("db.Close: %v", err)
		}
	}()

	ctx, cncl := context.WithTimeout(context.Background(), cfg.SetUpTimeout)
	defer cncl()

	if err := db.PingContext(ctx); err != nil {
		log.Printf("db.PingContext: %v", err)
		return 1
	}

	if err := cfg.SetUpFunc(ctx, db); err != nil {
		log.Printf("SetUpFunc: %v", err)
		return 1
	}

	state.DB = db

	defer func() {
		ctx, cncl := context.WithTimeout(context.Background(), cfg.SetUpTimeout)
		defer cncl()
		if err := cfg.CleanUpFunc(ctx, db); err != nil {
			log.Printf("CleanUpFunc: %v", err)
		}
	}()

	return m.Run()
}

func defaultConnect() (*sql.DB, error) {
	dsn, ok := os.LookupEnv(dsnEnvVar)
	if !ok {
		return nil, fmt.Errorf("expected environment variable: %v", dsnEnvVar)
	}

	parts := strings.SplitN(dsn, ":", 2)
	if len(parts) != 2 {
		return nil, errors.New(`expected GOTESTING_URL="DRIVER:DSN_INFORMATION"`)
	}

	return sql.Open(parts[0], parts[1])
}

func defaultSetUp(context.Context, *sql.DB) error {
	return nil
}

func defaultCleanUp(context.Context, *sql.DB) error {
	return nil
}

func defaultSkip() bool {
	if !flag.Parsed() {
		flag.Parse()
	}
	return testing.Short()
}
