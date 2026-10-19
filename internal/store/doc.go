// Copyright (c) 2025 Reliant Labs

// Package store is the P5b spike's forge-ORM store layer: entity structs,
// <Entity>Store interfaces and package-level delegates projected by the
// pinned forge from db/migrations, plus the owned <entity>_repo_ext.go seams
// that hold the hand-tuned queries the generated CRUD cannot express.
//
// forge always writes its ORM into internal/db, where reliant's hand-written
// layer already declares Message, Chat, Workflow and friends. The spike
// therefore projects the ORM in a scratch project over the same migrations and
// re-packages it here, which is what an ORM target package (F5b) would do.
// It runs side by side with the sqlc path; nothing in production imports it.
// See research/forge-migration/08-p5b-spike.md.
package store
