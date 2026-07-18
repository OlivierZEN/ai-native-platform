package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/OlivierZEN/ai-native-platform/internal/api"
	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/cli"
	mcpserver "github.com/OlivierZEN/ai-native-platform/internal/mcp"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, in io.Reader, out, diagnostics io.Writer) int {
	invoker := capability.NewInvoker(capability.NewRegistry(capability.SystemCapabilityDefinitions()), 32)
	if len(args) >= 2 && args[0] == "mcp" && args[1] == "stdio" && len(args) == 2 {
		if err := mcpserver.RunStdio(ctx, invoker); err != nil && !errors.Is(err, io.EOF) {
			_, _ = fmt.Fprintf(diagnostics, "mcp stdio terminated: %v\n", err)
			return 1
		}
		return 0
	}
	if len(args) >= 1 && args[0] == "serve" {
		return serve(ctx, args[1:], invoker, out)
	}
	return cli.Run(ctx, invoker, args, in, out, diagnostics)
}

func serve(ctx context.Context, args []string, invoker *capability.Invoker, out io.Writer) int {
	listen := "127.0.0.1:8080"
	if len(args) == 2 && args[0] == "--listen" && args[1] != "" {
		listen = args[1]
	} else if len(args) != 0 {
		_ = json.NewEncoder(out).Encode(map[string]any{"status": "failed", "error": map[string]string{"code": "VALIDATION_FAILED", "message": "expected: serve [--listen <address>]"}})
		return 1
	}

	server := &http.Server{Addr: listen, Handler: api.NewHandler(invoker)}
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		_ = json.NewEncoder(out).Encode(map[string]any{"status": "failed", "error": map[string]string{"code": "INTERNAL", "message": err.Error()}})
		return 1
	}
	return 0
}
