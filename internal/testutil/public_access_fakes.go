package testutil

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/obalunenko/Fallout-Terminal/internal/tunnel"
)

type fakeClockTimer struct {
	deadline time.Time
	channel  chan time.Time
	fired    bool
}

func (c *FakeClock) After(duration time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	channel := make(chan time.Time, 1)
	c.timers = append(c.timers, fakeClockTimer{deadline: c.now.Add(duration), channel: channel})
	c.fireTimersLocked()
	return channel
}

func (c *FakeClock) fireTimersLocked() {
	for index := range c.timers {
		timer := &c.timers[index]
		if !timer.fired && !c.now.Before(timer.deadline) {
			timer.fired = true
			timer.channel <- timer.deadline
		}
	}
}

type FakeSecretStore struct {
	mu sync.Mutex

	secrets map[tunnel.SecretRef][]byte

	PresenceErrors map[tunnel.SecretRef]error
	ReplaceErrors  map[tunnel.SecretRef]error
	DeleteErrors   map[tunnel.SecretRef]error
	UseError       error
	lastUseCleared bool
}

func NewFakeSecretStore() *FakeSecretStore {
	return &FakeSecretStore{
		secrets:        make(map[tunnel.SecretRef][]byte),
		PresenceErrors: make(map[tunnel.SecretRef]error),
		ReplaceErrors:  make(map[tunnel.SecretRef]error),
		DeleteErrors:   make(map[tunnel.SecretRef]error),
	}
}

func (store *FakeSecretStore) Presence(_ context.Context, ref tunnel.SecretRef) (tunnel.SecretPresence, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.PresenceErrors[ref]; err != nil {
		return tunnel.SecretUnknown, err
	}
	if len(store.secrets[ref]) == 0 {
		return tunnel.SecretAbsent, nil
	}
	return tunnel.SecretPresent, nil
}

func (store *FakeSecretStore) Replace(_ context.Context, ref tunnel.SecretRef, value []byte) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !ref.Valid() {
		return errors.New("invalid secret reference")
	}
	if err := store.ReplaceErrors[ref]; err != nil {
		return err
	}
	clear(store.secrets[ref])
	store.secrets[ref] = append([]byte(nil), value...)
	return nil
}

func (store *FakeSecretStore) Delete(_ context.Context, ref tunnel.SecretRef) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.DeleteErrors[ref]; err != nil {
		return err
	}
	clear(store.secrets[ref])
	delete(store.secrets, ref)
	return nil
}

func (store *FakeSecretStore) WithSecrets(_ context.Context, refs []tunnel.SecretRef, callback func(*tunnel.SecretUse) error) error {
	store.mu.Lock()
	if store.UseError != nil {
		err := store.UseError
		store.mu.Unlock()
		return err
	}
	use := &tunnel.SecretUse{}
	for _, ref := range refs {
		value := store.secrets[ref]
		if len(value) == 0 {
			store.mu.Unlock()
			return errors.New("required secret is absent")
		}
		switch ref {
		case tunnel.ProviderAccountToken:
			use.ProviderToken = append([]byte(nil), value...)
		case tunnel.PlayerBasicAuthPassword:
			use.PlayerPassword = append([]byte(nil), value...)
		default:
			store.mu.Unlock()
			return errors.New("invalid secret reference")
		}
	}
	store.mu.Unlock()

	err := callback(use)
	use.Clear()
	store.mu.Lock()
	store.lastUseCleared = len(use.ProviderToken) == 0 && len(use.PlayerPassword) == 0
	store.mu.Unlock()
	return err
}

func (store *FakeSecretStore) LastUseCleared() bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.lastUseCleared
}

type FakeTunnelService struct {
	mu sync.Mutex

	Endpoint *FakeTunnelEndpoint
	StartErr error
	gate     chan struct{}
	released bool
	starts   int
	active   int
	cleared  bool
	upstream string
}

type FakePublicIngressFactory struct {
	mu sync.Mutex

	StartErr error
	starts   int
	active   int
}

func NewFakePublicIngressFactory() *FakePublicIngressFactory {
	return &FakePublicIngressFactory{}
}

