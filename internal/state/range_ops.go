package state

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) ReplaceConversationMessagesRange(ctx context.Context, conversationID uuid.UUID, spec ReplaceRangeSpec) (ReplaceRangeResult, error) {
	if spec.FromID == nil && spec.ToID == nil {
		return ReplaceRangeResult{}, ErrInvalidRangeSpec
	}
	var result ReplaceRangeResult
	err := s.runTx(ctx, func(tx pgx.Tx) error {
		metadata, err := s.loadConversationMetadataForUpdate(ctx, tx, conversationID)
		if err != nil {
			return err
		}

		order, err := fetchConversationOrder(ctx, tx, conversationID)
		if err != nil {
			return err
		}

		indexMap := make(map[uuid.UUID]int, len(order))
		for idx, id := range order {
			indexMap[id] = idx
		}

		working := make([]uuid.UUID, len(order))
		copy(working, order)

		var (
			insertPos int
			replaced  []uuid.UUID
		)

		switch {
		case spec.FromID != nil && spec.ToID != nil:
			fromIdx, ok := indexMap[*spec.FromID]
			if !ok {
				return ErrMessageNotFound
			}
			toIdx, ok := indexMap[*spec.ToID]
			if !ok {
				return ErrMessageNotFound
			}
			if fromIdx > toIdx {
				return ErrInvalidRangeSpec
			}
			replaced = append(replaced, working[fromIdx:toIdx+1]...)
			working = append(working[:fromIdx], working[toIdx+1:]...)
			insertPos = fromIdx
		case spec.FromID != nil:
			fromIdx, ok := indexMap[*spec.FromID]
			if !ok {
				return ErrMessageNotFound
			}
			if len(spec.InsertIDs) == 0 {
				return ErrInvalidRangeSpec
			}
			insertPos = fromIdx + 1
		case spec.ToID != nil:
			toIdx, ok := indexMap[*spec.ToID]
			if !ok {
				return ErrMessageNotFound
			}
			if len(spec.InsertIDs) == 0 {
				return ErrInvalidRangeSpec
			}
			insertPos = toIdx
		}

		if len(spec.InsertIDs) > 0 {
			if err := ensureMessagesExist(ctx, tx, conversationID, spec.InsertIDs); err != nil {
				return err
			}
			idsCopy := append([]uuid.UUID(nil), spec.InsertIDs...)
			if insertPos > len(working) {
				insertPos = len(working)
			}
			working = append(working[:insertPos], append(idsCopy, working[insertPos:]...)...)
		}

		if err := ensureUnique(working); err != nil {
			return err
		}

		if err := rewriteConversationOrder(ctx, tx, conversationID, working); err != nil {
			return err
		}

		var updatedAt time.Time
		if err := tx.QueryRow(ctx,
			`UPDATE conversations SET message_count = $2, updated_at = NOW(), version = version + 1 WHERE id = $1 RETURNING message_count, updated_at, version`,
			conversationID,
			int64(len(working)),
		).Scan(&metadata.MessageCount, &updatedAt, &metadata.Version); err != nil {
			return fmt.Errorf("update conversation: %w", err)
		}
		metadata.UpdatedAt = updatedAt
		result.Metadata = metadata
		result.ReplacedMessageIDs = replaced
		result.InsertedMessageIDs = append([]uuid.UUID(nil), spec.InsertIDs...)
		return nil
	})
	if err != nil {
		return ReplaceRangeResult{}, err
	}
	return result, nil
}

