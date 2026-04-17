package model

import "time"

type CreateSessionRequest struct {
	SessionID       string
	EnvironmentName string
	Cwd             string
	Env             map[string]string
	Labels          map[string]string
	Mounts          []Mount
}

type ExecuteSessionRequest struct {
	Command   string
	Args      []string
	Cwd       string
	TimeoutMS int64
}

type BuildEnvironmentRequest struct {
	Target string
}

type UpsertEnvironmentRequest struct {
	Name            string
	Description     string
	ImageRepository string
	ImageTag        string
	DefaultCwd      string
	DefaultEnv      map[string]string
	AgentPrompt     string
	Mounts          []Mount
	Resources       ResourceSpec
	Enabled         bool
	DefaultExecute  ExecutePreset
	Build           BuildSpec
}

type SessionView struct {
	SessionID       string
	EnvironmentName string
	ContainerID     string
	Image           string
	DefaultCwd      string
	RootfsPath      string
	Labels          map[string]string
	Resources       ResourceSpec
	Mounts          []Mount
	CreatedAt       time.Time
	Status          SessionStatus
	StoppedAt       time.Time
}

type CreateSessionResult struct {
	SessionView
	DurationMS int64
}

type StopSessionResult struct {
	SessionID  string
	Status     SessionStatus
	DurationMS int64
}

type ExecuteSessionResult struct {
	SessionID        string
	ExitCode         int
	Stdout           string
	Stderr           string
	WorkingDirectory string
	TimedOut         bool
	DurationMS       int64
	StartedAt        time.Time
	FinishedAt       time.Time
}

func (r *ExecuteSessionResult) Succeeded() bool {
	return r != nil && r.ExitCode == 0 && r.Stderr == ""
}

type SessionList struct {
	Items    []*SessionView
	Total    int
	Page     int
	PageSize int
}

type SessionExecutionList struct {
	Items    []*SessionExecution
	Total    int
	Page     int
	PageSize int
}

type SessionCreateTemplate struct {
	MountTemplateRoot string
	DefaultMounts     []Mount
}

type ImageMetadataView struct {
	CreatedAt       time.Time
	TotalSizeBytes  *int64
	UniqueSizeBytes *int64
}

type EnvironmentView struct {
	Name                  string
	Description           string
	ImageRepository       string
	ImageTag              string
	ImageRef              string
	Available             bool
	DefaultCwd            string
	DefaultEnv            map[string]string
	AgentPrompt           string
	Mounts                []Mount
	Resources             ResourceSpec
	Enabled               bool
	DefaultExecute        ExecutePreset
	Build                 BuildSpec
	ImageMetadata         *ImageMetadataView
	AvailableBuildTargets []string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	LastBuild             *BuildJob
	YAML                  string
}

type EnvironmentAgentPrompt struct {
	EnvironmentName string
	HasPrompt       bool
	Prompt          string
	UpdatedAt       time.Time
}

type EnvironmentFile struct {
	Path       string
	Size       int64
	ModifiedAt time.Time
	Type       string
	Content    string
}
