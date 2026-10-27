package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMergeGateBudgetContract(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot, "scripts/merge-gate-budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got gateBudgets
	if err = json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	want := gateBudgets{Wall: 300, Steps: map[string]int{"plan": 25, "npm-r": 60, "npm-frontends-web": 60, "npm-tools-protoc-gen-es": 15, "generate": 60, "plan-2": 20, "build": 45, "test": 120, "typecheck": 120, "lint": 60, "proto": 15, "vet": 60, "migsafety": 10, "migorder": 15}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("budget changed: got %+v want %+v; tier slow checks into release, do not raise the 300s target", got, want)
	}
	// --amend critical path: plan, install, generate, replan, parallel readers.
	if got.Steps["plan"]+60+got.Steps["generate"]+got.Steps["plan-2"]+120 > got.Wall {
		t.Fatal("critical path budgets exceed 300s")
	}
}

func TestGateReportFailureFirstAndHealthNotPRBlame(t *testing.T) {
	dir := t.TempDir()
	log := "--- FAIL: TestExact/sub (package example/pkg)\n" + strings.Repeat("noise\n", 1000) + "\x1b[31m FAIL \x1b[0m src/a.test.ts > suite > exact name\n"
	os.WriteFile(filepath.Join(dir, "test.log"), []byte(log), 0600)
	os.WriteFile(filepath.Join(dir, "test.row"), []byte("test 1 301 0\n"), 0600)
	var out bytes.Buffer
	rc := runGateReport([]string{"-dir", dir, "-wall", "301", "-budgets", filepath.Join(repoRoot, "scripts/merge-gate-budgets.json")}, &out, &out)
	if rc != 0 {
		t.Fatalf("timing must not be PR failure: %d %s", rc, &out)
	}
	s := out.String()
	if !strings.HasPrefix(s, "--- FAIL: TestExact/sub (package example/pkg)\nFAIL src/a.test.ts > suite > exact name") {
		t.Fatalf("failure identity not first: %s", s)
	}
	if !strings.Contains(s, "requirement NOT MET") || !strings.Contains(s, "NOT PR blame") || strings.Contains(s, "noise") {
		t.Fatalf("bad health report: %s", s)
	}
}

func TestBudgetFilterFailureIncludesPackage(t *testing.T) {
	var out bytes.Buffer
	f := newBudgetFilter(0, nil, &out)
	f.Write([]byte(`{"Action":"fail","Package":"example/p","Test":"TestEarly/sub"}` + "\n"))
	if !strings.Contains(out.String(), "--- FAIL: TestEarly/sub (package example/p)") {
		t.Fatal(out.String())
	}
}

func TestMergeTierDefersReleaseOnlyLanes(t *testing.T) {
	pl := &Plan{Files: []FileReport{{Path: "internal/handlers/chat/rpc_get.go"}}, Go: GoPlan{LaneFull: selected("pin"), Packages: []string{"./internal/handlers/chat"}}, Web: WebPlan{LaneFull: selected("fallback"), Tests: []string{"src/x.test.ts"}}, Electron: ElectronPlan{LaneFull: selected("read")}, DocGen: DocGenPlan{LaneFull: selected("input")}, ForgeGen: ForgeGenPlan{LaneFull: selected("handler")}}
	mergeTier(pl)
	if pl.Go.Full || pl.Web.Full || pl.Electron.Full || pl.DocGen.Full || pl.ForgeGen.Full || len(pl.Web.Tests) != 0 {
		t.Fatalf("release lane leaked: %+v", pl)
	}
	if len(pl.Go.Packages) != 1 || len(pl.ReleaseDeferred) == 0 {
		t.Fatal("lost scoped tests or deferral accounting")
	}
}

func TestMergeTierCodegenInputs(t *testing.T) {
	for _, p := range []string{"proto/a.proto", "db/migrations/1.up.sql", "internal/foo/contract.go", "forge.yaml"} {
		pl := &Plan{Files: []FileReport{{Path: p}}, ForgeGen: ForgeGenPlan{LaneFull: selected("input")}}
		mergeTier(pl)
		if !pl.ForgeGen.Full {
			t.Errorf("missed input %s", p)
		}
	}
	for _, p := range []string{"gen/a.pb.go", "internal/handlers/chat/rpc_get_test.go", ".forge/hashes.json"} {
		pl := &Plan{Files: []FileReport{{Path: p}}, ForgeGen: ForgeGenPlan{LaneFull: selected("output")}}
		mergeTier(pl)
		if pl.ForgeGen.Full {
			t.Errorf("output selected as input: %s", p)
		}
	}
}

func TestMergeTierCoverageAndWiring(t *testing.T) {
	read := func(p string) string {
		t.Helper()
		b, e := os.ReadFile(filepath.Join(repoRoot, p))
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	merge, release := read("scripts/merge-gate.sh"), read("scripts/release-gate.sh")
	for _, s := range []string{"plan -merge", "-lanes go,web -p", "phase proto task proto:lint", "phase vet", "gate-report", "summary.md"} {
		if !strings.Contains(merge, s) {
			t.Errorf("merge missing %s", s)
		}
	}
	for _, s := range []string{"task generate:docs:check", "task build:web:verify", "task test ;", "task test:integration", "task test:e2e", "go build ./...", "go vet ./...", "STEPS=(npm generate docs lint build webbuild", "INCOMPLETE", `($required - [.steps[]`} {
		if !strings.Contains(release, s) {
			t.Errorf("release missing %s", s)
		}
	}
	pl := &Plan{Merge: true, Go: GoPlan{Packages: []string{"./internal/x"}}}
	dir := gateDir(t, pl, map[string]int{"build": 0, "test": 0})
	problems := strings.Join(verifyGate(filepath.Join(dir, "plan.json"), dir), ";")
	if !strings.Contains(problems, "proto") || !strings.Contains(problems, "vet") {
		t.Fatalf("missing obliged checks: %s", problems)
	}
}
