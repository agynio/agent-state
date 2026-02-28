package state

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
)

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrInvalidRangeSpec     = errors.New("invalid range specification")
	ErrMessageNotFound      = errors.New("message not found")
	ErrDuplicateMessage     = errors.New("duplicate message id")
	ErrSnapshotNotFound     = errors.New("snapshot not found")
)

type Role int16

const (
	RoleUnspecified Role = 0
	RoleUser        Role = 1
	RoleAssistant   Role = 2
	RoleTool        Role = 3
)

type ImageDetail int16

const (
	ImageDetailUnspecified ImageDetail = 0
	ImageDetailAuto        ImageDetail = 1
	ImageDetailLow         ImageDetail = 2
	ImageDetailHigh        ImageDetail = 3
)

type MessageBody struct {
	Text       *TextBody
	ToolOutput *ToolOutputBody
}

type TextBody struct {
	Text string
}

type ToolOutputBody struct {
	ToolName   string
	ToolCallID string
	Metadata   map[string]string
	Text       *ToolTextOutput
	Image      *ToolImageOutput
}

type ToolTextOutput struct {
	Text string
}

type ToolImageOutput struct {
	ExternalURL string
	MimeType    string
	Detail      ImageDetail
	WidthPX     int32
	HeightPX    int32
	SHA256      string
}

func (b MessageBody) Validate() error {
	if b.Text != nil && b.ToolOutput != nil {
		return errors.New("message body has multiple variants")
	}
	if b.Text == nil && b.ToolOutput == nil {
		return errors.New("message body missing variant")
	}
	if b.ToolOutput != nil {
		return b.ToolOutput.validate()
	}
	return nil
}

func (b *ToolOutputBody) validate() error {
	if b.Text != nil && b.Image != nil {
		return errors.New("tool output has multiple variants")
	}
	if b.Text == nil && b.Image == nil {
		return errors.New("tool output missing variant")
	}
	return nil
}

func (b MessageBody) MarshalDeterministicJSON() ([]byte, error) {
	type toolText struct {
		Text string `json:"text"`
	}
	type toolImage struct {
		ExternalURL string      `json:"external_url"`
		MimeType    string      `json:"mime_type"`
		Detail      ImageDetail `json:"detail"`
		WidthPX     int32       `json:"width_px"`
		HeightPX    int32       `json:"height_px"`
		SHA256      string      `json:"sha256"`
	}
	type toolOutput struct {
		ToolName   string      `json:"tool_name"`
		ToolCallID string      `json:"tool_call_id"`
		Metadata   [][2]string `json:"metadata,omitempty"`
		Text       *toolText   `json:"text,omitempty"`
		Image      *toolImage  `json:"image,omitempty"`
	}
	type payload struct {
		Text       *TextBody   `json:"text,omitempty"`
		ToolOutput *toolOutput `json:"tool_output,omitempty"`
	}

	result := payload{}
	if b.Text != nil {
		result.Text = b.Text
	}
	if b.ToolOutput != nil {
		sortedMeta := make([][2]string, 0, len(b.ToolOutput.Metadata))
		if len(b.ToolOutput.Metadata) > 0 {
			keys := make([]string, 0, len(b.ToolOutput.Metadata))
			for k := range b.ToolOutput.Metadata {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				sortedMeta = append(sortedMeta, [2]string{k, b.ToolOutput.Metadata[k]})
			}
		}
		out := &toolOutput{
			ToolName:   b.ToolOutput.ToolName,
			ToolCallID: b.ToolOutput.ToolCallID,
			Metadata:   sortedMeta,
		}
		if b.ToolOutput.Text != nil {
			out.Text = &toolText{Text: b.ToolOutput.Text.Text}
		}
		if b.ToolOutput.Image != nil {
			out.Image = &toolImage{
				ExternalURL: b.ToolOutput.Image.ExternalURL,
				MimeType:    b.ToolOutput.Image.MimeType,
				Detail:      b.ToolOutput.Image.Detail,
				WidthPX:     b.ToolOutput.Image.WidthPX,
				HeightPX:    b.ToolOutput.Image.HeightPX,
				SHA256:      b.ToolOutput.Image.SHA256,
			}
		}
		result.ToolOutput = out
	}
	return json.Marshal(result)
}

type Message struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	Role           Role
	Kind           string
	Body           MessageBody
	CreatedAt      time.Time
}

type ConversationMetadata struct {
	ID           uuid.UUID
	MessageCount int64
	UpdatedAt    time.Time
	Version      int64
}

type AppendMessageInput struct {
	Role Role
	Kind string
	Body MessageBody
}

type MessageListFilter struct {
	Roles []Role
	Kinds []string
}

type PageCursor struct {
	AfterPosition int64
}

type MessageListResult struct {
	Metadata   ConversationMetadata
	Messages   []Message
	NextCursor *PageCursor
}

type MessageIDListResult struct {
	Metadata   ConversationMetadata
	MessageIDs []uuid.UUID
	NextCursor *PageCursor
}

type ReplaceRangeSpec struct {
	FromID    *uuid.UUID
	ToID      *uuid.UUID
	InsertIDs []uuid.UUID
}

type ReplaceRangeResult struct {
	Metadata           ConversationMetadata
	ReplacedMessageIDs []uuid.UUID
	InsertedMessageIDs []uuid.UUID
}

type DeleteRangeSpec struct {
	FromID *uuid.UUID
	ToID   *uuid.UUID
}

type DeleteRangeResult struct {
	Metadata          ConversationMetadata
	DeletedMessageIDs []uuid.UUID
}

type Snapshot struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	CreatedAt      time.Time
}

type SnapshotListResult struct {
	Messages   []Message
	NextCursor *PageCursor
}

type SnapshotIDListResult struct {
	MessageIDs []uuid.UUID
	NextCursor *PageCursor
}
