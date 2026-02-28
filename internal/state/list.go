package state

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func (s *Store) ListConversationMessages(ctx context.Context, conversationID uuid.UUID, filter MessageListFilter, pageSize int32, cursor *PageCursor) (MessageListResult, error) {
	metadata, err := s.loadConversationMetadata(ctx, conversationID)
	if err != nil {
		return MessageListResult{}, err
	}
	limit := normalizePageSize(pageSize)

	query := strings.Builder{}
	query.WriteString(`SELECT m.id, m.conversation_id, m.role, m.kind, m.body_text, m.tool_name, m.tool_call_id, m.tool_text_output, m.tool_image_external_url, m.tool_image_mime_type, m.tool_image_detail, m.tool_image_width_px, m.tool_image_height_px, m.tool_image_sha256, m.tool_metadata, m.created_at, ci.position
        FROM conversation_items ci
        JOIN messages m ON m.id = ci.message_id
        WHERE ci.conversation_id = $1`)

	args := []any{conversationID}
	paramIndex := 2

	if len(filter.Roles) > 0 {
		query.WriteString(fmt.Sprintf(" AND m.role = ANY($%d)", paramIndex))
		args = append(args, rolesToInt16(filter.Roles))
		paramIndex++
	}
	if len(filter.Kinds) > 0 {
		query.WriteString(fmt.Sprintf(" AND m.kind = ANY($%d)", paramIndex))
		args = append(args, filter.Kinds)
		paramIndex++
	}
	if cursor != nil {
		query.WriteString(fmt.Sprintf(" AND ci.position > $%d", paramIndex))
		args = append(args, cursor.AfterPosition)
		paramIndex++
	}

	query.WriteString(fmt.Sprintf(" ORDER BY ci.position ASC LIMIT $%d", paramIndex))
	args = append(args, int(limit)+1)

	rows, err := s.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return MessageListResult{}, err
	}
	defer rows.Close()

	messages := make([]Message, 0, limit)
	var (
		nextCursor   *PageCursor
		lastPosition int64
		hasMore      bool
	)
	for rows.Next() {
		var row messageRow
		var position int64
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
			&position,
		); err != nil {
			return MessageListResult{}, err
		}
		if int32(len(messages)) == limit {
			hasMore = true
			break
		}
		msg, err := row.toDomain()
		if err != nil {
			return MessageListResult{}, err
		}
		messages = append(messages, msg)
		lastPosition = position
	}
	if err := rows.Err(); err != nil {
		return MessageListResult{}, err
	}
	if hasMore {
		nextCursor = &PageCursor{AfterPosition: lastPosition}
	}
	return MessageListResult{
		Metadata:   metadata,
		Messages:   messages,
		NextCursor: nextCursor,
	}, nil
}

func (s *Store) ListConversationMessageIDs(ctx context.Context, conversationID uuid.UUID, pageSize int32, cursor *PageCursor) (MessageIDListResult, error) {
	metadata, err := s.loadConversationMetadata(ctx, conversationID)
	if err != nil {
		return MessageIDListResult{}, err
	}
	limit := normalizePageSize(pageSize)

	query := strings.Builder{}
	query.WriteString(`SELECT ci.position, ci.message_id FROM conversation_items ci WHERE ci.conversation_id = $1`)

	args := []any{conversationID}
	paramIndex := 2
	if cursor != nil {
		query.WriteString(fmt.Sprintf(" AND ci.position > $%d", paramIndex))
		args = append(args, cursor.AfterPosition)
		paramIndex++
	}
	query.WriteString(fmt.Sprintf(" ORDER BY ci.position ASC LIMIT $%d", paramIndex))
	args = append(args, int(limit)+1)

	rows, err := s.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return MessageIDListResult{}, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0, limit)
	var (
		nextCursor   *PageCursor
		lastPosition int64
		hasMore      bool
	)
	for rows.Next() {
		var position int64
		var id uuid.UUID
		if err := rows.Scan(&position, &id); err != nil {
			return MessageIDListResult{}, err
		}
		if int32(len(ids)) == limit {
			hasMore = true
			break
		}
		ids = append(ids, id)
		lastPosition = position
	}
	if err := rows.Err(); err != nil {
		return MessageIDListResult{}, err
	}
	if hasMore {
		nextCursor = &PageCursor{AfterPosition: lastPosition}
	}

	return MessageIDListResult{
		Metadata:   metadata,
		MessageIDs: ids,
		NextCursor: nextCursor,
	}, nil
}

func rolesToInt16(roles []Role) []int16 {
	out := make([]int16, len(roles))
	for i, role := range roles {
		out[i] = int16(role)
	}
	return out
}
