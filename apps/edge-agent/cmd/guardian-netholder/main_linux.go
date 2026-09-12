//go:build linux

// Command guardian-netholder owns one decoy's network namespace (ADR 0019).
//
// It is started by the privileged helper as the only process of a holder
// container, with CAP_NET_ADMIN over that namespace and nothing else, and it
// runs until the container is stopped. It takes no arguments and opens no
// sockets but netlink.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/netholder"
)

const convergeInterval = 200 * time.Millisecond

func main() {
	if len(os.Args) != 1 {
		// Nothing it could be told on a command line is something it should
		// believe.
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err := netholder.Run(ctx, convergeInterval); err != nil && !errors.Is(err, context.Canceled) {
		os.Exit(1)
	}
}
