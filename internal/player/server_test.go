package player

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	playerv1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1/playerv1connect"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestServerAdmitsHTTP1AndUnencryptedHTTP2RequestStreaming(t *testing.T) {
	coordinator := newConnectTestCoordinator(t)
	service, err := NewConnectService(ConnectServiceConfig{Coordinator: coordinator})
	require.NoError(t, err)
	server, err := NewServer(t.Context(), Config{
		Address: "127.0.0.1:0",
		Assets: fstest.MapFS{
			"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>Player</title>")},
		},
		Connect: service,
	})
	require.NoError(t, err)
	_, err = server.Start(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
		defer cancel()
		require.NoError(t, server.Stop(ctx))
	})

	http1Response, err := http.Get(server.Info().LocalURL)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, http1Response.Body)
	require.NoError(t, err)
	require.NoError(t, http1Response.Body.Close())
	require.Equal(t, 1, http1Response.ProtoMajor)

	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	http2Client := &http.Client{Transport: &http.Transport{Protocols: protocols}}
	http2Response, err := http2Client.Get(server.Info().LocalURL)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, http2Response.Body)
	require.NoError(t, err)
	require.NoError(t, http2Response.Body.Close())
	require.Equal(t, 2, http2Response.ProtoMajor)

	clientID := "server-http2-tab"
	client := playerv1connect.NewPlayerServiceClient(http2Client, server.Info().LocalURL)
	subscriptionContext, cancelSubscription := context.WithCancel(t.Context())
	t.Cleanup(cancelSubscription)
	subscription, err := client.Subscribe(subscriptionContext, connect.NewRequest(&playerv1.SubscribeRequest{
		ClientInstanceId: &clientID,
	}))
	require.NoError(t, err)
	require.True(t, subscription.Receive(), "initial subscription: %v", subscription.Err())
	snapshot := subscription.Msg().GetSnapshot()
	require.NotNil(t, snapshot)

	uplink := client.PresentationUplink(t.Context())
	require.NoError(t, uplink.Send(&playerv1.PresentationUplinkRequest{
		Payload: &playerv1.PresentationUplinkRequest_Open{Open: &playerv1.PresentationUplinkOpen{
			RecognitionHandle: snapshot.GetRecognitionHandle(), ClientInstanceId: clientID, UplinkGeneration: 1,
		}},
	}))
	require.True(t, subscription.Receive(), "ready before request EOF: %v", subscription.Err())
	require.Equal(t, uint64(1), subscription.Msg().GetPresentationUplinkResult().GetUplinkGeneration())
	_, err = uplink.CloseAndReceive()
	require.NoError(t, err)
}

func TestServerRecordsOnlyUnexpectedServeExit(t *testing.T) {
	t.Parallel()

	logs := testutil.NewRecordingLogger()
	server := &Server{log: logs}

	server.recordServeExit(http.ErrServerClosed)
	require.Empty(t, logs.Records(), "normal HTTP shutdown must remain silent")

	serveErr := errors.New("listener accept failed")
	server.recordServeExit(serveErr)
	records := logs.Records()
	require.Len(t, records, 1)
	require.Equal(t, "error", records[0].Level)
	require.Equal(t, "player server stopped unexpectedly", records[0].Message)
	require.Equal(t, "player.serve", records[0].Fields["operation"])
	require.Equal(t, "serve_failed", records[0].Fields["error_category"])
	require.NotContains(t, records[0].Fields, "error")
	require.NotContains(t, fmt.Sprintf("%#v", records[0]), serveErr.Error())
}

func TestServerRecordsOnlySafePlayerBoundaryCorrelation(t *testing.T) {
	t.Parallel()

	logs := testutil.NewRecordingLogger()
	server := &Server{log: logs}
	server.recordBoundaryAudit(BoundaryAuditEvent{
		Name: "player.boundary.request_outcome", Outcome: "not-controller",
		SessionID: "logical-1", Role: "observer", RequestID: "request-1", Mode: "navigate",
	})

	records := logs.Records()
	require.Len(t, records, 1)
	require.Equal(t, "warn", records[0].Level)
	require.Equal(t, "player boundary audit event", records[0].Message)
	require.Equal(t, map[string]any{
		"event": "player.boundary.request_outcome", "outcome": "not-controller",
		"session_id": "logical-1", "role": "observer", "request_id": "request-1", "mode": "navigate",
	}, records[0].Fields)
	require.NotContains(t, fmt.Sprintf("%#v", records[0]), "recognition")
}
