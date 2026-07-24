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
	"github.com/OlivierZEN/ai-native-platform/internal/authorization"
	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/cli"
	"github.com/OlivierZEN/ai-native-platform/internal/config"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/OlivierZEN/ai-native-platform/internal/database/migrate"
	"github.com/OlivierZEN/ai-native-platform/internal/identity"
	mcpserver "github.com/OlivierZEN/ai-native-platform/internal/mcp"
	"github.com/OlivierZEN/ai-native-platform/internal/metadata"
	"github.com/OlivierZEN/ai-native-platform/internal/metering"
	"github.com/OlivierZEN/ai-native-platform/internal/observability"
	"github.com/OlivierZEN/ai-native-platform/internal/operations"
	internalprovisioning "github.com/OlivierZEN/ai-native-platform/internal/provisioning"
	"github.com/OlivierZEN/ai-native-platform/internal/record"
	"github.com/OlivierZEN/ai-native-platform/internal/tenant"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, in io.Reader, out, diagnostics io.Writer) int {
	cfg, err := config.LoadEnv()
	if err != nil {
		return writeStartupFailure(out, capability.CodeValidationFailed, err.Error())
	}
	logger, err := observability.New(cfg.Log, diagnostics)
	if err != nil {
		return writeStartupFailure(out, capability.CodeValidationFailed, err.Error())
	}
	if len(args) == 2 && args[0] == "db" && args[1] == "migrate" {
		pool, err := database.OpenPool(ctx, cfg.Database)
		if err != nil {
			return writeStartupFailure(out, capability.CodeInternal, err.Error())
		}
		defer pool.Close()
		if err := migrate.Apply(ctx, pool, migrate.Builtin()); err != nil {
			logger.Error("database migration failed", "error", err)
			return writeStartupFailure(out, capability.CodeInternal, "database migration failed")
		}
		_ = json.NewEncoder(out).Encode(map[string]string{"status": "succeeded"})
		return 0
	}
	definitions := capability.SystemCapabilityDefinitions()
	var poolsToClose []interface{ Close() }
	var tenantService *tenant.Service
	var usageService *metering.Service
	if cfg.ControlDatabaseURL != "" {
		controlDatabase := cfg.Database
		controlDatabase.URL = cfg.ControlDatabaseURL
		controlPool, err := database.OpenPool(ctx, controlDatabase)
		if err != nil {
			return writeStartupFailure(out, capability.CodeInternal, "control database unavailable")
		}
		poolsToClose = append(poolsToClose, controlPool)
		runtimeDatabase := cfg.Database
		runtimeDatabase.URL = cfg.RuntimeDatabaseURL
		runtimePool, err := database.OpenPool(ctx, runtimeDatabase)
		if err != nil {
			controlPool.Close()
			return writeStartupFailure(out, capability.CodeInternal, "runtime database unavailable")
		}
		poolsToClose = append(poolsToClose, runtimePool)
		tenantService = tenant.NewService(controlPool, operations.ClaimBoundPort{})
		// Company provisioning is intentionally not a public capability. All new
		// tenants enter through the AgentCiCi-validated internal route below.
		definitions = append(definitions, tenant.CapabilityDefinitions(tenantService, false)...)
		metadataService := metadata.NewService(runtimePool, controlPool)
		definitions = append(definitions, metadata.CapabilityDefinitions(metadataService)...)
		usageService = metering.NewService(runtimePool, controlPool)
		definitions = append(definitions, metering.CapabilityDefinitions(usageService)...)
		recordService := record.NewService(runtimePool, controlPool, usageService)
		definitions = append(definitions, record.CapabilityDefinitions(recordService)...)
		authorizationService := authorization.NewService(runtimePool, controlPool)
		definitions = append(definitions, authorization.CapabilityDefinitions(authorizationService)...)
	}
	defer func() {
		for index := len(poolsToClose) - 1; index >= 0; index-- {
			poolsToClose[index].Close()
		}
	}()
	// A nil concrete *metering.Service stored in an interface is non-nil and
	// would panic on every invocation in stateless CLI/MCP mode. Only attach
	// metering after the database-backed service has actually been initialized.
	var meter capability.Meter
	if usageService != nil {
		meter = usageService
	}
	invoker := capability.NewMeteredInvoker(capability.NewRegistry(definitions), 32, meter)
	var verifier *identity.Verifier
	if cfg.Identity.Issuer != "" || len(cfg.Identity.TrustedIssuers) > 0 {
		verifier, err = identity.NewVerifier(cfg.Identity)
		if err != nil {
			return writeStartupFailure(out, capability.CodeValidationFailed, err.Error())
		}
	}
	if len(args) >= 2 && args[0] == "mcp" && args[1] == "stdio" && len(args) == 2 {
		if verifier == nil || cfg.Identity.Token == "" {
			return writeStartupFailure(out, capability.CodeUnauthenticated, "MCP stdio requires configured agent identity")
		}
		principal, verifyErr := verifier.Verify(ctx, cfg.Identity.Token)
		if verifyErr != nil {
			return writeStartupFailure(out, capability.CodeUnauthenticated, "MCP stdio agent identity is invalid")
		}
		if err := mcpserver.RunStdioAs(ctx, invoker, principal); err != nil && !errors.Is(err, io.EOF) {
			_, _ = fmt.Fprintf(diagnostics, "mcp stdio terminated: %v\n", err)
			return 1
		}
		return 0
	}
	if len(args) >= 1 && args[0] == "serve" {
		if verifier == nil {
			return writeStartupFailure(out, capability.CodeUnauthenticated, "capability API requires configured identity verification")
		}
		if !cfg.Provisioning.Enabled() {
			return writeStartupFailure(out, capability.CodeValidationFailed, "AgentCiCi-controlled provisioning configuration is required")
		}
		routes := http.NewServeMux()
		if tenantService == nil {
			return writeStartupFailure(out, capability.CodeValidationFailed, "controlled provisioning requires application database roles")
		}
		routes.Handle("/internal/v1/company-provisionings", internalprovisioning.NewHandler(tenantService, cfg.Provisioning))
		routes.Handle("/mcp", mcpserver.NewAuthenticatedStreamableHTTPHandler(invoker, verifier.VerifyWithExpiration))
		routes.Handle("/", api.NewAuthenticatedHandler(invoker, verifier))
		return serve(ctx, args[1:], routes, cfg.HTTPListen, logger, out)
	}
	if verifier != nil && cfg.Identity.Token != "" {
		principal, verifyErr := verifier.Verify(ctx, cfg.Identity.Token)
		if verifyErr != nil {
			return writeStartupFailure(out, capability.CodeUnauthenticated, "CLI agent identity is invalid")
		}
		return cli.RunAs(ctx, invoker, principal, args, in, out, diagnostics)
	}
	return cli.Run(ctx, invoker, args, in, out, diagnostics)
}

func serve(ctx context.Context, args []string, handler http.Handler, listen string, logger interface {
	Info(string, ...any)
	Error(string, ...any)
}, out io.Writer) int {
	if len(args) == 2 && args[0] == "--listen" && args[1] != "" {
		listen = args[1]
	} else if len(args) != 0 {
		_ = json.NewEncoder(out).Encode(map[string]any{"status": "failed", "error": map[string]string{"code": "VALIDATION_FAILED", "message": "expected: serve [--listen <address>]"}})
		return 1
	}

	server := &http.Server{Addr: listen, Handler: handler}
	logger.Info("capability API starting", "listen", listen)
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("capability API terminated", "error", err)
		_ = json.NewEncoder(out).Encode(map[string]any{"status": "failed", "error": map[string]string{"code": "INTERNAL", "message": "HTTP server unavailable"}})
		return 1
	}
	return 0
}

func writeStartupFailure(out io.Writer, code capability.ErrorCode, message string) int {
	_ = json.NewEncoder(out).Encode(map[string]any{"status": "failed", "error": map[string]any{"code": code, "message": message}})
	return 1
}
