package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"agent-container-hub/internal/config"
	"agent-container-hub/internal/model"
	"agent-container-hub/internal/runtime"
	"agent-container-hub/internal/store"
)

const (
	BuildEventSnapshot         = "snapshot"
	BuildEventStatus           = "status"
	BuildEventLog              = "log"
	BuildEventComplete         = "complete"
	discoveredImageBuildOutput = "discovered existing host image during startup sync"
	BuildTargetDefault         = "build"
	BuildTargetCN              = "build-cn"
	buildSubscriberBufferSize  = 32
	buildEventSendTimeout      = 250 * time.Millisecond
	buildSmokeCheckTimeout     = 30 * time.Second
)

var makeTargetPatterns = map[string]*regexp.Regexp{
	BuildTargetDefault: regexp.MustCompile(`(?m)^build\s*:(?:$|\s)`),
	BuildTargetCN:      regexp.MustCompile(`(?m)^build-cn\s*:(?:$|\s)`),
}

type BuildEvent struct {
	Type  string
	Job   *model.BuildJob
	Chunk string
}

type BuildService struct {
	lifecycleCtx    context.Context
	cfg             config.Config
	store           store.BuildJobStore
	envs            store.EnvironmentStore
	runtime         runtime.Provider
	logger          *slog.Logger
	locks           *namedLock
	mu              sync.RWMutex
	activeJobs      map[string]*activeBuildJob
	activeJobsByEnv map[string]string
}

type activeBuildJob struct {
	mu               sync.RWMutex
	job              *model.BuildJob
	subscribers      map[int]chan BuildEvent
	nextSubscriberID int
	releaseLock      func()
}

type buildOutputSink struct {
	write func(string)
}

func (s buildOutputSink) Write(payload []byte) (int, error) {
	if len(payload) > 0 && s.write != nil {
		s.write(string(payload))
	}
	return len(payload), nil
}

func NewBuildService(lifecycleCtx context.Context, cfg config.Config, st store.BuildJobStore, envs store.EnvironmentStore, provider runtime.Provider, logger *slog.Logger) *BuildService {
	if logger == nil {
		logger = slog.Default()
	}
	if lifecycleCtx == nil {
		lifecycleCtx = context.Background()
	}
	return &BuildService{
		lifecycleCtx:    lifecycleCtx,
		cfg:             cfg,
		store:           st,
		envs:            envs,
		runtime:         provider,
		logger:          logger,
		locks:           newNamedLock(),
		activeJobs:      make(map[string]*activeBuildJob),
		activeJobsByEnv: make(map[string]string),
	}
}

func (s *BuildService) StartBuildJob(ctx context.Context, name string, req model.BuildEnvironmentRequest) (*model.BuildJob, error) {
	environment, release, err := s.prepareBuild(ctx, name)
	if err != nil {
		return nil, err
	}
	buildContexts, err := s.resolveBuildContexts(environment)
	if err != nil {
		release()
		return nil, err
	}
	target, useMake, err := s.resolveBuildTarget(environment.Name, req.Target)
	if err != nil {
		release()
		return nil, err
	}

	jobID, err := generateID()
	if err != nil {
		release()
		return nil, err
	}
	job := &model.BuildJob{
		ID:              "build-" + jobID,
		EnvironmentName: environment.Name,
		ImageRef:        environment.ImageRef(),
		Target:          target,
		Status:          model.BuildJobStatusBuilding,
		StartedAt:       time.Now().UTC(),
	}

	active := &activeBuildJob{
		job:         job,
		subscribers: make(map[int]chan BuildEvent),
		releaseLock: release,
	}
	s.registerActiveJob(active)

	if useMake && (target != BuildTargetDefault || makeCommandAvailable()) {
		go s.runMakeBuildJob(s.newBuildJobContext(), active, environment.Clone(), s.environmentConfigDir(environment.Name), target, buildContexts)
		return job.Clone(), nil
	}

	buildDir := filepath.Join(s.cfg.BuildRoot, job.ID)
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		s.unregisterActiveJob(job.ID, environment.Name)
		release()
		return nil, fmt.Errorf("create build dir: %w", err)
	}

	dockerfilePath := filepath.Join(buildDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(environment.Build.Dockerfile), 0o644); err != nil {
		s.unregisterActiveJob(job.ID, environment.Name)
		release()
		s.warnIfCleanupFails("remove build dir after dockerfile write failed", buildDir, os.RemoveAll(buildDir))
		return nil, fmt.Errorf("write dockerfile: %w", err)
	}

	go s.runDirectBuildJob(s.newBuildJobContext(), active, environment.Clone(), buildDir, dockerfilePath, buildContexts)
	return job.Clone(), nil
}

