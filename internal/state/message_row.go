package state

import (
	"time"

	"github.com/google/uuid"
)

type messageRow struct {
	ID                   uuid.UUID
	ConversationID       uuid.UUID
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
}

func (r messageRow) toDomain() (Message, error) {
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
		ID:             r.ID,
		ConversationID: r.ConversationID,
		Role:           Role(r.Role),
		Kind:           r.Kind,
		Body:           body,
		CreatedAt:      r.CreatedAt,
	}, nil
}
