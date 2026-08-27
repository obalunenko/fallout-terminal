package tunnel_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/obalunenko/Fallout-Terminal/internal/tunnel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startProtocolAwareUpstream(t *testing.T, handler http.Handler) string {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	server := &http.Server{Handler: handler, Protocols: protocols}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
		defer cancel()
		require.NoError(t, server.Shutdown(ctx))
	})
	return "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(listener.Addr().(*net.TCPAddr).Port))
}

const (
	publicIngressHost     = "public.example"
	publicIngressUsername = "players"
	publicIngressPassword = "synthetic-player-input"
)

func publicIngressRequest(t *testing.T, ingressURL, method, path, host, username, password string) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), method, ingressURL+path, nil)
	require.NoError(t, err)
	request.Host = host
	if username != "" {
		request.SetBasicAuth(username, password)
	}
	return request
}

func TestPublicIngressStartsDeniedAndProtectsStaticUnaryAndStreaming(t *testing.T) {
	var upstreamCalls atomic.Int64
	var authorizationForwarded atomic.Bool
	var upstreamProtocol atomic.Int64
	releaseUpdate := make(chan struct{})
	upstreamURL := startProtocolAwareUpstream(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		upstreamCalls.Add(1)
		upstreamProtocol.Store(int64(request.ProtoMajor))
		if request.Header.Get("Authorization") != "" {
			authorizationForwarded.Store(true)
		}
		switch request.URL.Path {
		case "/stream":
			response.Header().Set("Content-Type", "application/connect+proto")
			_, _ = io.WriteString(response, "snapshot\n")
			response.(http.Flusher).Flush()
			select {
			case <-releaseUpdate:
				_, _ = io.WriteString(response, "update\n")
			case <-request.Context().Done():
			}
		case "/unary":
			response.WriteHeader(http.StatusNoContent)
		default:
			response.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(response, "player")
		}
	}))

	ingress, err := tunnel.NewPublicIngressFactory().Start(t.Context(), upstreamURL)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ingress.Close(context.WithoutCancel(t.Context()))) })
	client := &http.Client{Timeout: time.Second}

	denied, err := client.Do(publicIngressRequest(t, ingress.URL().String(), http.MethodGet, "/", publicIngressHost, publicIngressUsername, publicIngressPassword))
	require.NoError(t, err)
	_ = denied.Body.Close()
	assert.NotEqual(t, http.StatusOK, denied.StatusCode)
	assert.Zero(t, upstreamCalls.Load())

	require.NoError(t, ingress.Activate(publicIngressHost, publicIngressUsername, []byte(publicIngressPassword)))
	for _, credentials := range []struct{ username, password string }{
		{},
		{username: publicIngressUsername, password: "synthetic-wrong-input"},
	} {
		response, requestErr := client.Do(publicIngressRequest(
			t, ingress.URL().String(), http.MethodGet, "/", publicIngressHost, credentials.username, credentials.password,
		))
		require.NoError(t, requestErr)
		_ = response.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Contains(t, response.Header.Get("WWW-Authenticate"), "Fallout Terminal Players")
	}

	unknown, err := client.Do(publicIngressRequest(t, ingress.URL().String(), http.MethodGet, "/", "unknown.example", publicIngressUsername, publicIngressPassword))
	require.NoError(t, err)
	_ = unknown.Body.Close()
	assert.NotEqual(t, http.StatusOK, unknown.StatusCode)
	assert.Empty(t, unknown.Header.Get("WWW-Authenticate"))

	staticResponse, err := client.Do(publicIngressRequest(t, ingress.URL().String(), http.MethodGet, "/", publicIngressHost, publicIngressUsername, publicIngressPassword))
	require.NoError(t, err)
	staticBody, err := io.ReadAll(staticResponse.Body)
	require.NoError(t, err)
	_ = staticResponse.Body.Close()
	assert.Equal(t, http.StatusOK, staticResponse.StatusCode)
	assert.Equal(t, "player", string(staticBody))
	assert.Equal(t, int64(2), upstreamProtocol.Load(), "ingress-to-player hop must use h2c")

	unaryResponse, err := client.Do(publicIngressRequest(t, ingress.URL().String(), http.MethodPost, "/unary", publicIngressHost, publicIngressUsername, publicIngressPassword))
	require.NoError(t, err)
	_ = unaryResponse.Body.Close()
	assert.Equal(t, http.StatusNoContent, unaryResponse.StatusCode)

	streamResponse, err := client.Do(publicIngressRequest(t, ingress.URL().String(), http.MethodPost, "/stream", publicIngressHost, publicIngressUsername, publicIngressPassword))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, streamResponse.StatusCode)
	reader := bufio.NewReader(streamResponse.Body)
	firstFrame := make(chan string, 1)
	go func() {
		line, _ := reader.ReadString('\n')
		firstFrame <- line
	}()
	select {
	case line := <-firstFrame:
		assert.Equal(t, "snapshot\n", line)
	case <-time.After(250 * time.Millisecond):
		t.Fatal("ingress buffered the first streaming frame")
	}
	close(releaseUpdate)
	line, err := reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "update\n", line)
	_ = streamResponse.Body.Close()
	assert.False(t, authorizationForwarded.Load())

	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	http2Client := &http.Client{Transport: &http.Transport{Protocols: protocols}, Timeout: time.Second}
	http2Response, err := http2Client.Do(publicIngressRequest(
		t, ingress.URL().String(), http.MethodGet, "/", publicIngressHost, publicIngressUsername, publicIngressPassword,
	))
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, http2Response.Body)
	require.NoError(t, err)
	require.NoError(t, http2Response.Body.Close())
	assert.Equal(t, 2, http2Response.ProtoMajor, "public-to-ingress hop must independently negotiate h2c")
	assert.Equal(t, int64(2), upstreamProtocol.Load(), "ingress-to-player hop must remain h2c")

	ingress.Deny()
	stale, err := client.Do(publicIngressRequest(t, ingress.URL().String(), http.MethodGet, "/", publicIngressHost, publicIngressUsername, publicIngressPassword))
	require.NoError(t, err)
	_ = stale.Body.Close()
	assert.NotEqual(t, http.StatusOK, stale.StatusCode)

	local, err := http.Get(upstreamURL + "/")
	require.NoError(t, err)
	_ = local.Body.Close()
	assert.Equal(t, http.StatusOK, local.StatusCode)
	assert.Empty(t, local.Header.Get("WWW-Authenticate"))
}

