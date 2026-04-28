package httpserver

import (
	"reflect"
	"testing"

	"agent-container-hub/internal/api"
	"agent-container-hub/internal/model"
)

func TestCreateSessionRequestToModelClonesEnv(t *testing.T) {
	req := api.CreateSessionRequest{
		SessionID:       "session-1",
		EnvironmentName: "shell",
		Cwd:             "/workspace",
		Env: map[string]string{
			"NODE_ENV": "production",
		},
	}

	got := createSessionRequestToModel(req)
	if !reflect.DeepEqual(got.Env, req.Env) {
		t.Fatalf("Env = %#v, want %#v", got.Env, req.Env)
	}

	req.Env["NODE_ENV"] = "development"
	if got.Env["NODE_ENV"] != "production" {
		t.Fatalf("mapped env should be cloned, got %#v", got.Env)
	}
}

func TestCreateSessionRequestToModelClonesNetworkPolicy(t *testing.T) {
	req := api.CreateSessionRequest{
		SessionID:       "session-1",
		EnvironmentName: "shell",
		NetworkPolicy: &model.NetworkPolicy{
			Whitelist: []string{"10.0.0.1"},
			Blacklist: []string{"8.8.8.8"},
		},
	}

	got := createSessionRequestToModel(req)
	req.NetworkPolicy.Whitelist[0] = "10.0.0.2"
	req.NetworkPolicy.Blacklist[0] = "1.1.1.1"

	if !reflect.DeepEqual(got.NetworkPolicy, &model.NetworkPolicy{Whitelist: []string{"10.0.0.1"}, Blacklist: []string{"8.8.8.8"}}) {
		t.Fatalf("NetworkPolicy = %#v, want cloned original", got.NetworkPolicy)
	}
}

func TestEnvironmentViewToAPIIncludesNetworkPolicy(t *testing.T) {
	view := &model.EnvironmentView{
		Name:          "shell",
		NetworkPolicy: &model.NetworkPolicy{Whitelist: []string{"10.0.0.0/8"}},
	}

	got := environmentViewToAPI(view)
	view.NetworkPolicy.Whitelist[0] = "192.168.0.0/16"

	if !reflect.DeepEqual(got.NetworkPolicy, &model.NetworkPolicy{Whitelist: []string{"10.0.0.0/8"}}) {
		t.Fatalf("NetworkPolicy = %#v, want cloned original", got.NetworkPolicy)
	}
}
