// Copyright (c) 2025 Reliant Labs
package project

import _ "embed"

// defaultProjectReliantMD is the reliant.md a new project is scaffolded with.
// It is the one place that tells agents in a fresh project what Reliant itself
// can do (scheduled/triggered workflows, forge), so keep it in sync with the
// workflow-builder skill description and the greenfield guidance.
//
//go:embed templates/project_reliant.md
var defaultProjectReliantMD string