func (s *BuildService) BuildEnvironment(ctx context.Context, name string) (*model.BuildJob, error) {
	started, err := s.StartBuildJob(ctx, name, model.BuildEnvironmentRequest{})
	if err != nil {
		return nil, err
	}
	snapshot, events, cancel, err := s.SubscribeBuildJob(ctx, started.ID)
	if err != nil {
		return nil, err
	}
	defer cancel()
	if isTerminalBuildStatus(snapshot.Status) || events == nil {
		return snapshot, nil
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event, ok := <-events:
			if !ok {
				return s.GetBuildJob(ctx, started.ID)
			}
			if event.Type == BuildEventComplete && event.Job != nil {
				return event.Job, nil
			}
		}
	}
}

func (s *BuildService) GetBuildJob(ctx context.Context, id string) (*model.BuildJob, error) {
	if active := s.getActiveJob(id); active != nil {
		return active.snapshot(), nil
	}
	job, err := s.store.GetBuildJob(ctx, id)
	if err != nil {
		return nil, err
	}
	return job.Clone(), nil
}

func (s *BuildService) LatestBuildJob(ctx context.Context, environmentName string) (*model.BuildJob, error) {
	environmentName = strings.TrimSpace(environmentName)
	if environmentName == "" {
		return nil, nil
	}
	if active := s.getActiveJobForEnvironment(environmentName); active != nil {
		return active.snapshot(), nil
	}
	jobs, err := s.store.ListBuildJobs(ctx, environmentName)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, nil
	}
	return jobs[0].Clone(), nil
}

func (s *BuildService) ReconcileExistingImages(ctx context.Context) error {
	environments, err := s.envs.ListEnvironments(ctx)
	if err != nil {
		return err
	}
	for _, environment := range environments {
		if environment == nil {
			continue
		}
		if err := s.reconcileEnvironmentImage(ctx, environment); err != nil {
			s.logger.Error("reconcile environment image failed",
				"environment", environment.Name,
				"image", environment.ImageRef(),
				"error", err,
			)
		}
	}
	return nil
}

func (s *BuildService) SubscribeBuildJob(_ context.Context, id string) (*model.BuildJob, <-chan BuildEvent, func(), error) {
	if active := s.getActiveJob(id); active != nil {
		return active.subscribe()
	}
	job, err := s.store.GetBuildJob(context.Background(), id)
	if err != nil {
		return nil, nil, nil, err
	}
	cancel := func() {}
	return job.Clone(), nil, cancel, nil
}

func (s *BuildService) reconcileEnvironmentImage(ctx context.Context, environment *model.Environment) error {
	imageRef := environment.ImageRef()
	if imageRef == "" {
		return nil
	}
	image, err := s.runtime.InspectImage(ctx, imageRef)
	if err != nil {
		if errors.Is(err, runtime.ErrImageNotFound) {
			return nil
		}
		return err
	}

	latest, err := s.LatestBuildJob(ctx, environment.Name)
	if err != nil {
		return err
	}
	if latest != nil && latest.ImageRef == imageRef {
		return nil
	}

	jobID, err := generateID()
	if err != nil {
		return err
	}
	createdAt := image.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	job := &model.BuildJob{
		ID:              "build-" + jobID,
		EnvironmentName: environment.Name,
		ImageRef:        imageRef,
		Status:          model.BuildJobStatusSucceeded,
		Output:          discoveredImageBuildOutput,
		StartedAt:       createdAt,
		FinishedAt:      createdAt,
	}
	if err := s.store.SaveBuildJob(ctx, job); err != nil {
		return err
	}
	s.logger.Info("reconciled existing host image",
		"environment", environment.Name,
		"image", imageRef,
		"image_id", image.ID,
		"build_job_id", job.ID,
	)
	return nil
}

