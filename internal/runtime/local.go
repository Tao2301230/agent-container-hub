package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agent-container-hub/internal/model"
)

type LocalProvider struct {
	mu         sync.RWMutex
	containers map[string]*localContainer
}

type localContainer struct {
	info   ContainerInfo
	cwd    string
	env    map[string]string
	mounts []model.Mount
}

func NewLocalProvider() *LocalProvider {
	return &LocalProvider{
		containers: make(map[string]*localContainer),
	}
}

func (p *LocalProvider) Name() string {
	return LocalProviderName
}

func (p *LocalProvider) Create(_ context.Context, opts CreateOptions) (ContainerInfo, error) {
	if err := model.ValidateEnvMap(opts.Env, "environment variable"); err != nil {
		return ContainerInfo{}, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.containers[opts.Name]; exists {
		return ContainerInfo{}, ErrContainerExists
	}

	info := ContainerInfo{
		ID:        opts.Name,
		Name:      opts.Name,
		Image:     strings.TrimSpace(opts.Image),
		State:     ContainerStopped,
		Labels:    model.CloneMap(opts.Labels),
		CreatedAt: time.Now().UTC(),
	}
	if info.Image == "" {
		info.Image = LocalImageRef
	}

	p.containers[opts.Name] = &localContainer{
		info:   info,
		cwd:    strings.TrimSpace(opts.Cwd),
		env:    model.CloneMap(opts.Env),
		mounts: append([]model.Mount(nil), opts.Mounts...),
	}
	return info, nil
}

func (p *LocalProvider) Start(_ context.Context, containerID string) (ContainerInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	container, ok := p.lookupLocked(containerID)
	if !ok {
		return ContainerInfo{}, ErrContainerNotFound
	}
	container.info.State = ContainerRunning
	return container.info, nil
}

func (p *LocalProvider) Exec(ctx context.Context, containerID string, opts ExecOptions) (ExecResult, error) {
	p.mu.RLock()
	container, ok := p.lookupLocked(containerID)
	if !ok {
		p.mu.RUnlock()
		return ExecResult{}, ErrContainerNotFound
	}
	if container.info.State != ContainerRunning {
		p.mu.RUnlock()
		return ExecResult{}, ErrContainerNotRunning
	}
	containerCopy := *container
	containerCopy.env = model.CloneMap(container.env)
	containerCopy.mounts = append([]model.Mount(nil), container.mounts...)
	p.mu.RUnlock()

	workingDir, err := resolveLocalWorkingDir(containerCopy, opts.Cwd)
	if err != nil {
		return ExecResult{}, err
	}

	startedAt := time.Now().UTC()
	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)
	cmd.Dir = workingDir
	cmd.Env = mergeEnviron(os.Environ(), containerCopy.env)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	finishedAt := time.Now().UTC()
	result := ExecResult{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = 124
		return result, nil
	}
	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return ExecResult{}, err
}

func (p *LocalProvider) Build(_ context.Context, _ BuildOptions) (BuildResult, error) {
	return BuildResult{}, fmt.Errorf("local runtime does not support image builds")
}

func (p *LocalProvider) Stop(_ context.Context, containerID string, _ time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	container, ok := p.lookupLocked(containerID)
	if !ok {
		return ErrContainerNotFound
	}
	container.info.State = ContainerStopped
	return nil
}

func (p *LocalProvider) Remove(_ context.Context, containerID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	container, ok := p.lookupLocked(containerID)
	if !ok {
		return ErrContainerNotFound
	}
	delete(p.containers, container.info.ID)
	return nil
}

func (p *LocalProvider) Inspect(_ context.Context, containerID string) (ContainerInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	container, ok := p.lookupLocked(containerID)
	if !ok {
		return ContainerInfo{}, ErrContainerNotFound
	}
	return container.info, nil
}

