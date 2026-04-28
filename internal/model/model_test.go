package model

import "testing"

func TestBuildSpecCloneCopiesBuildContexts(t *testing.T) {
	t.Parallel()

	spec := BuildSpec{
		BuildArgs:     map[string]string{"NPM_REGISTRY": "https://registry.npmjs.org"},
		BuildContexts: map[string]string{"minimax_skills": "../../skills-market"},
		SmokeArgs:     []string{"-lc", "echo ok"},
	}

	cloned := spec.Clone()
	cloned.BuildArgs["NPM_REGISTRY"] = "changed"
	cloned.BuildContexts["minimax_skills"] = "/tmp/skills"
	cloned.SmokeArgs[0] = "changed"

	if spec.BuildArgs["NPM_REGISTRY"] != "https://registry.npmjs.org" {
		t.Fatalf("BuildArgs mutated in source: %+v", spec.BuildArgs)
	}
	if spec.BuildContexts["minimax_skills"] != "../../skills-market" {
		t.Fatalf("BuildContexts mutated in source: %+v", spec.BuildContexts)
	}
	if spec.SmokeArgs[0] != "-lc" {
		t.Fatalf("SmokeArgs mutated in source: %+v", spec.SmokeArgs)
	}
}

func TestValidateBuildContextMap(t *testing.T) {
	t.Parallel()

	if err := ValidateBuildContextMap(map[string]string{"minimax_skills": "../../skills-market"}); err != nil {
		t.Fatalf("ValidateBuildContextMap() error = %v", err)
	}
	if err := ValidateBuildContextMap(map[string]string{"MiniMax": "../../skills-market"}); err == nil {
		t.Fatal("ValidateBuildContextMap() error = nil, want invalid context name")
	}
	if err := ValidateBuildContextMap(map[string]string{"minimax_skills": ""}); err == nil {
		t.Fatal("ValidateBuildContextMap() error = nil, want empty path failure")
	}
}

func TestValidateNetworkPolicy(t *testing.T) {
	t.Parallel()

	policy := &NetworkPolicy{
		Whitelist: []string{"10.0.0.1", "192.168.0.0/16", "2001:db8::1", "2001:db8::/32"},
		Blacklist: []string{"8.8.8.8", "::1/128"},
	}
	if err := ValidateNetworkPolicy(policy); err != nil {
		t.Fatalf("ValidateNetworkPolicy() error = %v", err)
	}
	if err := ValidateNetworkPolicy(&NetworkPolicy{}); err != nil {
		t.Fatalf("ValidateNetworkPolicy(empty) error = %v", err)
	}
	if err := ValidateNetworkPolicy(nil); err != nil {
		t.Fatalf("ValidateNetworkPolicy(nil) error = %v", err)
	}
}

func TestValidateNetworkPolicyRejectsInvalidEntries(t *testing.T) {
	t.Parallel()

	cases := []*NetworkPolicy{
		{Whitelist: []string{""}},
		{Whitelist: []string{"not-an-ip"}},
		{Blacklist: []string{"192.168.0.0/99"}},
	}
	for _, policy := range cases {
		if err := ValidateNetworkPolicy(policy); err == nil {
			t.Fatalf("ValidateNetworkPolicy(%+v) error = nil, want invalid entry", policy)
		}
	}
}

func TestNetworkPolicyCloneCopiesSlices(t *testing.T) {
	t.Parallel()

	policy := &NetworkPolicy{
		Whitelist: []string{"10.0.0.1"},
		Blacklist: []string{"8.8.8.8"},
	}
	cloned := policy.Clone()
	cloned.Whitelist[0] = "10.0.0.2"
	cloned.Blacklist[0] = "1.1.1.1"

	if policy.Whitelist[0] != "10.0.0.1" || policy.Blacklist[0] != "8.8.8.8" {
		t.Fatalf("NetworkPolicy.Clone() shared slices: %+v", policy)
	}
}
