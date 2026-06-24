package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BindAddr                 string
	AuthToken                string
	StateDBPath              string
	ConfigRoot               string
	RootfsRoot               string
	BuildRoot                string
	StateDir                 string
	LogDir                   string
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

const (
	defaultBindHost           = "127.0.0.1"
	defaultBindPort           = 8080
	hubConfigFileName         = "hub.yml"
	removedLocalEngineMessage = "ENGINE=" + "local has been removed; set ENGINE to docker, podman, or auto, or leave empty for auto-detect"
)

type hubFileConfig struct {
	Runtime   hubRuntimeConfig   `yaml:"runtime"`
	Paths     hubPathsConfig     `yaml:"paths"`
	Execution hubExecutionConfig `yaml:"execution"`
	HTTPLogs  hubHTTPLogsConfig  `yaml:"http_logs"`
	UI        hubUIConfig        `yaml:"ui"`
}

type hubRuntimeConfig struct {
	NetworkPolicyHelperImage *string `yaml:"network_policy_helper_image"`
}

type hubPathsConfig struct {
	StateDBPath              *string `yaml:"state_db_path"`
	RootfsRoot               *string `yaml:"rootfs_root"`
	BuildRoot                *string `yaml:"build_root"`
	SessionMountTemplateRoot *string `yaml:"session_mount_template_root"`
}

type hubExecutionConfig struct {
	DefaultCommandTimeout *string `yaml:"default_command_timeout"`
	DeleteRootfsOnStop    *bool   `yaml:"delete_rootfs_on_stop"`
	LogPersist            *bool   `yaml:"log_persist"`
	LogMaxOutputBytes     *int    `yaml:"log_max_output_bytes"`
}

type hubHTTPLogsConfig struct {
	AccessEnabled *bool `yaml:"access_enabled"`
	ErrorEnabled  *bool `yaml:"error_enabled"`
}

type hubUIConfig struct {
	DisplayTimezone *string `yaml:"display_timezone"`
}

type cliOptions struct {
	configDir string
	dataDir   string
	stateDir  string
	logDir    string
	bindAddr  string
}

func Load() (Config, error) {
	return LoadWithArgs(os.Args[1:])
}

