package player

import (
	"context"
	"errors"
	"sync"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	playerv1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1"
	"google.golang.org/protobuf/proto"
)

const defaultSubscriptionQueueSize = 32

var (
	errSubscriptionContextRequired   = errors.New("player subscription context is required")
	errSubscriptionClosed            = errors.New("player subscription closed")
	errSubscriptionReplaced          = errors.New("player subscription replaced")
	errSubscriptionUnregistered      = errors.New("player subscription unregistered")
	errSubscriptionHubClosed         = errors.New("player subscription hub closed")
	errSubscriptionRevisionRegressed = errors.New("player subscription revision regressed")
	errSubscriptionQueueOverflow     = errors.New("player subscription queue overflow")
	errPresentationUplinkUnavailable = errors.New("presentation uplink subscription is unavailable")
	errPresentationUplinkGeneration  = errors.New("presentation uplink generation is stale")
	errPresentationUplinkLimit       = errors.New("presentation uplink concurrency limit reached")
	errPresentationUplinkReplaced    = errors.New("presentation uplink replaced")
)

// Subscription owns the bounded outbound queue and cancellation lifecycle for
// one physical Connect server stream. Its complete snapshot is sent directly
// before Updates is drained, so queued values can never precede it.
type Subscription struct {
	id        domain.ConnectionID
	sessionID domain.LogicalSessionID
	snapshot  *playerv1.SubscriptionMessage
	updates   chan *playerv1.SubscriptionMessage
	targeted  chan *playerv1.SubscriptionMessage
	done      chan struct{}
	context   context.Context
	cancel    context.CancelCauseFunc
	clientID  string

	mu           sync.Mutex
	lastRevision uint64
	closeOnce    sync.Once
}

// NewSubscription constructs one physical stream with an immutable first
// snapshot and a bounded queue for strictly newer compound updates.
func NewSubscription(parent context.Context, id domain.ConnectionID, sessionID domain.LogicalSessionID, snapshot *playerv1.SubscriptionMessage, queueSize int, clientInstanceID ...string) *Subscription {
	if parent == nil {
		panic(errSubscriptionContextRequired)
	}
	if queueSize <= 0 {
		queueSize = defaultSubscriptionQueueSize
	}
	ctx, cancel := context.WithCancelCause(parent)
	clientID := ""
	if len(clientInstanceID) != 0 {
		clientID = clientInstanceID[0]
	}
	stream := &Subscription{
		id: id, sessionID: sessionID, snapshot: cloneSubscriptionMessage(snapshot),
		updates: make(chan *playerv1.SubscriptionMessage, queueSize), targeted: make(chan *playerv1.SubscriptionMessage, 1),
		done: make(chan struct{}), context: ctx, cancel: cancel, clientID: clientID,
	}
	if messageSnapshot := stream.snapshot.GetSnapshot(); messageSnapshot != nil {
		stream.lastRevision = messageSnapshot.GetRevision()
	}
	go func() {
		<-ctx.Done()
		stream.close()
	}()
	return stream
}

// ClientInstanceID returns the optional immutable page-lifetime routing ID.
func (stream *Subscription) ClientInstanceID() string {
	if stream == nil {
		return ""
	}
	return stream.clientID
}

// RecognitionHandle returns the detached recognition identity from the
// mandatory first snapshot. It is routing context, not authorization.
func (stream *Subscription) RecognitionHandle() domain.RecognitionHandle {
	if stream == nil || stream.snapshot == nil || stream.snapshot.GetSnapshot() == nil {
		return ""
	}
	return domain.RecognitionHandle(stream.snapshot.GetSnapshot().GetRecognitionHandle())
}

// ID returns the physical stream identity.
func (stream *Subscription) ID() domain.ConnectionID {
	if stream == nil {
		return ""
	}
	return stream.id
}

// SessionID returns the process-local logical owner.
func (stream *Subscription) SessionID() domain.LogicalSessionID {
	if stream == nil {
		return ""
	}
	return stream.sessionID
}

// Snapshot returns a detached copy of the mandatory first message.
func (stream *Subscription) Snapshot() *playerv1.SubscriptionMessage {
	if stream == nil {
		return nil
	}
	return cloneSubscriptionMessage(stream.snapshot)
}

// Updates returns the bounded post-snapshot delivery queue.
func (stream *Subscription) Updates() <-chan *playerv1.SubscriptionMessage {
	if stream == nil {
		closed := make(chan *playerv1.SubscriptionMessage)
		close(closed)
		return closed
	}
	return stream.updates
}

// Targeted returns the independent capacity-one result route.
func (stream *Subscription) Targeted() <-chan *playerv1.SubscriptionMessage {
	if stream == nil {
		closed := make(chan *playerv1.SubscriptionMessage)
		close(closed)
		return closed
	}
	return stream.targeted
}

