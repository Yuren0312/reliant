package daemonruntime

import (
	"context"
	"github.com/reliant-labs/reliant/internal/config"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkflowSourcesNestedRepos(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Seed the same nested-repo source cache memories and skills consume, so
	// this test needs no git subprocess and remains fast under -short.
	repoSourcesMu.Lock()
	repoSourcesCache[root] = repoSourcesEntry{sources: []string{"forge", "control-plane"}, expires: time.Now().Add(repoSourcesTTL)}
	repoSourcesMu.Unlock()
	t.Cleanup(func() { repoSourcesMu.Lock(); delete(repoSourcesCache, root); repoSourcesMu.Unlock() })
	writeRepoFile(t, filepath.Join(root, "control-plane/.reliant/workflows"), "merge-steward.yaml", "name: merge-steward\n")
	writeRepoFile(t, filepath.Join(root, "forge/.reliant/workflows"), "build.yaml", "name: build\n")
	writeRepoFile(t, filepath.Join(root, "control-plane/.reliant/workflows/merge-steward/scenarios"), "check.yaml", "name: check\n")
	sources := readWorkflowSources(root)
	scenarios, _ := indexScenarios(sources)
	require.Len(t, scenarios, 1)
	require.Equal(t, "merge-steward", scenarios[0].WorkflowSlug)
	require.Equal(t, "control-plane/.reliant/workflows/merge-steward/scenarios/check.yaml", scenarios[0].RelativePath)
	indexed, version := indexWorkflows(sources)
	require.Len(t, indexed, 2)
	require.Equal(t, "control-plane/.reliant/workflows/merge-steward.yaml", indexed[0].RelativePath)
	record, err := (&filesystemConfigStore{}).GetProjectConfigRecord(context.Background(), root)
	require.NoError(t, err)
	stored, err := config.ParseStoredWorkflows(record.ProjectWorkflowsJSON)
	require.NoError(t, err)
	e, err := config.ProjectWorkflowIndex(stored).Lookup("merge-steward")
	require.NoError(t, err)
	require.Equal(t, "control-plane", e.Repo)
	require.NoError(t, os.WriteFile(filepath.Join(root, "control-plane/.reliant/workflows/merge-steward.yaml"), []byte("name: merge-steward\ndescription: changed\n"), 0600))
	_, changed := indexWorkflows(readWorkflowSources(root))
	require.NotEqual(t, version, changed, "nested edits must change snapshot hash")
	require.NoError(t, os.Remove(filepath.Join(root, "forge/.reliant/workflows/build.yaml")))
	remaining, _ := indexWorkflows(readWorkflowSources(root))
	require.Len(t, remaining, 1)
}