func (p *LocalProvider) InspectImage(_ context.Context, imageRef string) (ImageInfo, error) {
	imageRef = strings.TrimSpace(imageRef)
	if imageRef == "" {
		imageRef = LocalImageRef
	}
	return ImageInfo{
		ID:        "local-host",
		Ref:       imageRef,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (p *LocalProvider) ListByLabel(_ context.Context, key, value string) ([]ContainerInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var infos []ContainerInfo
	for _, container := range p.containers {
		if container.info.Labels[key] == value {
			infos = append(infos, container.info)
		}
	}
	return infos, nil
}

func (p *LocalProvider) ListImageMetadata(context.Context) (map[string]ImageMetadata, error) {
	return map[string]ImageMetadata{
		LocalImageRef: {
			Ref:       LocalImageRef,
			CreatedAt: time.Now().UTC(),
		},
	}, nil
}

func (p *LocalProvider) lookupLocked(idOrName string) (*localContainer, bool) {
	if container, ok := p.containers[idOrName]; ok {
		return container, true
	}
	for _, container := range p.containers {
		if container.info.Name == idOrName {
			return container, true
		}
	}
	return nil, false
}

func resolveLocalWorkingDir(container localContainer, execCwd string) (string, error) {
	target := strings.TrimSpace(execCwd)
	if target == "" {
		target = container.cwd
	}
	if target == "" {
		return defaultLocalWorkingDir()
	}
	if hostDir, ok, err := resolveExistingLocalDir(target); ok || err != nil {
		return hostDir, err
	}
	if hostDir, ok, err := resolveMountedLocalDir(container.mounts, target); ok || err != nil {
		return hostDir, err
	}
	return "", fmt.Errorf("local runtime working directory %q does not exist or is not a directory", target)
}

func defaultLocalWorkingDir() (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return ensureLocalDir(workingDir, workingDir)
}

func resolveExistingLocalDir(target string) (string, bool, error) {
	clean := filepath.Clean(strings.TrimSpace(target))
	if clean == "" {
		return "", false, nil
	}
	if !filepath.IsAbs(clean) {
		absolute, err := filepath.Abs(clean)
		if err != nil {
			return "", false, err
		}
		clean = absolute
	}
	dir, err := ensureLocalDir(clean, target)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return dir, true, nil
}

func resolveMountedLocalDir(mounts []model.Mount, target string) (string, bool, error) {
	target = normalizeMountTarget(target)
	if target == "" {
		return "", false, nil
	}
	bestIndex := -1
	bestLength := -1
	for index, mount := range mounts {
		destination := normalizeMountTarget(mount.Destination)
		if destination == "" {
			continue
		}
		if target != destination && !strings.HasPrefix(target, destination+"/") {
			continue
		}
		if len(destination) > bestLength {
			bestIndex = index
			bestLength = len(destination)
		}
	}
	if bestIndex < 0 {
		return "", false, nil
	}
	selected := mounts[bestIndex]
	destination := normalizeMountTarget(selected.Destination)
	relative := strings.TrimPrefix(target, destination)
	relative = strings.TrimPrefix(relative, "/")
	hostPath := selected.Source
	if relative != "" {
		hostPath = filepath.Join(hostPath, filepath.FromSlash(relative))
	}
	dir, err := ensureLocalDir(hostPath, target)
	if err != nil {
		return "", false, err
	}
	return dir, true, nil
}

func ensureLocalDir(path string, original string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fs.ErrNotExist
		}
		return "", fmt.Errorf("inspect local working directory %q: %w", original, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("local runtime working directory %q is not a directory", original)
	}
	return filepath.Clean(path), nil
}

func normalizeMountTarget(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(trimmed))
}

func mergeEnviron(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return append([]string(nil), base...)
	}
	merged := make(map[string]string, len(base)+len(overrides))
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		merged[key] = value
	}
	for key, value := range overrides {
		merged[key] = value
	}
	out := make([]string, 0, len(merged))
	for key, value := range merged {
		out = append(out, key+"="+value)
	}
	return out
}
