package sandbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agent-container-hub/internal/config"
	"agent-container-hub/internal/model"
	"agent-container-hub/internal/runtime"
	"agent-container-hub/internal/store"
)

var (
	ErrValidation = errors.New("validation failed")
	ErrBusy       = errors.New("session busy")
	ErrConflict   = errors.New("session configuration conflict")
)

const sessionStopTimeout = 5 * time.Second

type SessionService struct {
	cfg     config.Config
	store   store.AppStore
	envs    store.EnvironmentStore
	runtime runtime.Provider
	logger  *slog.Logger
	locks   *namedLock
}

func NewSessionService(cfg config.Config, st store.AppStore, envs store.EnvironmentStore, provider runtime.Provider, logger *slog.Logger) *SessionService {
	if logger == nil {
		logger = slog.Default()
	}
	return &SessionService{
		cfg:     cfg,
		store:   st,
		envs:    envs,
		runtime: provider,
		logger:  logger,
		locks:   newNamedLock(),
	}
}

func (s *SessionService) Create(ctx context.Context, req model.CreateSessionRequest) (*model.CreateSessionResult, error) {
	startedAt := time.Now().UTC()
	environmentName := strings.TrimSpace(req.EnvironmentName)
	if err := model.ValidateEnvironmentName(environmentName); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrValidation, err)
	}

	environment, err := s.envs.GetEnvironment(ctx, environmentName)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("%w: environment not found", store.ErrNotFound)
		}
		return nil, err
	}
	if !environment.Enabled {
		return nil, fmt.Errorf("%w: environment is disabled", ErrValidation)
	}
	if !runtime.IsLocalRuntime(s.runtime.Name()) {
		available, err := inspectLocalImageAvailability(ctx, s.runtime, environment.ImageRef(), s.logger)
		if err != nil {
			s.logger.Error("image availability check failed",
				"environment", environmentName,
				"image", environment.ImageRef(),
				"error", err,
			)
			return nil, err
		}
		if !available {
			return nil, fmt.Errorf("%w: image %q not found locally", ErrValidation, environment.ImageRef())
		}
	}
	if err := model.ValidateEnvMap(environment.DefaultEnv, "default_env"); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrValidation, err)
	}
	if err := model.ValidateEnvMap(req.Env, "env"); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrValidation, err)
	}
	env := mergedSessionEnv(environment.DefaultEnv, req.Env)

	req.SessionID = normalizeSessionID(req.SessionID)
	if req.SessionID == "" {
		token, err := generateID()
		if err != nil {
			return nil, fmt.Errorf("generate session token: %w", err)
		}
		req.SessionID = "session-" + token
	}
	if err := validateSessionID(req.SessionID); err != nil {
		return nil, err
	}
	sessionID := req.SessionID

	release, acquired := s.locks.tryLock(sessionID)
	if !acquired {
		return nil, ErrBusy
	}
	defer release()

	if _, err := s.store.GetSession(ctx, sessionID); err == nil {
		return nil, fmt.Errorf("%w: session already exists", ErrConflict)
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	if runtime.IsLocalRuntime(s.runtime.Name()) {
		return s.createLocalSession(ctx, req, environment, startedAt)
	}

	rootfsPath := filepath.Join(s.cfg.RootfsRoot, sessionID)
	if err := os.MkdirAll(rootfsPath, 0o755); err != nil {
		return nil, fmt.Errorf("create rootfs: %w", err)
	}

	mounts, callerProvidesWorkspace, err := s.buildSessionMounts(environment.Mounts, req.Mounts, rootfsPath)
	if err != nil {
		s.warnIfCleanupFails("remove rootfs after mount preparation failed", rootfsPath, os.RemoveAll(rootfsPath))
		return nil, err
	}
	if callerProvidesWorkspace {
		if err := os.RemoveAll(rootfsPath); err != nil {
			return nil, fmt.Errorf("remove unused rootfs: %w", err)
		}
		rootfsPath = ""
	}

	containerLabels := model.CloneMap(req.Labels)
	if containerLabels == nil {
		containerLabels = make(map[string]string)
	}
	containerLabels[runtime.SessionIDLabel] = sessionID
	containerLabels[runtime.CreatedAtLabel] = time.Now().UTC().Format(time.RFC3339Nano)
	containerLabels["sandbox.environment"] = environment.Name
	if rootfsPath != "" {
		containerLabels[runtime.RootfsLabel] = rootfsPath
	}

	cwd := sessionDefaultCwd(req.Cwd, environment.DefaultCwd)
	imageRef := strings.TrimSpace(environment.ImageRef())
	if runtime.IsLocalRuntime(s.runtime.Name()) && imageRef == "" {
		imageRef = runtime.LocalImageRef
	}

	info, err := s.runtime.Create(ctx, runtime.CreateOptions{
		Name:      sessionID,
		Image:     imageRef,
		Cwd:       cwd,
		Env:       model.CloneMap(env),
		Mounts:    mounts,
		Resources: environment.Resources,
		Labels:    containerLabels,
	})
	if err != nil {
		if rootfsPath != "" {
			s.warnIfCleanupFails("remove rootfs after session create failed", rootfsPath, os.RemoveAll(rootfsPath))
		}
		if errors.Is(err, runtime.ErrContainerExists) {
			return nil, fmt.Errorf("%w: session already exists", ErrConflict)
		}
		s.logger.Error("session create runtime failed",
			"session_id", sessionID,
			"environment", environment.Name,
			"image", environment.ImageRef(),
			"rootfs", rootfsPath,
			"error", err,
		)
		return nil, err
	}
	started, err := s.runtime.Start(ctx, info.ID)
	if err != nil {
		s.logger.Error("session start failed",
			"session_id", sessionID,
			"environment", environment.Name,
			"image", environment.ImageRef(),
			"container_id", info.ID,
			"error", err,
		)
		s.warnIfCleanupFails("remove container after session start failed", info.ID, s.runtime.Remove(ctx, info.ID))
		if rootfsPath != "" {
			s.warnIfCleanupFails("remove rootfs after session start failed", rootfsPath, os.RemoveAll(rootfsPath))
		}
		return nil, err
	}

	session := &model.Session{
		ID:              sessionID,
		ContainerID:     info.ID,
		EnvironmentName: environment.Name,
		Image:           imageRef,
		DefaultCwd:      cwd,
		RootfsPath:      rootfsPath,
		Env:             model.CloneMap(env),
		Mounts:          append([]model.Mount(nil), mounts...),
		Resources:       environment.Resources,
		Labels:          model.CloneMap(req.Labels),
		Status:          model.SessionStatusActive,
		CreatedAt:       time.Now().UTC(),
	}
	if err := s.store.SaveSession(ctx, session); err != nil {
		return nil, err
	}

	response := sessionToCreateResult(session, durationMilliseconds(startedAt, time.Now().UTC()))
	response.Status = model.SessionStatusActive
	s.logger.Info("session created", "session_id", session.ID, "environment", session.EnvironmentName, "image", session.Image)
	if started.State != runtime.ContainerRunning {
		s.logger.Warn("session started with non-running state", "session_id", session.ID, "state", started.State)
	}
	return response, nil
}

