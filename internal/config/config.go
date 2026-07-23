package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var internalServiceIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

type Config struct {
	HTTPListen         string
	Database           Database
	ControlDatabaseURL string
	RuntimeDatabaseURL string
	Log                Log
	Identity           Identity
	Provisioning       Provisioning
}

type Database struct {
	URL         string
	MaxConns    int32
	MinConns    int32
	MaxLifetime time.Duration
	MaxIdleTime time.Duration
}

type Log struct {
	Level  string
	Format string
}

type Identity struct {
	Issuer    string
	Audience  string
	Algorithm string
	HMACKey   string
	Token     string
}

type Provisioning struct {
	AgentCiCiBaseURL string
	AgentCiCiHMACKey string
	CallerKeys       map[string]string
}

func (p Provisioning) Enabled() bool {
	return p.AgentCiCiBaseURL != "" && p.AgentCiCiHMACKey != "" && len(p.CallerKeys) > 0
}

func LoadEnv() (Config, error) {
	return Load(os.Getenv)
}

func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		HTTPListen: valueOr(getenv("AI_NATIVE_HTTP_LISTEN"), "127.0.0.1:8080"),
		Database: Database{
			URL:         getenv("AI_NATIVE_DATABASE_URL"),
			MaxConns:    16,
			MinConns:    0,
			MaxLifetime: time.Hour,
			MaxIdleTime: 15 * time.Minute,
		},
		ControlDatabaseURL: getenv("AI_NATIVE_CONTROL_DATABASE_URL"),
		RuntimeDatabaseURL: getenv("AI_NATIVE_RUNTIME_DATABASE_URL"),
		Log: Log{
			Level:  valueOr(strings.ToLower(getenv("AI_NATIVE_LOG_LEVEL")), "info"),
			Format: valueOr(strings.ToLower(getenv("AI_NATIVE_LOG_FORMAT")), "json"),
		},
		Identity: Identity{
			Issuer:    getenv("AI_NATIVE_IDENTITY_ISSUER"),
			Audience:  getenv("AI_NATIVE_IDENTITY_AUDIENCE"),
			Algorithm: getenv("AI_NATIVE_IDENTITY_ALGORITHM"),
			HMACKey:   getenv("AI_NATIVE_IDENTITY_HMAC_KEY"),
			Token:     getenv("AI_NATIVE_IDENTITY_TOKEN"),
		},
		Provisioning: Provisioning{
			AgentCiCiBaseURL: strings.TrimRight(getenv("AI_NATIVE_AGENTCICI_BASE_URL"), "/"),
			AgentCiCiHMACKey: getenv("AI_NATIVE_AGENTCICI_HMAC_KEY"),
		},
	}
	callerKeys, parseErr := parseCallerKeys(getenv("AI_NATIVE_PROVISIONING_CALLER_KEYS"))
	if parseErr != nil {
		return Config{}, parseErr
	}
	cfg.Provisioning.CallerKeys = callerKeys

	var err error
	if cfg.Database.MaxConns, err = parseInt32(getenv("AI_NATIVE_DATABASE_MAX_CONNS"), cfg.Database.MaxConns, 1); err != nil {
		return Config{}, fmt.Errorf("invalid database max connections")
	}
	if cfg.Database.MinConns, err = parseInt32(getenv("AI_NATIVE_DATABASE_MIN_CONNS"), cfg.Database.MinConns, 0); err != nil {
		return Config{}, fmt.Errorf("invalid database min connections")
	}
	if cfg.Database.MinConns > cfg.Database.MaxConns {
		return Config{}, fmt.Errorf("database min connections exceeds max connections")
	}
	if cfg.Database.MaxLifetime, err = parseDuration(getenv("AI_NATIVE_DATABASE_MAX_LIFETIME"), cfg.Database.MaxLifetime); err != nil {
		return Config{}, fmt.Errorf("invalid database max lifetime")
	}
	if cfg.Database.MaxIdleTime, err = parseDuration(getenv("AI_NATIVE_DATABASE_MAX_IDLE_TIME"), cfg.Database.MaxIdleTime); err != nil {
		return Config{}, fmt.Errorf("invalid database max idle time")
	}
	if cfg.Database.URL != "" {
		if !validDatabaseURL(cfg.Database.URL) {
			return Config{}, fmt.Errorf("invalid database URL")
		}
	}
	if cfg.ControlDatabaseURL != "" && !validDatabaseURL(cfg.ControlDatabaseURL) {
		return Config{}, fmt.Errorf("invalid control database URL")
	}
	if cfg.RuntimeDatabaseURL != "" && !validDatabaseURL(cfg.RuntimeDatabaseURL) {
		return Config{}, fmt.Errorf("invalid runtime database URL")
	}
	if (cfg.ControlDatabaseURL == "") != (cfg.RuntimeDatabaseURL == "") {
		return Config{}, fmt.Errorf("control and runtime database URLs must be configured together")
	}
	if cfg.Log.Level != "debug" && cfg.Log.Level != "info" && cfg.Log.Level != "warn" && cfg.Log.Level != "error" {
		return Config{}, fmt.Errorf("invalid log level")
	}
	if cfg.Log.Format != "json" && cfg.Log.Format != "text" {
		return Config{}, fmt.Errorf("invalid log format")
	}
	if err := validateIdentity(cfg.Identity); err != nil {
		return Config{}, err
	}
	if err := validateProvisioning(cfg.Provisioning); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func parseCallerKeys(raw string) (map[string]string, error) {
	if raw == "" {
		return map[string]string{}, nil
	}
	values := map[string]string{}
	for _, item := range strings.Split(raw, ";") {
		parts := strings.SplitN(item, "=", 2)
		serviceID := strings.TrimSpace(parts[0])
		if len(parts) != 2 || !internalServiceIDPattern.MatchString(serviceID) || len(parts[1]) < 32 {
			return nil, fmt.Errorf("invalid provisioning caller configuration")
		}
		if _, exists := values[serviceID]; exists {
			return nil, fmt.Errorf("invalid provisioning caller configuration")
		}
		values[serviceID] = parts[1]
	}
	return values, nil
}

func validateProvisioning(p Provisioning) error {
	present := 0
	for _, value := range []bool{p.AgentCiCiBaseURL != "", p.AgentCiCiHMACKey != "", len(p.CallerKeys) > 0} {
		if value {
			present++
		}
	}
	if present == 0 {
		return nil
	}
	if present != 3 || len(p.AgentCiCiHMACKey) < 32 {
		return fmt.Errorf("controlled provisioning configuration must be complete")
	}
	parsed, err := url.Parse(p.AgentCiCiBaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("invalid AgentCiCi provisioning URL")
	}
	return nil
}

func validDatabaseURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") && parsed.Host != ""
}

func validateIdentity(identity Identity) error {
	present := 0
	for _, value := range []string{identity.Issuer, identity.Audience, identity.Algorithm, identity.HMACKey} {
		if value != "" {
			present++
		}
	}
	if present == 0 {
		return nil
	}
	if present != 4 {
		return fmt.Errorf("identity configuration must be complete")
	}
	if identity.Algorithm != "HS256" {
		return fmt.Errorf("identity algorithm is not allowed")
	}
	if len(identity.HMACKey) < 32 {
		return fmt.Errorf("identity HMAC key is too short")
	}
	return nil
}

func parseInt32(raw string, fallback, minimum int32) (int32, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value < int64(minimum) {
		return 0, fmt.Errorf("invalid integer")
	}
	return int32(value), nil
}

func parseDuration(raw string, fallback time.Duration) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid duration")
	}
	return value, nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
