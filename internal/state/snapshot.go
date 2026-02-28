package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateSnapshot(ctx context.Context, conversationID uuid.UUID, messageIDs []uuid.UUID) (Snapshot, error) {
	if len(messageIDs) == 0 {
		return Snapshot{}, fmt.Errorf("snapshot requires at least one message")
	}
	var snapshot Snapshot
	err := s.runTx(ctx, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT TRUE FROM conversations WHERE id = $1`, conversationID).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrConversationNotFound
			}
			return err
		}

		uniqueIDs := uniqueUUIDs(messageIDs)
		rows, err := tx.Query(ctx,
			`SELECT id, conversation_id, role, kind, body_text, tool_name, tool_call_id, tool_text_output,
                    tool_image_external_url, tool_image_mime_type, tool_image_detail, tool_image_width_px,
                    tool_image_height_px, tool_image_sha256, tool_metadata, created_at
               FROM messages
              WHERE conversation_id = $1 AND id = ANY($2::uuid[])`,
			conversationID,
			uniqueIDs,
		)
		if err != nil {
			return fmt.Errorf("load messages: %w", err)
		}
		defer rows.Close()

		fetched := make(map[uuid.UUID]messageRow, len(uniqueIDs))
		for rows.Next() {
			var row messageRow
			if err := rows.Scan(
				&row.ID,
				&row.ConversationID,
				&row.Role,
				&row.Kind,
				&row.BodyText,
				&row.ToolName,
				&row.ToolCallID,
				&row.ToolTextOutput,
				&row.ToolImageExternalURL,
				&row.ToolImageMimeType,
				&row.ToolImageDetail,
				&row.ToolImageWidthPX,
				&row.ToolImageHeightPX,
				&row.ToolImageSHA256,
				&row.ToolMetadata,
				&row.CreatedAt,
			); err != nil {
				return err
			}
			fetched[row.ID] = row
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range uniqueIDs {
			if _, ok := fetched[id]; !ok {
				return fmt.Errorf("%w: %s", ErrMessageNotFound, id)
			}
		}

		snapshotID := uuid.New()
		var createdAt time.Time
		if err := tx.QueryRow(ctx, `INSERT INTO snapshots (id, conversation_id) VALUES ($1, $2) RETURNING created_at`, snapshotID, conversationID).Scan(&createdAt); err != nil {
			return fmt.Errorf("insert snapshot: %w", err)
		}

		for idx, id := range messageIDs {
			row, ok := fetched[id]
			if !ok {
				return fmt.Errorf("%w: %s", ErrMessageNotFound, id)
			}
			msg, err := row.toDomain()
			if err != nil {
				return err
			}
			bodyBytes, err := msg.Body.MarshalDeterministicJSON()
			if err != nil {
				return fmt.Errorf("serialize body: %w", err)
			}
			sha := sha256.Sum256(bodyBytes)
			hash := hex.EncodeToString(sha[:])
			cols, err := encodeMessageBody(msg.Body)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO snapshot_items (
                    snapshot_id, order_idx, message_id, role, kind, body_text, tool_name, tool_call_id,
                    tool_text_output, tool_image_external_url, tool_image_mime_type, tool_image_detail,
                    tool_image_width_px, tool_image_height_px, tool_image_sha256, tool_metadata,
                    body_sha256, created_at
                ) VALUES (
                    $1, $2, $3, $4, $5, $6, $7, $8,
                    $9, $10, $11, $12,
                    $13, $14, $15, $16,
                    $17, $18
                )`,
				snapshotID,
				idx,
				id,
				int16(msg.Role),
				msg.Kind,
				cols.BodyText,
				cols.ToolName,
				cols.ToolCallID,
				cols.ToolTextOutput,
				cols.ToolImageExternalURL,
				cols.ToolImageMimeType,
				cols.ToolImageDetail,
				cols.ToolImageWidthPX,
				cols.ToolImageHeightPX,
				cols.ToolImageSHA256,
				cols.ToolMetadata,
				hash,
				msg.CreatedAt,
			); err != nil {
				return fmt.Errorf("insert snapshot item: %w", err)
			}
		}

		snapshot = Snapshot{ID: snapshotID, ConversationID: conversationID, CreatedAt: createdAt}
		return nil
	})
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (s *Store) ListSnapshotMessages(ctx context.Context, snapshotID uuid.UUID, pageSize int32, cursor *PageCursor) (SnapshotListResult, error) {
	snapshot, err := s.loadSnapshot(ctx, snapshotID)
	if err != nil {
		return SnapshotListResult{}, err
	}
	limit := normalizePageSize(pageSize)

	builder := strings.Builder{}
	builder.WriteString(`SELECT si.message_id, si.role, si.kind, si.body_text, si.tool_name, si.tool_call_id,
        si.tool_text_output, si.tool_image_external_url, si.tool_image_mime_type, si.tool_image_detail,
        si.tool_image_width_px, si.tool_image_height_px, si.tool_image_sha256, si.tool_metadata,
        si.created_at, si.order_idx
        FROM snapshot_items si
        WHERE si.snapshot_id = $1`)

	args := []any{snapshotID}
	paramIdx := 2
	if cursor != nil {
		builder.WriteString(fmt.Sprintf(" AND si.order_idx > $%d", paramIdx))
		args = append(args, cursor.AfterPosition)
		paramIdx++
	}
	builder.WriteString(fmt.Sprintf(" ORDER BY si.order_idx ASC LIMIT $%d", paramIdx))
	args = append(args, int(limit)+1)

	rows, err := s.pool.Query(ctx, builder.String(), args...)
	if err != nil {
		return SnapshotListResult{}, err
	}
	defer rows.Close()

	messages := make([]Message, 0, limit)
	var (
		lastOrder int64
		hasMore   bool
	)
	for rows.Next() {
		var row snapshotItemRow
		if err := rows.Scan(
			&row.MessageID,
			&row.Role,
			&row.Kind,
			&row.BodyText,
			&row.ToolName,
			&row.ToolCallID,
			&row.ToolTextOutput,
			&row.ToolImageExternalURL,
			&row.ToolImageMimeType,
			&row.ToolImageDetail,
			&row.ToolImageWidthPX,
			&row.ToolImageHeightPX,
			&row.ToolImageSHA256,
			&row.ToolMetadata,
			&row.CreatedAt,
			&row.OrderIdx,
		); err != nil {
			return SnapshotListResult{}, err
		}
		if int32(len(messages)) == limit {
			hasMore = true
			break
		}
		msg, err := row.toDomain(snapshot.ConversationID)
		if err != nil {
			return SnapshotListResult{}, err
		}
		messages = append(messages, msg)
		lastOrder = row.OrderIdx
	}
	if err := rows.Err(); err != nil {
		return SnapshotListResult{}, err
	}

	var nextCursor *PageCursor
	if hasMore {
		nextCursor = &PageCursor{AfterPosition: lastOrder}
	}

	return SnapshotListResult{
		Messages:   messages,
		NextCursor: nextCursor,
	}, nil
}