func (s *BuildService) prepareBuild(ctx context.Context, name string) (*model.Environment, func(), error) {
	name = strings.TrimSpace(name)
	if err := model.ValidateEnvironmentName(name); err != nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrValidation, err)
	}
	environment, err := s.envs.GetEnvironment(ctx, name)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(environment.Build.Dockerfile) == "" {
		return nil, nil, fmt.Errorf("%w: build.dockerfile is required", ErrValidation)
	}
	if err := model.ValidateEnvMap(environment.Build.BuildArgs, "build.build_args"); err != nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrValidation, err)
	}
	if err := model.ValidateBuildContextMap(environment.Build.BuildContexts); err != nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrValidation, err)
	}
	release, acquired := s.locks.tryLock(environment.Name)
	if !acquired {
		return nil, nil, fmt.Errorf("%w: build already in progress for environment %q", ErrConflict, environment.Name)
	}
	return environment, release, nil
}

func (s *BuildService) runDirectBuildJob(ctx context.Context, active *activeBuildJob, environment *model.Environment, buildDir string, dockerfilePath string, buildContexts map[string]string) {
	defer func() {
		s.warnIfCleanupFails("remove build dir after direct build", buildDir, os.RemoveAll(buildDir))
	}()

	result, err := s.runtime.Build(ctx, runtime.BuildOptions{
		ContextDir:     buildDir,
		DockerfilePath: dockerfilePath,
		DockerfileBody: environment.Build.Dockerfile,
		Image:          environment.ImageRef(),
		BuildArgs:      model.CloneMap(environment.Build.BuildArgs),
		BuildContexts:  model.CloneMap(buildContexts),
		OutputSink: buildOutputSink{write: func(chunk string) {
			s.appendBuildOutput(active, chunk)
		}},
	})
	if result.Output != "" {
		s.setBuildOutput(active, result.Output)
	}
	if err != nil {
		s.logger.Error("environment build failed",
			"environment", environment.Name,
			"image", environment.ImageRef(),
			"build_job_id", active.job.ID,
			"error", err,
		)
		s.finishBuildJob(active, model.BuildJobStatusFailed, err.Error(), result.FinishedAt)
		return
	}

	if strings.TrimSpace(environment.Build.SmokeCommand) != "" {
		s.setBuildStatus(active, model.BuildJobStatusSmokeChecking)
		if smokeErr := s.runSmokeCheck(ctx, environment); smokeErr != nil {
			s.logger.Error("environment smoke check failed",
				"environment", environment.Name,
				"image", environment.ImageRef(),
				"build_job_id", active.job.ID,
				"error", smokeErr,
			)
			s.finishBuildJob(active, model.BuildJobStatusFailed, smokeErr.Error(), time.Now().UTC())
			return
		}
	}

	s.logger.Info("environment built", "environment", environment.Name, "image", environment.ImageRef())
	s.finishBuildJob(active, model.BuildJobStatusSucceeded, "", time.Now().UTC())
}

func (s *BuildService) runMakeBuildJob(ctx context.Context, active *activeBuildJob, environment *model.Environment, workingDir, target string, buildContexts map[string]string) {
	result, err := s.runMakeBuild(ctx, environment, workingDir, target, buildContexts, buildOutputSink{write: func(chunk string) {
		s.appendBuildOutput(active, chunk)
	}})
	if result.Output != "" {
		s.setBuildOutput(active, result.Output)
	}
	if err != nil {
		s.logger.Error("environment make build failed",
			"environment", environment.Name,
			"image", environment.ImageRef(),
			"target", target,
			"build_job_id", active.job.ID,
			"error", err,
		)
		s.finishBuildJob(active, model.BuildJobStatusFailed, err.Error(), result.FinishedAt)
		return
	}

	if strings.TrimSpace(environment.Build.SmokeCommand) != "" {
		s.setBuildStatus(active, model.BuildJobStatusSmokeChecking)
		if smokeErr := s.runSmokeCheck(ctx, environment); smokeErr != nil {
			s.logger.Error("environment smoke check failed",
				"environment", environment.Name,
				"image", environment.ImageRef(),
				"target", target,
				"build_job_id", active.job.ID,
				"error", smokeErr,
			)
			s.finishBuildJob(active, model.BuildJobStatusFailed, smokeErr.Error(), time.Now().UTC())
			return
		}
	}

	s.logger.Info("environment built", "environment", environment.Name, "image", environment.ImageRef(), "target", target)
	s.finishBuildJob(active, model.BuildJobStatusSucceeded, "", time.Now().UTC())
}

