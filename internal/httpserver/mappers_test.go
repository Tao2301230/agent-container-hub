package httpserver

import (
	"reflect"
	"testing"

	"agent-container-hub/internal/api"
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