// PublishTargeted waits only for this subscription's targeted slot or
// cancellation. It never consumes or closes canonical update capacity.
func (stream *Subscription) PublishTargeted(ctx context.Context, message *playerv1.SubscriptionMessage) bool {
	if stream == nil || ctx == nil || message == nil || message.GetPresentationUplinkResult() == nil {
		return false
	}
	select {
	case <-stream.done:
		return false
	default:
	}
	copy := cloneSubscriptionMessage(message)
	select {
	case stream.targeted <- copy:
		return true
	case <-stream.done:
		return false
	case <-ctx.Done():
		return false
	}
}

// Offer enqueues one strictly newer compound update without blocking. An
// invalid/same-revision value or full queue closes only this stream.
func (stream *Subscription) Offer(message *playerv1.SubscriptionMessage) bool {
	if stream == nil || message == nil || message.GetUpdate() == nil {
		return false
	}
	revision := message.GetUpdate().GetRevision()
	stream.mu.Lock()
	if revision == stream.lastRevision {
		stream.mu.Unlock()
		return true
	}
	if revision < stream.lastRevision {
		stream.mu.Unlock()
		stream.closeWithCause(errSubscriptionRevisionRegressed)
		return false
	}
	select {
	case <-stream.done:
		stream.mu.Unlock()
		return false
	default:
	}
	copy := cloneSubscriptionMessage(message)
	select {
	case stream.updates <- copy:
		stream.lastRevision = revision
		stream.mu.Unlock()
		return true
	default:
		stream.mu.Unlock()
		stream.closeWithCause(errSubscriptionQueueOverflow)
		return false
	}
}

