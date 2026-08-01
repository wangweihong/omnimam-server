package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wangweihong/omnimam/backend/pkg/mcpproxy"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	endpointDefault := os.Getenv("OMNIMAM_MCP_ENDPOINT")
	if endpointDefault == "" {
		endpointDefault = "http://127.0.0.1:8080/mcp"
	}
	flags := flag.NewFlagSet("omnimam-mcp-proxy", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	endpoint := flags.String("endpoint", endpointDefault, "OmniMAM MCP Streamable HTTP endpoint")
	timeout := flags.Duration("timeout", 30*time.Second, "per-request HTTP timeout")
	maxMessageBytes := flags.Int("max-message-bytes", 1<<20, "maximum stdio request/response bytes")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("omnimam-mcp-proxy accepts no positional arguments")
	}
	proxy, err := mcpproxy.New(mcpproxy.Config{
		Endpoint: *endpoint, Token: os.Getenv("OMNIMAM_MCP_TOKEN"),
		Timeout: *timeout, MaxMessageBytes: *maxMessageBytes,
	})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return proxy.Run(ctx, os.Stdin, os.Stdout)
}
