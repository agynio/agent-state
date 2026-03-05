package e2e

import (
	"context"
	"net"
	"testing"
	"time"

	agentstatev1 "github.com/agynio/agent-state/gen/go/agynio/api/agent_state/v1"
	"github.com/agynio/agent-state/internal/db"
	"github.com/agynio/agent-state/internal/server"
	"github.com/agynio/agent-state/internal/state"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestAgentStateServiceE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	require.NoError(t, db.ApplyMigrations(ctx, pool))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	agentstatev1.RegisterAgentStateServiceServer(grpcServer, server.New(state.NewStore(pool)))

	go func() {
		_ = grpcServer.Serve(lis)
	}()
	defer grpcServer.GracefulStop()

	conn, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	require.NoError(t, err)
	defer conn.Close()

	client := agentstatev1.NewAgentStateServiceClient(conn)

	conversationID := uuid.NewString()

	appendResp, err := client.AppendConversationMessages(ctx, &agentstatev1.AppendConversationMessagesRequest{
		ConversationId: conversationID,
		Messages: []*agentstatev1.AppendMessageInput{
			{
				Role: agentstatev1.Role_ROLE_USER,
				Kind: "user",
				Body: &agentstatev1.MessageBody{Body: &agentstatev1.MessageBody_Text{Text: &agentstatev1.TextBody{Text: "hello"}}},
			},
			{
				Role: agentstatev1.Role_ROLE_ASSISTANT,
				Kind: "assistant",
				Body: &agentstatev1.MessageBody{Body: &agentstatev1.MessageBody_Text{Text: &agentstatev1.TextBody{Text: "hi!"}}},
			},
			{
				Role: agentstatev1.Role_ROLE_TOOL,
				Kind: "tool_output",
				Body: &agentstatev1.MessageBody{Body: &agentstatev1.MessageBody_ToolOutput{ToolOutput: &agentstatev1.ToolOutputBody{
					ToolName:   "fetch",
					ToolCallId: "call-1",
					Output:     &agentstatev1.ToolOutputBody_Text{Text: &agentstatev1.ToolTextOutput{Text: "result"}},
					Metadata:   map[string]string{"source": "unit"},
				}}},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, appendResp.Appended, 3)

	msg1ID := appendResp.Appended[0].GetId()
	msg2ID := appendResp.Appended[1].GetId()
	msg3ID := appendResp.Appended[2].GetId()

	listResp, err := client.ListConversationMessages(ctx, &agentstatev1.ListConversationMessagesRequest{
		ConversationId: conversationID,
		PageSize:       2,
	})
	require.NoError(t, err)
	require.Len(t, listResp.Messages, 2)
	require.NotEmpty(t, listResp.NextPageToken)
	require.Equal(t, msg1ID, listResp.Messages[0].GetId())
	require.Equal(t, msg2ID, listResp.Messages[1].GetId())

	listResp2, err := client.ListConversationMessages(ctx, &agentstatev1.ListConversationMessagesRequest{
		ConversationId: conversationID,
		PageToken:      listResp.NextPageToken,
	})
	require.NoError(t, err)
	require.Len(t, listResp2.Messages, 1)
	require.Empty(t, listResp2.NextPageToken)
	require.Equal(t, msg3ID, listResp2.Messages[0].GetId())

	idsResp, err := client.ListConversationMessageIds(ctx, &agentstatev1.ListConversationMessageIdsRequest{
		ConversationId: conversationID,
		PageSize:       2,
	})
	require.NoError(t, err)
	require.Len(t, idsResp.MessageIds, 2)
	require.NotEmpty(t, idsResp.NextPageToken)

	idsResp2, err := client.ListConversationMessageIds(ctx, &agentstatev1.ListConversationMessageIdsRequest{
		ConversationId: conversationID,
		PageToken:      idsResp.NextPageToken,
	})
	require.NoError(t, err)
	require.Equal(t, []string{msg3ID}, idsResp2.MessageIds)

	metaResp, err := client.GetConversation(ctx, &agentstatev1.GetConversationRequest{ConversationId: conversationID})
	require.NoError(t, err)
	require.Equal(t, int64(3), metaResp.Metadata.GetMessageCount())

	replaceResp, err := client.ReplaceConversationMessagesRange(ctx, &agentstatev1.ReplaceConversationMessagesRangeRequest{
		ConversationId: conversationID,
		FromMessageId:  msg1ID,
		ToMessageId:    msg1ID,
	})
	require.NoError(t, err)
	require.Equal(t, []string{msg1ID}, replaceResp.ReplacedMessageIds)
	require.Empty(t, replaceResp.InsertedMessageIds)

	metaResp, err = client.GetConversation(ctx, &agentstatev1.GetConversationRequest{ConversationId: conversationID})
	require.NoError(t, err)
	require.Equal(t, int64(2), metaResp.Metadata.GetMessageCount())

	insertResp, err := client.ReplaceConversationMessagesRange(ctx, &agentstatev1.ReplaceConversationMessagesRangeRequest{
		ConversationId: conversationID,
		FromMessageId:  msg2ID,
		NewMessageIds:  []string{msg1ID},
	})
	require.NoError(t, err)
	require.Equal(t, []string{msg1ID}, insertResp.InsertedMessageIds)

	idsAfterInsert, err := client.ListConversationMessageIds(ctx, &agentstatev1.ListConversationMessageIdsRequest{ConversationId: conversationID})
	require.NoError(t, err)
	require.Equal(t, []string{msg2ID, msg1ID, msg3ID}, idsAfterInsert.MessageIds)

	reorderResp, err := client.ReplaceConversationMessagesRange(ctx, &agentstatev1.ReplaceConversationMessagesRangeRequest{
		ConversationId: conversationID,
		FromMessageId:  msg2ID,
		ToMessageId:    msg1ID,
		NewMessageIds:  []string{msg1ID, msg2ID},
	})
	require.NoError(t, err)
	require.Equal(t, []string{msg2ID, msg1ID}, reorderResp.ReplacedMessageIds)

	idsAfterReorder, err := client.ListConversationMessageIds(ctx, &agentstatev1.ListConversationMessageIdsRequest{ConversationId: conversationID})
	require.NoError(t, err)
	require.Equal(t, []string{msg1ID, msg2ID, msg3ID}, idsAfterReorder.MessageIds)

	delHeadResp, err := client.DeleteConversationMessagesRange(ctx, &agentstatev1.DeleteConversationMessagesRangeRequest{
		ConversationId: conversationID,
		ToMessageId:    msg1ID,
	})
	require.NoError(t, err)
	require.Equal(t, []string{msg1ID}, delHeadResp.DeletedMessageIds)

	delTailResp, err := client.DeleteConversationMessagesRange(ctx, &agentstatev1.DeleteConversationMessagesRangeRequest{
		ConversationId: conversationID,
		FromMessageId:  msg3ID,
	})
	require.NoError(t, err)
	require.Equal(t, []string{msg3ID}, delTailResp.DeletedMessageIds)

	idsAfterDeletes, err := client.ListConversationMessageIds(ctx, &agentstatev1.ListConversationMessageIdsRequest{ConversationId: conversationID})
	require.NoError(t, err)
	require.Equal(t, []string{msg2ID}, idsAfterDeletes.MessageIds)

	appendResp2, err := client.AppendConversationMessages(ctx, &agentstatev1.AppendConversationMessagesRequest{
		ConversationId: conversationID,
		Messages: []*agentstatev1.AppendMessageInput{
			{
				Role: agentstatev1.Role_ROLE_ASSISTANT,
				Kind: "assistant",
				Body: &agentstatev1.MessageBody{Body: &agentstatev1.MessageBody_Text{Text: &agentstatev1.TextBody{Text: "follow up"}}},
			},
		},
	})
	require.NoError(t, err)
	msg4ID := appendResp2.Appended[0].GetId()

	appendResp3, err := client.AppendConversationMessages(ctx, &agentstatev1.AppendConversationMessagesRequest{
		ConversationId: conversationID,
		Messages: []*agentstatev1.AppendMessageInput{
			{
				Role: agentstatev1.Role_ROLE_TOOL,
				Kind: "tool_output",
				Body: &agentstatev1.MessageBody{Body: &agentstatev1.MessageBody_ToolOutput{ToolOutput: &agentstatev1.ToolOutputBody{
					ToolName:   "vision",
					ToolCallId: "call-2",
					Output: &agentstatev1.ToolOutputBody_Image{Image: &agentstatev1.ToolImageOutput{
						ExternalUrl: "https://example.com/image.png",
						MimeType:    "image/png",
						Detail:      agentstatev1.ImageDetail_IMAGE_DETAIL_HIGH,
						WidthPx:     512,
						HeightPx:    512,
						Sha256:      "abc123",
					}},
				}}},
			},
		},
	})
	require.NoError(t, err)
	msg5ID := appendResp3.Appended[0].GetId()

	idsFinal, err := client.ListConversationMessageIds(ctx, &agentstatev1.ListConversationMessageIdsRequest{ConversationId: conversationID})
	require.NoError(t, err)
	require.Equal(t, []string{msg2ID, msg4ID, msg5ID}, idsFinal.MessageIds)

	snapshotResp, err := client.CreateSnapshot(ctx, &agentstatev1.CreateSnapshotRequest{
		ConversationId: conversationID,
		MessageIds:     []string{msg2ID, msg4ID},
	})
	require.NoError(t, err)
	snapshotID := snapshotResp.Snapshot.GetSnapshotId()

	snapList1, err := client.ListSnapshotMessages(ctx, &agentstatev1.ListSnapshotMessagesRequest{
		SnapshotId: snapshotID,
		PageSize:   1,
	})
	require.NoError(t, err)
	require.Len(t, snapList1.Messages, 1)
	require.Equal(t, msg2ID, snapList1.Messages[0].GetId())
	require.NotEmpty(t, snapList1.NextPageToken)

	snapList2, err := client.ListSnapshotMessages(ctx, &agentstatev1.ListSnapshotMessagesRequest{
		SnapshotId: snapshotID,
		PageToken:  snapList1.NextPageToken,
	})
	require.NoError(t, err)
	require.Len(t, snapList2.Messages, 1)
	require.Empty(t, snapList2.NextPageToken)
	require.Equal(t, msg4ID, snapList2.Messages[0].GetId())

	snapIDs, err := client.ListSnapshotMessageIds(ctx, &agentstatev1.ListSnapshotMessageIdsRequest{SnapshotId: snapshotID})
	require.NoError(t, err)
	require.Equal(t, []string{msg2ID, msg4ID}, snapIDs.MessageIds)
}
