package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeHubConfig(t *testing.T, configDir, body string) {
	t.Helper()
	configRoot := filepath.Join(configDir, "configs")
	if err := os.MkdirAll(configRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(configRoot, "hub.yml"), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AUTH_TOKEN",
		"SERVER_HOST",
		"SERVER_PORT",
		"BIND_ADDR",
		"STATE_DB_PATH",
		"CONFIG_ROOT",
		"ROOTFS_ROOT",
		"BUILD_ROOT",
		"SESSION_MOUNT_TEMPLATE_ROOT",
		"ENGINE",
		"DEFAULT_COMMAND_TIMEOUT",
		"DELETE_ROOTFS_ON_STOP",
		"HTTP_ACCESS_LOG_ENABLED",
		"HTTP_ERROR_LOG_ENABLED",
		"ENABLE_EXEC_LOG_PERSIST",
		"EXEC_LOG_MAX_OUTPUT_BYTES",
		"NETWORK_POLICY_HELPER_IMAGE",
		"DISPLAY_TIMEZONE",
		"TZ",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadUsesServerHostAndPortEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SERVER_HOST", "127.0.0.1")
	t.Setenv("SERVER_PORT", "18080")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.BindAddr != "127.0.0.1:18080" {
		t.Fatalf("BindAddr = %q, want SERVER_HOST/SERVER_PORT", cfg.BindAddr)
	}
}

func TestLoadBindAddrOverridesServerHostAndPortEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SERVER_HOST", "127.0.0.1")
	t.Setenv("SERVER_PORT", "18080")
	t.Setenv("BIND_ADDR", "127.0.0.1:19090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.BindAddr != "127.0.0.1:19090" {
		t.Fatalf("BindAddr = %q, want BIND_ADDR override", cfg.BindAddr)
	}
}

func TestLoadBindAddrSkipsInvalidServerPortEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SERVER_PORT", "not-a-port")
	t.Setenv("BIND_ADDR", "127.0.0.1:19090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.BindAddr != "127.0.0.1:19090" {
		t.Fatalf("BindAddr = %q, want BIND_ADDR override", cfg.BindAddr)
	}
}

func TestLoadDefaultsWithoutHubConfig(t *testing.T) {
	clearConfigEnv(t)
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.BindAddr != "127.0.0.1:8080" {
		t.Fatalf("BindAddr = %q, want default", cfg.BindAddr)
	}
	if cfg.Engine != "auto" {
		t.Fatalf("Engine = %q, want auto", cfg.Engine)
	}
}

func TestLoadReadsHubYAML(t *testing.T) {
	clearConfigEnv(t)
	configDir := t.TempDir()
	writeHubConfig(t, configDir, `
runtime:
  network_policy_helper_image: registry.example.com/helper:v2
paths:
  state_db_path: ../state/hub.db
  rootfs_root: ../state/rootfs
  build_root: ../state/builds
  session_mount_template_root: ../templates
execution:
  default_command_timeout: 45s
  delete_rootfs_on_stop: false
  log_persist: true
  log_max_output_bytes: 12345
http_logs:
  access_enabled: true
  error_enabled: true
ui:
  display_timezone: UTC
`)

	cfg, err := LoadWithArgs([]string{"--config-dir", configDir})
	if err != nil {
		t.Fatalf("LoadWithArgs() error = %v", err)
	}

	if cfg.BindAddr != "127.0.0.1:8080" {
		t.Fatalf("BindAddr = %q, want default", cfg.BindAddr)
	}
	if cfg.Engine != "auto" {
		t.Fatalf("Engine = %q, want default", cfg.Engine)
	}
	if cfg.NetworkPolicyHelperImage != "registry.example.com/helper:v2" {
		t.Fatalf("NetworkPolicyHelperImage = %q, want YAML value", cfg.NetworkPolicyHelperImage)
	}
	if want := filepath.Join(configDir, "state", "hub.db"); cfg.StateDBPath != want {
		t.Fatalf("StateDBPath = %q, want %q", cfg.StateDBPath, want)
	}
	if want := filepath.Join(configDir, "state", "rootfs"); cfg.RootfsRoot != want {
		t.Fatalf("RootfsRoot = %q, want %q", cfg.RootfsRoot, want)
	}
	if want := filepath.Join(configDir, "state", "builds"); cfg.BuildRoot != want {
		t.Fatalf("BuildRoot = %q, want %q", cfg.BuildRoot, want)
	}
	if want := filepath.Join(configDir, "templates"); cfg.SessionMountTemplateRoot != want {
		t.Fatalf("SessionMountTemplateRoot = %q, want %q", cfg.SessionMountTemplateRoot, want)
	}
	if cfg.DefaultCommandTimeout.String() != "45s" {
		t.Fatalf("DefaultCommandTimeout = %s, want 45s", cfg.DefaultCommandTimeout)
	}
	if cfg.DeleteRootfsOnStop {
		t.Fatal("DeleteRootfsOnStop = true, want false")
	}
	if !cfg.EnableExecLogPersist {
		t.Fatal("EnableExecLogPersist = false, want true")
	}
	if cfg.ExecLogMaxOutputBytes != 12345 {
		t.Fatalf("ExecLogMaxOutputBytes = %d, want 12345", cfg.ExecLogMaxOutputBytes)
	}
	if !cfg.HTTPAccessLogEnabled || !cfg.HTTPErrorLogEnabled {
		t.Fatalf("HTTP log flags = %t/%t, want true/true", cfg.HTTPAccessLogEnabled, cfg.HTTPErrorLogEnabled)
	}
	if cfg.DisplayTimezone != "UTC" {
		t.Fatalf("DisplayTimezone = %q, want UTC", cfg.DisplayTimezone)
	}
}

