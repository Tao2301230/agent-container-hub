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
