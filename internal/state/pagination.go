package state

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

const (
	defaultPageSize int32 = 50
	maxPageSize     int32 = 100
)

func normalizePageSize(size int32) int32 {
	if size <= 0 {
		return defaultPageSize
	}
	if size > maxPageSize {
		return maxPageSize
	}
	return size
}

type conversationPageToken struct {
	ConversationID string `json:"conversation_id"`
	Position       int64  `json:"position"`
}

func EncodeConversationPageToken(id uuid.UUID, position int64) (string, error) {
	payload := conversationPageToken{ConversationID: id.String(), Position: position}
	buf, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func DecodeConversationPageToken(token string) (uuid.UUID, int64, error) {
	if token == "" {
		return uuid.UUID{}, 0, errors.New("empty token")
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return uuid.UUID{}, 0, fmt.Errorf("decode token: %w", err)
	}
	var payload conversationPageToken
	if err := json.Unmarshal(data, &payload); err != nil {
		return uuid.UUID{}, 0, fmt.Errorf("unmarshal token: %w", err)
	}
	id, err := uuid.Parse(payload.ConversationID)
	if err != nil {
		return uuid.UUID{}, 0, fmt.Errorf("parse conversation id: %w", err)
	}
	return id, payload.Position, nil
}

type snapshotPageToken struct {
	SnapshotID string `json:"snapshot_id"`
	OrderIndex int64  `json:"order_index"`
}

func EncodeSnapshotPageToken(id uuid.UUID, orderIdx int64) (string, error) {
	payload := snapshotPageToken{SnapshotID: id.String(), OrderIndex: orderIdx}
	buf, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func DecodeSnapshotPageToken(token string) (uuid.UUID, int64, error) {
	if token == "" {
		return uuid.UUID{}, 0, errors.New("empty token")
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return uuid.UUID{}, 0, fmt.Errorf("decode token: %w", err)
	}
	var payload snapshotPageToken
	if err := json.Unmarshal(data, &payload); err != nil {
		return uuid.UUID{}, 0, fmt.Errorf("unmarshal token: %w", err)
	}
	id, err := uuid.Parse(payload.SnapshotID)
	if err != nil {
		return uuid.UUID{}, 0, fmt.Errorf("parse snapshot id: %w", err)
	}
	return id, payload.OrderIndex, nil
}