func TestLoadIgnoresHubYAMLServerConfig(t *testing.T) {
	clearConfigEnv(t)
	configDir := t.TempDir()
	writeHubConfig(t, configDir, `
server:
  host: 0.0.0.0
  port: 18080
`)

	cfg, err := LoadWithArgs([]string{"--config-dir", configDir})
	if err != nil {
		t.Fatalf("LoadWithArgs() error = %v", err)
	}
	if cfg.BindAddr != "127.0.0.1:8080" {
		t.Fatalf("BindAddr = %q, want hub.yml server config ignored", cfg.BindAddr)
	}
}

func TestLoadIgnoresHubYAMLEngine(t *testing.T) {
	clearConfigEnv(t)
	configDir := t.TempDir()
	writeHubConfig(t, configDir, `
runtime:
  engine: podman
`)

	cfg, err := LoadWithArgs([]string{"--config-dir", configDir})
	if err != nil {
		t.Fatalf("LoadWithArgs() error = %v", err)
	}
	if cfg.Engine != "auto" {
		t.Fatalf("Engine = %q, want hub.yml runtime.engine ignored", cfg.Engine)
	}
}

func TestLoadEnvOverridesHubYAML(t *testing.T) {
	clearConfigEnv(t)
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	currentWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() after Chdir error = %v", err)
	}
	configDir := t.TempDir()
	writeHubConfig(t, configDir, `
paths:
  state_db_path: ../state/hub.db
execution:
  log_persist: false
ui:
  display_timezone: UTC
`)
	t.Setenv("BIND_ADDR", "127.0.0.1:19090")
	t.Setenv("ENGINE", "docker")
	t.Setenv("STATE_DB_PATH", "./env/hub.db")
	t.Setenv("ROOTFS_ROOT", "./env/rootfs")
	t.Setenv("BUILD_ROOT", "./env/builds")
	t.Setenv("ENABLE_EXEC_LOG_PERSIST", "true")
	t.Setenv("DISPLAY_TIMEZONE", "Asia/Tokyo")
	t.Setenv("SERVER_HOST", "127.0.0.1")
	t.Setenv("SERVER_PORT", "18080")

	cfg, err := LoadWithArgs([]string{"--config-dir", configDir})
	if err != nil {
		t.Fatalf("LoadWithArgs() error = %v", err)
	}

	if cfg.BindAddr != "127.0.0.1:19090" {
		t.Fatalf("BindAddr = %q, want env override", cfg.BindAddr)
	}
	if cfg.Engine != "docker" {
		t.Fatalf("Engine = %q, want docker", cfg.Engine)
	}
	if want := filepath.Join(currentWD, "env", "hub.db"); cfg.StateDBPath != want {
		t.Fatalf("StateDBPath = %q, want %q", cfg.StateDBPath, want)
	}
	if !cfg.EnableExecLogPersist {
		t.Fatal("EnableExecLogPersist = false, want env override")
	}
	if cfg.DisplayTimezone != "Asia/Tokyo" {
		t.Fatalf("DisplayTimezone = %q, want env override", cfg.DisplayTimezone)
	}
}

func TestLoadUsesEngineEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ENGINE", "podman")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Engine != "podman" {
		t.Fatalf("Engine = %q, want env override", cfg.Engine)
	}
}

func TestLoadCLIOverridesEnvAndHubYAML(t *testing.T) {
	clearConfigEnv(t)
	configDir := t.TempDir()
	dataDir := t.TempDir()
	writeHubConfig(t, configDir, `
paths:
  state_db_path: ../state/hub.db
  rootfs_root: ../state/rootfs
  build_root: ../state/builds
`)
	t.Setenv("BIND_ADDR", "127.0.0.1:19090")
	t.Setenv("STATE_DB_PATH", "/tmp/ignored.db")
	t.Setenv("ROOTFS_ROOT", "/tmp/ignored-rootfs")
	t.Setenv("BUILD_ROOT", "/tmp/ignored-builds")
	t.Setenv("SERVER_HOST", "127.0.0.1")
	t.Setenv("SERVER_PORT", "18080")

	cfg, err := LoadWithArgs([]string{
		"--config-dir", configDir,
		"--data-dir", dataDir,
		"--bind-addr", "127.0.0.1:20000",
	})
	if err != nil {
		t.Fatalf("LoadWithArgs() error = %v", err)
	}

	if cfg.BindAddr != "127.0.0.1:20000" {
		t.Fatalf("BindAddr = %q, want CLI override", cfg.BindAddr)
	}
	if want := filepath.Join(dataDir, "hub.db"); cfg.StateDBPath != want {
		t.Fatalf("StateDBPath = %q, want %q", cfg.StateDBPath, want)
	}
	if want := filepath.Join(dataDir, "rootfs"); cfg.RootfsRoot != want {
		t.Fatalf("RootfsRoot = %q, want %q", cfg.RootfsRoot, want)
	}
	if want := filepath.Join(dataDir, "builds"); cfg.BuildRoot != want {
		t.Fatalf("BuildRoot = %q, want %q", cfg.BuildRoot, want)
	}
}

func TestLoadRejectsInvalidServerPortEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SERVER_PORT", "70000")

	_, err := Load()
	if err == nil {
		t.Fatal("LoadWithArgs() error = nil, want invalid port error")
	}
	if !strings.Contains(err.Error(), "SERVER_PORT") {
		t.Fatalf("LoadWithArgs() error = %q, want SERVER_PORT", err)
	}
}

func TestLoadRejectsInvalidHubDuration(t *testing.T) {
	clearConfigEnv(t)
	configDir := t.TempDir()
	writeHubConfig(t, configDir, `
execution:
  default_command_timeout: nope
`)

	_, err := LoadWithArgs([]string{"--config-dir", configDir})
	if err == nil {
		t.Fatal("LoadWithArgs() error = nil, want invalid duration error")
	}
	if !strings.Contains(err.Error(), "execution.default_command_timeout") {
		t.Fatalf("LoadWithArgs() error = %q, want execution.default_command_timeout", err)
	}
}

func TestLoadRejectsInvalidHubDisplayTimezone(t *testing.T) {
	clearConfigEnv(t)
	configDir := t.TempDir()
	writeHubConfig(t, configDir, `
ui:
  display_timezone: NotA/Real_Timezone
`)

	_, err := LoadWithArgs([]string{"--config-dir", configDir})
	if err == nil {
		t.Fatal("LoadWithArgs() error = nil, want invalid timezone")
	}
}

func TestLoadRejectsRemoteServerHostWithoutAuthToken(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SERVER_HOST", "0.0.0.0")
	t.Setenv("SERVER_PORT", "8080")

	_, err := Load()
	if err == nil {
		t.Fatal("LoadWithArgs() error = nil, want missing auth token error")
	}
	if !strings.Contains(err.Error(), "AUTH_TOKEN is required") {
		t.Fatalf("LoadWithArgs() error = %q, want auth token error", err)
	}
}

