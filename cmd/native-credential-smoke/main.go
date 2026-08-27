// Command native-credential-smoke exercises the platform-protected production
// credential namespace on matching CI hosts without printing secret material.
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/platform"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/tunnel"
)

const smokeTimeout = 30 * time.Second

func main() {
	var canaryPath string
	var expectUnavailable bool
	flag.StringVar(&canaryPath, "canary-file", "", "private file receiving redacted leak-scan canaries")
	flag.BoolVar(&expectUnavailable, "expect-unavailable", false, "require a fail-closed unavailable, locked, or denied native store")
	flag.Parse()
	if flag.NArg() != 0 || (!expectUnavailable && canaryPath == "") {
		fmt.Fprintln(os.Stderr, "native-credential-smoke: usage: --canary-file PATH [--expect-unavailable]")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), smokeTimeout)
	defer cancel()
	store := platform.NewPlatformSecureCredentialStore(true)
	if expectUnavailable {
		if err := verifyUnavailable(ctx, store); err != nil {
			fmt.Fprintf(os.Stderr, "native-credential-smoke: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("native-credential-smoke: fail-closed native store state passed")
		return
	}
	if err := verifyRoundTrip(ctx, store, canaryPath); err != nil {
		fmt.Fprintf(os.Stderr, "native-credential-smoke: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("native-credential-smoke: protected write/read/replace/delete round trip passed")
}

func verifyRoundTrip(ctx context.Context, store tunnel.SecretStore, canaryPath string) error {
	first, err := randomCanary()
	if err != nil {
		return err
	}
	second, err := randomCanary()
	if err != nil {
		clear(first)
		return err
	}
	defer clear(first)
	defer clear(second)
	if err := writeCanaries(canaryPath, first, second); err != nil {
		return err
	}

	refs := []tunnel.SecretRef{tunnel.ProviderAccountToken, tunnel.PlayerBasicAuthPassword}
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		for _, ref := range refs {
			_ = store.Delete(cleanupCtx, ref)
		}
	}
	defer cleanup()

	for _, ref := range refs {
		if err := store.Replace(ctx, ref, first); err != nil {
			return fmt.Errorf("initial protected replacement for %s: %w", ref.Account(), err)
		}
		if err := requirePresence(ctx, store, ref, tunnel.SecretPresent); err != nil {
			return err
		}
		if err := requireStoredValue(ctx, store, ref, first); err != nil {
			return err
		}
		if err := store.Replace(ctx, ref, second); err != nil {
			return fmt.Errorf("second protected replacement for %s: %w", ref.Account(), err)
		}
		if err := requireStoredValue(ctx, store, ref, second); err != nil {
			return err
		}
		if err := store.Delete(ctx, ref); err != nil {
			return fmt.Errorf("delete protected value for %s: %w", ref.Account(), err)
		}
		if err := requirePresence(ctx, store, ref, tunnel.SecretAbsent); err != nil {
			return err
		}
	}
	return nil
}

func verifyUnavailable(ctx context.Context, store tunnel.SecretStore) error {
	_, err := store.Presence(ctx, tunnel.ProviderAccountToken)
	if errors.Is(err, tunnel.ErrSecretStoreUnavailable) ||
		errors.Is(err, tunnel.ErrSecretStoreLocked) ||
		errors.Is(err, tunnel.ErrSecretStoreDenied) {
		return nil
	}
	if err == nil {
		return errors.New("native credential store unexpectedly remained available")
	}
	return fmt.Errorf("native credential store returned an imprecise failure category: %w", err)
}

func requirePresence(
	ctx context.Context,
	store tunnel.SecretStore,
	ref tunnel.SecretRef,
	want tunnel.SecretPresence,
) error {
	got, err := store.Presence(ctx, ref)
	if err != nil {
		return fmt.Errorf("inspect protected presence for %s: %w", ref.Account(), err)
	}
	if got != want {
		return fmt.Errorf("protected presence for %s is %d, want %d", ref.Account(), got, want)
	}
	return nil
}

func requireStoredValue(ctx context.Context, store tunnel.SecretStore, ref tunnel.SecretRef, want []byte) error {
	matched := false
	err := store.WithSecrets(ctx, []tunnel.SecretRef{ref}, func(use *tunnel.SecretUse) error {
		switch ref {
		case tunnel.ProviderAccountToken:
			matched = subtle.ConstantTimeCompare(use.ProviderToken, want) == 1
		case tunnel.PlayerBasicAuthPassword:
			matched = subtle.ConstantTimeCompare(use.PlayerPassword, want) == 1
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scoped protected read for %s: %w", ref.Account(), err)
	}
	if !matched {
		return fmt.Errorf("scoped protected read for %s did not return the replaced value", ref.Account())
	}
	return nil
}

func randomCanary() ([]byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		clear(raw)
		return nil, errors.New("cryptographic canary generation failed")
	}
	encoded := make([]byte, hex.EncodedLen(len(raw)))
	hex.Encode(encoded, raw)
	clear(raw)
	return encoded, nil
}

func writeCanaries(path string, values ...[]byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create private canary file: %w", err)
	}
	var writeErr error
	for _, value := range values {
		if _, err := file.Write(value); err != nil {
			writeErr = err
			break
		}
		if _, err := file.Write([]byte{'\n'}); err != nil {
			writeErr = err
			break
		}
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write private canary file: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close private canary file: %w", closeErr)
	}
	return nil
}
