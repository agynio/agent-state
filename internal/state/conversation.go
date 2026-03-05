package state

import (
	"context"

	"github.com/google/uuid"
)

func (s *Store) GetConversationMetadata(ctx context.Context, conversationID uuid.UUID) (ConversationMetadata, error) {
	return s.loadConversationMetadata(ctx, conversationID)
}
