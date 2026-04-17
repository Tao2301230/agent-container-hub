package httpserver

import (
	"agent-container-hub/internal/api"
	"agent-container-hub/internal/model"
)

func createSessionRequestToModel(req api.CreateSessionRequest) model.CreateSessionRequest {
	return model.CreateSessionRequest{
		SessionID:       req.SessionID,
		EnvironmentName: req.EnvironmentName,
		Cwd:             req.Cwd,
		Env:             model.CloneMap(req.Env),
		Labels:          model.CloneMap(req.Labels),
		Mounts:          append([]model.Mount(nil), req.Mounts...),
	}
}

func executeSessionRequestToModel(req api.ExecuteSessionRequest) model.ExecuteSessionRequest {
	return model.ExecuteSessionRequest{
		Command:   req.Command,
		Args:      append([]string(nil), req.Args...),
		Cwd:       req.Cwd,
		TimeoutMS: req.TimeoutMS,
	}
}

func upsertEnvironmentRequestToModel(req api.UpsertEnvironmentRequest) model.UpsertEnvironmentRequest {
	return model.UpsertEnvironmentRequest{
		Name:            req.Name,
		Description:     req.Description,
		ImageRepository: req.ImageRepository,
		ImageTag:        req.ImageTag,
		DefaultCwd:      req.DefaultCwd,
		DefaultEnv:      model.CloneMap(req.DefaultEnv),
		AgentPrompt:     req.AgentPrompt,
		Mounts:          append([]model.Mount(nil), req.Mounts...),
		Resources:       req.Resources,
		Enabled:         req.Enabled,
		DefaultExecute:  req.DefaultExecute.Clone(),
		Build:           req.Build.Clone(),
	}
}

func buildEnvironmentRequestToModel(req api.BuildEnvironmentRequest) model.BuildEnvironmentRequest {
	return model.BuildEnvironmentRequest{Target: req.Target}
}

func sessionViewToAPI(session *model.SessionView) *api.SessionResponse {
	if session == nil {
		return nil
	}
	return &api.SessionResponse{
		SessionID:       session.SessionID,
		EnvironmentName: session.EnvironmentName,
		ContainerID:     session.ContainerID,
		Image:           session.Image,
		DefaultCwd:      session.DefaultCwd,
		RootfsPath:      session.RootfsPath,
		Labels:          model.CloneMap(session.Labels),
		Resources:       session.Resources,
		Mounts:          append([]model.Mount(nil), session.Mounts...),
		CreatedAt:       session.CreatedAt,
		Status:          string(session.Status),
		StoppedAt:       session.StoppedAt,
	}
}

func createSessionResultToAPI(result *model.CreateSessionResult) *api.CreateSessionResponse {
	if result == nil {
		return nil
	}
	return &api.CreateSessionResponse{
		SessionResponse: *sessionViewToAPI(&result.SessionView),
		DurationMS:      result.DurationMS,
	}
}

func stopSessionResultToAPI(result *model.StopSessionResult) *api.StopSessionResponse {
	if result == nil {
		return nil
	}
	return &api.StopSessionResponse{
		SessionID:  result.SessionID,
		Status:     string(result.Status),
		DurationMS: result.DurationMS,
	}
}

func executeSessionErrorResponse(result *model.ExecuteSessionResult) api.ExecuteSessionErrorResponse {
	if result == nil {
		return api.ExecuteSessionErrorResponse{Mode: "sandbox"}
	}
	return api.ExecuteSessionErrorResponse{
		ExitCode:         result.ExitCode,
		Mode:             "sandbox",
		WorkingDirectory: result.WorkingDirectory,
		Stdout:           result.Stdout,
		Stderr:           result.Stderr,
	}
}

func sessionListToAPI(list *model.SessionList) *api.SessionListResponse {
	if list == nil {
		return nil
	}
	items := make([]*api.SessionResponse, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, sessionViewToAPI(item))
	}
	return &api.SessionListResponse{
		Items:    items,
		Total:    list.Total,
		Page:     list.Page,
		PageSize: list.PageSize,
	}
}

func sessionExecutionToAPI(item *model.SessionExecution) *api.SessionExecutionResponse {
	if item == nil {
		return nil
	}
	return &api.SessionExecutionResponse{
		ID:              item.ID,
		SessionID:       item.SessionID,
		Command:         item.Command,
		Args:            append([]string(nil), item.Args...),
		Cwd:             item.Cwd,
		TimeoutMS:       item.TimeoutMS,
		ExitCode:        item.ExitCode,
		Stdout:          item.Stdout,
		Stderr:          item.Stderr,
		StdoutTruncated: item.StdoutTruncated,
		StderrTruncated: item.StderrTruncated,
		TimedOut:        item.TimedOut,
		DurationMS:      item.DurationMS,
		StartedAt:       item.StartedAt,
		FinishedAt:      item.FinishedAt,
	}
}