func LoadWithArgs(args []string) (Config, error) {
	options, err := parseCLIOptions(args)
	if err != nil {
		return Config{}, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return Config{}, fmt.Errorf("getwd: %w", err)
	}
	configRoot := getEnv("CONFIG_ROOT", filepath.Join(cwd, "configs"))
	stateDirDefault := filepath.Join(cwd, "run")
	logDirDefault := stateDirDefault
	if options.configDir != "" {
		configRoot = filepath.Join(options.configDir, "configs")
	}
	if options.stateDir != "" {
		stateDirDefault = options.stateDir
		logDirDefault = stateDirDefault
	}
	if options.logDir != "" {
		logDirDefault = options.logDir
	}
	configRoot, err = absolutePath(configRoot)
	if err != nil {
		return Config{}, fmt.Errorf("normalize config root: %w", err)
	}
	cfg := Config{
		BindAddr:                 net.JoinHostPort(defaultBindHost, strconv.Itoa(defaultBindPort)),
		AuthToken:                strings.TrimSpace(os.Getenv("AUTH_TOKEN")),
		StateDBPath:              filepath.Join(cwd, "data", "hub.db"),
		ConfigRoot:               configRoot,
		RootfsRoot:               filepath.Join(cwd, "data", "rootfs"),
		BuildRoot:                filepath.Join(cwd, "data", "builds"),
		StateDir:                 stateDirDefault,
		LogDir:                   logDirDefault,
		Engine:                   "auto",
		DefaultCommandTimeout:    30 * time.Second,
		DeleteRootfsOnStop:       true,
		ExecLogMaxOutputBytes:    65536,
		NetworkPolicyHelperImage: "agent-container-hub/network-policy-helper:latest",
		DisplayTimezone:          resolveDisplayTimezone("", ""),
	}
	if err := applyHubFileConfig(&cfg, filepath.Join(cfg.ConfigRoot, hubConfigFileName)); err != nil {
		return Config{}, err
	}
	if err := applyEnvOverrides(&cfg); err != nil {
		return Config{}, err
	}
	if options.dataDir != "" {
		cfg.StateDBPath = filepath.Join(options.dataDir, "hub.db")
		cfg.RootfsRoot = filepath.Join(options.dataDir, "rootfs")
		cfg.BuildRoot = filepath.Join(options.dataDir, "builds")
	}
	if options.bindAddr != "" {
		cfg.BindAddr = options.bindAddr
	}
	if cfg.StateDBPath, err = absolutePath(cfg.StateDBPath); err != nil {
		return Config{}, fmt.Errorf("normalize state db path: %w", err)
	}
	if cfg.RootfsRoot, err = absolutePath(cfg.RootfsRoot); err != nil {
		return Config{}, fmt.Errorf("normalize rootfs root: %w", err)
	}
	if cfg.BuildRoot, err = absolutePath(cfg.BuildRoot); err != nil {
		return Config{}, fmt.Errorf("normalize build root: %w", err)
	}
	if cfg.StateDir, err = absolutePath(cfg.StateDir); err != nil {
		return Config{}, fmt.Errorf("normalize state dir: %w", err)
	}
	if cfg.LogDir, err = absolutePath(cfg.LogDir); err != nil {
		return Config{}, fmt.Errorf("normalize log dir: %w", err)
	}
	if cfg.SessionMountTemplateRoot, err = absolutePath(cfg.SessionMountTemplateRoot); err != nil {
		return Config{}, fmt.Errorf("normalize session mount template root: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyHubFileConfig(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read hub config %s: %w", path, err)
	}
	var fileConfig hubFileConfig
	if err := yaml.Unmarshal(data, &fileConfig); err != nil {
		return fmt.Errorf("parse hub config %s: %w", path, err)
	}
	baseDir := filepath.Dir(path)
	if fileConfig.Runtime.NetworkPolicyHelperImage != nil {
		cfg.NetworkPolicyHelperImage = strings.TrimSpace(*fileConfig.Runtime.NetworkPolicyHelperImage)
	}
	if fileConfig.Paths.StateDBPath != nil {
		cfg.StateDBPath = resolveHubPath(baseDir, *fileConfig.Paths.StateDBPath)
	}
	if fileConfig.Paths.RootfsRoot != nil {
		cfg.RootfsRoot = resolveHubPath(baseDir, *fileConfig.Paths.RootfsRoot)
	}
	if fileConfig.Paths.BuildRoot != nil {
		cfg.BuildRoot = resolveHubPath(baseDir, *fileConfig.Paths.BuildRoot)
	}
	if fileConfig.Paths.SessionMountTemplateRoot != nil {
		cfg.SessionMountTemplateRoot = resolveHubPath(baseDir, *fileConfig.Paths.SessionMountTemplateRoot)
	}
	if fileConfig.Execution.DefaultCommandTimeout != nil {
		parsed, err := time.ParseDuration(strings.TrimSpace(*fileConfig.Execution.DefaultCommandTimeout))
		if err != nil {
			return fmt.Errorf("parse execution.default_command_timeout: %w", err)
		}
		cfg.DefaultCommandTimeout = parsed
	}
	if fileConfig.Execution.DeleteRootfsOnStop != nil {
		cfg.DeleteRootfsOnStop = *fileConfig.Execution.DeleteRootfsOnStop
	}
	if fileConfig.Execution.LogPersist != nil {
		cfg.EnableExecLogPersist = *fileConfig.Execution.LogPersist
	}
	if fileConfig.Execution.LogMaxOutputBytes != nil {
		cfg.ExecLogMaxOutputBytes = *fileConfig.Execution.LogMaxOutputBytes
	}
	if fileConfig.HTTPLogs.AccessEnabled != nil {
		cfg.HTTPAccessLogEnabled = *fileConfig.HTTPLogs.AccessEnabled
	}
	if fileConfig.HTTPLogs.ErrorEnabled != nil {
		cfg.HTTPErrorLogEnabled = *fileConfig.HTTPLogs.ErrorEnabled
	}
	if fileConfig.UI.DisplayTimezone != nil {
		cfg.DisplayTimezone = strings.TrimSpace(*fileConfig.UI.DisplayTimezone)
	}
	return nil
}

