package tunnel

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

const publicIngressRealm = `Basic realm="Fallout Terminal Players"`

var (
	errPublicIngressContextRequired = errors.New("public ingress context is required")
	errPublicIngressClosed          = errors.New("public ingress closed")
)

// PublicIngress is the application-owned public admission boundary. It has no
// game state and forwards accepted requests to the sole player service.
type PublicIngress interface {
	URL() *url.URL
	Activate(host, username string, password []byte) error
	Deny()
	Close(context.Context) error
}

// PublicIngressFactory starts one loopback-only ingress in deny-all mode.
type PublicIngressFactory interface {
	Start(context.Context, string) (PublicIngress, error)
}

type loopbackPublicIngressFactory struct{}

type ingressAuthorization struct {
	host           string
	usernameDigest [sha256.Size]byte
	passwordDigest [sha256.Size]byte
}

type loopbackPublicIngress struct {
	listener  net.Listener
	server    *http.Server
	transport *http.Transport
	url       *url.URL
	policy    atomic.Pointer[ingressAuthorization]
	context   context.Context
	cancel    context.CancelCauseFunc

	closeMu  sync.Mutex
	closed   bool
	closing  chan struct{}
	closeErr error
}

func NewPublicIngressFactory() PublicIngressFactory {
	return loopbackPublicIngressFactory{}
}