func (s *SessionService) createLocalSession(ctx context.Context, req model.CreateSessionRequest, environment *model.Environment, startedAt time.Time) (*model.CreateSessionResult, error) {
	mounts, _, err := s.buildSessionMounts(environment.Mounts, req.Mounts, "")
	if err != nil {
		return nil, err
	}
	env := mergedSessionEnv(environment.DefaultEnv, req.Env)

	containerLabels := model.CloneMap(req.Labels)
	if containerLabels == nil {
		containerLabels = make(map[string]string)
	}
	containerLabels[runtime.SessionIDLabel] = req.SessionID
	containerLabels[runtime.CreatedAtLabel] = time.Now().UTC().Format(time.RFC3339Nano)
	containerLabels["sandbox.environment"] = environment.Name

	cwd := localSessionDefaultCwd(req.Cwd, environment.DefaultCwd)
	imageRef := strings.TrimSpace(environment.ImageRef())
	if imageRef == "" {
		imageRef = runtime.LocalImageRef
	}

	info, err := s.runtime.Create(ctx, runtime.CreateOptions{
		Name:      req.SessionID,
		Image:     imageRef,
		Cwd:       cwd,
		Env:       model.CloneMap(env),
		Mounts:    mounts,
		Resources: environment.Resources,
		Labels:    containerLabels,
	})
	if err != nil {
		if errors.Is(err, runtime.ErrContainerExists) {
			return nil, fmt.Errorf("%w: session already exists", ErrConflict)
		}
		s.logger.Error("local session create runtime failed",
			"session_id", req.SessionID,
			"environment", environment.Name,
			"cwd", cwd,
			"error", err,
		)
		return nil, err
	}
	started, err := s.runtime.Start(ctx, info.ID)
	if err != nil {
		s.logger.Error("local session start failed",
			"session_id", req.SessionID,
			"environment", environment.Name,
			"container_id", info.ID,
			"error", err,
		)
		return nil, err
	}

	session := &model.Session{
		ID:              req.SessionID,
		ContainerID:     info.ID,
		EnvironmentName: environment.Name,
		Image:           imageRef,
		DefaultCwd:      cwd,
		RootfsPath:      "",
		Env:             model.CloneMap(env),
		Mounts:          append([]model.Mount(nil), mounts...),
		Resources:       environment.Resources,
		Labels:          model.CloneMap(req.Labels),
		Status:          model.SessionStatusActive,
		CreatedAt:       time.Now().UTC(),
	}
	if err := s.store.SaveSession(ctx, session); err != nil {
		return nil, err
	}

	response := sessionToCreateResult(session, durationMilliseconds(startedAt, time.Now().UTC()))
	response.Status = model.SessionStatusActive
	s.logger.Info("local session created", "session_id", session.ID, "environment", session.EnvironmentName, "cwd", session.DefaultCwd)
	if started.State != runtime.ContainerRunning {
		s.logger.Warn("local session started with non-running state", "session_id", session.ID, "state", started.State)
	}
	return response, nil
}