func sessionExecutionListToAPI(list *model.SessionExecutionList) *api.SessionExecutionListResponse {
	if list == nil {
		return nil
	}
	items := make([]*api.SessionExecutionResponse, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, sessionExecutionToAPI(item))
	}
	return &api.SessionExecutionListResponse{
		Items:    items,
		Total:    list.Total,
		Page:     list.Page,
		PageSize: list.PageSize,
	}
}

func sessionCreateTemplateToAPI(result *model.SessionCreateTemplate) *api.SessionCreateTemplateResponse {
	if result == nil {
		return nil
	}
	return &api.SessionCreateTemplateResponse{
		MountTemplateRoot: result.MountTemplateRoot,
		DefaultMounts:     append([]model.Mount(nil), result.DefaultMounts...),
	}
}

func imageMetadataViewToAPI(metadata *model.ImageMetadataView) *api.ImageMetadataResponse {
	if metadata == nil {
		return nil
	}
	return &api.ImageMetadataResponse{
		CreatedAt:       metadata.CreatedAt,
		TotalSizeBytes:  metadata.TotalSizeBytes,
		UniqueSizeBytes: metadata.UniqueSizeBytes,
	}
}

func buildJobToAPI(job *model.BuildJob) *api.BuildJobResponse {
	if job == nil {
		return nil
	}
	return &api.BuildJobResponse{
		ID:              job.ID,
		EnvironmentName: job.EnvironmentName,
		ImageRef:        job.ImageRef,
		Target:          job.Target,
		Status:          string(job.Status),
		Output:          job.Output,
		Error:           job.Error,
		StartedAt:       job.StartedAt,
		FinishedAt:      job.FinishedAt,
	}
}

func environmentViewToAPI(view *model.EnvironmentView) *api.EnvironmentResponse {
	if view == nil {
		return nil
	}
	return &api.EnvironmentResponse{
		Name:                  view.Name,
		Description:           view.Description,
		ImageRepository:       view.ImageRepository,
		ImageTag:              view.ImageTag,
		ImageRef:              view.ImageRef,
		Available:             view.Available,
		DefaultCwd:            view.DefaultCwd,
		DefaultEnv:            model.CloneMap(view.DefaultEnv),
		AgentPrompt:           view.AgentPrompt,
		Mounts:                append([]model.Mount(nil), view.Mounts...),
		Resources:             view.Resources,
		Enabled:               view.Enabled,
		DefaultExecute:        view.DefaultExecute.Clone(),
		Build:                 view.Build.Clone(),
		ImageMetadata:         imageMetadataViewToAPI(view.ImageMetadata),
		AvailableBuildTargets: append([]string(nil), view.AvailableBuildTargets...),
		CreatedAt:             view.CreatedAt,
		UpdatedAt:             view.UpdatedAt,
		LastBuild:             buildJobToAPI(view.LastBuild),
		YAML:                  view.YAML,
	}
}

func environmentsToAPI(views []*model.EnvironmentView) []*api.EnvironmentResponse {
	response := make([]*api.EnvironmentResponse, 0, len(views))
	for _, view := range views {
		response = append(response, environmentViewToAPI(view))
	}
	return response
}

func environmentAgentPromptToAPI(prompt *model.EnvironmentAgentPrompt) *api.EnvironmentAgentPromptResponse {
	if prompt == nil {
		return nil
	}
	return &api.EnvironmentAgentPromptResponse{
		EnvironmentName: prompt.EnvironmentName,
		HasPrompt:       prompt.HasPrompt,
		Prompt:          prompt.Prompt,
		UpdatedAt:       prompt.UpdatedAt,
	}
}

func environmentFileToAPI(file *model.EnvironmentFile) *api.EnvironmentFileResponse {
	if file == nil {
		return nil
	}
	return &api.EnvironmentFileResponse{
		Path:       file.Path,
		Size:       file.Size,
		ModifiedAt: file.ModifiedAt,
		Type:       file.Type,
		Content:    file.Content,
	}
}

func environmentFilesToAPI(files []*model.EnvironmentFile) []*api.EnvironmentFileResponse {
	response := make([]*api.EnvironmentFileResponse, 0, len(files))
	for _, file := range files {
		response = append(response, environmentFileToAPI(file))
	}
	return response
}

func sessionsToAPI(sessions []*model.SessionView) []*api.SessionResponse {
	response := make([]*api.SessionResponse, 0, len(sessions))
	for _, session := range sessions {
		response = append(response, sessionViewToAPI(session))
	}
	return response
}
