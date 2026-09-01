package player

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/google/go-cmp/cmp"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	playerv1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1/playerv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

type countingConnectCoordinator struct {
	ConnectCoordinator
	mutations atomic.Int64
}

func TestPresentationUplinkBypassesWholeBodyBufferingOnlyForExactProcedure(t *testing.T) {
	started := make(chan struct{}, 1)
	rpc := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		started <- struct{}{}
		response.WriteHeader(http.StatusOK)
	})
	handler := NewApplicationHandler(nil, "/fallout.terminal.player.v1.PlayerService/", rpc)

	reader, writer := io.Pipe()
	request := httptest.NewRequest(http.MethodPost, playerv1connect.PlayerServicePresentationUplinkProcedure, reader)
	request.Host = "player.test"
	request.Header.Set("Origin", "http://player.test")
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		assert.FailNow(t, "stream handler waited for request EOF")
	}
	require.NoError(t, writer.Close())
	<-done

	blockedReader, blockedWriter := io.Pipe()
	ordinary := httptest.NewRequest(http.MethodPost, playerv1connect.PlayerServiceSetPresentationProcedure, blockedReader)
	ordinary.Host = "player.test"
	ordinary.Header.Set("Origin", "http://player.test")
	ordinaryResponse := httptest.NewRecorder()
	ordinaryDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(ordinaryResponse, ordinary)
		close(ordinaryDone)
	}()
	require.Never(t, func() bool { return len(started) != 0 }, 20*time.Millisecond, time.Millisecond)
	require.NoError(t, blockedWriter.Close())
	<-ordinaryDone
}

func (coordinator *countingConnectCoordinator) DispatchPlayerActionForRecognition(handle domain.RecognitionHandle, command domain.RuntimeCommand) domain.ActionResult {
	coordinator.mutations.Add(1)
	return coordinator.ConnectCoordinator.DispatchPlayerActionForRecognition(handle, command)
}

func TestConnectHTTPRejectsDecodedCompressedUnknownAndMalformedBodiesBeforeCanonicalMutation(t *testing.T) {
	t.Parallel()

	base := newConnectTestCoordinator(t)
	coordinator := &countingConnectCoordinator{ConnectCoordinator: base}
	service, err := NewConnectService(ConnectServiceConfig{Coordinator: coordinator})
	if err != nil {
		require.NoError(t, err)
	}
	rpcPath, rpcHandler := NewConnectHandler(service)
	handler := NewApplicationHandler(playerAssets(), rpcPath, rpcHandler)
	request := &playerv1.NavigateRequest{
		RecognitionHandle: "recognition-1", RequestId: "request-1", BroadcastId: "broadcast-1", TerminalId: "terminal-1",
		Action: &playerv1.NavigateRequest_Back{Back: &playerv1.NavigateBack{}},
	}
	unknown := protowire.AppendTag(nil, 100, protowire.BytesType)
	unknown = protowire.AppendBytes(unknown, bytes.Repeat([]byte{'x'}, MaxUncompressedMessageBytes))
	request.ProtoReflect().SetUnknown(unknown)
	oversized, err := proto.Marshal(request)
	if err != nil {
		require.NoError(t, err)
	}
	require.Falsef(t, len(oversized) <= MaxUncompressedMessageBytes || len(oversized) >= MaxEncodedBodyBytes,
		"unknown-field fixture size = %d, want between decoded and encoded limits", len(oversized))

	var compressed bytes.Buffer
	zipper, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		require.NoError(t, err)
	}
	if _, err := zipper.Write(oversized); err != nil {
		require.NoError(t, err)
	}
	if err := zipper.Close(); err != nil {
		require.NoError(t, err)
	}
	require.Falsef(t, compressed.Len() >= MaxUncompressedMessageBytes,
		"compressed fixture size = %d, want below decoded limit", compressed.Len())

	tests := []struct {
		name            string
		body            []byte
		contentEncoding string
		wantStatus      int
		wantCode        string
	}{
		{name: "unknown field growth", body: oversized, wantStatus: http.StatusTooManyRequests, wantCode: "resource_exhausted"},
		{name: "compressed expansion", body: compressed.Bytes(), contentEncoding: "gzip", wantStatus: http.StatusTooManyRequests, wantCode: "resource_exhausted"},
		{name: "malformed bounded protobuf", body: []byte{0x0a}, wantStatus: http.StatusBadRequest, wantCode: "invalid_argument"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			httpRequest := httptest.NewRequest(http.MethodPost, "http://player.test/fallout.terminal.player.v1.PlayerService/Navigate", bytes.NewReader(test.body))
			httpRequest.Header.Set("Content-Type", "application/proto")
			if test.contentEncoding != "" {
				httpRequest.Header.Set("Content-Encoding", test.contentEncoding)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httpRequest)
			require.Falsef(t, recorder.Code != test.wantStatus || !strings.Contains(recorder.Body.String(), test.wantCode),
				"status/body = %d %q, want %d containing %q", recorder.Code, recorder.Body.String(), test.wantStatus, test.wantCode)

		})
	}
	{
		got := coordinator.mutations.Load()
		require.Falsef(t, got != 0,
			"canonical mutation calls = %d, want zero", got)
	}
	require.Falsef(t, base.Revision() != 2,
		"canonical revision changed after boundary rejection: %d", base.Revision())

}