func (s *BuildService) registerActiveJob(active *activeBuildJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeJobs[active.job.ID] = active
	s.activeJobsByEnv[active.job.EnvironmentName] = active.job.ID
}

func (s *BuildService) unregisterActiveJob(jobID, environmentName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.activeJobs, strings.TrimSpace(jobID))
	if s.activeJobsByEnv[strings.TrimSpace(environmentName)] == strings.TrimSpace(jobID) {
		delete(s.activeJobsByEnv, strings.TrimSpace(environmentName))
	}
}

func (s *BuildService) getActiveJob(id string) *activeBuildJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeJobs[strings.TrimSpace(id)]
}

func (s *BuildService) getActiveJobForEnvironment(environmentName string) *activeBuildJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobID := s.activeJobsByEnv[strings.TrimSpace(environmentName)]
	if jobID == "" {
		return nil
	}
	return s.activeJobs[jobID]
}

func (s *BuildService) appendBuildOutput(active *activeBuildJob, chunk string) {
	if chunk == "" {
		return
	}
	subscribers := active.appendOutput(chunk)
	for _, subscriber := range subscribers {
		s.sendBuildEvent(subscriber, BuildEvent{Type: BuildEventLog, Chunk: chunk})
	}
}

func (s *BuildService) setBuildOutput(active *activeBuildJob, output string) {
	if output == "" {
		return
	}
	active.mu.Lock()
	active.job.Output = output
	active.mu.Unlock()
}

func (s *BuildService) setBuildStatus(active *activeBuildJob, status model.BuildJobStatus) {
	snapshot, subscribers := active.setStatus(status)
	for _, subscriber := range subscribers {
		s.sendBuildEvent(subscriber, BuildEvent{Type: BuildEventStatus, Job: snapshot})
	}
}

func (s *BuildService) finishBuildJob(active *activeBuildJob, status model.BuildJobStatus, errMessage string, finishedAt time.Time) {
	snapshot, subscribers, releaseLock := active.finish(status, errMessage, finishedAt)
	if saveErr := s.store.SaveBuildJob(context.Background(), snapshot.Clone()); saveErr != nil {
		s.logger.Error("save build job failed", "build_job_id", snapshot.ID, "error", saveErr)
	}

	for _, subscriber := range subscribers {
		s.sendBuildEvent(subscriber, BuildEvent{Type: BuildEventComplete, Job: snapshot})
		close(subscriber)
	}

	s.mu.Lock()
	delete(s.activeJobs, snapshot.ID)
	if s.activeJobsByEnv[snapshot.EnvironmentName] == snapshot.ID {
		delete(s.activeJobsByEnv, snapshot.EnvironmentName)
	}
	s.mu.Unlock()

	if releaseLock != nil {
		releaseLock()
	}
}

func (s *BuildService) runSmokeCheck(ctx context.Context, environment *model.Environment) error {
	name, err := generateID()
	if err != nil {
		return err
	}
	workspace := filepath.Join(s.cfg.BuildRoot, "smoke-"+name)
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return err
	}
	defer func() {
		s.warnIfCleanupFails("remove smoke workspace after build", workspace, os.RemoveAll(workspace))
	}()

	info, err := s.runtime.Create(ctx, runtime.CreateOptions{
		Name:  "smoke-" + name,
		Image: environment.ImageRef(),
		Cwd:   sessionDefaultCwd("", environment.DefaultCwd),
		Env:   model.CloneMap(environment.DefaultEnv),
		Mounts: []model.Mount{{
			Source:      workspace,
			Destination: runtime.DefaultMountPath,
		}},
		Resources: environment.Resources,
		Labels: map[string]string{
			runtime.ManagedByLabel: "agent-container-hub",
		},
	})
	if err != nil {
		return err
	}
	defer func() {
		s.warnIfCleanupFails("remove smoke container after build", info.ID, s.runtime.Remove(context.Background(), info.ID))
	}()

	if _, err := s.runtime.Start(ctx, info.ID); err != nil {
		return err
	}
	result, err := s.runtime.Exec(ctx, info.ID, runtime.ExecOptions{
		Command: environment.Build.SmokeCommand,
		Args:    append([]string(nil), environment.Build.SmokeArgs...),
		Cwd:     sessionDefaultCwd("", environment.DefaultCwd),
		Timeout: buildSmokeCheckTimeout,
	})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("smoke check failed with exit code %d", result.ExitCode)
	}
	return nil
}

