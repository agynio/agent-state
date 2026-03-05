package server

import (
	"context"
	"errors"
	"fmt"

	agentstatev1 "github.com/agynio/agent-state/gen/go/agynio/api/agent_state/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/agynio/agent-state/internal/state"
)

type Server struct {
	agentstatev1.UnimplementedAgentStateServiceServer
	store *state.Store
}

func New(store *state.Store) *Server {
	return &Server{store: store}
}

func (s *Server) AppendConversationMessages(ctx context.Context, req *agentstatev1.AppendConversationMessagesRequest) (*agentstatev1.AppendConversationMessagesResponse, error) {
	conversationID, err := parseUUID(req.GetConversationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "conversation_id: %v", err)
	}
	if len(req.GetMessages()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "messages must be provided")
	}
	inputs := make([]state.AppendMessageInput, len(req.GetMessages()))
	for i, msg := range req.GetMessages() {
		input, err := fromProtoAppendMessage(msg)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "message %d: %v", i, err)
		}
		inputs[i] = input
	}

	metadata, appended, err := s.store.AppendConversationMessages(ctx, conversationID, inputs)
	if err != nil {
		return nil, toStatusError(err)
	}

	response := &agentstatev1.AppendConversationMessagesResponse{
		Metadata: toProtoConversationMetadata(metadata),
		Appended: make([]*agentstatev1.Message, len(appended)),
	}
	for i, msg := range appended {
		response.Appended[i] = toProtoMessage(msg)
	}
	return response, nil
}

func (s *Server) ListConversationMessages(ctx context.Context, req *agentstatev1.ListConversationMessagesRequest) (*agentstatev1.ListConversationMessagesResponse, error) {
	conversationID, err := parseUUID(req.GetConversationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "conversation_id: %v", err)
	}

	var cursor *state.PageCursor
	if token := req.GetPageToken(); token != "" {
		tokenID, position, err := state.DecodeConversationPageToken(token)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid page_token: %v", err)
		}
		if tokenID != conversationID {
			return nil, status.Error(codes.InvalidArgument, "page_token does not match conversation")
		}
		cursor = &state.PageCursor{AfterPosition: position}
	}

	filter := state.MessageListFilter{}
	if len(req.GetRoles()) > 0 {
		filter.Roles = make([]state.Role, len(req.GetRoles()))
		for i, role := range req.GetRoles() {
			filter.Roles[i] = state.Role(role)
		}
	}
	if len(req.GetKinds()) > 0 {
		filter.Kinds = append(filter.Kinds, req.GetKinds()...)
	}

	result, err := s.store.ListConversationMessages(ctx, conversationID, filter, req.GetPageSize(), cursor)
	if err != nil {
		return nil, toStatusError(err)
	}

	resp := &agentstatev1.ListConversationMessagesResponse{
		Metadata: toProtoConversationMetadata(result.Metadata),
		Messages: make([]*agentstatev1.Message, len(result.Messages)),
	}
	for i, msg := range result.Messages {
		resp.Messages[i] = toProtoMessage(msg)
	}
	if result.NextCursor != nil {
		token, err := state.EncodeConversationPageToken(conversationID, result.NextCursor.AfterPosition)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "encode page token: %v", err)
		}
		resp.NextPageToken = token
	}
	return resp, nil
}