func TestApplicationHandlerRejectsCrossOriginMalformedHostAndOversizedBodiesBeforeRPC(t *testing.T) {
	t.Parallel()

	var calls int
	rpc := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		calls++
		response.WriteHeader(http.StatusNoContent)
	})
	handler := NewApplicationHandler(playerAssets(), "/fallout.terminal.player.v1.PlayerService/", rpc)

	tests := []struct {
		name    string
		host    string
		origin  string
		body    []byte
		path    string
		status  int
		calls   int
		code    string
		chunked bool
	}{
		{name: "same origin", host: "player.test", origin: "https://player.test", path: "/fallout.terminal.player.v1.PlayerService/Navigate", status: http.StatusNoContent, calls: 1},
		{name: "foreign origin", host: "player.test", origin: "https://evil.example", path: "/fallout.terminal.player.v1.PlayerService/Navigate", status: http.StatusForbidden, calls: 1},
		{name: "malformed host", host: "player.test bad", path: "/fallout.terminal.player.v1.PlayerService/Navigate", status: http.StatusForbidden, calls: 1},
		{name: "encoded body over eight KiB", host: "player.test", body: bytes.Repeat([]byte{'x'}, MaxEncodedBodyBytes+1), path: "/fallout.terminal.player.v1.PlayerService/Navigate", status: http.StatusTooManyRequests, calls: 1, code: "resource_exhausted"},
		{name: "chunked encoded body over eight KiB", host: "player.test", body: bytes.Repeat([]byte{'x'}, MaxEncodedBodyBytes+1), path: "/fallout.terminal.player.v1.PlayerService/Navigate", status: http.StatusTooManyRequests, calls: 1, code: "resource_exhausted", chunked: true},
		{name: "unsupported player procedure", host: "player.test", path: "/fallout.terminal.player.v1.PlayerService/PrivateDiagnostics", status: http.StatusNotImplemented, calls: 1, code: "unimplemented"},
		{name: "unsupported public service", host: "player.test", path: "/fallout.terminal.player.v1.AdminService/Inspect", status: http.StatusNotImplemented, calls: 1, code: "unimplemented"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://player.test"+test.path, bytes.NewReader(test.body))
			request.Host = test.host
			request.Header.Set("Content-Type", "application/proto")
			if test.chunked {
				request.ContentLength = -1
			}
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			require.Falsef(t, recorder.Code != test.status,
				"status = %d, want %d; body=%q", recorder.Code, test.status, recorder.Body.String())
			require.Falsef(t, calls != test.calls,
				"RPC calls = %d, want %d", calls, test.calls)

			if test.code != "" {
				require.Contains(t, recorder.Body.String(), test.code)
			}
			require.False(t, recorder.Header().Get("Access-Control-Allow-Origin") == "*",
				"public handler emitted wildcard CORS")

		})
	}
}