func mergedSessionEnv(defaultEnv map[string]string, overrides map[string]string) map[string]string {
	env := model.CloneMap(defaultEnv)
	for key, value := range overrides {
		if env == nil {
			env = make(map[string]string, len(overrides))
		}
		env[key] = value
	}
	return env
}

func (s *SessionService) CreateTemplate(context.Context) (*model.SessionCreateTemplate, error) {
	root := strings.TrimSpace(s.cfg.SessionMountTemplateRoot)
	response := &model.SessionCreateTemplate{
		MountTemplateRoot: root,
		DefaultMounts:     []model.Mount{},
	}
	if root == "" {
		return response, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read session mount template root: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" || "/"+name == runtime.DefaultMountPath {
			continue
		}
		response.DefaultMounts = append(response.DefaultMounts, model.Mount{
			Source:      filepath.Join(root, name),
			Destination: "/" + name,
		})
	}
	sort.Slice(response.DefaultMounts, func(i, j int) bool {
		return response.DefaultMounts[i].Destination < response.DefaultMounts[j].Destination
	})
	return response, nil
}

func (s *SessionService) Execute(ctx context.Context, sessionID string, req model.ExecuteSessionRequest) (*model.ExecuteSessionResult, error) {
	sessionID = normalizeSessionID(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: session_id is required", ErrValidation)
	}
	if strings.TrimSpace(req.Command) == "" {
		return nil, fmt.Errorf("%w: command is required", ErrValidation)
	}
	release, acquired := s.locks.tryLock(sessionID)
	if !acquired {
		return nil, ErrBusy
	}
	defer release()

	session, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.Status != model.SessionStatusActive {
		return nil, fmt.Errorf("%w: session is not active", ErrValidation)
	}

	execCwd := session.DefaultCwd
	if strings.TrimSpace(req.Cwd) != "" {
		execCwd = req.Cwd
	}
	execOpts := runtime.ExecOptions{
		Command: req.Command,
		Args:    append([]string(nil), req.Args...),
		Cwd:     execCwd,
		Timeout: timeoutFor(req.TimeoutMS, s.cfg.DefaultCommandTimeout),
	}

	result, err := s.execOnSession(ctx, session, execOpts)
	if err != nil {
		return nil, err
	}

	response := executeResponse(sessionID, execCwd, result)
	if s.cfg.EnableExecLogPersist {
		execution := executionFromResult(sessionID, req, execCwd, result, s.cfg.ExecLogMaxOutputBytes)
		if err := s.store.SaveSessionExecution(ctx, execution); err != nil {
			return nil, err
		}
	}
	return response, nil
}

