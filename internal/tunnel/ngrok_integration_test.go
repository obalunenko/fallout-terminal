package tunnel_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	playerv1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1/playerv1connect"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/tunnel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type publicStreamProbeService struct {
	playerv1connect.UnimplementedPlayerServiceHandler
	upstreamArrival chan time.Time
	targeted        chan *playerv1.SubscriptionMessage
	playerProtocols chan string
}

func (service *publicStreamProbeService) Subscribe(
	ctx context.Context,
	_ *playerv1.SubscribeRequest,
	stream playerv1connect.PlayerServiceSubscribeServerStream,
) error {
	select {
	case service.upstreamArrival <- time.Now():
	default:
	}
	if err := stream.Send(&playerv1.SubscriptionMessage{Payload: &playerv1.SubscriptionMessage_Snapshot{
		Snapshot: &playerv1.PersonalizedSnapshot{RecognitionHandle: "integration-handle", Revision: 1},
	}}); err != nil {
		return err
	}
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	updatePending := true
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case targeted := <-service.targeted:
			if err := stream.Send(targeted); err != nil {
				return err
			}
		case <-timer.C:
			if updatePending {
				updatePending = false
				if err := stream.Send(&playerv1.SubscriptionMessage{Payload: &playerv1.SubscriptionMessage_Update{
					Update: &playerv1.CompoundUpdate{Revision: 2},
				}}); err != nil {
					return err
				}
			}
		}
	}
}