func (s *BuildService) resolveBuildTarget(environmentName, requestedTarget string) (string, bool, error) {
	availableTargets, err := discoverAvailableBuildTargets(s.environmentConfigDir(environmentName))
	if err != nil {
		return "", false, err
	}
	requestedTarget = strings.TrimSpace(requestedTarget)
	if requestedTarget == "" {
		if containsString(availableTargets, BuildTargetDefault) {
			return BuildTargetDefault, true, nil
		}
		return "", false, nil
	}
	if !isSupportedBuildTarget(requestedTarget) {
		return "", false, fmt.Errorf("%w: unsupported build target %q", ErrValidation, requestedTarget)
	}
	if !containsString(availableTargets, requestedTarget) {
		return "", false, fmt.Errorf("%w: build target %q is not available for environment %q", ErrValidation, requestedTarget, environmentName)
	}
	return requestedTarget, true, nil
}

func (s *BuildService) environmentConfigDir(name string) string {
	return filepath.Join(s.cfg.ConfigRoot, "environments", strings.TrimSpace(name))
}

func (s *BuildService) runMakeBuild(ctx context.Context, environment *model.Environment, workingDir, target string, buildContexts map[string]string, outputSink io.Writer) (runtime.BuildResult, error) {
	startedAt := time.Now().UTC()
	makePath, lookupErr := exec.LookPath("make")
	if lookupErr != nil {
		finishedAt := time.Now().UTC()
		return runtime.BuildResult{
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
		}, fmt.Errorf("run make %s: make is required for this build target but was not found in PATH", target)
	}
	cmd := exec.CommandContext(ctx, makePath, target)
	cmd.Dir = workingDir
	cmd.Env = buildCommandEnv(environment, buildContexts, s.runtime.Name())

	var output bytes.Buffer
	writer := io.MultiWriter(&output, outputSink)
	cmd.Stdout = writer
	cmd.Stderr = writer

	err := cmd.Run()
	finishedAt := time.Now().UTC()
	result := runtime.BuildResult{
		Output:     strings.TrimSpace(output.String()),
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	}
	if err != nil {
		return result, fmt.Errorf("run make %s: %w", target, err)
	}
	return result, nil
}

func buildCommandEnv(environment *model.Environment, buildContexts map[string]string, containerEngine string) []string {
	env := append([]string(nil), os.Environ()...)
	env = append(env, "IMAGE_NAME="+strings.TrimSpace(environment.ImageRepository))
	env = append(env, "TAG="+strings.TrimSpace(environment.ImageTag))
	containerEngine = strings.TrimSpace(containerEngine)
	if containerEngine == "" {
		containerEngine = "docker"
	}
	env = append(env, "CONTAINER_ENGINE="+containerEngine)
	for key, value := range environment.Build.BuildArgs {
		env = append(env, key+"="+value)
	}
	if len(buildContexts) > 0 {
		env = append(env, "DOCKER_BUILDKIT=1")
		for key, value := range buildContexts {
			env = append(env, buildContextEnvName(key)+"="+value)
		}
	}
	return env
}

func makeCommandAvailable() bool {
	_, err := exec.LookPath("make")
	return err == nil
}

func (s *BuildService) resolveBuildContexts(environment *model.Environment) (map[string]string, error) {
	if environment == nil || len(environment.Build.BuildContexts) == 0 {
		return nil, nil
	}
	environmentDir := s.environmentConfigDir(environment.Name)
	resolved := make(map[string]string, len(environment.Build.BuildContexts))
	for key, rawPath := range environment.Build.BuildContexts {
		candidate := strings.TrimSpace(rawPath)
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(environmentDir, candidate)
		}
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			return nil, fmt.Errorf("%w: resolve build context %q path %q: %v", ErrValidation, key, rawPath, err)
		}
		info, err := os.Stat(absolute)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("%w: build context %q path %q resolved to %q, which does not exist", ErrValidation, key, rawPath, absolute)
			}
			return nil, fmt.Errorf("%w: inspect build context %q path %q: %v", ErrValidation, key, absolute, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("%w: build context %q path %q must be a directory", ErrValidation, key, absolute)
		}
		resolved[key] = absolute
	}
	return resolved, nil
}