func applyEnvOverrides(cfg *Config) error {
	cfg.AuthToken = strings.TrimSpace(os.Getenv("AUTH_TOKEN"))
	bindAddr := strings.TrimSpace(os.Getenv("BIND_ADDR"))
	if bindAddr != "" {
		cfg.BindAddr = bindAddr
	} else {
		if err := applyServerEnvOverrides(cfg); err != nil {
			return err
		}
	}
	cfg.StateDBPath = getEnv("STATE_DB_PATH", cfg.StateDBPath)
	cfg.RootfsRoot = getEnv("ROOTFS_ROOT", cfg.RootfsRoot)
	cfg.BuildRoot = getEnv("BUILD_ROOT", cfg.BuildRoot)
	cfg.SessionMountTemplateRoot = getEnv("SESSION_MOUNT_TEMPLATE_ROOT", cfg.SessionMountTemplateRoot)
	cfg.Engine = getEnv("ENGINE", cfg.Engine)
	cfg.DefaultCommandTimeout = getEnvDuration("DEFAULT_COMMAND_TIMEOUT", cfg.DefaultCommandTimeout)
	cfg.DeleteRootfsOnStop = getEnvBool("DELETE_ROOTFS_ON_STOP", cfg.DeleteRootfsOnStop)
	cfg.HTTPAccessLogEnabled = getEnvBool("HTTP_ACCESS_LOG_ENABLED", cfg.HTTPAccessLogEnabled)
	cfg.HTTPErrorLogEnabled = getEnvBool("HTTP_ERROR_LOG_ENABLED", cfg.HTTPErrorLogEnabled)
	cfg.EnableExecLogPersist = getEnvBool("ENABLE_EXEC_LOG_PERSIST", cfg.EnableExecLogPersist)
	cfg.ExecLogMaxOutputBytes = getEnvInt("EXEC_LOG_MAX_OUTPUT_BYTES", cfg.ExecLogMaxOutputBytes)
	cfg.NetworkPolicyHelperImage = getEnv("NETWORK_POLICY_HELPER_IMAGE", cfg.NetworkPolicyHelperImage)
	displayTimezone := strings.TrimSpace(os.Getenv("DISPLAY_TIMEZONE"))
	tz := strings.TrimSpace(os.Getenv("TZ"))
	if displayTimezone != "" || tz != "" {
		cfg.DisplayTimezone = resolveDisplayTimezone(displayTimezone, tz)
	}
	return nil
}

func applyServerEnvOverrides(cfg *Config) error {
	host := strings.TrimSpace(os.Getenv("SERVER_HOST"))
	portValue := strings.TrimSpace(os.Getenv("SERVER_PORT"))
	if host == "" && portValue == "" {
		return nil
	}
	currentHost, currentPort, err := net.SplitHostPort(cfg.BindAddr)
	if err != nil {
		return fmt.Errorf("split bind address: %w", err)
	}
	if host == "" {
		host = currentHost
	}
	if portValue == "" {
		portValue = currentPort
	}
	port, err := strconv.Atoi(portValue)
	if err != nil {
		return fmt.Errorf("SERVER_PORT must be an integer between 0 and 65535: %w", err)
	}
	if port < 0 || port > 65535 {
		return fmt.Errorf("SERVER_PORT must be between 0 and 65535")
	}
	cfg.BindAddr = net.JoinHostPort(host, strconv.Itoa(port))
	return nil
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
	if c.StateDir == "" || c.LogDir == "" {
		return fmt.Errorf("runtime directories are required")
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

func parseCLIOptions(args []string) (cliOptions, error) {
	var options cliOptions
	for index := 0; index < len(args); index++ {
		arg := strings.TrimSpace(args[index])
		if arg == "" {
			continue
		}
		name, value, hasInlineValue := strings.Cut(arg, "=")
		assign := func(target *string) error {
			if hasInlineValue {
				*target = strings.TrimSpace(value)
				return nil
			}
			if index+1 >= len(args) {
				return fmt.Errorf("missing value for %s", name)
			}
			index++
			*target = strings.TrimSpace(args[index])
			return nil
		}
		switch name {
		case "--config-dir":
			if err := assign(&options.configDir); err != nil {
				return cliOptions{}, err
			}
		case "--data-dir":
			if err := assign(&options.dataDir); err != nil {
				return cliOptions{}, err
			}
		case "--state-dir":
			if err := assign(&options.stateDir); err != nil {
				return cliOptions{}, err
			}
		case "--log-dir":
			if err := assign(&options.logDir); err != nil {
				return cliOptions{}, err
			}
		case "--bind-addr":
			if err := assign(&options.bindAddr); err != nil {
				return cliOptions{}, err
			}
		}
	}
	return options, nil
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

func resolveHubPath(baseDir, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(baseDir, path)
}
