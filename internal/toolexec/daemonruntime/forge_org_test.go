// Copyright (c) 2025 Reliant Labs
package daemonruntime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// The org a request names reaches forge's environment as RELIANT_FORGE_ORG,
// which the credential helper reads; a request naming none adds nothing.
func TestForgeOrg_ReachesTheChildEnvironment(t *testing.T) {
	ctx, err := bindForgeOrg(context.Background(), " org-b ")
	if err != nil {
		t.Fatal(err)
	}
	if got := forgeOrgEnv(ctx); len(got) != 1 || got[0] != "RELIANT_FORGE_ORG=org-b" {
		t.Fatalf("env = %v", got)
	}
	ctx, err = bindForgeOrg(context.Background(), "")
	if err != nil || forgeOrgEnv(ctx) != nil {
		t.Fatalf("an unnamed org must add nothing: %v %v", err, forgeOrgEnv(ctx))
	}
}

func TestForgeOrg_RefusesAnIdThatCouldNotBeOne(t *testing.T) {
	for _, bad := range []string{"a b", "a\nb", "x\x00", strings.Repeat("a", 129)} {
		if _, err := bindForgeOrg(context.Background(), bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestForgeOrg_HandlerCarriesTheRequestedOrgToTheRunner(t *testing.T) {
	previous := runForge
	defer func() { runForge = previous }()
	var got string
	runForge = func(ctx context.Context, _ string, _ []string) (forgeCommandResult, error) {
		got = forgeOrgOf(ctx)
		return forgeCommandResult{Stdout: []byte(`{}`)}, nil
	}
	payload, err := json.Marshal(map[string]any{"project_path": forgeProject(t), "org_id": "org-b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handleForgeTopology(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if got != "org-b" {
		t.Fatalf("runner org = %q, want org-b", got)
	}
}

// A detached deploy job outlives its request, so the org rides the job.
func TestForgeOrg_SurvivesIntoADetachedJob(t *testing.T) {
	ctx, _ := bindForgeOrg(context.Background(), "org-b")
	job := &forgeDeployJob{orgID: forgeOrgOf(ctx)}
	jobCtx := withForgeOrg(context.Background(), job.orgID)
	if forgeOrgOf(jobCtx) != "org-b" {
		t.Fatalf("job context lost the org: %q", forgeOrgOf(jobCtx))
	}
}