func (factory *FakePublicIngressFactory) Start(_ context.Context, _ string) (tunnel.PublicIngress, error) {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	factory.starts++
	if factory.StartErr != nil {
		return nil, factory.StartErr
	}
	factory.active++
	return &FakePublicIngress{
		factory: factory,
		url:     url.URL{Scheme: "http", Host: "127.0.0.1:43690"},
	}, nil
}

func (factory *FakePublicIngressFactory) StartCalls() int {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	return factory.starts
}

func (factory *FakePublicIngressFactory) ActiveIngresses() int {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	return factory.active
}

type FakePublicIngress struct {
	mu sync.Mutex

	factory *FakePublicIngressFactory
	url     url.URL
	closed  bool
}

func (ingress *FakePublicIngress) URL() *url.URL {
	copyURL := ingress.url
	return &copyURL
}

func (ingress *FakePublicIngress) Activate(_, _ string, _ []byte) error {
	ingress.mu.Lock()
	defer ingress.mu.Unlock()
	if ingress.closed {
		return errors.New("fake public ingress is closed")
	}
	return nil
}

func (ingress *FakePublicIngress) Deny() {}

func (ingress *FakePublicIngress) Close(_ context.Context) error {
	ingress.mu.Lock()
	if ingress.closed {
		ingress.mu.Unlock()
		return nil
	}
	ingress.closed = true
	factory := ingress.factory
	ingress.mu.Unlock()

	if factory != nil {
		factory.mu.Lock()
		if factory.active > 0 {
			factory.active--
		}
		factory.mu.Unlock()
	}
	return nil
}

func NewFakeTunnelService(endpoint *FakeTunnelEndpoint) *FakeTunnelService {
	service := &FakeTunnelService{Endpoint: endpoint, released: true}
	if endpoint != nil {
		endpoint.owner = service
	}
	return service
}

func (service *FakeTunnelService) DelayStart() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.gate = make(chan struct{})
	service.released = false
}

func (service *FakeTunnelService) ReleaseStart() {
	service.mu.Lock()
	defer service.mu.Unlock()
	if !service.released {
		close(service.gate)
		service.released = true
	}
}

func (service *FakeTunnelService) Start(ctx context.Context, request tunnel.TunnelStartRequest) (tunnel.TunnelEndpoint, error) {
	service.mu.Lock()
	service.starts++
	service.upstream = request.UpstreamURL
	gate := service.gate
	released := service.released
	endpoint := service.Endpoint
	startErr := service.StartErr
	service.mu.Unlock()

	defer func() {
		request.Clear()
		service.mu.Lock()
		service.cleared = len(request.AccountToken) == 0
		service.mu.Unlock()
	}()
	if !released {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if startErr != nil {
		return nil, startErr
	}
	if endpoint == nil {
		return nil, errors.New("fake tunnel has no endpoint")
	}
	service.mu.Lock()
	service.active++
	service.mu.Unlock()
	return endpoint, nil
}

func (service *FakeTunnelService) StartCalls() int {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.starts
}

func (service *FakeTunnelService) ActiveEndpoints() int {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.active
}

func (service *FakeTunnelService) LastStartSecretsCleared() bool {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.cleared
}

func (service *FakeTunnelService) LastUpstreamURL() string {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.upstream
}

func (service *FakeTunnelService) endpointClosed() {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.active > 0 {
		service.active--
	}
}

type FakeTunnelEndpoint struct {
	mu sync.Mutex

	endpointURL *url.URL
	done        chan struct{}
	doneOnce    sync.Once
	closed      bool
	owner       *FakeTunnelService
	CloseErr    error
	closeCalls  int
}

func NewFakeTunnelEndpoint(rawURL string) *FakeTunnelEndpoint {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		panic(fmt.Sprintf("testutil: invalid fake endpoint URL: %v", err))
	}
	return &FakeTunnelEndpoint{endpointURL: parsed, done: make(chan struct{})}
}

func (endpoint *FakeTunnelEndpoint) URL() *url.URL {
	endpoint.mu.Lock()
	defer endpoint.mu.Unlock()
	copyURL := *endpoint.endpointURL
	return &copyURL
}

