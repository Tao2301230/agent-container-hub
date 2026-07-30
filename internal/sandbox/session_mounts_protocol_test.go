package sandbox

import (
	"strings"
	"testing"

	"agent-container-hub/internal/model"
	"agent-container-hub/internal/runtime"
)

func TestNormalizeMaskedPathsAcceptsDualRootV2WorkspaceChildren(t *testing.T) {
	mounts := []model.Mount{
		{Source: "/host/project", Destination: "/workspace"},
		{Source: "/host/chat", Destination: "/chat"},
	}
	got, err := normalizeMaskedPaths(
		runtime.WorkspaceProtocolDualRootV2,
		[]string{"/workspace/runtime/chats"},
		mounts,
		map[string]string{"workspaceProtocol": runtime.WorkspaceProtocolDualRootV2},
	)
	if err != nil {
		t.Fatalf("normalizeMaskedPaths() error = %v", err)
	}
	if len(got) != 1 || got[0] != "/workspace/runtime/chats" {
		t.Fatalf("masked paths = %#v", got)
	}
	if err := validateDualRootLayout(runtime.WorkspaceProtocolDualRootV2, "/workspace", mounts); err != nil {
		t.Fatalf("validateDualRootLayout() error = %v", err)
	}
}

func TestNormalizeMaskedPathsRejectsOldProtocolAndMountOverlap(t *testing.T) {
	mounts := []model.Mount{
		{Source: "/host/project", Destination: "/workspace"},
		{Source: "/host/chat", Destination: "/chat"},
	}
	if _, err := normalizeMaskedPaths("workspace-chat-v2", nil, mounts, map[string]string{
		"workspaceProtocol": "workspace-chat-v2",
	}); err == nil || !strings.Contains(err.Error(), "unsupported workspaceProtocol") {
		t.Fatalf("old protocol error = %v", err)
	}
	overlapping := append(mounts, model.Mount{Source: "/host/cache", Destination: "/workspace/runtime/chats/cache"})
	if _, err := normalizeMaskedPaths(
		runtime.WorkspaceProtocolDualRootV2,
		[]string{"/workspace/runtime/chats"},
		overlapping,
		nil,
	); err == nil || !strings.Contains(err.Error(), "overlaps mount destination") {
		t.Fatalf("overlap error = %v", err)
	}
}

func TestValidateDualRootLayoutRequiresWorkspaceChatAndWorkspaceCwd(t *testing.T) {
	base := []model.Mount{
		{Source: "/host/project", Destination: "/workspace"},
		{Source: "/host/chat", Destination: "/chat"},
	}
	if err := validateDualRootLayout(runtime.WorkspaceProtocolDualRootV2, "/chat", base); err == nil {
		t.Fatal("non-workspace cwd must be rejected")
	}
	if err := validateDualRootLayout(runtime.WorkspaceProtocolDualRootV2, "/workspace", base[:1]); err == nil {
		t.Fatal("missing /chat mount must be rejected")
	}
}