func (s *Store) DeleteConversationMessagesRange(ctx context.Context, conversationID uuid.UUID, spec DeleteRangeSpec) (DeleteRangeResult, error) {
	if spec.FromID == nil && spec.ToID == nil {
		return DeleteRangeResult{}, ErrInvalidRangeSpec
	}
	var result DeleteRangeResult
	err := s.runTx(ctx, func(tx pgx.Tx) error {
		metadata, err := s.loadConversationMetadataForUpdate(ctx, tx, conversationID)
		if err != nil {
			return err
		}

		order, err := fetchConversationOrder(ctx, tx, conversationID)
		if err != nil {
			return err
		}
		indexMap := make(map[uuid.UUID]int, len(order))
		for idx, id := range order {
			indexMap[id] = idx
		}

		if len(order) == 0 {
			return ErrInvalidRangeSpec
		}

		fromIdx := 0
		toIdx := len(order) - 1
		if spec.FromID != nil {
			var ok bool
			fromIdx, ok = indexMap[*spec.FromID]
			if !ok {
				return ErrMessageNotFound
			}
		}
		if spec.ToID != nil {
			var ok bool
			toIdx, ok = indexMap[*spec.ToID]
			if !ok {
				return ErrMessageNotFound
			}
		}
		if fromIdx > toIdx {
			return ErrInvalidRangeSpec
		}

		deleted := append([]uuid.UUID(nil), order[fromIdx:toIdx+1]...)
		remaining := append([]uuid.UUID{}, order[:fromIdx]...)
		remaining = append(remaining, order[toIdx+1:]...)

		if err := rewriteConversationOrder(ctx, tx, conversationID, remaining); err != nil {
			return err
		}
		if len(deleted) > 0 {
			if _, err := tx.Exec(ctx, `DELETE FROM messages WHERE conversation_id = $1 AND id = ANY($2::uuid[])`, conversationID, deleted); err != nil {
				return fmt.Errorf("delete messages: %w", err)
			}
		}

		var updatedAt time.Time
		if err := tx.QueryRow(ctx,
			`UPDATE conversations SET message_count = $2, updated_at = NOW(), version = version + 1 WHERE id = $1 RETURNING message_count, updated_at, version`,
			conversationID,
			int64(len(remaining)),
		).Scan(&metadata.MessageCount, &updatedAt, &metadata.Version); err != nil {
			return fmt.Errorf("update conversation: %w", err)
		}
		metadata.UpdatedAt = updatedAt
		result.Metadata = metadata
		result.DeletedMessageIDs = deleted
		return nil
	})
	if err != nil {
		return DeleteRangeResult{}, err
	}
	return result, nil
}

func fetchConversationOrder(ctx context.Context, tx pgx.Tx, conversationID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `SELECT message_id FROM conversation_items WHERE conversation_id = $1 ORDER BY position ASC`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("load conversation order: %w", err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func ensureMessagesExist(ctx context.Context, tx pgx.Tx, conversationID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	presence := make(map[uuid.UUID]bool, len(ids))
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := presence[id]; !ok {
			unique = append(unique, id)
		}
		presence[id] = false
	}
	rows, err := tx.Query(ctx, `SELECT id FROM messages WHERE conversation_id = $1 AND id = ANY($2::uuid[])`, conversationID, unique)
	if err != nil {
		return fmt.Errorf("verify message ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return err
		}
		presence[id] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for id, ok := range presence {
		if !ok {
			return fmt.Errorf("%w: %s", ErrMessageNotFound, id)
		}
	}
	return nil
}

func ensureUnique(ids []uuid.UUID) error {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return ErrDuplicateMessage
		}
		seen[id] = struct{}{}
	}
	return nil
}

func rewriteConversationOrder(ctx context.Context, tx pgx.Tx, conversationID uuid.UUID, order []uuid.UUID) error {
	if _, err := tx.Exec(ctx, `DELETE FROM conversation_items WHERE conversation_id = $1`, conversationID); err != nil {
		return fmt.Errorf("clear conversation order: %w", err)
	}
	for _, id := range order {
		if _, err := tx.Exec(ctx, `INSERT INTO conversation_items (conversation_id, message_id) VALUES ($1, $2)`, conversationID, id); err != nil {
			return fmt.Errorf("insert conversation item: %w", err)
		}
	}
	return nil
}