func TestTypedSoundManifestAllowsOnlyEightCategoriesAndSafeSortedAssets(t *testing.T) {
	t.Parallel()

	service, err := NewConnectService(ConnectServiceConfig{Coordinator: newConnectTestCoordinator(t), Assets: playerAssets()})
	if err != nil {
		require.NoError(t, err)
	}
	tests := []struct {
		category playerv1.SoundCategory
		want     []string
	}{
		{playerv1.SoundCategory_SOUND_CATEGORY_AMBIENT, []string{"sounds/ambient/HISS.OGG", "sounds/ambient/hum.wav", "sounds/ambient/theme.m4a"}},
		{playerv1.SoundCategory_SOUND_CATEGORY_HACK_GOOD, []string{"sounds/hack-good/good.mp3"}},
		{playerv1.SoundCategory_SOUND_CATEGORY_HACK_BAD, []string{"sounds/hack-bad/bad.wav"}},
		{playerv1.SoundCategory_SOUND_CATEGORY_MENU_FOCUS, []string{"sounds/menu-focus/focus.wav"}},
		{playerv1.SoundCategory_SOUND_CATEGORY_SINGLE, []string{"sounds/single/single.wav"}},
		{playerv1.SoundCategory_SOUND_CATEGORY_MULTIPLE, []string{"sounds/multiple/multiple.wav"}},
		{playerv1.SoundCategory_SOUND_CATEGORY_ENTER, []string{"sounds/enter/enter.wav"}},
		{playerv1.SoundCategory_SOUND_CATEGORY_CHARSCROLL, []string{"sounds/charscroll/scroll.wav"}},
	}
	for _, test := range tests {
		response, err := service.SoundManifest(t.Context(), connect.NewRequest(&playerv1.SoundManifestRequest{Category: test.category}))
		require.Falsef(t, err != nil,
			"SoundManifest(%s): %v", test.category, err)
		assert.Falsef(t, !cmp.Equal(response.Msg.Assets, test.want),
			"SoundManifest(%s) assets = %#v, want %#v", test.category, response.Msg.Assets, test.want)

	}
	for _, invalid := range []playerv1.SoundCategory{playerv1.SoundCategory_SOUND_CATEGORY_UNSPECIFIED, 999} {
		_, err := service.SoundManifest(t.Context(), connect.NewRequest(&playerv1.SoundManifestRequest{Category: invalid}))
		assert.Falsef(t, connect.CodeOf(err) != connect.CodeInvalidArgument,
			"SoundManifest(%d) code = %s, want invalid_argument", invalid, connect.CodeOf(err))

	}

	empty, err := NewConnectService(ConnectServiceConfig{Coordinator: newConnectTestCoordinator(t), Assets: fstest.MapFS{}})
	if err != nil {
		require.NoError(t, err)
	}
	response, err := empty.SoundManifest(t.Context(), connect.NewRequest(&playerv1.SoundManifestRequest{Category: playerv1.SoundCategory_SOUND_CATEGORY_AMBIENT}))
	require.Falsef(t, err != nil || len(response.Msg.Assets) != 0,
		"missing sound category = %#v, %v; want empty success", response, err)

}

