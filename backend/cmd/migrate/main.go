// migrate is a small dev CLI over the embedded migrations:
//
//	go run ./cmd/migrate up        # apply all
//	go run ./cmd/migrate down N    # roll back N steps
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/tsdlamongan/whcms/backend/internal/platform/db"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://root:postgres@localhost:5432/whmcs_e2e?sslmode=disable"
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: migrate up | migrate down <steps>")
	}
	switch args[0] {
	case "up":
		if err := db.Migrate(url); err != nil {
			return err
		}
		fmt.Println("migrations applied")
		return nil
	case "down":
		steps := 1
		if len(args) > 1 {
			n, err := strconv.Atoi(args[1])
			if err != nil || n < 1 {
				return fmt.Errorf("invalid steps %q", args[1])
			}
			steps = n
		}
		if err := db.MigrateDown(url, steps); err != nil {
			return err
		}
		fmt.Printf("rolled back %d step(s)\n", steps)
		return nil
	default:
		return fmt.Errorf("unknown command %q (use up|down)", args[0])
	}
}