func buildContextEnvName(name string) string {
	var builder strings.Builder
	builder.WriteString("BUILD_CONTEXT_")
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteByte(byte(r - 'a' + 'A'))
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	return builder.String()
}

func AvailableBuildTargets(configRoot, environmentName string) ([]string, error) {
	return discoverAvailableBuildTargets(filepath.Join(configRoot, "environments", strings.TrimSpace(environmentName)))
}

func discoverAvailableBuildTargets(environmentDir string) ([]string, error) {
	payload, err := os.ReadFile(filepath.Join(environmentDir, "Makefile"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read environment Makefile: %w", err)
	}
	content := string(payload)
	targets := make([]string, 0, len(makeTargetPatterns))
	for _, target := range []string{BuildTargetDefault, BuildTargetCN} {
		if makeTargetPatterns[target].MatchString(content) {
			targets = append(targets, target)
		}
	}
	return targets, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isSupportedBuildTarget(target string) bool {
	switch target {
	case BuildTargetDefault, BuildTargetCN:
		return true
	default:
		return false
	}
}

func (j *activeBuildJob) snapshot() *model.BuildJob {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.job.Clone()
}

func (j *activeBuildJob) subscribe() (*model.BuildJob, <-chan BuildEvent, func(), error) {
	ch := make(chan BuildEvent, buildSubscriberBufferSize)
	j.mu.Lock()
	id := j.nextSubscriberID
	j.nextSubscriberID++
	j.subscribers[id] = ch
	snapshot := j.job.Clone()
	j.mu.Unlock()

	cancel := func() {
		j.mu.Lock()
		subscriber, ok := j.subscribers[id]
		if ok {
			delete(j.subscribers, id)
		}
		j.mu.Unlock()
		if ok {
			close(subscriber)
		}
	}
	return snapshot, ch, cancel, nil
}

func (j *activeBuildJob) appendOutput(chunk string) []chan BuildEvent {
	j.mu.Lock()
	j.job.Output += chunk
	subscribers := j.subscriberSnapshotLocked()
	j.mu.Unlock()
	return subscribers
}

func (j *activeBuildJob) setStatus(status model.BuildJobStatus) (*model.BuildJob, []chan BuildEvent) {
	j.mu.Lock()
	j.job.Status = status
	snapshot := j.job.Clone()
	subscribers := j.subscriberSnapshotLocked()
	j.mu.Unlock()
	return snapshot, subscribers
}

func (j *activeBuildJob) finish(status model.BuildJobStatus, errMessage string, finishedAt time.Time) (*model.BuildJob, []chan BuildEvent, func()) {
	j.mu.Lock()
	j.job.Status = status
	j.job.Error = errMessage
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}
	j.job.FinishedAt = finishedAt
	snapshot := j.job.Clone()
	subscribers := j.subscriberSnapshotLocked()
	j.subscribers = make(map[int]chan BuildEvent)
	releaseLock := j.releaseLock
	j.releaseLock = nil
	j.mu.Unlock()
	return snapshot, subscribers, releaseLock
}

func (j *activeBuildJob) subscriberSnapshotLocked() []chan BuildEvent {
	subscribers := make([]chan BuildEvent, 0, len(j.subscribers))
	for _, subscriber := range j.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	return subscribers
}

func isTerminalBuildStatus(status model.BuildJobStatus) bool {
	switch status {
	case model.BuildJobStatusSucceeded, model.BuildJobStatusFailed:
		return true
	default:
		return false
	}
}

func (s *BuildService) sendBuildEvent(ch chan BuildEvent, event BuildEvent) {
	select {
	case ch <- event:
	default:
		// Slow subscribers are allowed to drop incremental updates; they will
		// resync from the next snapshot or terminal event.
		if event.Type == BuildEventLog {
			return
		}
		timer := time.NewTimer(buildEventSendTimeout)
		defer timer.Stop()
		defer func() {
			_ = recover()
		}()
		select {
		case ch <- event:
		case <-timer.C:
			s.logger.Warn("dropping build event for slow subscriber", "event_type", event.Type)
		}
	}
}

func (s *BuildService) newBuildJobContext() context.Context {
	return s.lifecycleCtx
}

func (s *BuildService) warnIfCleanupFails(action, target string, err error) {
	if err == nil {
		return
	}
	s.logger.Warn(action, "target", target, "error", err)
}

var _ io.Writer = buildOutputSink{}