func (s *SessionService) execOnSession(ctx context.Context, session *model.Session, execOpts runtime.ExecOptions) (runtime.ExecResult, error) {
	target := session.ContainerID
	if target == "" {
		target = session.ID
	}

	result, err := s.runtime.Exec(ctx, target, execOpts)
	if err == nil {
		return result, nil
	}
	if errors.Is(err, runtime.ErrContainerNotFound) || errors.Is(err, runtime.ErrContainerNotRunning) {
		if markErr := s.markSessionUnavailable(ctx, session, time.Now().UTC()); markErr != nil {
			return runtime.ExecResult{}, markErr
		}
		return runtime.ExecResult{}, fmt.Errorf("%w: session is no longer executable; recreate the session", ErrConflict)
	}
	s.logger.Error("session exec failed",
		"session_id", session.ID,
		"container_id", target,
		"command", execOpts.Command,
		"cwd", execOpts.Cwd,
		"error", err,
	)
	return runtime.ExecResult{}, err
}

func (s *SessionService) Stop(ctx context.Context, sessionID string) (*model.StopSessionResult, error) {
	startedAt := time.Now().UTC()
	sessionID = normalizeSessionID(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: session_id is required", ErrValidation)
	}

	release, acquired := s.locks.tryLock(sessionID)
	if !acquired {
		return nil, ErrBusy
	}
	defer release()

	session, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.Status != model.SessionStatusActive {
		return &model.StopSessionResult{
			SessionID:  sessionID,
			Status:     session.Status,
			DurationMS: durationMilliseconds(startedAt, time.Now().UTC()),
		}, nil
	}

	target := sessionID
	if session.ContainerID != "" {
		target = session.ContainerID
	}
	if runtime.IsLocalRuntime(s.runtime.Name()) {
		_ = s.runtime.Stop(ctx, target, sessionStopTimeout)
		_ = s.runtime.Remove(ctx, target)
		if err := s.markSessionStopped(ctx, session, time.Now().UTC(), false); err != nil {
			return nil, err
		}
		return &model.StopSessionResult{
			SessionID:  sessionID,
			Status:     model.SessionStatusStopped,
			DurationMS: durationMilliseconds(startedAt, time.Now().UTC()),
		}, nil
	}
	if err := s.runtime.Stop(ctx, target, sessionStopTimeout); err != nil && !errors.Is(err, runtime.ErrContainerNotFound) {
		s.logger.Error("session stop failed",
			"session_id", session.ID,
			"container_id", target,
			"error", err,
		)
		return nil, err
	}
	if err := s.runtime.Remove(ctx, target); err != nil && !errors.Is(err, runtime.ErrContainerNotFound) {
		s.logger.Error("session remove failed",
			"session_id", session.ID,
			"container_id", target,
			"error", err,
		)
		return nil, err
	}
	if err := s.markSessionStopped(ctx, session, time.Now().UTC(), s.cfg.DeleteRootfsOnStop); err != nil {
		return nil, err
	}

	return &model.StopSessionResult{
		SessionID:  sessionID,
		Status:     model.SessionStatusStopped,
		DurationMS: durationMilliseconds(startedAt, time.Now().UTC()),
	}, nil
}

func (s *SessionService) List(ctx context.Context) ([]*model.SessionView, error) {
	sessions, err := s.store.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	responses := make([]*model.SessionView, 0, len(sessions))
	for _, session := range sessions {
		responses = append(responses, sessionToView(session))
	}
	return responses, nil
}

