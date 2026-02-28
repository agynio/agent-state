package state

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) AppendConversationMessages(ctx context.Context, conversationID uuid.UUID, inputs []AppendMessageInput) (ConversationMetadata, []Message, error) {
	if len(inputs) == 0 {
		return ConversationMetadata{}, nil, fmt.Errorf("append requires at least one message")
	}
	var (
		metadata ConversationMetadata
		result   []Message
	)
	err := s.runTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO conversations (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`, conversationID); err != nil {
			return fmt.Errorf("ensure conversation: %w", err)
		}

		result = make([]Message, 0, len(inputs))
		for _, input := range inputs {
			cols, err := encodeMessageBody(input.Body)
			if err != nil {
				return err
			}
			messageID := uuid.New()
			var createdAt time.Time
			if err := tx.QueryRow(ctx,
				`INSERT INTO messages (
                    id, conversation_id, role, kind, body_text, tool_name, tool_call_id,
                    tool_text_output, tool_image_external_url, tool_image_mime_type,
                    tool_image_detail, tool_image_width_px, tool_image_height_px,
                    tool_image_sha256, tool_metadata
                ) VALUES (
                    $1, $2, $3, $4, $5, $6, $7,
                    $8, $9, $10,
                    $11, $12, $13,
                    $14, $15
                ) RETURNING created_at`,
				messageID,
				conversationID,
				int16(input.Role),
				input.Kind,
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
			).Scan(&createdAt); err != nil {
				return fmt.Errorf("insert message: %w", err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO conversation_items (conversation_id, message_id) VALUES ($1, $2)`, conversationID, messageID); err != nil {
				return fmt.Errorf("insert conversation item: %w", err)
			}

			msg, err := messageRow{
				ID:                   messageID,
				ConversationID:       conversationID,
				Role:                 int16(input.Role),
				Kind:                 input.Kind,
				BodyText:             cols.BodyText,
				ToolName:             cols.ToolName,
				ToolCallID:           cols.ToolCallID,
				ToolTextOutput:       cols.ToolTextOutput,
				ToolImageExternalURL: cols.ToolImageExternalURL,
				ToolImageMimeType:    cols.ToolImageMimeType,
				ToolImageDetail:      cols.ToolImageDetail,
				ToolImageWidthPX:     cols.ToolImageWidthPX,
				ToolImageHeightPX:    cols.ToolImageHeightPX,
				ToolImageSHA256:      cols.ToolImageSHA256,
				ToolMetadata:         cols.ToolMetadata,
				CreatedAt:            createdAt,
			}.toDomain()
			if err != nil {
				return err
			}
			result = append(result, msg)
		}

		var updatedAt time.Time
		if err := tx.QueryRow(ctx,
			`UPDATE conversations
                SET message_count = message_count + $2,
                    updated_at = NOW(),
                    version = version + 1
              WHERE id = $1
              RETURNING message_count, updated_at, version`,
			conversationID,
			int64(len(inputs)),
		).Scan(&metadata.MessageCount, &updatedAt, &metadata.Version); err != nil {
			return fmt.Errorf("update conversation metadata: %w", err)
		}
		metadata.ID = conversationID
		metadata.UpdatedAt = updatedAt
		return nil
	})
	if err != nil {
		return ConversationMetadata{}, nil, err
	}
	return metadata, result, nil
}
