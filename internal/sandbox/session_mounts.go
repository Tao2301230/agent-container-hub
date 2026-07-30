package sandbox

import (
	"fmt"
	"os"
	"path"
	"strings"

	"agent-container-hub/internal/model"
	"agent-container-hub/internal/runtime"
)

func (s *SessionService) buildSessionMounts(environmentMounts, requestMounts []model.Mount, rootfsPath string) ([]model.Mount, bool, error) {
	normalizedEnvMounts, err := s.normalizeMountList(environmentMounts)
	if err != nil {
		return nil, false, err
	}
	normalizedRequestMounts, err := s.normalizeMountList(requestMounts)
	if err != nil {
		return nil, false, err
	}

	callerProvidesWorkspace := false
	for _, mount := range normalizedRequestMounts {
		if mount.Destination == runtime.DefaultMountPath {
			callerProvidesWorkspace = true
			break
		}
	}

	rootfsMount := model.Mount{
		Source:      runtime.NormalizeMountSource(rootfsPath),
		Destination: runtime.DefaultMountPath,
	}

	destinations := make(map[string]struct{})
	for _, mount := range normalizedEnvMounts {
		if mount.Destination == runtime.DefaultMountPath {
			return nil, false, fmt.Errorf("%w: mount destination %s is reserved for the rootfs", ErrValidation, runtime.DefaultMountPath)
		}
		if err := validateMountDestination(mount.Destination, destinations); err != nil {
			return nil, false, err
		}
	}
	if !callerProvidesWorkspace {
		destinations[rootfsMount.Destination] = struct{}{}
	}
	for _, mount := range normalizedRequestMounts {
		if err := validateMountDestination(mount.Destination, destinations); err != nil {
			return nil, false, err
		}
	}

	mounts := append([]model.Mount(nil), normalizedEnvMounts...)
	mounts = append(mounts, normalizedRequestMounts...)
	if !callerProvidesWorkspace {
		mounts = append(mounts, rootfsMount)
	}
	return mounts, callerProvidesWorkspace, nil
}

func (s *SessionService) normalizeMountList(mounts []model.Mount) ([]model.Mount, error) {
	normalized := make([]model.Mount, 0, len(mounts))
	for _, mount := range mounts {
		source := strings.TrimSpace(mount.Source)
		if source == "" {
			return nil, fmt.Errorf("%w: mount source is required", ErrValidation)
		}
		destination := normalizeContainerPath(mount.Destination)
		if destination == "" {
			return nil, fmt.Errorf("%w: mount destination is required", ErrValidation)
		}
		normalizedSource := runtime.NormalizeMountSource(source)
		if _, err := os.Stat(normalizedSource); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("%w: mount source does not exist: %s", ErrValidation, normalizedSource)
			}
			return nil, fmt.Errorf("stat mount source %s: %w", normalizedSource, err)
		}
		normalized = append(normalized, model.Mount{
			Source:      normalizedSource,
			Destination: destination,
			ReadOnly:    mount.ReadOnly,
		})
	}
	return normalized, nil
}

func validateMountDestination(destination string, seen map[string]struct{}) error {
	if _, exists := seen[destination]; exists {
		return fmt.Errorf("%w: mount destination %s is duplicated", ErrValidation, destination)
	}
	seen[destination] = struct{}{}
	return nil
}

func normalizeMaskedPaths(workspaceProtocol string, rawPaths []string, mounts []model.Mount, labels map[string]string) ([]string, error) {
	protocol := strings.TrimSpace(workspaceProtocol)
	labelProtocol := strings.TrimSpace(labels["workspaceProtocol"])
	if protocol == "" {
		protocol = labelProtocol
	}
	if protocol == "" && len(rawPaths) == 0 {
		return nil, nil
	}
	if protocol != runtime.WorkspaceProtocolDualRootV2 {
		return nil, fmt.Errorf("%w: unsupported workspaceProtocol %q", ErrValidation, protocol)
	}
	if labelProtocol != "" && labelProtocol != protocol {
		return nil, fmt.Errorf("%w: workspaceProtocol label must match the request protocol", ErrValidation)
	}
	seen := make(map[string]struct{}, len(rawPaths))
	normalized := make([]string, 0, len(rawPaths))
	for _, rawPath := range rawPaths {
		maskedPath := normalizeContainerPath(rawPath)
		if maskedPath == "" || maskedPath == runtime.DefaultMountPath ||
			!strings.HasPrefix(maskedPath, runtime.DefaultMountPath+"/") {
			return nil, fmt.Errorf("%w: masked path must be a child of %s", ErrValidation, runtime.DefaultMountPath)
		}
		if _, exists := seen[maskedPath]; exists {
			return nil, fmt.Errorf("%w: masked path %s is duplicated", ErrValidation, maskedPath)
		}
		for _, mount := range mounts {
			destination := normalizeContainerPath(mount.Destination)
			if destination == runtime.DefaultMountPath {
				continue
			}
			if pathContains(maskedPath, destination) || pathContains(destination, maskedPath) {
				return nil, fmt.Errorf("%w: masked path %s overlaps mount destination %s", ErrValidation, maskedPath, destination)
			}
		}
		seen[maskedPath] = struct{}{}
		normalized = append(normalized, maskedPath)
	}
	return normalized, nil
}

func validateDualRootLayout(workspaceProtocol string, cwd string, mounts []model.Mount) error {
	if strings.TrimSpace(workspaceProtocol) == "" {
		return nil
	}
	if strings.TrimSpace(workspaceProtocol) != runtime.WorkspaceProtocolDualRootV2 {
		return fmt.Errorf("%w: unsupported workspaceProtocol %q", ErrValidation, workspaceProtocol)
	}
	if normalizeContainerPath(cwd) != runtime.DefaultMountPath {
		return fmt.Errorf("%w: %s requires cwd %s", ErrValidation, runtime.WorkspaceProtocolDualRootV2, runtime.DefaultMountPath)
	}
	destinations := make(map[string]bool, len(mounts))
	for _, mount := range mounts {
		destinations[normalizeContainerPath(mount.Destination)] = true
	}
	for _, required := range []string{runtime.DefaultMountPath, "/chat"} {
		if !destinations[required] {
			return fmt.Errorf("%w: %s requires mount %s", ErrValidation, runtime.WorkspaceProtocolDualRootV2, required)
		}
	}
	return nil
}

func pathContains(root string, candidate string) bool {
	return candidate == root || strings.HasPrefix(candidate, strings.TrimRight(root, "/")+"/")
}

func normalizeContainerPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	clean := path.Clean(value)
	if clean == "." {
		return ""
	}
	return clean
}
