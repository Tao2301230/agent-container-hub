package runtime

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"

	"agent-container-hub/internal/model"
)

const networkPolicyChain = "ACH_OUTPUT"

type NetworkPolicyApplier interface {
	ApplyNetworkPolicy(ctx context.Context, containerID string, policy *model.NetworkPolicy) error
}

func (p *CLIProvider) ApplyNetworkPolicy(ctx context.Context, containerID string, policy *model.NetworkPolicy) error {
	if policy.IsEmpty() {
		return nil
	}
	resolvedID, err := p.resolveContainerReference(ctx, containerID)
	if err != nil {
		return err
	}
	script, err := buildIptablesScript(policy)
	if err != nil {
		return err
	}
	if strings.TrimSpace(script) == "" {
		return nil
	}
	helperImage := strings.TrimSpace(p.networkPolicyHelperImage)
	if helperImage == "" {
		helperImage = DefaultNetworkPolicyHelperImage
	}
	args := []string{
		"run",
		"--rm",
		"--network", "container:" + resolvedID,
		"--cap-drop=ALL",
		"--cap-add=NET_ADMIN",
		"--security-opt=no-new-privileges",
		"--read-only",
		"--tmpfs", "/run:rw,noexec,nosuid,nodev,size=1m",
		helperImage,
		"sh",
		"-ceu",
		script,
	}
	result, err := p.runCommand(ctx, args...)
	if err != nil {
		return p.commandError(args, result, err, "apply network policy with helper container")
	}
	return nil
}

func buildIptablesScript(policy *model.NetworkPolicy) (string, error) {
	if policy.IsEmpty() {
		return "", nil
	}
	if err := model.ValidateNetworkPolicy(policy); err != nil {
		return "", err
	}

	rules := map[string][]string{
		"iptables":  buildIptablesRulesForFamily("iptables", policy),
		"ip6tables": buildIptablesRulesForFamily("ip6tables", policy),
	}
	var script strings.Builder
	script.WriteString("set -e\n")
	for _, family := range []string{"iptables", "ip6tables"} {
		commands := rules[family]
		if len(commands) == 0 {
			continue
		}
		script.WriteString(family + " -N " + networkPolicyChain + " 2>/dev/null || true\n")
		script.WriteString(family + " -F " + networkPolicyChain + "\n")
		script.WriteString("while " + family + " -D OUTPUT -j " + networkPolicyChain + " 2>/dev/null; do :; done\n")
		script.WriteString(family + " -I OUTPUT 1 -j " + networkPolicyChain + "\n")
		for _, command := range commands {
			script.WriteString(command)
			script.WriteByte('\n')
		}
	}
	return script.String(), nil
}

func buildIptablesRulesForFamily(family string, policy *model.NetworkPolicy) []string {
	commands := make([]string, 0)
	for _, entry := range entriesForFamily(policy.Blacklist, family) {
		commands = append(commands, fmt.Sprintf("%s -A %s -d %s -j DROP", family, networkPolicyChain, strings.TrimSpace(entry)))
	}

	whitelist := entriesForFamily(policy.Whitelist, family)
	if len(policy.Whitelist) == 0 {
		if len(commands) > 0 {
			commands = append(commands, fmt.Sprintf("%s -A %s -j RETURN", family, networkPolicyChain))
		}
		return commands
	}

	commands = append(commands,
		fmt.Sprintf("%s -A %s -o lo -j ACCEPT", family, networkPolicyChain),
		fmt.Sprintf("%s -A %s -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT", family, networkPolicyChain),
	)
	for _, entry := range whitelist {
		commands = append(commands, fmt.Sprintf("%s -A %s -d %s -j ACCEPT", family, networkPolicyChain, strings.TrimSpace(entry)))
	}
	commands = append(commands, fmt.Sprintf("%s -A %s -j DROP", family, networkPolicyChain))
	return commands
}

func entriesForFamily(entries []string, family string) []string {
	return slices.DeleteFunc(append([]string(nil), entries...), func(entry string) bool {
		entryFamily, err := iptablesCommand(entry)
		if err != nil {
			return true
		}
		return entryFamily != family
	})
}

func iptablesCommand(entry string) (string, error) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return "", fmt.Errorf("network policy entry must not be empty")
	}
	if strings.Contains(entry, "/") {
		ip, _, err := net.ParseCIDR(entry)
		if err != nil {
			return "", err
		}
		if ip.To4() != nil {
			return "iptables", nil
		}
		return "ip6tables", nil
	}
	ip := net.ParseIP(entry)
	if ip == nil {
		return "", fmt.Errorf("network policy entry %q must be an IP address or CIDR", entry)
	}
	if ip.To4() != nil {
		return "iptables", nil
	}
	return "ip6tables", nil
}