func TestLoadNormalizesRelativePaths(t *testing.T) {
	clearConfigEnv(t)
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	t.Setenv("STATE_DB_PATH", "./data/state.db")
	t.Setenv("CONFIG_ROOT", "./configs")
	t.Setenv("ROOTFS_ROOT", "./data/rootfs")
	t.Setenv("BUILD_ROOT", "./data/builds")
	t.Setenv("SESSION_MOUNT_TEMPLATE_ROOT", "./zenmind-env")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	currentWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() after Chdir error = %v", err)
	}

	if want := filepath.Join(currentWD, "data", "state.db"); cfg.StateDBPath != want {
		t.Fatalf("StateDBPath = %q, want %q", cfg.StateDBPath, want)
	}
	if want := filepath.Join(currentWD, "data", "rootfs"); cfg.RootfsRoot != want {
		t.Fatalf("RootfsRoot = %q, want %q", cfg.RootfsRoot, want)
	}
	if want := filepath.Join(currentWD, "configs"); cfg.ConfigRoot != want {
		t.Fatalf("ConfigRoot = %q, want %q", cfg.ConfigRoot, want)
	}
	if want := filepath.Join(currentWD, "data", "builds"); cfg.BuildRoot != want {
		t.Fatalf("BuildRoot = %q, want %q", cfg.BuildRoot, want)
	}
	if want := filepath.Join(currentWD, "zenmind-env"); cfg.SessionMountTemplateRoot != want {
		t.Fatalf("SessionMountTemplateRoot = %q, want %q", cfg.SessionMountTemplateRoot, want)
	}
}

