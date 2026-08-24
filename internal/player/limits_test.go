package player

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPresentationUplinkLimitDefaults(t *testing.T) {
	require.Equal(t, 120, PresentationIntentRatePerSecond)
	require.Equal(t, 32, PresentationIntentRateBurst)
	require.Equal(t, 30*time.Second, PresentationUplinkIdleLifetime)
	require.Equal(t, 5*time.Minute, PresentationUplinkMaximumLifetime)
	require.Equal(t, 32, MaximumConcurrentPresentationUplinks)
}

func TestPresentationUplinkBodyEnforcesIdleAndMaximumLifetime(t *testing.T) {
	for _, test := range []struct {
		name    string
		idle    time.Duration
		maximum time.Duration
		want    error
	}{
		{name: "idle", idle: 10 * time.Millisecond, maximum: time.Second, want: errPresentationUplinkIdleTimeout},
		{name: "maximum", idle: time.Second, maximum: 10 * time.Millisecond, want: errPresentationUplinkMaxLifetime},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(t.Context())
			reader, writer := io.Pipe()
			defer func() { require.NoError(t, writer.Close()) }()
			body := newPresentationUplinkBody(reader, cancel, test.idle, test.maximum)
			defer body.Stop()
			readDone := make(chan error, 1)
			go func() {
				_, err := body.Read(make([]byte, 1))
				readDone <- err
			}()
			require.Eventually(t, func() bool { return ctx.Err() != nil }, time.Second, time.Millisecond)
			require.ErrorIs(t, context.Cause(ctx), test.want)
			require.Error(t, <-readDone)
		})
	}
}

func TestPresentationUplinkBodyActivityResetsOnlyIdleDeadline(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	reader, writer := io.Pipe()
	body := newPresentationUplinkBody(reader, cancel, 30*time.Millisecond, 55*time.Millisecond)
	defer body.Stop()
	defer func() { require.NoError(t, writer.Close()) }()
	readDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, body)
		readDone <- err
	}()
	for range 4 {
		_, err := writer.Write([]byte{1})
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}
	require.NoError(t, ctx.Err(), "activity must keep the idle deadline open")
	require.Eventually(t, func() bool { return ctx.Err() != nil }, time.Second, time.Millisecond)
	require.ErrorIs(t, context.Cause(ctx), errPresentationUplinkMaxLifetime)
	require.Error(t, <-readDone)
}

func TestPresentationUplinkRateLimiterUsesInjectedClock(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := NewPresentationRateLimiter(func() time.Time { return now })
	for range PresentationIntentRateBurst {
		require.True(t, limiter.Allow())
	}
	require.False(t, limiter.Allow())
	now = now.Add(time.Second)
	require.True(t, limiter.Allow())
}
