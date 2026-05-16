package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	BindAddr                 string
	AuthToken                string
	StateDBPath              string
	ConfigRoot               string
	RootfsRoot               string
	BuildRoot                string
	SessionMountTemplateRoot string
	Engine                   string
	DefaultCommandTimeout    time.Duration
	DeleteRootfsOnStop       bool
	HTTPAccessLogEnabled     bool
	HTTPErrorLogEnabled      bool
	EnableExecLogPersist     bool
	ExecLogMaxOutputBytes    int
	NetworkPolicyHelperImage string
	DisplayTimezone          string
}

const removedLocalEngineMessage = "ENGINE=" + "local has been removed; set ENGINE to docker, podman, or auto, or leave empty for auto-detect"

func Load() (Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Config{}, fmt.Errorf("getwd: %w", err)
	}
	serviceConfigDir := strings.TrimSpace(os.Getenv("SERVICE_CONFIG_DIR"))
	serviceDataDir := strings.TrimSpace(os.Getenv("ZENMIND_SERVICE_DATA_DIR"))
	configRootDefault := filepath.Join(cwd, "configs")
	stateDBPathDefault := filepath.Join(cwd, "data", "hub.db")
	rootfsRootDefault := filepath.Join(cwd, "data", "rootfs")
	buildRootDefault := filepath.Join(cwd, "data", "builds")
	if serviceConfigDir != "" {
		configRootDefault = filepath.Join(serviceConfigDir, "configs")
	}
	if serviceDataDir != "" {
		stateDBPathDefault = filepath.Join(serviceDataDir, "hub.db")
		rootfsRootDefault = filepath.Join(serviceDataDir, "rootfs")
		buildRootDefault = filepath.Join(serviceDataDir, "builds")
	}
	cfg := Config{
		BindAddr:                 getEnv("BIND_ADDR", "127.0.0.1:8080"),
		AuthToken:                strings.TrimSpace(os.Getenv("AUTH_TOKEN")),
		StateDBPath:              getEnv("STATE_DB_PATH", stateDBPathDefault),
		ConfigRoot:               getEnv("CONFIG_ROOT", configRootDefault),
		RootfsRoot:               getEnv("ROOTFS_ROOT", rootfsRootDefault),
		BuildRoot:                getEnv("BUILD_ROOT", buildRootDefault),
		SessionMountTemplateRoot: getEnv("SESSION_MOUNT_TEMPLATE_ROOT", ""),
		Engine:                   strings.TrimSpace(os.Getenv("ENGINE")),
		DefaultCommandTimeout:    getEnvDuration("DEFAULT_COMMAND_TIMEOUT", 30*time.Second),
		DeleteRootfsOnStop:       getEnvBool("DELETE_ROOTFS_ON_STOP", true),
		HTTPAccessLogEnabled:     getEnvBool("HTTP_ACCESS_LOG_ENABLED", false),
		HTTPErrorLogEnabled:      getEnvBool("HTTP_ERROR_LOG_ENABLED", false),
		EnableExecLogPersist:     getEnvBool("ENABLE_EXEC_LOG_PERSIST", false),
		ExecLogMaxOutputBytes:    getEnvInt("EXEC_LOG_MAX_OUTPUT_BYTES", 65536),
		NetworkPolicyHelperImage: getEnv("NETWORK_POLICY_HELPER_IMAGE", "agent-container-hub/network-policy-helper:latest"),
		DisplayTimezone: resolveDisplayTimezone(
			strings.TrimSpace(os.Getenv("DISPLAY_TIMEZONE")),
			strings.TrimSpace(os.Getenv("TZ")),
		),
	}
	if cfg.StateDBPath, err = absolutePath(cfg.StateDBPath); err != nil {
		return Config{}, fmt.Errorf("normalize state db path: %w", err)
	}
	if cfg.ConfigRoot, err = absolutePath(cfg.ConfigRoot); err != nil {
		return Config{}, fmt.Errorf("normalize config root: %w", err)
	}
	if cfg.RootfsRoot, err = absolutePath(cfg.RootfsRoot); err != nil {
		return Config{}, fmt.Errorf("normalize rootfs root: %w", err)
	}
	if cfg.BuildRoot, err = absolutePath(cfg.BuildRoot); err != nil {
		return Config{}, fmt.Errorf("normalize build root: %w", err)
	}
	if cfg.SessionMountTemplateRoot, err = absolutePath(cfg.SessionMountTemplateRoot); err != nil {
		return Config{}, fmt.Errorf("normalize session mount template root: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("bind address is required")
	}
	if strings.EqualFold(c.Engine, "local") {
		return fmt.Errorf(removedLocalEngineMessage)
	}
	host, _, err := net.SplitHostPort(c.BindAddr)
	if err != nil {
		return fmt.Errorf("invalid bind address: %w", err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" && c.AuthToken == "" {
		return fmt.Errorf("AUTH_TOKEN is required when binding to %q", host)
	}
	if c.StateDBPath == "" || c.ConfigRoot == "" || c.RootfsRoot == "" || c.BuildRoot == "" {
		return fmt.Errorf("state paths are required")
	}
	if c.ExecLogMaxOutputBytes < 0 {
		return fmt.Errorf("EXEC_LOG_MAX_OUTPUT_BYTES must be >= 0")
	}
	if strings.TrimSpace(c.NetworkPolicyHelperImage) == "" {
		return fmt.Errorf("NETWORK_POLICY_HELPER_IMAGE is required")
	}
	if _, err := time.LoadLocation(c.DisplayTimezone); err != nil {
		return fmt.Errorf("DISPLAY_TIMEZONE / TZ %q is not a valid IANA timezone: %w", c.DisplayTimezone, err)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

func absolutePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	path = filepath.Clean(path)
	return filepath.Abs(path)
}