// Count reports active physical subscriptions.
func (hub *SubscriptionHub) Count() int {
	if hub == nil {
		return 0
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return len(hub.byID)
}

// Close cancels and releases this physical stream idempotently.
func (stream *Subscription) Close() {
	stream.closeWithCause(errSubscriptionClosed)
}

func (stream *Subscription) closeWithCause(cause error) {
	if stream == nil {
		return
	}
	stream.cancel(cause)
	stream.close()
}

func (stream *Subscription) close() {
	stream.closeOnce.Do(func() { close(stream.done) })
}

// Done closes when this physical stream terminates.
func (stream *Subscription) Done() <-chan struct{} {
	if stream == nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return stream.done
}

// SubscriptionHub indexes physical streams by logical session and offers one
// detached personalized update to each currently responsive sibling.
type SubscriptionHub struct {
	mu        sync.Mutex
	byID      map[domain.ConnectionID]*Subscription
	bySession map[domain.LogicalSessionID]map[domain.ConnectionID]*Subscription
	byClient  map[string]*Subscription
	uplinks   map[string]*PresentationUplink
}

// NewSubscriptionHub returns an empty physical-stream registry.
func NewSubscriptionHub() *SubscriptionHub {
	return &SubscriptionHub{
		byID:      make(map[domain.ConnectionID]*Subscription),
		bySession: make(map[domain.LogicalSessionID]map[domain.ConnectionID]*Subscription),
		byClient:  make(map[string]*Subscription),
		uplinks:   make(map[string]*PresentationUplink),
	}
}

// Register adds a stream and removes any older stream with the same physical ID.
func (hub *SubscriptionHub) Register(stream *Subscription) {
	if hub == nil || stream == nil || stream.ID() == "" || stream.SessionID() == "" {
		return
	}
	hub.mu.Lock()
	if previous := hub.byID[stream.ID()]; previous != nil {
		hub.removeLocked(previous)
		previous.closeWithCause(errSubscriptionReplaced)
	}
	if stream.ClientInstanceID() != "" {
		if previous := hub.byClient[stream.ClientInstanceID()]; previous != nil && previous != stream {
			hub.removeLocked(previous)
			previous.closeWithCause(errSubscriptionReplaced)
		}
		hub.byClient[stream.ClientInstanceID()] = stream
	}
	hub.byID[stream.ID()] = stream
	siblings := hub.bySession[stream.SessionID()]
	if siblings == nil {
		siblings = make(map[domain.ConnectionID]*Subscription)
		hub.bySession[stream.SessionID()] = siblings
	}
	siblings[stream.ID()] = stream
	hub.mu.Unlock()
}

// Unregister removes and closes one physical stream.
func (hub *SubscriptionHub) Unregister(id domain.ConnectionID) {
	if hub == nil || id == "" {
		return
	}
	hub.mu.Lock()
	stream := hub.byID[id]
	if stream != nil {
		hub.removeLocked(stream)
	}
	hub.mu.Unlock()
	if stream != nil {
		stream.closeWithCause(errSubscriptionUnregistered)
	}
}

// Offer sends one logical update to every currently responsive physical stream.
func (hub *SubscriptionHub) Offer(sessionID domain.LogicalSessionID, message *playerv1.SubscriptionMessage) {
	if hub == nil || sessionID == "" || message == nil {
		return
	}
	hub.mu.Lock()
	streams := make([]*Subscription, 0, len(hub.bySession[sessionID]))
	for _, stream := range hub.bySession[sessionID] {
		streams = append(streams, stream)
	}
	hub.mu.Unlock()
	for _, stream := range streams {
		if !stream.Offer(message) {
			hub.Unregister(stream.ID())
		}
	}
}

// CloseAll detaches and cancels every physical stream without holding the hub
// lock across cancellation callbacks. It is safe to call repeatedly.
func (hub *SubscriptionHub) CloseAll() {
	if hub == nil {
		return
	}
	hub.mu.Lock()
	streams := make([]*Subscription, 0, len(hub.byID))
	for _, stream := range hub.byID {
		streams = append(streams, stream)
	}
	hub.byID = make(map[domain.ConnectionID]*Subscription)
	hub.bySession = make(map[domain.LogicalSessionID]map[domain.ConnectionID]*Subscription)
	hub.byClient = make(map[string]*Subscription)
	uplinks := make([]*PresentationUplink, 0, len(hub.uplinks))
	for _, uplink := range hub.uplinks {
		uplinks = append(uplinks, uplink)
	}
	hub.uplinks = make(map[string]*PresentationUplink)
	hub.mu.Unlock()
	for _, uplink := range uplinks {
		uplink.Close(errSubscriptionHubClosed)
	}
	for _, stream := range streams {
		stream.closeWithCause(errSubscriptionHubClosed)
	}
}

// CloseUplinks cancels every presentation optimization stream while retaining
// the physical subscriptions that carry canonical state. It is safe to call
// repeatedly, allowing a lifecycle boundary to rotate uplinks without forcing
// an authoritative downlink reconnect.
func (hub *SubscriptionHub) CloseUplinks(cause error) {
	if hub == nil {
		return
	}
	hub.mu.Lock()
	uplinks := make([]*PresentationUplink, 0, len(hub.uplinks))
	for _, uplink := range hub.uplinks {
		uplinks = append(uplinks, uplink)
	}
	hub.uplinks = make(map[string]*PresentationUplink)
	hub.mu.Unlock()
	for _, uplink := range uplinks {
		uplink.Close(cause)
	}
}

func (hub *SubscriptionHub) removeLocked(stream *Subscription) {
	delete(hub.byID, stream.ID())
	if stream.ClientInstanceID() != "" && hub.byClient[stream.ClientInstanceID()] == stream {
		delete(hub.byClient, stream.ClientInstanceID())
		if uplink := hub.uplinks[stream.ClientInstanceID()]; uplink != nil {
			delete(hub.uplinks, stream.ClientInstanceID())
			uplink.Close(errSubscriptionUnregistered)
		}
	}
	siblings := hub.bySession[stream.SessionID()]
	delete(siblings, stream.ID())
	if len(siblings) == 0 {
		delete(hub.bySession, stream.SessionID())
	}
}

// LatestIntentMailbox retains one newest unprocessed generated intent.
type LatestIntentMailbox struct {
	mu      sync.Mutex
	pending *playerv1.PresentationIntent
	notify  chan struct{}
	done    chan struct{}
	close   sync.Once
	closed  bool
}

// NewLatestIntentMailbox constructs an empty process-local mailbox.
func NewLatestIntentMailbox() *LatestIntentMailbox {
	return &LatestIntentMailbox{notify: make(chan struct{}, 1), done: make(chan struct{})}
}

// Offer atomically replaces an older unprocessed value.
func (mailbox *LatestIntentMailbox) Offer(intent *playerv1.PresentationIntent) bool {
	if mailbox == nil || intent == nil {
		return false
	}
	mailbox.mu.Lock()
	if mailbox.closed {
		mailbox.mu.Unlock()
		return false
	}
	mailbox.pending = proto.Clone(intent).(*playerv1.PresentationIntent)
	mailbox.mu.Unlock()
	select {
	case mailbox.notify <- struct{}{}:
	default:
	}
	return true
}

// Take removes the newest pending value or waits for offer/cancellation.
func (mailbox *LatestIntentMailbox) Take(ctx context.Context) (*playerv1.PresentationIntent, bool) {
	if mailbox == nil || ctx == nil {
		return nil, false
	}
	for {
		mailbox.mu.Lock()
		intent := mailbox.pending
		mailbox.pending = nil
		mailbox.mu.Unlock()
		if intent != nil {
			return intent, true
		}
		select {
		case <-mailbox.notify:
		case <-mailbox.done:
			mailbox.mu.Lock()
			intent := mailbox.pending
			mailbox.pending = nil
			mailbox.mu.Unlock()
			if intent != nil {
				return intent, true
			}
			return nil, false
		case <-ctx.Done():
			return nil, false
		}
	}
}

// Close clears pending state and wakes a waiting processor.
func (mailbox *LatestIntentMailbox) Close(_ error) {
	if mailbox == nil {
		return
	}
	mailbox.close.Do(func() {
		mailbox.mu.Lock()
		mailbox.closed = true
		mailbox.pending = nil
		mailbox.mu.Unlock()
		close(mailbox.done)
	})
}

// Finish rejects future offers but lets the processor consume the final
// pending value before Take reports closure.
func (mailbox *LatestIntentMailbox) Finish() {
	if mailbox == nil {
		return
	}
	mailbox.close.Do(func() {
		mailbox.mu.Lock()
		mailbox.closed = true
		mailbox.mu.Unlock()
		close(mailbox.done)
	})
}

// PresentationUplink is one generation-bound server worker lease.
type PresentationUplink struct {
	Binding      PresentationUplinkBinding
	ConnectionID domain.ConnectionID
	Subscription *Subscription
	Mailbox      *LatestIntentMailbox
	Limiter      *PresentationRateLimiter
	Context      context.Context
	cancel       context.CancelCauseFunc
	close        sync.Once
}

// BindUplink validates a current physical subscription and atomically replaces
// an older generation for the same browser tab.
func (hub *SubscriptionHub) BindUplink(parent context.Context, binding PresentationUplinkBinding) (*PresentationUplink, error) {
	if hub == nil || parent == nil || binding.ClientInstanceID == "" || binding.Generation == 0 {
		return nil, errPresentationUplinkUnavailable
	}
	hub.mu.Lock()
	subscription := hub.byClient[binding.ClientInstanceID]
	if subscription == nil || subscription.RecognitionHandle() != binding.RecognitionHandle {
		hub.mu.Unlock()
		return nil, errPresentationUplinkUnavailable
	}
	previous := hub.uplinks[binding.ClientInstanceID]
	if previous != nil && binding.Generation <= previous.Binding.Generation {
		hub.mu.Unlock()
		return nil, errPresentationUplinkGeneration
	}
	if previous == nil && len(hub.uplinks) >= MaximumConcurrentPresentationUplinks {
		hub.mu.Unlock()
		return nil, errPresentationUplinkLimit
	}
	ctx, cancel := context.WithCancelCause(parent)
	uplink := &PresentationUplink{
		Binding: binding, ConnectionID: subscription.ID(), Subscription: subscription,
		Mailbox: NewLatestIntentMailbox(), Limiter: NewPresentationRateLimiter(nil), Context: ctx, cancel: cancel,
	}
	hub.uplinks[binding.ClientInstanceID] = uplink
	hub.mu.Unlock()
	if previous != nil {
		previous.Close(errPresentationUplinkReplaced)
	}
	go func() {
		select {
		case <-subscription.Done():
			uplink.Close(errSubscriptionUnregistered)
		case <-ctx.Done():
		}
	}()
	return uplink, nil
}

// Current reports whether this lease remains the hub's accepted generation.
func (hub *SubscriptionHub) Current(uplink *PresentationUplink) bool {
	if hub == nil || uplink == nil {
		return false
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return hub.uplinks[uplink.Binding.ClientInstanceID] == uplink && hub.byID[uplink.ConnectionID] == uplink.Subscription
}

// ReleaseUplink removes only the matching current generation.
func (hub *SubscriptionHub) ReleaseUplink(uplink *PresentationUplink, cause error) {
	if hub == nil || uplink == nil {
		return
	}
	hub.mu.Lock()
	if hub.uplinks[uplink.Binding.ClientInstanceID] == uplink {
		delete(hub.uplinks, uplink.Binding.ClientInstanceID)
	}
	hub.mu.Unlock()
	uplink.Close(cause)
}

// Close cancels the lease and clears its pending intent idempotently.
func (uplink *PresentationUplink) Close(cause error) {
	if uplink == nil {
		return
	}
	uplink.close.Do(func() {
		uplink.Mailbox.Close(cause)
		uplink.cancel(cause)
	})
}

func cloneSubscriptionMessage(message *playerv1.SubscriptionMessage) *playerv1.SubscriptionMessage {
	if message == nil {
		return nil
	}
	return proto.Clone(message).(*playerv1.SubscriptionMessage)
}
