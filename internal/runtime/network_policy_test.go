package runtime

import (
	"reflect"
	"testing"

	"agent-container-hub/internal/model"
)

func TestBuildIptablesCommandsBlacklistOnly(t *testing.T) {
	t.Parallel()

	got, err := buildIptablesCommands(&model.NetworkPolicy{
		Blacklist: []string{"8.8.8.8", "2001:4860:4860::8888"},
	})
	if err != nil {
		t.Fatalf("buildIptablesCommands() error = %v", err)
	}
	want := [][]string{
		{"iptables", "-A", "OUTPUT", "-d", "8.8.8.8", "-j", "DROP"},
		{"ip6tables", "-A", "OUTPUT", "-d", "2001:4860:4860::8888", "-j", "DROP"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestBuildIptablesCommandsWhitelistAddsDefaultDropsForBothFamilies(t *testing.T) {
	t.Parallel()

	got, err := buildIptablesCommands(&model.NetworkPolicy{
		Whitelist: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatalf("buildIptablesCommands() error = %v", err)
	}
	want := [][]string{
		{"iptables", "-A", "OUTPUT", "-o", "lo", "-j", "ACCEPT"},
		{"iptables", "-A", "OUTPUT", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"},
		{"ip6tables", "-A", "OUTPUT", "-o", "lo", "-j", "ACCEPT"},
		{"ip6tables", "-A", "OUTPUT", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"},
		{"iptables", "-A", "OUTPUT", "-d", "10.0.0.0/8", "-j", "ACCEPT"},
		{"iptables", "-A", "OUTPUT", "-j", "DROP"},
		{"ip6tables", "-A", "OUTPUT", "-j", "DROP"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestBuildIptablesCommandsBlacklistPrecedesWhitelist(t *testing.T) {
	t.Parallel()

	got, err := buildIptablesCommands(&model.NetworkPolicy{
		Whitelist: []string{"10.0.0.0/8"},
		Blacklist: []string{"10.0.0.2"},
	})
	if err != nil {
		t.Fatalf("buildIptablesCommands() error = %v", err)
	}
	if len(got) == 0 || !reflect.DeepEqual(got[0], []string{"iptables", "-A", "OUTPUT", "-d", "10.0.0.2", "-j", "DROP"}) {
		t.Fatalf("first command = %#v, want blacklist drop first", got)
	}
}

func TestBuildIptablesCommandsEmptyPolicyNoops(t *testing.T) {
	t.Parallel()

	got, err := buildIptablesCommands(&model.NetworkPolicy{})
	if err != nil {
		t.Fatalf("buildIptablesCommands() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("commands = %#v, want none", got)
	}
}
