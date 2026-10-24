package validation

import (
	"testing"

	reliantv1 "github.com/reliant-labs/reliant/gen/reliant/v1"
	wfyaml "github.com/reliant-labs/reliant/internal/workflow/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticAnalysisValidatesReferencedChildStructure(t *testing.T) {
	child, err := wfyaml.ParseWorkflow([]byte("name: child\nentry: [missing]\nnodes:\n  - {id: a, type: approval, args: {title: ok}}\n"))
	require.NoError(t, err)
	root := refChainWorkflow(t, "root", "project://child")
	result := StaticAnalysis(root, func(ref string) (*reliantv1.Workflow, error) {
		require.Equal(t, "project://child", ref)
		return child, nil
	})
	require.True(t, result.HasErrors(), "a child's invalid graph must block its parent")
	assert.Contains(t, result.Error(), "missing")
	assert.Contains(t, result.Error(), "child")
}

func TestStaticAnalysisCachesDefinitionPerCanonicalRef(t *testing.T) {
	root := refChainWorkflow(t, "root", "project://child")
	child := refChainWorkflow(t, "child", "project://root")
	calls := 0
	result := StaticAnalysisWithOptions(root, &ValidationOptions{
		CanonicalWorkflowRef: "project://root",
		WorkflowLoader: func(ref string) (*reliantv1.Workflow, error) {
			calls++
			require.LessOrEqual(t, calls, 2, "cyclic definitions must not cause unbounded validation")
			if ref == "project://child" {
				return child, nil
			}
			return root, nil
		},
	})
	assert.True(t, result.HasErrors(), "ref cycles remain invalid")
	assert.Contains(t, result.Error(), "cycle")
	assert.Equal(t, 1, calls, "root and alias are the same already-loaded definition")
}
