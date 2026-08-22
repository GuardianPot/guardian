package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
)

const fixtureCrashExitCode = 42

func main() {
	if len(os.Args) == 4 && os.Args[1] == "--w8-fixture" {
		if err := runWALFixture(os.Args[2], os.Args[3]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	fmt.Println("guardian edge-agent skeleton")
}

func runWALFixture(mode, dbPath string) error {
	ctx := context.Background()
	q, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer q.Close()

	const eventID = "w8-crash-fixture-event"
	switch mode {
	case "crash":
		if _, err := q.Enqueue(ctx, eventID, []byte("fixture durable payload")); err != nil {
			return fmt.Errorf("seed fixture event: %w", err)
		}
		event, ok, err := q.Claim(ctx, time.Now(), 100*time.Millisecond)
		if err != nil {
			return fmt.Errorf("claim fixture event: %w", err)
		}
		if !ok || event.ID != eventID || event.Attempts != 1 {
			return fmt.Errorf("unexpected crash claim: event=%+v available=%t", event, ok)
		}
		fmt.Printf("fixture claimed %s attempt=%d; simulating abrupt process death\n", event.ID, event.Attempts)
		os.Exit(fixtureCrashExitCode)
	case "recover":
		event, ok, err := q.Claim(ctx, time.Now(), time.Second)
		if err != nil {
			return fmt.Errorf("claim replayed fixture event: %w", err)
		}
		if !ok {
			return errors.New("replayed fixture event was not available")
		}
		if event.ID != eventID || event.Attempts != 2 {
			return fmt.Errorf("unexpected replayed event: %+v", event)
		}
		if err := q.Ack(ctx, event.ID, event.Attempts, time.Now()); err != nil {
			return fmt.Errorf("ack replayed fixture event: %w", err)
		}
		stats, err := q.Stats(ctx)
		if err != nil {
			return fmt.Errorf("read fixture stats: %w", err)
		}
		if stats.Delivered != 1 || stats.Pending != 0 || stats.Inflight != 0 {
			return fmt.Errorf("unexpected final fixture stats: %+v", stats)
		}
		fmt.Printf("fixture replayed %s attempt=%d and delivered exactly once\n", event.ID, event.Attempts)
		return nil
	default:
		return fmt.Errorf("unknown W8 fixture mode %q", mode)
	}
	return nil
}
