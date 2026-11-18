// Command aipctl is the administrative CLI: migrations, bootstrap, seed, replay.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/queue"
	"github.com/gjovanovicst/ai-engineering-platform/migrations"
)

var version = "dev"

const usage = `aipctl — AI Engineering Platform CLI

Usage:
  aipctl <command> [flags]

Commands:
  version     Print the version
  migrate     Apply migrations (goose; E1-S3)
  bootstrap   Create the first organization and owner account (E1-S3)
  seed        Insert demo data (E1-S5)
  replay      Replay a recorded agent run (E3-S0a)
  retention   Archive old run events and purge expired auth rows (E10-S2)
  tasks       List project tasks (API; AIP_API_URL/ORG_ID/TOKEN)
  start       Start a run for a task ref
  run         Show a run
  approve     Approve/reject an approval
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	switch cmd := os.Args[1]; cmd {
	case "version":
		fmt.Println("aipctl", version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	case "migrate":
		runMigrate(os.Args[2:])
	case "retention":
		runRetention()
	case "tasks":
		runTasks(os.Args[2:])
	case "start":
		runStart(os.Args[2:])
	case "run":
		runShow(os.Args[2:])
	case "approve":
		runApprove(os.Args[2:])
	case "seed":
		runSeed()
	case "bootstrap", "replay":
		fmt.Fprintf(os.Stderr, "aipctl %s: not implemented yet (see docs/05-mvp-plan.md)\n", cmd)
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
}

func runMigrate(args []string) {
	command := "up"
	if len(args) > 0 {
		command = args[0]
	}

	dsn := os.Getenv("AIP_DB_MIGRATOR_DSN")
	if dsn == "" {
		dsn = os.Getenv("AIP_DB_DSN")
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "AIP_DB_MIGRATOR_DSN (or AIP_DB_DSN) is required")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := db.Migrate(ctx, dsn, migrations.FS, command); err != nil {
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", command, err)
		os.Exit(1)
	}
	if command == "up" {
		pool, err := db.Connect(ctx, dsn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "queue migrate: %v\n", err)
			os.Exit(1)
		}
		defer pool.Close()
		if err := queue.Migrate(ctx, pool); err != nil {
			fmt.Fprintf(os.Stderr, "queue migrate: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("migrate %s: ok\n", command)
}