func TestHTTPHandlerServesStaticAssetsAndIndexFallback(t *testing.T) {
	t.Parallel()

	assets := playerAssets()
	handler := NewHTTPHandler(assets)

	tests := []struct {
		name        string
		path        string
		status      int
		contentType string
		body        string
	}{
		{
			name:        "root index",
			path:        "/",
			status:      http.StatusOK,
			contentType: "text/html",
			body:        "player-shell",
		},
		{
			name:        "Vue JavaScript bundle",
			path:        "/assets/index-player.js",
			status:      http.StatusOK,
			contentType: "javascript",
			body:        "player-vue-runtime",
		},
		{
			name:        "emitted font asset",
			path:        "/assets/Fixedsys-player.ttf",
			status:      http.StatusOK,
			contentType: "font/ttf",
			body:        "fake-font",
		},
		{
			name:        "extensionless browser route falls back to index",
			path:        "/terminal/root/status",
			status:      http.StatusOK,
			contentType: "text/html",
			body:        "player-shell",
		},
		{
			name:   "missing asset does not fall back",
			path:   "/missing.js",
			status: http.StatusNotFound,
		},
		{
			name:   "directories are not listed",
			path:   "/sounds/",
			status: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := serveRequest(t, handler, test.path)
			require.Falsef(t, recorder.Code != test.status,
				"GET %s status = %d, want %d; body = %q", test.path, recorder.Code, test.status, recorder.Body.String())
			assert.Falsef(t, test.contentType != "" && !strings.Contains(recorder.Header().Get("Content-Type"), test.contentType),
				"GET %s Content-Type = %q, want it to contain %q", test.path, recorder.Header().Get("Content-Type"), test.contentType)
			assert.Falsef(t, test.body != "" && !strings.Contains(recorder.Body.String(), test.body),
				"GET %s body = %q, want it to contain %q", test.path, recorder.Body.String(), test.body)

		})
	}
}

func TestHTTPHandlerDoesNotExposePrivilegedOverseerAssets(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler(playerAssets())
	for _, requestPath := range []string{
		"/client.js",
		"/sound.js",
		"/presentation-uplink.js",
		"/main.ts",
		"/overseer.css",
		"/Fixedsys.ttf",
		"/assets/overseer.js",
	} {
		t.Run(requestPath, func(t *testing.T) {
			recorder := serveRequest(t, handler, requestPath)
			require.Equal(t, http.StatusNotFound, recorder.Code)
			require.NotContains(t, recorder.Body.String(), "overseer-shell")
		})
	}
}

func TestHTTPHandlerRejectsTraversalWithoutNormalizingItIntoAnAsset(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler(playerAssets())
	for _, requestPath := range []string{
		"/../outside.txt",
		"/%2e%2e/outside.txt",
		"/sounds/ambient/../../../outside.txt",
		"/sounds/%2e%2e",
	} {
		t.Run(requestPath, func(t *testing.T) {
			recorder := serveRequest(t, handler, requestPath)
			require.Falsef(t, recorder.Code != http.StatusNotFound,
				"GET %s status = %d, want 404; body = %q", requestPath, recorder.Code, recorder.Body.String())
			require.Falsef(t, strings.Contains(recorder.Body.String(), "outside-client-root"),
				"GET %s exposed a normalized asset: %q", requestPath, recorder.Body.String())

		})
	}
}

func TestHTTPHandlerSetsPlayerSecurityHeaders(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler(playerAssets())
	for _, requestPath := range []string{"/", "/terminal/root", "/sounds/ambient/hum.wav"} {
		t.Run(requestPath, func(t *testing.T) {
			recorder := serveRequest(t, handler, requestPath)
			require.Falsef(t, recorder.Code != http.StatusOK,
				"GET %s status = %d, want 200", requestPath, recorder.Code)
			{

				got := recorder.Header().Get("X-Content-Type-Options")
				assert.Falsef(t, got != "nosniff",
					"X-Content-Type-Options = %q, want nosniff", got)
			}

			policy := recorder.Header().Get("Content-Security-Policy")
			for _, directive := range []string{"default-src 'self'", "connect-src 'self'", "media-src 'self'", "object-src 'none'", "frame-ancestors 'none'"} {
				assert.Falsef(t, !strings.Contains(policy, directive),
					"Content-Security-Policy = %q, want directive %q", policy, directive)

			}
		})
	}
}