func TestPublicIngressRejectsUnsafeActivationAndClosesIdempotently(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)
	ingress, err := tunnel.NewPublicIngressFactory().Start(t.Context(), upstream.URL)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ingress.Close(context.WithoutCancel(t.Context()))) })

	for _, activation := range []struct {
		host, username, password string
	}{
		{host: "bad host", username: publicIngressUsername, password: publicIngressPassword},
		{host: publicIngressHost, username: "bad:name", password: publicIngressPassword},
		{host: publicIngressHost, username: publicIngressUsername, password: "short"},
	} {
		assert.Error(t, ingress.Activate(activation.host, activation.username, []byte(activation.password)))
	}
	require.NoError(t, ingress.Activate(strings.ToUpper(publicIngressHost)+".", publicIngressUsername, []byte(publicIngressPassword)))
	assert.NoError(t, ingress.Close(t.Context()))
	assert.NoError(t, ingress.Close(t.Context()))
}

func TestPublicIngressFactoryRejectsNonLoopbackOrMalformedUpstream(t *testing.T) {
	for _, upstream := range []string{
		"https://127.0.0.1:3690",
		"http://example.com:3690",
		"http://127.0.0.1:3690/path",
		"http://user@127.0.0.1:3690",
	} {
		ingress, err := tunnel.NewPublicIngressFactory().Start(t.Context(), upstream)
		assert.Error(t, err, upstream)
		assert.Nil(t, ingress, upstream)
	}
}
