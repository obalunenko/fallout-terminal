package player

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
)

const (
	// MaxUncompressedMessageBytes is the effective protobuf message limit for
	// every public player request, including unknown fields.
	MaxUncompressedMessageBytes = 4 << 10
	// MaxEncodedBodyBytes permits bounded Connect framing overhead while
	// rejecting oversized HTTP input before application adapters run.
	MaxEncodedBodyBytes = 8 << 10
	// MaxDecompressedMessageBytes prevents compressed input from expanding past
	// the same effective player-controlled protobuf limit.
	MaxDecompressedMessageBytes = 4 << 10
	// PresentationIntentRatePerSecond bounds received semantic intent frames.
	PresentationIntentRatePerSecond = 120
	// PresentationIntentRateBurst permits short animation-cadence bursts.
	PresentationIntentRateBurst = 32
	// PresentationUplinkIdleLifetime closes an inactive optimization stream.
	PresentationUplinkIdleLifetime = 30 * time.Second
	// PresentationUplinkMaximumLifetime rotates long-lived optimization streams.
	PresentationUplinkMaximumLifetime = 5 * time.Minute
	// MaximumConcurrentPresentationUplinks bounds process-wide stream workers.
	MaximumConcurrentPresentationUplinks = 32
)

// ErrResourceExhausted identifies a transport/message bound violation without
// retaining or exposing the rejected request bytes.
var ErrResourceExhausted = errors.New("public player request exceeds configured limit")

// PresentationRateLimiter is a small deterministic token bucket owned by one
// uplink. It deliberately avoids a runtime dependency for fixed process limits.
type PresentationRateLimiter struct {
	mu     sync.Mutex
	now    func() time.Time
	last   time.Time
	tokens float64
}

// NewPresentationRateLimiter constructs a full-burst limiter with an injected
// monotonic clock seam for deterministic tests.
func NewPresentationRateLimiter(now func() time.Time) *PresentationRateLimiter {
	if now == nil {
		now = time.Now
	}
	current := now()
	return &PresentationRateLimiter{now: now, last: current, tokens: PresentationIntentRateBurst}
}

// Allow consumes one received-frame token.
func (limiter *PresentationRateLimiter) Allow() bool {
	if limiter == nil {
		return false
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	current := limiter.now()
	elapsed := current.Sub(limiter.last).Seconds()
	if elapsed > 0 {
		limiter.tokens += elapsed * PresentationIntentRatePerSecond
		if limiter.tokens > PresentationIntentRateBurst {
			limiter.tokens = PresentationIntentRateBurst
		}
		limiter.last = current
	}
	if limiter.tokens < 1 {
		return false
	}
	limiter.tokens--
	return true
}

// ValidateMessageSize counts known and unknown protobuf fields and rejects the
// value before any canonical adapter or service invocation.
func ValidateMessageSize(message proto.Message) error {
	if message == nil {
		return nil
	}
	if size := proto.Size(message); size > MaxUncompressedMessageBytes {
		return fmt.Errorf("%w: protobuf message is %d bytes; maximum is %d", ErrResourceExhausted, size, MaxUncompressedMessageBytes)
	}
	return nil
}

// ReadEncodedBody reads one bounded encoded HTTP body without retaining a
// prefix when the body crosses the configured limit.
func ReadEncodedBody(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxEncodedBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read public player request: %w", err)
	}
	if len(data) > MaxEncodedBodyBytes {
		return nil, fmt.Errorf("%w: encoded body maximum is %d bytes", ErrResourceExhausted, MaxEncodedBodyBytes)
	}
	return data, nil
}
