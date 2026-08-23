package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged"
	"google.golang.org/grpc"
)

const shutdownTimeout = 5 * time.Second

var version = "dev"

type repeatedValue []string

func (v *repeatedValue) String() string { return fmt.Sprint([]string(*v)) }
func (v *repeatedValue) Set(value string) error {
	*v = append(*v, value)
	return nil
}

type invocation struct {
	showVersion   bool
	interfaces    repeatedValue
	namespaces    repeatedValue
	workloads     repeatedValue
	addressRanges repeatedValue
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "guardian-edge-privd: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	parsed, err := parseInvocation(args)
	if err != nil {
		return err
	}
	if parsed.showVersion {
		_, err := fmt.Fprintln(stdout, version)
		return err
	}
	if os.Geteuid() != 0 {
		return errors.New("helper must run as root")
	}
	edgeIdentity, err := user.Lookup("guardian-edge")
	if err != nil {
		return errors.New("guardian-edge identity is unavailable")
	}
	uid, err := strconv.ParseUint(edgeIdentity.Uid, 10, 32)
	if err != nil {
		return errors.New("guardian-edge UID is invalid")
	}
	gid, err := strconv.ParseUint(edgeIdentity.Gid, 10, 32)
	if err != nil {
		return errors.New("guardian-edge GID is invalid")
	}
	allowlist, err := privileged.CompileAllowlist(privileged.AllowlistInput{
		Interfaces:    parsed.interfaces,
		Namespaces:    parsed.namespaces,
		Workloads:     parsed.workloads,
		AddressRanges: parsed.addressRanges,
	})
	if err != nil {
		return fmt.Errorf("compile privileged allowlist: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	recorder := privileged.NewSlogAuditRecorder(logger)
	listener, err := privileged.ListenUnixSocket(privileged.SocketOptions{
		Path:          privileged.DefaultSocketPath,
		OwnerUID:      0,
		GroupGID:      uint32(gid),
		DirectoryMode: 0o750,
		SocketMode:    0o660,
	})
	if err != nil {
		return err
	}
	defer listener.Close()

	server, err := privileged.NewServer(privileged.ServerConfig{
		PeerPolicy: privileged.PeerPolicy{
			AllowedUID: uint32(uid),
			AllowedGID: uint32(gid),
			Verifier:   privileged.ProcProcessVerifier{},
		},
		Allowlist: allowlist,
		Adapter:   privileged.UnsupportedAdapter{},
		Audit:     recorder,
	})
	if err != nil {
		return err
	}
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()
	logger.Info("privileged helper started", "socket", privileged.DefaultSocketPath, "api_version", privileged.APIVersion)

	select {
	case err := <-serveResult:
		if err == nil || errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return fmt.Errorf("serve privileged helper: %w", err)
	case <-ctx.Done():
		stopped := make(chan struct{})
		go func() {
			server.GracefulStop()
			close(stopped)
		}()
		timer := time.NewTimer(shutdownTimeout)
		defer timer.Stop()
		select {
		case <-stopped:
		case <-timer.C:
			server.Stop()
			<-stopped
		}
		if err := <-serveResult; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("stop privileged helper: %w", err)
		}
		logger.Info("privileged helper stopped")
		return nil
	}
}

func parseInvocation(args []string) (invocation, error) {
	var parsed invocation
	flags := flag.NewFlagSet("guardian-edge-privd", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&parsed.showVersion, "version", false, "print version")
	flags.Var(&parsed.interfaces, "allow-interface", "allow one exact Linux interface name")
	flags.Var(&parsed.namespaces, "allow-namespace", "allow one exact Guardian network namespace")
	flags.Var(&parsed.workloads, "allow-workload", "allow one exact Guardian workload ID")
	flags.Var(&parsed.addressRanges, "allow-address-range", "allow one canonical IP prefix")
	if err := flags.Parse(args); err != nil {
		return invocation{}, fmt.Errorf("parse options: %w", err)
	}
	if flags.NArg() != 0 {
		return invocation{}, errors.New("positional arguments are forbidden")
	}
	if parsed.showVersion && len(args) != 1 {
		return invocation{}, errors.New("--version cannot be combined with operation policy")
	}
	return parsed, nil
}