func (endpoint *FakeTunnelEndpoint) Done() <-chan struct{} {
	return endpoint.done
}

func (endpoint *FakeTunnelEndpoint) Complete() {
	endpoint.doneOnce.Do(func() { close(endpoint.done) })
}

func (endpoint *FakeTunnelEndpoint) Close(_ context.Context) error {
	endpoint.mu.Lock()
	endpoint.closeCalls++
	if endpoint.CloseErr != nil {
		err := endpoint.CloseErr
		endpoint.mu.Unlock()
		return err
	}
	if endpoint.closed {
		endpoint.mu.Unlock()
		return nil
	}
	endpoint.closed = true
	owner := endpoint.owner
	endpoint.mu.Unlock()
	endpoint.Complete()
	if owner != nil {
		owner.endpointClosed()
	}
	return nil
}

type fakeAtomicFile struct {
	filesystem *FakeFileSystem
	path       string
	closed     bool
}

func (file *fakeAtomicFile) Write(payload []byte) (int, error) {
	file.filesystem.mu.Lock()
	defer file.filesystem.mu.Unlock()
	if file.closed {
		return 0, fs.ErrClosed
	}
	file.filesystem.files[file.path] = append(file.filesystem.files[file.path], payload...)
	return len(payload), nil
}

func (file *fakeAtomicFile) Sync() error { return nil }

func (file *fakeAtomicFile) Chmod(mode fs.FileMode) error {
	file.filesystem.mu.Lock()
	defer file.filesystem.mu.Unlock()
	file.filesystem.modes[file.path] = mode
	return nil
}

func (file *fakeAtomicFile) Close() error {
	file.filesystem.mu.Lock()
	defer file.filesystem.mu.Unlock()
	file.closed = true
	return nil
}

func (file *fakeAtomicFile) Name() string { return file.path }

func (f *FakeFileSystem) CreateTemp(directory, pattern string) (tunnel.SyncWriteCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensureMaps()
	f.temps++
	name := strings.Replace(pattern, "*", fmt.Sprintf("%06d", f.temps), 1)
	path := filepath.Join(directory, name)
	f.files[path] = nil
	f.modes[path] = 0o600
	return &fakeAtomicFile{filesystem: f, path: path}, nil
}

func (f *FakeFileSystem) Chmod(path string, mode fs.FileMode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensureMaps()
	if _, fileExists := f.files[path]; !fileExists {
		if _, directoryExists := f.dirs[path]; !directoryExists {
			return &fs.PathError{Op: "chmod", Path: path, Err: fs.ErrNotExist}
		}
	}
	f.modes[path] = mode
	return nil
}

type fakeSyncDirectory struct{}

func (fakeSyncDirectory) Sync() error  { return nil }
func (fakeSyncDirectory) Close() error { return nil }

func (f *FakeFileSystem) Open(path string) (tunnel.SyncCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.dirs[path]; !ok {
		return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
	}
	return fakeSyncDirectory{}, nil
}

type FakeSnapshotPublisher struct {
	mu        sync.Mutex
	snapshots []tunnel.PublicAccessSnapshot
}

func NewFakeSnapshotPublisher() *FakeSnapshotPublisher { return &FakeSnapshotPublisher{} }

func (publisher *FakeSnapshotPublisher) Publish(snapshot tunnel.PublicAccessSnapshot) {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	publisher.snapshots = append(publisher.snapshots, snapshot)
}

func (publisher *FakeSnapshotPublisher) Snapshots() []tunnel.PublicAccessSnapshot {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	return append([]tunnel.PublicAccessSnapshot(nil), publisher.snapshots...)
}

var _ tunnel.SecretStore = (*FakeSecretStore)(nil)
var _ tunnel.TunnelService = (*FakeTunnelService)(nil)
var _ tunnel.TunnelEndpoint = (*FakeTunnelEndpoint)(nil)
var _ tunnel.Clock = (*FakeClock)(nil)
var _ tunnel.FileSystem = (*FakeFileSystem)(nil)