func (service *publicStreamProbeService) PresentationUplink(
	ctx context.Context,
	stream playerv1connect.PlayerServicePresentationUplinkServerStream,
) (*playerv1.PresentationUplinkResponse, error) {
	first, err := stream.Receive()
	if err != nil {
		return nil, err
	}
	open := first.GetOpen()
	if open == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, "open frame required")
	}
	service.targeted <- &playerv1.SubscriptionMessage{Payload: &playerv1.SubscriptionMessage_PresentationUplinkResult{
		PresentationUplinkResult: &playerv1.PresentationUplinkResult{
			ClientInstanceId: open.GetClientInstanceId(), UplinkGeneration: open.GetUplinkGeneration(),
			Payload: &playerv1.PresentationUplinkResult_Ready{Ready: &playerv1.PresentationUplinkReady{}},
		},
	}}
	for {
		msg, err := stream.Receive()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		intent := msg.GetIntent()
		if intent == nil {
			continue
		}
		select {
		case service.targeted <- &playerv1.SubscriptionMessage{Payload: &playerv1.SubscriptionMessage_PresentationUplinkResult{
			PresentationUplinkResult: &playerv1.PresentationUplinkResult{
				ClientInstanceId: open.GetClientInstanceId(), UplinkGeneration: open.GetUplinkGeneration(),
				Payload: &playerv1.PresentationUplinkResult_Action{Action: &playerv1.ActionResult{
					RequestId: intent.GetRequestId(), Accepted: true, Revision: 3,
				}},
			},
		}}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return &playerv1.PresentationUplinkResponse{}, nil
}

func (service *publicStreamProbeService) SoundManifest(
	context.Context, *playerv1.SoundManifestRequest,
) (*playerv1.SoundManifestResponse, error) {
	return &playerv1.SoundManifestResponse{}, nil
}

type subscribeHeaderEvidence struct {
	status      int
	contentType string
	at          time.Time
}

type publicAuthTransport struct {
	base     http.RoundTripper
	username string
	password []byte
	headers  chan subscribeHeaderEvidence
}

func (transport *publicAuthTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	forwarded := request.Clone(request.Context())
	forwarded.SetBasicAuth(transport.username, string(transport.password))
	response, err := transport.base.RoundTrip(forwarded)
	if response != nil && strings.HasSuffix(request.URL.Path, "/Subscribe") {
		select {
		case transport.headers <- subscribeHeaderEvidence{
			status: response.StatusCode, contentType: response.Header.Get("Content-Type"), at: time.Now(),
		}:
		default:
		}
	}
	return response, err
}

func TestEmbeddedNgrokSDKOptInAuthenticatedGeneratedSubscribe(t *testing.T) {
	if os.Getenv("FALLOUT_NGROK_INTEGRATION") != "1" {
		t.Skip("NOT RUN: explicit real-ngrok integration opt-in was not provided")
	}
	token := []byte(os.Getenv("FALLOUT_NGROK_AUTHTOKEN"))
	password := []byte(os.Getenv("FALLOUT_PUBLIC_TEST_PASSWORD"))
	t.Cleanup(func() { clear(token) })
	t.Cleanup(func() { clear(password) })
	if len(token) == 0 || len(password) < tunnel.MinimumPlayerPasswordBytes {
		t.Skip("NOT RUN: external real-ngrok test credentials are unavailable")
	}
	username := strings.TrimSpace(os.Getenv("FALLOUT_PUBLIC_TEST_USERNAME"))
	if username == "" {
		username = tunnel.DefaultPlayerUsername
	}
	reservedDomain := strings.TrimSpace(os.Getenv("FALLOUT_NGROK_RESERVED_DOMAIN"))

	listener, err := net.Listen("tcp4", tunnel.PlayerUpstreamAddress)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	probe := &publicStreamProbeService{
		upstreamArrival: make(chan time.Time, 1),
		targeted:        make(chan *playerv1.SubscriptionMessage, 4),
		playerProtocols: make(chan string, 4),
	}
	rpcServer := connect.NewServer()
	playerv1connect.RegisterPlayerServiceHandler(rpcServer, probe)
	rpcHandler := http.NewServeMux()
	connecthttp.Mount(rpcHandler, rpcServer)
	rpcPath := "/" + playerv1connect.PlayerServiceName + "/"
	mux := http.NewServeMux()
	mux.Handle(rpcPath, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		select {
		case probe.playerProtocols <- request.Proto:
		default:
		}
		rpcHandler.ServeHTTP(response, request)
	}))
	mux.HandleFunc("/", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	server := &http.Server{Handler: mux, Protocols: protocols}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer cancel()
		require.NoError(t, server.Shutdown(ctx))
	})

	ingress, err := tunnel.NewPublicIngressFactory().Start(t.Context(), "http://"+tunnel.PlayerUpstreamAddress)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ingress.Close(context.WithoutCancel(t.Context()))) })

	service := tunnel.NewEmbeddedNgrokService()
	endpoint, err := service.Start(t.Context(), tunnel.TunnelStartRequest{
		UpstreamURL: ingress.URL().String(), ReservedDomain: reservedDomain,
		AccountToken: token, Timeout: 30 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		ingress.Deny()
		require.NoError(t, endpoint.Close(context.WithoutCancel(t.Context())))
	})

	publicURL := endpoint.URL()
	require.NotNil(t, publicURL)
	require.Equal(t, "https", publicURL.Scheme)
	if reservedDomain != "" {
		assert.Equal(t, strings.TrimSuffix(strings.ToLower(reservedDomain), "."), strings.ToLower(publicURL.Hostname()))
	}
	require.NoError(t, ingress.Activate(publicURL.Hostname(), username, password))

	client := &http.Client{Timeout: 15 * time.Second}
	for _, test := range []struct {
		name, username string
		password       []byte
		want           int
	}{
		{name: "missing", want: http.StatusUnauthorized},
		{name: "wrong", username: username, password: []byte("synthetic-wrong-input"), want: http.StatusUnauthorized},
		{name: "correct", username: username, password: password, want: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, requestErr := http.NewRequestWithContext(t.Context(), http.MethodGet, publicURL.String(), nil)
			require.NoError(t, requestErr)
			if test.username != "" {
				request.SetBasicAuth(test.username, string(test.password))
			}
			response, requestErr := client.Do(request)
			require.NoError(t, requestErr)
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			assert.Equal(t, test.want, response.StatusCode)
		})
	}

	headers := make(chan subscribeHeaderEvidence, 1)
	authenticatedClient := &http.Client{Transport: &publicAuthTransport{
		base: http.DefaultTransport, username: username, password: password, headers: headers,
	}}
	generated := playerv1connect.NewPlayerServiceClient(connect.NewClient(connecthttp.NewTransport(authenticatedClient, publicURL.String(), connecthttp.WithReadMaxBytes(0))))
	streamDeadline, stopStreamDeadline := context.WithTimeoutCause(t.Context(), 5*time.Second, errors.New("test ngrok stream timed out"))
	streamContext, cancelStream := context.WithCancelCause(streamDeadline)
	t.Cleanup(func() {
		cancelStream(errors.New("test ngrok stream completed"))
		stopStreamDeadline()
	})
	startedAt := time.Now()
	clientID := "real-ngrok-integration-tab"
	stream, err := generated.Subscribe(streamContext, &playerv1.SubscribeRequest{ClientInstanceId: &clientID})
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })
	first, err := stream.Receive()
	require.NoError(t, err, "initial public Subscribe frame")
	firstAt := time.Now()
	snapshot := first.GetSnapshot()
	require.NotNil(t, snapshot)
	var updateAt time.Time
	receiveUntil := func(accept func(*playerv1.SubscriptionMessage) bool, description string) {
		t.Helper()
		for {
			message, err := stream.Receive()
			require.NoError(t, err, description)
			if message.GetUpdate() != nil && updateAt.IsZero() {
				updateAt = time.Now()
			}
			if accept(message) {
				return
			}
		}
	}
	uplink, err := generated.PresentationUplink(streamContext)
	require.NoError(t, err)
	require.NoError(t, uplink.Send(&playerv1.PresentationUplinkRequest{Payload: &playerv1.PresentationUplinkRequest_Open{
		Open: &playerv1.PresentationUplinkOpen{
			ClientInstanceId: clientID, UplinkGeneration: 1, RecognitionHandle: snapshot.GetRecognitionHandle(),
		},
	}}))
	receiveUntil(func(message *playerv1.SubscriptionMessage) bool {
		return message.GetPresentationUplinkResult().GetReady() != nil
	}, "public uplink ready")
	require.NoError(t, uplink.Send(&playerv1.PresentationUplinkRequest{Payload: &playerv1.PresentationUplinkRequest_Intent{
		Intent: &playerv1.PresentationIntent{RequestId: "real-ngrok-presentation"},
	}}))
	receiveUntil(func(message *playerv1.SubscriptionMessage) bool {
		return message.GetPresentationUplinkResult().GetAction().GetRequestId() == "real-ngrok-presentation"
	}, "public uplink action")
	_, err = uplink.CloseAndReceive()
	require.NoError(t, err)
	if updateAt.IsZero() {
		receiveUntil(func(message *playerv1.SubscriptionMessage) bool {
			return message.GetUpdate().GetRevision() == 2
		}, "later public Subscribe frame")
	}
	evidenceContext, stopEvidence := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(stopEvidence)
	receiveProtocol := func() string {
		t.Helper()
		select {
		case protocol := <-probe.playerProtocols:
			return protocol
		case <-evidenceContext.Done():
			require.NoError(t, context.Cause(evidenceContext), "timed out waiting for player HTTP protocol evidence")
			return ""
		}
	}
	for range 2 {
		assert.Equal(t, "HTTP/2.0", receiveProtocol())
	}

	var headerEvidence subscribeHeaderEvidence
	select {
	case headerEvidence = <-headers:
	case <-evidenceContext.Done():
		require.NoError(t, context.Cause(evidenceContext), "timed out waiting for public response-header evidence")
	}
	var upstreamAt time.Time
	select {
	case upstreamAt = <-probe.upstreamArrival:
	case <-evidenceContext.Done():
		require.NoError(t, context.Cause(evidenceContext), "timed out waiting for player upstream-arrival evidence")
	}
	assert.Equal(t, http.StatusOK, headerEvidence.status)
	assert.Equal(t, "application/connect+proto", headerEvidence.contentType)
	t.Logf(
		"public stream evidence status=%d content_type=%q upstream_arrival_ms=%d response_headers_ms=%d first_snapshot_ms=%d later_update_ms=%d",
		headerEvidence.status,
		headerEvidence.contentType,
		upstreamAt.Sub(startedAt).Milliseconds(),
		headerEvidence.at.Sub(startedAt).Milliseconds(),
		firstAt.Sub(startedAt).Milliseconds(),
		updateAt.Sub(startedAt).Milliseconds(),
	)
}
