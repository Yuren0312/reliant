// Copyright (c) 2025 Reliant Labs
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/reliant-labs/forge/pkg/orm"
)

// runControlLockTimeout bounds how long a caller waits for another request's
// run-control critical section (see internal/db/run_control_lock.go).
const runControlLockTimeout = "15s"

// ErrRunControlInTx refuses a run-control lock taken inside a transaction: it
// would be released at THAT transaction's commit, which is not the critical
// section the caller means.
var ErrRunControlInTx = errors.New("run-control lock must not be taken inside a transaction")

// LockChatRunControl is the forge-ORM port of db.Repo.LockChatRunControl: a
// transaction-scoped advisory lock held by a READ COMMITTED transaction that
// exists only to own it, released by the returned func or when ctx ends.
//
// It takes the *orm.Client rather than an orm.Context: orm.Context can only
// run a closure-scoped transaction (RunTx*), and this lock must outlive the
// call that takes it. The transaction is deliberately NOT put in the caller's
// context, so the caller's reads and writes commit independently of it.
func LockChatRunControl(ctx context.Context, client *orm.Client, chatID string) (func(), error) {
	if client == nil {
		return nil, errors.New("database connection not initialized")
	}
	// orm has no exported "is ctx in a transaction" probe. AfterCommit fails
	// with ErrNoTransaction exactly when there is none — the only exported
	// signal (F12). The registered func is a no-op.
	if err := orm.AfterCommit(ctx, func(context.Context) {}); err == nil {
		return nil, ErrRunControlInTx
	}

	tx, err := client.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin run-control lock transaction: %w", err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '`+runControlLockTimeout+`'`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("set run-control lock timeout: %w", err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "run-control:"+chatID); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("acquire run-control lock: %w", err)
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			// Nothing was written; rolling back lets the lock go.
			_ = tx.Rollback()
		})
	}, nil
}
