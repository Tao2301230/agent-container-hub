package runtime

import (
	"context"
	"os"
	"strings"
	"testing"

	"agent-container-hub/internal/model"
)

func TestBuildIptablesScriptBlacklistOnly(t *testing.T) {
	t.Parallel()

	got, err := buildIptablesScript(&model.NetworkPolicy{
		Blacklist: []string{"8.8.8.8", "2001:4860:4860::8888"},
	})
	if err != nil {
		t.Fatalf("buildIptablesScript() error = %v", err)
	}
	assertContainsInOrder(t, got,
		"iptables -N ACH_OUTPUT 2>/dev/null || true",
		"iptables -F ACH_OUTPUT",
		"while iptables -D OUTPUT -j ACH_OUTPUT 2>/dev/null; do :; done",
		"iptables -I OUTPUT 1 -j ACH_OUTPUT",
		"iptables -A ACH_OUTPUT -d 8.8.8.8 -j DROP",
		"iptables -A ACH_OUTPUT -j RETURN",
		"ip6tables -A ACH_OUTPUT -d 2001:4860:4860::8888 -j DROP",
		"ip6tables -A ACH_OUTPUT -j RETURN",
	)
	if strings.Contains(got, " -j DROP\niptables -A ACH_OUTPUT -o lo") {
		t.Fatalf("blacklist-only script unexpectedly added whitelist rules:\n%s", got)
	}
}

func TestBuildIptablesScriptWhitelistAddsDefaultDropsForBothFamilies(t *testing.T) {
	t.Parallel()

	got, err := buildIptablesScript(&model.NetworkPolicy{
		Whitelist: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatalf("buildIptablesScript() error = %v", err)
	}
	assertContainsInOrder(t, got,
		"iptables -A ACH_OUTPUT -o lo -j ACCEPT",
		"iptables -A ACH_OUTPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT",
		"iptables -A ACH_OUTPUT -d 10.0.0.0/8 -j ACCEPT",
		"iptables -A ACH_OUTPUT -j DROP",
		"ip6tables -A ACH_OUTPUT -o lo -j ACCEPT",
		"ip6tables -A ACH_OUTPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT",
		"ip6tables -A ACH_OUTPUT -j DROP",
	)
}

func TestBuildIptablesScriptBlacklistPrecedesWhitelist(t *testing.T) {
	t.Parallel()

	got, err := buildIptablesScript(&model.NetworkPolicy{
		Whitelist: []string{"10.0.0.0/8"},
		Blacklist: []string{"10.0.0.2"},
	})
	if err != nil {
		t.Fatalf("buildIptablesScript() error = %v", err)
	}
	assertContainsInOrder(t, got,
		"iptables -A ACH_OUTPUT -d 10.0.0.2 -j DROP",
		"iptables -A ACH_OUTPUT -o lo -j ACCEPT",
		"iptables -A ACH_OUTPUT -d 10.0.0.0/8 -j ACCEPT",
	)
}

func TestBuildIptablesScriptEmptyPolicyNoops(t *testing.T) {
	t.Parallel()

	got, err := buildIptablesScript(&model.NetworkPolicy{})
	if err != nil {
		t.Fatalf("buildIptablesScript() error = %v", err)
	}
	if got != "" {
		t.Fatalf("script = %q, want empty", got)
	}
}

func TestApplyNetworkPolicyRunsHelperContainer(t *testing.T) {
	t.Parallel()

	binary, logPath := writeFakeRuntimeBinary(t)
	provider := &CLIProvider{
		binary:                   binary,
		networkPolicyHelperImage: "custom/network-policy-helper:test",
	}

	err := provider.ApplyNetworkPolicy(context.Background(), "demo", &model.NetworkPolicy{
		Whitelist: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatalf("ApplyNetworkPolicy() error = %v", err)
	}
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	logText := string(logData)
	for _, want := range []string{
		"run --rm --network container:ctr-demo",
		"--cap-drop=ALL",
		"--cap-add=NET_ADMIN",
		"--security-opt=no-new-privileges",
		"--read-only",
		"--tmpfs /run:rw,noexec,nosuid,nodev,size=1m",
		"custom/network-policy-helper:test sh -ceu",
		"iptables -I OUTPUT 1 -j ACH_OUTPUT",
		"iptables -A ACH_OUTPUT -d 10.0.0.0/8 -j ACCEPT",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("ApplyNetworkPolicy() log = %q, want %q", logText, want)
		}
	}
	if strings.Contains(logText, "exec ctr-demo iptables") {
		t.Fatalf("ApplyNetworkPolicy() used container exec instead of helper: %s", logText)
	}
}

func assertContainsInOrder(t *testing.T, text string, needles ...string) {
	t.Helper()

	offset := 0
	for _, needle := range needles {
		index := strings.Index(text[offset:], needle)
		if index < 0 {
			t.Fatalf("text missing %q after offset %d:\n%s", needle, offset, text)
		}
		offset += index + len(needle)
	}
}