func (s *Server) ReplaceConversationMessagesRange(ctx context.Context, req *agentstatev1.ReplaceConversationMessagesRangeRequest) (*agentstatev1.ReplaceConversationMessagesRangeResponse, error) {
	conversationID, err := parseUUID(req.GetConversationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "conversation_id: %v", err)
	}
	spec := state.ReplaceRangeSpec{}
	if req.GetFromMessageId() != "" {
		id, err := parseUUID(req.GetFromMessageId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "from_message_id: %v", err)
		}
		spec.FromID = &id
	}
	if req.GetToMessageId() != "" {
		id, err := parseUUID(req.GetToMessageId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "to_message_id: %v", err)
		}
		spec.ToID = &id
	}
	if len(req.GetNewMessageIds()) > 0 {
		spec.InsertIDs = make([]uuid.UUID, len(req.GetNewMessageIds()))
		for i, raw := range req.GetNewMessageIds() {
			id, err := parseUUID(raw)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "new_message_ids[%d]: %v", i, err)
			}
			spec.InsertIDs[i] = id
		}
	}

	result, err := s.store.ReplaceConversationMessagesRange(ctx, conversationID, spec)
	if err != nil {
		return nil, toStatusError(err)
	}
	resp := &agentstatev1.ReplaceConversationMessagesRangeResponse{
		Metadata:           toProtoConversationMetadata(result.Metadata),
		ReplacedMessageIds: uuidsToStrings(result.ReplacedMessageIDs),
		InsertedMessageIds: uuidsToStrings(result.InsertedMessageIDs),
	}
	return resp, nil
}

func (s *Server) DeleteConversationMessagesRange(ctx context.Context, req *agentstatev1.DeleteConversationMessagesRangeRequest) (*agentstatev1.DeleteConversationMessagesRangeResponse, error) {
	conversationID, err := parseUUID(req.GetConversationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "conversation_id: %v", err)
	}
	spec := state.DeleteRangeSpec{}
	if req.GetFromMessageId() != "" {
		id, err := parseUUID(req.GetFromMessageId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "from_message_id: %v", err)
		}
		spec.FromID = &id
	}
	if req.GetToMessageId() != "" {
		id, err := parseUUID(req.GetToMessageId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "to_message_id: %v", err)
		}
		spec.ToID = &id
	}

	result, err := s.store.DeleteConversationMessagesRange(ctx, conversationID, spec)
	if err != nil {
		return nil, toStatusError(err)
	}
	resp := &agentstatev1.DeleteConversationMessagesRangeResponse{
		Metadata:          toProtoConversationMetadata(result.Metadata),
		DeletedMessageIds: uuidsToStrings(result.DeletedMessageIDs),
	}
	return resp, nil
}

func (s *Server) GetConversation(ctx context.Context, req *agentstatev1.GetConversationRequest) (*agentstatev1.GetConversationResponse, error) {
	conversationID, err := parseUUID(req.GetConversationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "conversation_id: %v", err)
	}
	metadata, err := s.store.GetConversationMetadata(ctx, conversationID)
	if err != nil {
		return nil, toStatusError(err)
	}
	return &agentstatev1.GetConversationResponse{Metadata: toProtoConversationMetadata(metadata)}, nil
}

func (s *Server) ListConversationMessageIds(ctx context.Context, req *agentstatev1.ListConversationMessageIdsRequest) (*agentstatev1.ListConversationMessageIdsResponse, error) {
	conversationID, err := parseUUID(req.GetConversationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "conversation_id: %v", err)
	}
	var cursor *state.PageCursor
	if token := req.GetPageToken(); token != "" {
		tokenID, position, err := state.DecodeConversationPageToken(token)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid page_token: %v", err)
		}
		if tokenID != conversationID {
			return nil, status.Error(codes.InvalidArgument, "page_token does not match conversation")
		}
		cursor = &state.PageCursor{AfterPosition: position}
	}

	result, err := s.store.ListConversationMessageIDs(ctx, conversationID, req.GetPageSize(), cursor)
	if err != nil {
		return nil, toStatusError(err)
	}
	resp := &agentstatev1.ListConversationMessageIdsResponse{
		MessageIds: uuidsToStrings(result.MessageIDs),
	}
	if result.NextCursor != nil {
		token, err := state.EncodeConversationPageToken(conversationID, result.NextCursor.AfterPosition)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "encode page token: %v", err)
		}
		resp.NextPageToken = token
	}
	return resp, nil
}