func (s *SessionService) Query(ctx context.Context, query store.SessionQuery) (*model.SessionList, error) {
	switch strings.ToLower(strings.TrimSpace(query.Status)) {
	case "", "active", "history", "all":
	default:
		return nil, fmt.Errorf("%w: status must be one of active, history, all", ErrValidation)
	}
	items, total, err := s.store.QuerySessions(ctx, query)
	if err != nil {
		return nil, err
	}
	page, pageSize := store.NormalizePagination(query.Pagination)
	responses := make([]*model.SessionView, 0, len(items))
	for _, item := range items {
		responses = append(responses, sessionToView(item))
	}
	return &model.SessionList{
		Items:    responses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *SessionService) Get(ctx context.Context, sessionID string) (*model.SessionView, error) {
	session, err := s.store.GetSession(ctx, normalizeSessionID(sessionID))
	if err != nil {
		return nil, err
	}
	return sessionToView(session), nil
}

func (s *SessionService) ListExecutions(ctx context.Context, sessionID string, pagination store.Pagination) (*model.SessionExecutionList, error) {
	sessionID = normalizeSessionID(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: session_id is required", ErrValidation)
	}
	if _, err := s.store.GetSession(ctx, sessionID); err != nil {
		return nil, err
	}
	items, total, err := s.store.ListSessionExecutions(ctx, sessionID, pagination)
	if err != nil {
		return nil, err
	}
	page, pageSize := store.NormalizePagination(pagination)
	responses := make([]*model.SessionExecution, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.Clone())
	}
	return &model.SessionExecutionList{
		Items:    responses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *SessionService) Reconcile(ctx context.Context) error {
	sessions, err := s.store.ListSessions(ctx)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		info, err := s.inspectSession(ctx, session)
		if err != nil {
			if errors.Is(err, runtime.ErrContainerNotFound) {
				if markErr := s.markSessionStopped(ctx, session, time.Now().UTC(), s.cfg.DeleteRootfsOnStop); markErr != nil {
					return markErr
				}
				continue
			}
			return err
		}
		if info.State != runtime.ContainerRunning {
			if markErr := s.markSessionStopped(ctx, session, time.Now().UTC(), s.cfg.DeleteRootfsOnStop); markErr != nil {
				return markErr
			}
			continue
		}
		if session.ContainerID != info.ID {
			session.ContainerID = info.ID
			if saveErr := s.store.SaveSession(ctx, session); saveErr != nil {
				return saveErr
			}
		}
	}
	return nil
}

func (s *SessionService) markSessionStopped(ctx context.Context, session *model.Session, stoppedAt time.Time, removeRootfs bool) error {
	session.Status = model.SessionStatusStopped
	session.StoppedAt = stoppedAt.UTC()
	if err := s.store.SaveSession(ctx, session); err != nil {
		return err
	}

	var rootfsErr error
	if removeRootfs && session.RootfsPath != "" {
		if err := os.RemoveAll(session.RootfsPath); err != nil {
			rootfsErr = fmt.Errorf("delete rootfs: %w", err)
		}
	}
	return rootfsErr
}

func (s *SessionService) inspectSession(ctx context.Context, session *model.Session) (runtime.ContainerInfo, error) {
	info, err := s.runtime.Inspect(ctx, session.ID)
	if err == nil {
		return info, nil
	}
	if !errors.Is(err, runtime.ErrContainerNotFound) || session.ContainerID == "" {
		return runtime.ContainerInfo{}, err
	}
	return s.runtime.Inspect(ctx, session.ContainerID)
}

func (s *SessionService) markSessionUnavailable(ctx context.Context, session *model.Session, stoppedAt time.Time) error {
	info, err := s.inspectSession(ctx, session)
	switch {
	case err == nil:
		if session.ContainerID != info.ID {
			session.ContainerID = info.ID
		}
		if info.State == runtime.ContainerRunning {
			return nil
		}
	case errors.Is(err, runtime.ErrContainerNotFound):
	default:
		s.logger.Error("session inspect failed during availability sync",
			"session_id", session.ID,
			"container_id", session.ContainerID,
			"error", err,
		)
		return err
	}
	return s.markSessionStopped(ctx, session, stoppedAt, s.cfg.DeleteRootfsOnStop)
}

func (s *SessionService) warnIfCleanupFails(action, target string, err error) {
	if err == nil {
		return
	}
	s.logger.Warn(action, "target", target, "error", err)
}