func TestLoadUsesRenamedDefaultStateDBPath(t *testing.T) {
	clearConfigEnv(t)
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	t.Setenv("BIND_ADDR", "127.0.0.1:0")
	t.Setenv("STATE_DB_PATH", "")
	t.Setenv("CONFIG_ROOT", "")
	t.Setenv("ROOTFS_ROOT", "")
	t.Setenv("BUILD_ROOT", "")
	t.Setenv("SESSION_MOUNT_TEMPLATE_ROOT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	currentWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	want := filepath.Join(currentWD, "data", "hub.db")
	if cfg.StateDBPath != want {
		t.Fatalf("StateDBPath = %q, want %q", cfg.StateDBPath, want)
	}
	if want := filepath.Join(currentWD, "configs"); cfg.ConfigRoot != want {
		t.Fatalf("ConfigRoot = %q, want %q", cfg.ConfigRoot, want)
	}
	if cfg.SessionMountTemplateRoot != "" {
		t.Fatalf("SessionMountTemplateRoot = %q, want empty", cfg.SessionMountTemplateRoot)
	}
	if !cfg.DeleteRootfsOnStop {
		t.Fatal("DeleteRootfsOnStop = false, want true")
	}
}

func TestLoadUsesCLIServiceLayoutDirsForDefaults(t *testing.T) {
	clearConfigEnv(t)
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	programDir := t.TempDir()
	if err := os.Chdir(programDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	configDir := t.TempDir()
	dataDir := t.TempDir()
	stateDir := t.TempDir()
	logDir := t.TempDir()
	t.Setenv("CONFIG_ROOT", "")
	t.Setenv("STATE_DB_PATH", "")
	t.Setenv("ROOTFS_ROOT", "")
	t.Setenv("BUILD_ROOT", "")

	cfg, err := LoadWithArgs([]string{
		"--config-dir", configDir,
		"--data-dir", dataDir,
		"--state-dir", stateDir,
		"--log-dir", logDir,
		"--bind-addr", "127.0.0.1:8080",
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if want := filepath.Join(configDir, "configs"); cfg.ConfigRoot != want {
		t.Fatalf("ConfigRoot = %q, want %q", cfg.ConfigRoot, want)
	}
	if want := filepath.Join(dataDir, "hub.db"); cfg.StateDBPath != want {
		t.Fatalf("StateDBPath = %q, want %q", cfg.StateDBPath, want)
	}
	if want := filepath.Join(dataDir, "rootfs"); cfg.RootfsRoot != want {
		t.Fatalf("RootfsRoot = %q, want %q", cfg.RootfsRoot, want)
	}
	if want := filepath.Join(dataDir, "builds"); cfg.BuildRoot != want {
		t.Fatalf("BuildRoot = %q, want %q", cfg.BuildRoot, want)
	}
	if cfg.StateDir != stateDir {
		t.Fatalf("StateDir = %q, want %q", cfg.StateDir, stateDir)
	}
	if cfg.LogDir != logDir {
		t.Fatalf("LogDir = %q, want %q", cfg.LogDir, logDir)
	}
	if cfg.BindAddr != "127.0.0.1:8080" {
		t.Fatalf("BindAddr = %q, want explicit flag", cfg.BindAddr)
	}
}

func TestLoadCLIServiceLayoutDirsOverridePathEnv(t *testing.T) {
	clearConfigEnv(t)
	configDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv("CONFIG_ROOT", "/tmp/ignored-config")
	t.Setenv("STATE_DB_PATH", "/tmp/ignored.db")
	t.Setenv("ROOTFS_ROOT", "/tmp/ignored-rootfs")
	t.Setenv("BUILD_ROOT", "/tmp/ignored-builds")
	t.Setenv("BIND_ADDR", "127.0.0.1:8080")

	cfg, err := LoadWithArgs([]string{
		"--config-dir", configDir,
		"--data-dir", dataDir,
		"--bind-addr", "127.0.0.1:8080",
	})
	if err != nil {
		t.Fatalf("LoadWithArgs() error = %v", err)
	}

	if want := filepath.Join(configDir, "configs"); cfg.ConfigRoot != want {
		t.Fatalf("ConfigRoot = %q, want %q", cfg.ConfigRoot, want)
	}
	if want := filepath.Join(dataDir, "hub.db"); cfg.StateDBPath != want {
		t.Fatalf("StateDBPath = %q, want %q", cfg.StateDBPath, want)
	}
	if want := filepath.Join(dataDir, "rootfs"); cfg.RootfsRoot != want {
		t.Fatalf("RootfsRoot = %q, want %q", cfg.RootfsRoot, want)
	}
	if want := filepath.Join(dataDir, "builds"); cfg.BuildRoot != want {
		t.Fatalf("BuildRoot = %q, want %q", cfg.BuildRoot, want)
	}
	if cfg.BindAddr != "127.0.0.1:8080" {
		t.Fatalf("BindAddr = %q, want flag value", cfg.BindAddr)
	}
}

func TestLoadParsesHTTPLogFlags(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("HTTP_ACCESS_LOG_ENABLED", "true")
	t.Setenv("HTTP_ERROR_LOG_ENABLED", "1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.HTTPAccessLogEnabled {
		t.Fatal("HTTPAccessLogEnabled = false, want true")
	}
	if !cfg.HTTPErrorLogEnabled {
		t.Fatal("HTTPErrorLogEnabled = false, want true")
	}
}

func TestLoadParsesDeleteRootfsOnStop(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DELETE_ROOTFS_ON_STOP", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DeleteRootfsOnStop {
		t.Fatal("DeleteRootfsOnStop = true, want false")
	}
}

func TestLoadNetworkPolicyHelperImage(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("NETWORK_POLICY_HELPER_IMAGE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.NetworkPolicyHelperImage != "agent-container-hub/network-policy-helper:latest" {
		t.Fatalf("NetworkPolicyHelperImage = %q, want default", cfg.NetworkPolicyHelperImage)
	}

	t.Setenv("NETWORK_POLICY_HELPER_IMAGE", "registry.example.com/policy-helper:v2")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() with override error = %v", err)
	}
	if cfg.NetworkPolicyHelperImage != "registry.example.com/policy-helper:v2" {
		t.Fatalf("NetworkPolicyHelperImage = %q, want override", cfg.NetworkPolicyHelperImage)
	}
}

func TestLoadRejectsRemovedLocalEngine(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ENGINE", "local")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want rejection for removed local engine")
	}
	if err.Error() != removedLocalEngineMessage {
		t.Fatalf("Load() error = %q, want %q", err.Error(), removedLocalEngineMessage)
	}
}

func TestLoadDisplayTimezoneOverride(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DISPLAY_TIMEZONE", "Asia/Tokyo")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DisplayTimezone != "Asia/Tokyo" {
		t.Fatalf("DisplayTimezone = %q, want Asia/Tokyo", cfg.DisplayTimezone)
	}
}

func TestLoadDisplayTimezoneFallsBackToTZEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DISPLAY_TIMEZONE", "")
	t.Setenv("TZ", "Europe/Berlin")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DisplayTimezone != "Europe/Berlin" {
		t.Fatalf("DisplayTimezone = %q, want Europe/Berlin", cfg.DisplayTimezone)
	}
}

func TestLoadDisplayTimezonePrefersDisplayTimezoneOverTZ(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DISPLAY_TIMEZONE", "America/New_York")
	t.Setenv("TZ", "Europe/Berlin")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DisplayTimezone != "America/New_York" {
		t.Fatalf("DisplayTimezone = %q, want America/New_York", cfg.DisplayTimezone)
	}
}

func TestLoadRejectsInvalidDisplayTimezone(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DISPLAY_TIMEZONE", "NotA/Real_Timezone")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid timezone")
	}
}
