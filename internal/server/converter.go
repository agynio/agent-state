package server

import (
	"fmt"

	agentstatev1 "github.com/agynio/agent-state/.gen/go/agynio/api/agent_state/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/agynio/agent-state/internal/state"
)

func fromProtoAppendMessage(input *agentstatev1.AppendMessageInput) (state.AppendMessageInput, error) {
	if input == nil {
		return state.AppendMessageInput{}, fmt.Errorf("message is nil")
	}
	if input.GetRole() == agentstatev1.Role_ROLE_UNSPECIFIED {
		return state.AppendMessageInput{}, fmt.Errorf("role must be specified")
	}
	body, err := fromProtoMessageBody(input.GetBody())
	if err != nil {
		return state.AppendMessageInput{}, err
	}
	return state.AppendMessageInput{
		Role: state.Role(input.GetRole()),
		Kind: input.GetKind(),
		Body: body,
	}, nil
}

func fromProtoMessageBody(body *agentstatev1.MessageBody) (state.MessageBody, error) {
	if body == nil {
		return state.MessageBody{}, fmt.Errorf("body is required")
	}
	switch b := body.GetBody().(type) {
	case *agentstatev1.MessageBody_Text:
		if b.Text == nil {
			return state.MessageBody{}, fmt.Errorf("text body missing payload")
		}
		return state.MessageBody{Text: &state.TextBody{Text: b.Text.GetText()}}, nil
	case *agentstatev1.MessageBody_ToolOutput:
		if b.ToolOutput == nil {
			return state.MessageBody{}, fmt.Errorf("tool_output missing payload")
		}
		tool := &state.ToolOutputBody{
			ToolName:   b.ToolOutput.GetToolName(),
			ToolCallID: b.ToolOutput.GetToolCallId(),
			Metadata:   map[string]string{},
		}
		for k, v := range b.ToolOutput.GetMetadata() {
			tool.Metadata[k] = v
		}
		switch out := b.ToolOutput.GetOutput().(type) {
		case *agentstatev1.ToolOutputBody_Text:
			if out.Text == nil {
				return state.MessageBody{}, fmt.Errorf("tool_output.text missing payload")
			}
			tool.Text = &state.ToolTextOutput{Text: out.Text.GetText()}
		case *agentstatev1.ToolOutputBody_Image:
			if out.Image == nil {
				return state.MessageBody{}, fmt.Errorf("tool_output.image missing payload")
			}
			tool.Image = &state.ToolImageOutput{
				ExternalURL: out.Image.GetExternalUrl(),
				MimeType:    out.Image.GetMimeType(),
				Detail:      state.ImageDetail(out.Image.GetDetail()),
				WidthPX:     out.Image.GetWidthPx(),
				HeightPX:    out.Image.GetHeightPx(),
				SHA256:      out.Image.GetSha256(),
			}
		default:
			return state.MessageBody{}, fmt.Errorf("tool_output missing output variant")
		}
		return state.MessageBody{ToolOutput: tool}, nil
	default:
		return state.MessageBody{}, fmt.Errorf("body variant not set")
	}
}

func toProtoMessage(message state.Message) *agentstatev1.Message {
	protoMsg := &agentstatev1.Message{
		Id:             message.ID.String(),
		ConversationId: message.ConversationID.String(),
		Role:           agentstatev1.Role(message.Role),
		Kind:           message.Kind,
		CreatedAt:      timestamppb.New(message.CreatedAt),
	}
	protoMsg.Body = toProtoMessageBody(message.Body)
	return protoMsg
}

func toProtoMessageBody(body state.MessageBody) *agentstatev1.MessageBody {
	result := &agentstatev1.MessageBody{}
	if body.Text != nil {
		result.Body = &agentstatev1.MessageBody_Text{Text: &agentstatev1.TextBody{Text: body.Text.Text}}
		return result
	}
	if body.ToolOutput != nil {
		tool := &agentstatev1.ToolOutputBody{
			ToolName:   body.ToolOutput.ToolName,
			ToolCallId: body.ToolOutput.ToolCallID,
			Metadata:   map[string]string{},
		}
		for k, v := range body.ToolOutput.Metadata {
			tool.Metadata[k] = v
		}
		if body.ToolOutput.Text != nil {
			tool.Output = &agentstatev1.ToolOutputBody_Text{Text: &agentstatev1.ToolTextOutput{Text: body.ToolOutput.Text.Text}}
		}
		if body.ToolOutput.Image != nil {
			tool.Output = &agentstatev1.ToolOutputBody_Image{Image: &agentstatev1.ToolImageOutput{
				ExternalUrl: body.ToolOutput.Image.ExternalURL,
				MimeType:    body.ToolOutput.Image.MimeType,
				Detail:      agentstatev1.ImageDetail(body.ToolOutput.Image.Detail),
				WidthPx:     body.ToolOutput.Image.WidthPX,
				HeightPx:    body.ToolOutput.Image.HeightPX,
				Sha256:      body.ToolOutput.Image.SHA256,
			}}
		}
		result.Body = &agentstatev1.MessageBody_ToolOutput{ToolOutput: tool}
		return result
	}
	return result
}

func toProtoConversationMetadata(metadata state.ConversationMetadata) *agentstatev1.ConversationMetadata {
	return &agentstatev1.ConversationMetadata{
		ConversationId: metadata.ID.String(),
		MessageCount:   metadata.MessageCount,
		UpdatedAt:      timestamppb.New(metadata.UpdatedAt),
	}
}

func toProtoSnapshot(snapshot state.Snapshot) *agentstatev1.ContextSnapshot {
	return &agentstatev1.ContextSnapshot{
		SnapshotId:     snapshot.ID.String(),
		ConversationId: snapshot.ConversationID.String(),
		CreatedAt:      timestamppb.New(snapshot.CreatedAt),
	}
}
