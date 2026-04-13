package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-container-hub/internal/model"
)

func TestNewAutoProviderSupportsLocal(t *testing.T) {
	t.Parallel()

	provider, err := NewAutoProvider(LocalProviderName, nil)
	if err != nil {
		t.Fatalf("NewAutoProvider() error = %v", err)
	}
	if provider.Name() != LocalProviderName {
		t.Fatalf("provider.Name() = %q, want %q", provider.Name(), LocalProviderName)
	}
}

func TestLocalProviderCreateExecStopRemove(t *testing.T) {
	t.Parallel()

	provider := NewLocalProvider()
	root := t.TempDir()
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	info, err := provider.Create(context.Background(), CreateOptions{
		Name:  "demo",
		Cwd:   DefaultMountPath,
		Env:   map[string]string{"LOCAL_TEST_VALUE": "ok"},
		Image: "",
		Mounts: []model.Mount{{
			Source:      root,
			Destination: DefaultMountPath,
		}},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if info.Image != LocalImageRef {
		t.Fatalf("Create() image = %q, want %q", info.Image, LocalImageRef)
	}

	if _, err := provider.Start(context.Background(), "demo"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	result, err := provider.Exec(context.Background(), "demo", ExecOptions{
		Command: "/bin/sh",
		Args:    []string{"-lc", "printf '%s:%s' \"$PWD\" \"$LOCAL_TEST_VALUE\""},
		Cwd:     DefaultMountPath,
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if result.Stdout != realRoot+":ok" {
		t.Fatalf("Exec() stdout = %q, want %q", result.Stdout, realRoot+":ok")
	}

	if err := provider.Stop(context.Background(), "demo", 0); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := provider.Remove(context.Background(), "demo"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := provider.Inspect(context.Background(), "demo"); !errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("Inspect() error = %v, want ErrContainerNotFound", err)
	}
}

func TestLocalProviderExecTimesOut(t *testing.T) {
	t.Parallel()

	provider := NewLocalProvider()
	root := t.TempDir()
	_, err := provider.Create(context.Background(), CreateOptions{
		Name: "timeout-demo",
		Cwd:  DefaultMountPath,
		Mounts: []model.Mount{{
			Source:      root,
			Destination: DefaultMountPath,
		}},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := provider.Start(context.Background(), "timeout-demo"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := provider.Exec(ctx, "timeout-demo", ExecOptions{
		Command: "/bin/sh",
		Args:    []string{"-lc", "sleep 1"},
		Cwd:     DefaultMountPath,
	})
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if !result.TimedOut {
		t.Fatal("Exec().TimedOut = false, want true")
	}
	if result.ExitCode != 124 {
		t.Fatalf("Exec().ExitCode = %d, want 124", result.ExitCode)
	}
}

func TestLocalProviderExecUsesHostWorkingDirectoryWithoutMounts(t *testing.T) {
	t.Parallel()

	provider := NewLocalProvider()
	root := t.TempDir()
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}

	if _, err := provider.Create(context.Background(), CreateOptions{
		Name: "host-cwd-demo",
		Cwd:  realRoot,
		Env:  map[string]string{"LOCAL_TEST_VALUE": "host"},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := provider.Start(context.Background(), "host-cwd-demo"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	result, err := provider.Exec(context.Background(), "host-cwd-demo", ExecOptions{
		Command: "/bin/sh",
		Args:    []string{"-lc", "printf '%s:%s' \"$PWD\" \"$LOCAL_TEST_VALUE\""},
		Cwd:     realRoot,
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if result.Stdout != realRoot+":host" {
		t.Fatalf("Exec() stdout = %q, want %q", result.Stdout, realRoot+":host")
	}
}

func TestLocalProviderExecRejectsMissingWorkingDirectory(t *testing.T) {
	t.Parallel()

	provider := NewLocalProvider()
	root := t.TempDir()

	if _, err := provider.Create(context.Background(), CreateOptions{
		Name: "missing-cwd-demo",
		Cwd:  root,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := provider.Start(context.Background(), "missing-cwd-demo"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	_, err := provider.Exec(context.Background(), "missing-cwd-demo", ExecOptions{
		Command: "/bin/sh",
		Args:    []string{"-lc", "pwd"},
		Cwd:     filepath.Join(root, "does-not-exist"),
	})
	if err == nil {
		t.Fatal("Exec() error = nil, want missing directory error")
	}
	if !strings.Contains(err.Error(), "does not exist or is not a directory") {
		t.Fatalf("Exec() error = %q, want missing directory detail", err.Error())
	}
}
