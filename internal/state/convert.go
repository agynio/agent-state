package state

import (
	"encoding/json"
	"errors"
	"fmt"
)

type messageColumns struct {
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
}

func encodeMessageBody(body MessageBody) (messageColumns, error) {
	if err := body.Validate(); err != nil {
		return messageColumns{}, err
	}
	cols := messageColumns{}
	if body.Text != nil {
		cols.BodyText = stringPtr(body.Text.Text)
		return cols, nil
	}
	out := body.ToolOutput
	cols.ToolName = stringPtr(out.ToolName)
	if out.ToolCallID != "" {
		cols.ToolCallID = stringPtr(out.ToolCallID)
	}
	if out.Text != nil {
		cols.ToolTextOutput = stringPtr(out.Text.Text)
	}
	if out.Image != nil {
		cols.ToolImageExternalURL = stringPtr(out.Image.ExternalURL)
		if out.Image.MimeType != "" {
			cols.ToolImageMimeType = stringPtr(out.Image.MimeType)
		}
		detail := int16(out.Image.Detail)
		cols.ToolImageDetail = &detail
		if out.Image.WidthPX != 0 {
			width := out.Image.WidthPX
			cols.ToolImageWidthPX = &width
		}
		if out.Image.HeightPX != 0 {
			height := out.Image.HeightPX
			cols.ToolImageHeightPX = &height
		}
		if out.Image.SHA256 != "" {
			cols.ToolImageSHA256 = stringPtr(out.Image.SHA256)
		}
	}
	if len(out.Metadata) > 0 {
		buf, err := json.Marshal(out.Metadata)
		if err != nil {
			return messageColumns{}, fmt.Errorf("marshal metadata: %w", err)
		}
		cols.ToolMetadata = buf
	}
	return cols, nil
}

func decodeMessageBody(cols messageColumns) (MessageBody, error) {
	body := MessageBody{}
	if cols.BodyText != nil {
		body.Text = &TextBody{Text: *cols.BodyText}
		return body, nil
	}
	if cols.ToolName != nil {
		tool := &ToolOutputBody{
			ToolName: *cols.ToolName,
			Metadata: map[string]string{},
		}
		if cols.ToolCallID != nil {
			tool.ToolCallID = *cols.ToolCallID
		}
		if cols.ToolTextOutput != nil {
			tool.Text = &ToolTextOutput{Text: *cols.ToolTextOutput}
		}
		if cols.ToolImageExternalURL != nil || cols.ToolImageMimeType != nil || cols.ToolImageDetail != nil {
			image := &ToolImageOutput{}
			if cols.ToolImageExternalURL != nil {
				image.ExternalURL = *cols.ToolImageExternalURL
			}
			if cols.ToolImageMimeType != nil {
				image.MimeType = *cols.ToolImageMimeType
			}
			if cols.ToolImageDetail != nil {
				image.Detail = ImageDetail(*cols.ToolImageDetail)
			}
			if cols.ToolImageWidthPX != nil {
				image.WidthPX = *cols.ToolImageWidthPX
			}
			if cols.ToolImageHeightPX != nil {
				image.HeightPX = *cols.ToolImageHeightPX
			}
			if cols.ToolImageSHA256 != nil {
				image.SHA256 = *cols.ToolImageSHA256
			}
			tool.Image = image
		}
		if cols.ToolMetadata != nil {
			if err := json.Unmarshal(cols.ToolMetadata, &tool.Metadata); err != nil {
				return MessageBody{}, fmt.Errorf("unmarshal metadata: %w", err)
			}
		}
		if tool.Metadata == nil {
			tool.Metadata = map[string]string{}
		}
		if tool.Text == nil && tool.Image == nil {
			return MessageBody{}, errors.New("tool output missing variant")
		}
		body.ToolOutput = tool
		return body, nil
	}
	return MessageBody{}, errors.New("message body missing variant")
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	s := v
	return &s
}