func (loopbackPublicIngressFactory) Start(ctx context.Context, rawUpstream string) (PublicIngress, error) {
	if ctx == nil {
		return nil, errPublicIngressContextRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	upstream, err := normalizeLoopbackHTTPURL(rawUpstream)
	if err != nil {
		return nil, errors.New(ErrorValidation.SafeMessage())
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, publicIngressListenFailure(err)
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || address.Port <= 0 {
		_ = listener.Close()
		return nil, errors.New(ErrorProviderFailure.SafeMessage())
	}
	ingressURL, err := url.Parse("http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(address.Port)))
	if err != nil {
		_ = listener.Close()
		return nil, errors.New(ErrorProviderFailure.SafeMessage())
	}
	lifetimeContext, cancel := context.WithCancelCause(context.WithoutCancel(ctx))
	ingress := &loopbackPublicIngress{listener: listener, url: ingressURL, context: lifetimeContext, cancel: cancel}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	upstreamProtocols := new(http.Protocols)
	upstreamProtocols.SetUnencryptedHTTP2(true)
	ingress.transport = &http.Transport{Protocols: upstreamProtocols}
	proxy.Transport = ingress.transport
	proxy.FlushInterval = -1
	proxy.ErrorHandler = func(response http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(response, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
	}
	ingressProtocols := new(http.Protocols)
	ingressProtocols.SetHTTP1(true)
	ingressProtocols.SetUnencryptedHTTP2(true)
	ingress.server = &http.Server{
		Handler: http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			ingress.serve(proxy, response, request)
		}),
		BaseContext: func(net.Listener) context.Context { return lifetimeContext },
		Protocols:   ingressProtocols,
	}
	go func() { _ = ingress.server.Serve(listener) }()
	return ingress, nil
}

func publicIngressListenFailure(err error) error {
	category, _ := redactedPublicAccessFailure(err)
	return publicAccessCategorizedError{
		category: category, diagnosticCode: DiagnosticPublicIngressListenFailed,
	}
}

func (ingress *loopbackPublicIngress) URL() *url.URL {
	if ingress == nil || ingress.url == nil {
		return nil
	}
	copyURL := *ingress.url
	return &copyURL
}

func (ingress *loopbackPublicIngress) Activate(host, username string, password []byte) error {
	if ingress == nil {
		return errors.New(ErrorProviderFailure.SafeMessage())
	}
	normalizedHost, err := normalizeIngressHost(host)
	if err != nil {
		return errors.New(ErrorValidation.SafeMessage())
	}
	username = strings.TrimSpace(username)
	if username == "" || strings.ContainsAny(username, ":\x00\r\n") ||
		ValidatePlayerPassword(password) != nil || len(password) > MaximumPlayerPasswordBytes {
		return errors.New(ErrorValidation.SafeMessage())
	}
	policy := &ingressAuthorization{
		host: normalizedHost, usernameDigest: sha256.Sum256([]byte(username)),
		passwordDigest: sha256.Sum256(password),
	}
	ingress.policy.Store(policy)
	return nil
}

func (ingress *loopbackPublicIngress) Deny() {
	if ingress != nil {
		ingress.policy.Store(nil)
	}
}

func (ingress *loopbackPublicIngress) Close(ctx context.Context) error {
	if ingress == nil {
		return nil
	}
	if ctx == nil {
		return errPublicIngressContextRequired
	}
	ingress.Deny()
	for {
		ingress.closeMu.Lock()
		if ingress.closed {
			err := ingress.closeErr
			ingress.closeMu.Unlock()
			return err
		}
		if ingress.closing != nil {
			done := ingress.closing
			ingress.closeMu.Unlock()
			select {
			case <-done:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		done := make(chan struct{})
		ingress.closing = done
		ingress.closeMu.Unlock()

		err := ingress.server.Shutdown(ctx)
		ingress.transport.CloseIdleConnections()
		if err != nil {
			err = errors.Join(err, ingress.server.Close())
		}
		ingress.closeMu.Lock()
		ingress.closed = true
		ingress.closeErr = err
		ingress.closing = nil
		close(done)
		ingress.closeMu.Unlock()
		if err != nil {
			ingress.cancel(err)
		} else {
			ingress.cancel(errPublicIngressClosed)
		}
		return err
	}
}

func (ingress *loopbackPublicIngress) serve(proxy *httputil.ReverseProxy, response http.ResponseWriter, request *http.Request) {
	policy := ingress.policy.Load()
	if policy == nil {
		http.NotFound(response, request)
		return
	}
	host, err := normalizeIngressHost(request.Host)
	if err != nil || host != policy.host {
		http.Error(response, http.StatusText(http.StatusMisdirectedRequest), http.StatusMisdirectedRequest)
		return
	}
	username, password, ok := request.BasicAuth()
	usernameDigest := sha256.Sum256([]byte(username))
	passwordDigest := sha256.Sum256([]byte(password))
	if !ok || subtle.ConstantTimeCompare(usernameDigest[:], policy.usernameDigest[:]) != 1 ||
		subtle.ConstantTimeCompare(passwordDigest[:], policy.passwordDigest[:]) != 1 {
		response.Header().Set("WWW-Authenticate", publicIngressRealm)
		http.Error(response, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	forwarded := request.Clone(request.Context())
	forwarded.Header.Del("Authorization")
	forwarded.Header.Del("Proxy-Authorization")
	proxy.ServeHTTP(response, forwarded)
}

func normalizeIngressHost(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, ":") {
		host, port, err := net.SplitHostPort(raw)
		ip := net.ParseIP(host)
		portNumber, portErr := strconv.Atoi(port)
		if err != nil || ip == nil || !ip.IsLoopback() || portErr != nil || portNumber <= 0 || portNumber > 65535 {
			return "", errors.New("public Host authority is invalid")
		}
		return net.JoinHostPort(ip.String(), strconv.Itoa(portNumber)), nil
	}
	return NormalizeReservedDomain(raw)
}

func normalizeLoopbackHTTPURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" ||
		parsed.Fragment != "" || parsed.Path != "" || parsed.Port() == "" {
		return nil, errors.New("private ingress URL is invalid")
	}
	ip := net.ParseIP(parsed.Hostname())
	port, portErr := strconv.Atoi(parsed.Port())
	if ip == nil || !ip.IsLoopback() || portErr != nil || port <= 0 || port > 65535 {
		return nil, errors.New("private ingress URL is invalid")
	}
	return &url.URL{Scheme: "http", Host: net.JoinHostPort(ip.String(), strconv.Itoa(port))}, nil
}
