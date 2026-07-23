package observability

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/OlivierZEN/ai-native-platform/internal/config"
)

func New(cfg config.Log, output io.Writer) (*slog.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	options := &slog.HandlerOptions{Level: level, ReplaceAttr: redactAttr}
	var handler slog.Handler
	switch cfg.Format {
	case "json":
		handler = slog.NewJSONHandler(output, options)
	case "text":
		handler = slog.NewTextHandler(output, options)
	default:
		return nil, fmt.Errorf("unsupported log format")
	}
	return slog.New(handler), nil
}

func parseLevel(value string) (slog.Level, error) {
	switch value {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level")
	}
}

func redactAttr(_ []string, attribute slog.Attr) slog.Attr {
	key := strings.ToLower(attribute.Key)
	for _, sensitive := range []string{"token", "secret", "password", "credential", "database_url", "dsn", "hmac_key"} {
		if strings.Contains(key, sensitive) {
			return slog.String(attribute.Key, "[REDACTED]")
		}
	}
	return attribute
}
