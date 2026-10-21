package postgres

import (
	"context"

	"github.com/reliant-labs/reliant/internal/db/core"
	pgdb "github.com/reliant-labs/reliant/internal/db/postgres/generated"
)

type chatStore struct {
	q pgdb.Querier
}

// NewChatStore creates the Postgres chat store: only the chat_updates insert
// is left here (P5b L1a ports it); the chats queries are internal/store's.
func NewChatStore(q pgdb.Querier) core.ChatStore {
	return &chatStore{q: q}
}

func (s *chatStore) CreateChatUpdate(ctx context.Context, update core.ChatUpdate) error {
	return s.q.CreateChatUpdate(ctx, pgdb.CreateChatUpdateParams{
		ID:             update.ID,
		ChatID:         update.ChatID,
		SequenceNumber: update.SequenceNumber,
		UpdateType:     int32(update.UpdateType),
		EntityID:       update.EntityID,
		Data:           string(update.Data),
		CreatedAt:      update.CreatedAt,
	})
}