func (s *Server) CreateSnapshot(ctx context.Context, req *agentstatev1.CreateSnapshotRequest) (*agentstatev1.CreateSnapshotResponse, error) {
	conversationID, err := parseUUID(req.GetConversationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "conversation_id: %v", err)
	}
	if len(req.GetMessageIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "message_ids must be provided")
	}
	ids := make([]uuid.UUID, len(req.GetMessageIds()))
	for i, raw := range req.GetMessageIds() {
		id, err := parseUUID(raw)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "message_ids[%d]: %v", i, err)
		}
		ids[i] = id
	}

	snapshot, err := s.store.CreateSnapshot(ctx, conversationID, ids)
	if err != nil {
		return nil, toStatusError(err)
	}
	return &agentstatev1.CreateSnapshotResponse{Snapshot: toProtoSnapshot(snapshot)}, nil
}

func (s *Server) ListSnapshotMessages(ctx context.Context, req *agentstatev1.ListSnapshotMessagesRequest) (*agentstatev1.ListSnapshotMessagesResponse, error) {
	snapshotID, err := parseUUID(req.GetSnapshotId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "snapshot_id: %v", err)
	}
	var cursor *state.PageCursor
	if token := req.GetPageToken(); token != "" {
		tokenID, orderIdx, err := state.DecodeSnapshotPageToken(token)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid page_token: %v", err)
		}
		if tokenID != snapshotID {
			return nil, status.Error(codes.InvalidArgument, "page_token does not match snapshot")
		}
		cursor = &state.PageCursor{AfterPosition: orderIdx}
	}

	result, err := s.store.ListSnapshotMessages(ctx, snapshotID, req.GetPageSize(), cursor)
	if err != nil {
		return nil, toStatusError(err)
	}
	resp := &agentstatev1.ListSnapshotMessagesResponse{
		Messages: make([]*agentstatev1.Message, len(result.Messages)),
	}
	for i, msg := range result.Messages {
		resp.Messages[i] = toProtoMessage(msg)
	}
	if result.NextCursor != nil {
		token, err := state.EncodeSnapshotPageToken(snapshotID, result.NextCursor.AfterPosition)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "encode page token: %v", err)
		}
		resp.NextPageToken = token
	}
	return resp, nil
}

func (s *Server) ListSnapshotMessageIds(ctx context.Context, req *agentstatev1.ListSnapshotMessageIdsRequest) (*agentstatev1.ListSnapshotMessageIdsResponse, error) {
	snapshotID, err := parseUUID(req.GetSnapshotId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "snapshot_id: %v", err)
	}
	var cursor *state.PageCursor
	if token := req.GetPageToken(); token != "" {
		tokenID, orderIdx, err := state.DecodeSnapshotPageToken(token)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid page_token: %v", err)
		}
		if tokenID != snapshotID {
			return nil, status.Error(codes.InvalidArgument, "page_token does not match snapshot")
		}
		cursor = &state.PageCursor{AfterPosition: orderIdx}
	}

	result, err := s.store.ListSnapshotMessageIDs(ctx, snapshotID, req.GetPageSize(), cursor)
	if err != nil {
		return nil, toStatusError(err)
	}
	resp := &agentstatev1.ListSnapshotMessageIdsResponse{
		MessageIds: uuidsToStrings(result.MessageIDs),
	}
	if result.NextCursor != nil {
		token, err := state.EncodeSnapshotPageToken(snapshotID, result.NextCursor.AfterPosition)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "encode page token: %v", err)
		}
		resp.NextPageToken = token
	}
	return resp, nil
}

func parseUUID(value string) (uuid.UUID, error) {
	if value == "" {
		return uuid.UUID{}, fmt.Errorf("value is empty")
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

func toStatusError(err error) error {
	switch {
	case errors.Is(err, state.ErrConversationNotFound), errors.Is(err, state.ErrMessageNotFound), errors.Is(err, state.ErrSnapshotNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, state.ErrInvalidRangeSpec), errors.Is(err, state.ErrDuplicateMessage):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}

func uuidsToStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
