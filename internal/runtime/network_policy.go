package runtime

import (
	"context"
	"fmt"
	"net"
	"strings"

	"agent-container-hub/internal/model"
)

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
	commands, err := buildIptablesCommands(policy)
	if err != nil {
		return err
	}
	for _, command := range commands {
		args := append([]string{"exec", resolvedID}, command...)
		result, err := p.runCommand(ctx, args...)
		if err != nil {
			return p.commandError(args, result, err, "")
		}
	}
	return nil
}

func buildIptablesCommands(policy *model.NetworkPolicy) ([][]string, error) {
	if policy.IsEmpty() {
		return nil, nil
	}
	if err := model.ValidateNetworkPolicy(policy); err != nil {
		return nil, err
	}

	commands := make([][]string, 0)
	for _, entry := range policy.Blacklist {
		family, err := iptablesCommand(entry)
		if err != nil {
			return nil, err
		}
		commands = append(commands, []string{family, "-A", "OUTPUT", "-d", strings.TrimSpace(entry), "-j", "DROP"})
	}

	if len(policy.Whitelist) == 0 {
		return commands, nil
	}

	for _, family := range []string{"iptables", "ip6tables"} {
		commands = append(commands,
			[]string{family, "-A", "OUTPUT", "-o", "lo", "-j", "ACCEPT"},
			[]string{family, "-A", "OUTPUT", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"},
		)
	}
	for _, entry := range policy.Whitelist {
		family, err := iptablesCommand(entry)
		if err != nil {
			return nil, err
		}
		commands = append(commands, []string{family, "-A", "OUTPUT", "-d", strings.TrimSpace(entry), "-j", "ACCEPT"})
	}
	for _, family := range []string{"iptables", "ip6tables"} {
		commands = append(commands, []string{family, "-A", "OUTPUT", "-j", "DROP"})
	}
	return commands, nil
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
