package player

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1/playerv1connect"
)

const playerContentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; media-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'"

var (
	allowedSoundExtensions = map[string]struct{}{
		".mp3":  {},
		".wav":  {},
		".ogg":  {},
		".m4a":  {},
		".webm": {},
	}
)

// NewHTTPHandler serves a player filesystem rooted at frontend/client/. The supplied
// filesystem is the handler's complete namespace; no host filesystem paths are
// opened or derived from requests.
func NewHTTPHandler(assets fs.FS) http.Handler {
	return &playerHTTPHandler{assets: assets}
}

// NewApplicationHandler mounts generated Connect procedures before the static
// player application. RPC paths never fall through to the SPA index, and all
// page, generated client, sound, and RPC traffic remains same-origin.
func NewApplicationHandler(assets fs.FS, rpcPath string, rpcHandler http.Handler) http.Handler {
	staticHandler := NewHTTPHandler(assets)
	errorWriter := connect.NewErrorWriter()
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if supportedRPCRequestPath(request.URL.Path, rpcPath) {
			response.Header().Set("X-Content-Type-Options", "nosniff")
			response.Header().Set("Content-Security-Policy", playerContentSecurityPolicy)
			if !validRequestHost(request.Host) {
				http.Error(response, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			if !sameHostOrigin(request) {
				http.Error(response, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			if request.URL.Path != playerv1connect.PlayerServicePresentationUplinkProcedure {
				if request.ContentLength > MaxEncodedBodyBytes {
					writeConnectBoundaryError(errorWriter, response, request, connect.CodeResourceExhausted, "public player request exceeds the encoded-body limit")
					return
				}
				if err := bufferBoundedRequestBody(request); err != nil {
					writeConnectBoundaryError(errorWriter, response, request, connect.CodeResourceExhausted, "public player request exceeds the encoded-body limit")
					return
				}
			} else {
				streamContext, cancelStream := context.WithCancelCause(request.Context())
				timedBody := newPresentationUplinkBody(
					request.Body,
					cancelStream,
					PresentationUplinkIdleLifetime,
					PresentationUplinkMaximumLifetime,
				)
				defer timedBody.Stop()
				request = request.Clone(streamContext)
				request.Body = timedBody
			}
			if rpcHandler == nil {
				writeConnectBoundaryError(errorWriter, response, request, connect.CodeUnimplemented, "public player procedure is not implemented")
				return
			}
			rpcHandler.ServeHTTP(response, request)
			return
		}
		if publicRPCRequestPath(request.URL.Path, rpcPath) {
			response.Header().Set("X-Content-Type-Options", "nosniff")
			response.Header().Set("Content-Security-Policy", playerContentSecurityPolicy)
			writeConnectBoundaryError(errorWriter, response, request, connect.CodeUnimplemented, "public player procedure is not implemented")
			return
		}
		staticHandler.ServeHTTP(response, request)
	})
}

var (
	errPresentationUplinkIdleTimeout = errors.New("presentation uplink idle timeout")
	errPresentationUplinkMaxLifetime = errors.New("presentation uplink maximum lifetime reached")
)

type presentationUplinkBody struct {
	body       io.ReadCloser
	cancel     context.CancelCauseFunc
	idle       time.Duration
	idleTimer  *time.Timer
	maxTimer   *time.Timer
	mu         sync.Mutex
	terminated bool
}

func newPresentationUplinkBody(
	body io.ReadCloser,
	cancel context.CancelCauseFunc,
	idle time.Duration,
	maximum time.Duration,
) *presentationUplinkBody {
	if body == nil {
		body = http.NoBody
	}
	stream := &presentationUplinkBody{body: body, cancel: cancel, idle: idle}
	stream.idleTimer = time.AfterFunc(idle, func() { stream.terminate(errPresentationUplinkIdleTimeout) })
	stream.maxTimer = time.AfterFunc(maximum, func() { stream.terminate(errPresentationUplinkMaxLifetime) })
	return stream
}

func (stream *presentationUplinkBody) Read(target []byte) (int, error) {
	n, err := stream.body.Read(target)
	if n > 0 {
		stream.mu.Lock()
		if !stream.terminated {
			stream.idleTimer.Reset(stream.idle)
		}
		stream.mu.Unlock()
	}
	return n, err
}

func (stream *presentationUplinkBody) Close() error {
	stream.Stop()
	return stream.body.Close()
}

func (stream *presentationUplinkBody) Stop() {
	if stream == nil {
		return
	}
	stream.mu.Lock()
	if !stream.terminated {
		stream.terminated = true
		stream.idleTimer.Stop()
		stream.maxTimer.Stop()
	}
	stream.mu.Unlock()
}

func (stream *presentationUplinkBody) terminate(cause error) {
	stream.mu.Lock()
	if stream.terminated {
		stream.mu.Unlock()
		return
	}
	stream.terminated = true
	stream.idleTimer.Stop()
	stream.maxTimer.Stop()
	stream.cancel(cause)
	stream.mu.Unlock()
	_ = stream.body.Close()
}

func supportedRPCRequestPath(requestPath, servicePath string) bool {
	if servicePath == "" {
		return false
	}
	_, supported := map[string]struct{}{
		playerv1connect.PlayerServiceSubscribeProcedure:          {},
		playerv1connect.PlayerServiceSelectCharacterProcedure:    {},
		playerv1connect.PlayerServiceNavigateProcedure:           {},
		playerv1connect.PlayerServiceGuessProcedure:              {},
		playerv1connect.PlayerServiceActivatePatternProcedure:    {},
		playerv1connect.PlayerServiceSetPresentationProcedure:    {},
		playerv1connect.PlayerServicePresentationUplinkProcedure: {},
		playerv1connect.PlayerServiceSoundManifestProcedure:      {},
	}[requestPath]
	return supported
}

func publicRPCRequestPath(requestPath, servicePath string) bool {
	servicePath = strings.TrimSuffix(servicePath, "/")
	if requestPath == servicePath || strings.HasPrefix(requestPath, servicePath+"/") {
		return true
	}
	return strings.HasPrefix(requestPath, "/fallout.terminal.player.v1.")
}

func bufferBoundedRequestBody(request *http.Request) error {
	if request == nil || request.Body == nil {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, MaxEncodedBodyBytes+1))
	closeErr := request.Body.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close encoded request body: %w", closeErr)
	}
	if len(body) > MaxEncodedBodyBytes {
		return errors.New("encoded request body exceeds limit")
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	return nil
}

func writeConnectBoundaryError(writer *connect.ErrorWriter, response http.ResponseWriter, request *http.Request, code connect.Code, message string) {
	if err := writer.Write(response, request, connect.NewError(code, errors.New(message))); err != nil {
		http.Error(response, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func validRequestHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" || strings.ContainsAny(host, "/\\\r\n\t @") {
		return false
	}
	if strings.HasPrefix(host, "[") {
		_, _, err := net.SplitHostPort(host)
		return err == nil || strings.HasSuffix(host, "]")
	}
	if strings.Count(host, ":") > 1 {
		return false
	}
	if strings.Contains(host, ":") {
		name, port, err := net.SplitHostPort(host)
		return err == nil && name != "" && port != ""
	}
	return true
}

// sameHostOrigin accepts non-browser clients without Origin and browser
// clients whose HTTP(S) origin host exactly matches the request Host.
func sameHostOrigin(request *http.Request) bool {
	if request == nil {
		return false
	}
	rawOrigin := strings.TrimSpace(request.Header.Get("Origin"))
	if rawOrigin == "" {
		return true
	}
	origin, err := url.Parse(rawOrigin)
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" {
		return false
	}
	return strings.EqualFold(origin.Host, request.Host)
}

type playerHTTPHandler struct {
	assets fs.FS
}

func (handler *playerHTTPHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Content-Security-Policy", playerContentSecurityPolicy)

	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if unsafePlayerPath(request.URL) {
		http.NotFound(response, request)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/api/") {
		http.NotFound(response, request)
		return
	}

	handler.serveAsset(response, request)
}

func (handler *playerHTTPHandler) serveAsset(response http.ResponseWriter, request *http.Request) {
	assetPath := strings.TrimPrefix(request.URL.Path, "/")
	if assetPath == "" {
		assetPath = "index.html"
	} else if strings.HasSuffix(assetPath, "/") {
		http.NotFound(response, request)
		return
	}

	if handler.serveExistingFile(response, request, assetPath) {
		return
	}
	if path.Ext(assetPath) == "" && handler.serveExistingFile(response, request, "index.html") {
		return
	}
	http.NotFound(response, request)
}

func (handler *playerHTTPHandler) serveExistingFile(response http.ResponseWriter, request *http.Request, name string) bool {
	if handler.assets == nil {
		return false
	}
	info, err := fs.Stat(handler.assets, name)
	if err != nil || info.IsDir() {
		return false
	}
	contents, err := fs.ReadFile(handler.assets, name)
	if err != nil {
		return false
	}
	http.ServeContent(response, request, name, info.ModTime(), bytes.NewReader(contents))
	return true
}

func unsafePlayerPath(requestURL *url.URL) bool {
	for _, requestPath := range []string{requestURL.Path, requestURL.RawPath} {
		if requestPath == "" {
			continue
		}
		decoded, err := url.PathUnescape(requestPath)
		if err != nil || strings.Contains(decoded, `\`) {
			return true
		}
		for segment := range strings.SplitSeq(decoded, "/") {
			if segment == "." || segment == ".." {
				return true
			}
		}
	}
	return false
}