func (s *Store) ListSnapshotMessageIDs(ctx context.Context, snapshotID uuid.UUID, pageSize int32, cursor *PageCursor) (SnapshotIDListResult, error) {
	if _, err := s.loadSnapshot(ctx, snapshotID); err != nil {
		return SnapshotIDListResult{}, err
	}
	limit := normalizePageSize(pageSize)

	builder := strings.Builder{}
	builder.WriteString(`SELECT si.order_idx, si.message_id FROM snapshot_items si WHERE si.snapshot_id = $1`)

	args := []any{snapshotID}
	paramIdx := 2
	if cursor != nil {
		builder.WriteString(fmt.Sprintf(" AND si.order_idx > $%d", paramIdx))
		args = append(args, cursor.AfterPosition)
		paramIdx++
	}
	builder.WriteString(fmt.Sprintf(" ORDER BY si.order_idx ASC LIMIT $%d", paramIdx))
	args = append(args, int(limit)+1)

	rows, err := s.pool.Query(ctx, builder.String(), args...)
	if err != nil {
		return SnapshotIDListResult{}, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0, limit)
	var (
		lastOrder int64
		hasMore   bool
	)
	for rows.Next() {
		var orderIdx int64
		var id uuid.UUID
		if err := rows.Scan(&orderIdx, &id); err != nil {
			return SnapshotIDListResult{}, err
		}
		if int32(len(ids)) == limit {
			hasMore = true
			break
		}
		ids = append(ids, id)
		lastOrder = orderIdx
	}
	if err := rows.Err(); err != nil {
		return SnapshotIDListResult{}, err
	}

	var nextCursor *PageCursor
	if hasMore {
		nextCursor = &PageCursor{AfterPosition: lastOrder}
	}

	return SnapshotIDListResult{
		MessageIDs: ids,
		NextCursor: nextCursor,
	}, nil
}

type snapshotItemRow struct {
	MessageID            uuid.UUID
	Role                 int16
	Kind                 string
	BodyText             *string
	ToolName             *string
	ToolCallID           *string
	ToolTextOutput       *string
	ToolImageExternalURL *string
	ToolImageMimeType    *string
	ToolImageDetail      *int16
	ToolImageWidthPX     *int32
	ToolImageHeightPX    *int32
	ToolImageSHA256      *string
	ToolMetadata         []byte
	CreatedAt            time.Time
	OrderIdx             int64
}

func (r snapshotItemRow) toDomain(conversationID uuid.UUID) (Message, error) {
	body, err := decodeMessageBody(messageColumns{
		BodyText:             r.BodyText,
		ToolName:             r.ToolName,
		ToolCallID:           r.ToolCallID,
		ToolTextOutput:       r.ToolTextOutput,
		ToolImageExternalURL: r.ToolImageExternalURL,
		ToolImageMimeType:    r.ToolImageMimeType,
		ToolImageDetail:      r.ToolImageDetail,
		ToolImageWidthPX:     r.ToolImageWidthPX,
		ToolImageHeightPX:    r.ToolImageHeightPX,
		ToolImageSHA256:      r.ToolImageSHA256,
		ToolMetadata:         r.ToolMetadata,
	})
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID:             r.MessageID,
		ConversationID: conversationID,
		Role:           Role(r.Role),
		Kind:           r.Kind,
		Body:           body,
		CreatedAt:      r.CreatedAt,
	}, nil
}

func (s *Store) loadSnapshot(ctx context.Context, snapshotID uuid.UUID) (Snapshot, error) {
	var snapshot Snapshot
	snapshot.ID = snapshotID
	if err := s.pool.QueryRow(ctx, `SELECT conversation_id, created_at FROM snapshots WHERE id = $1`, snapshotID).Scan(&snapshot.ConversationID, &snapshot.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Snapshot{}, ErrSnapshotNotFound
		}
		return Snapshot{}, err
	}
	return snapshot, nil
}

func uniqueUUIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