func TestBrowserRecognitionNeverUsesHTTPURLsOrWeakensOriginAndHeaders(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler(playerAssets())
	const secretToken = "opaque-browser-token-that-must-not-be-reflected"

	for _, requestPath := range []string{
		"/api/session",
		"/api/token",
		"/api/browser-token",
		"/api/identity",
	} {
		recorder := serveRequest(t, handler, requestPath)
		assert.Falsef(t, recorder.Code != http.StatusNotFound,
			"GET %s status = %d, want no recognition endpoint", requestPath, recorder.Code)

	}

	for _, requestPath := range []string{
		"/?browserToken=" + secretToken,
		"/assets/index-player.js?token=" + secretToken,
		"/terminal/root?session=" + secretToken,
	} {
		recorder := serveRequest(t, handler, requestPath)
		serialized := recorder.Body.String() + recorder.Header().Get("Location") + recorder.Header().Get("Set-Cookie")
		assert.Falsef(t, strings.Contains(serialized, secretToken),
			"GET %s reflected recognition material in an HTTP response", requestPath)
		assert.Falsef(t, recorder.Header().Get("X-Content-Type-Options") != "nosniff",
			"GET %s lost nosniff", requestPath)
		{

			policy := recorder.Header().Get("Content-Security-Policy")
			assert.Falsef(t, policy != playerContentSecurityPolicy,
				"GET %s CSP changed: %q", requestPath, policy)
		}

	}

	for _, test := range []struct {
		origin string
		want   bool
	}{
		{origin: "", want: true},
		{origin: "https://player.test", want: true},
		{origin: "http://player.test", want: true},
		{origin: "https://evil.example", want: false},
	} {
		request := httptest.NewRequest(http.MethodGet, "http://player.test/", nil)
		request.Host = "player.test"
		if test.origin != "" {
			request.Header.Set("Origin", test.origin)
		}
		{
			got := sameHostOrigin(request)
			assert.Falsef(t, got != test.want,
				"sameHostOrigin(%q) = %t, want %t", test.origin, got, test.want)
		}

	}

	bundle := serveRequest(t, handler, "/assets/index-player.js")
	require.Equal(t, http.StatusOK, bundle.Code)
	for _, forbidden := range []string{secretToken, "?token", "?session", "overseer-shell"} {
		assert.NotContains(t, bundle.Body.String(), forbidden)
	}
}

func TestHTTPHandlerReturnsNotFoundWhenRequiredAssetsAreMissing(t *testing.T) {
	t.Parallel()

	handler := NewHTTPHandler(fstest.MapFS{})
	for _, requestPath := range []string{"/", "/terminal/root", "/assets/index-player.js"} {
		t.Run(requestPath, func(t *testing.T) {
			recorder := serveRequest(t, handler, requestPath)
			require.Falsef(t, recorder.Code != http.StatusNotFound,
				"GET %s status = %d, want 404; body = %q", requestPath, recorder.Code, recorder.Body.String())

		})
	}
}

func playerAssets() fs.FS {
	return fstest.MapFS{
		"index.html":                   {Data: []byte(`<!doctype html><div id="playerApp">player-shell</div><script type="module" src="/assets/index-player.js"></script>`)},
		"assets/index-player.js":       {Data: []byte("const runtime = 'player-vue-runtime';")},
		"assets/Fixedsys-player.ttf":   {Data: []byte("fake-font")},
		"outside.txt":                  {Data: []byte("outside-client-root")},
		"sounds/ambient/HISS.OGG":      {Data: []byte("ogg")},
		"sounds/ambient/hum.wav":       {Data: []byte("wav")},
		"sounds/ambient/theme.m4a":     {Data: []byte("m4a")},
		"sounds/ambient/README.txt":    {Data: []byte("not audio")},
		"sounds/charscroll/scroll.wav": {Data: []byte("wav")},
		"sounds/enter/enter.wav":       {Data: []byte("wav")},
		"sounds/hack-bad/bad.wav":      {Data: []byte("wav")},
		"sounds/hack-good/good.mp3":    {Data: []byte("mp3")},
		"sounds/menu-focus/focus.wav":  {Data: []byte("wav")},
		"sounds/menu-focus/empty.txt":  {Data: []byte("not audio")},
		"sounds/multiple/multiple.wav": {Data: []byte("wav")},
		"sounds/single/single.wav":     {Data: []byte("wav")},
	}
}

func serveRequest(t *testing.T, handler http.Handler, requestPath string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "http://player.test"+requestPath, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}
