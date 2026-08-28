package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	GeneratedPasswordEntropyBytes = 16
	MinimumPlayerPasswordBytes    = 8
	MaximumPlayerPasswordBytes    = 128
)

var (
	ErrSecretStoreLocked        = errors.New("keychain is locked")
	ErrSecretStoreDenied        = errors.New("keychain access was denied")
	ErrSecretStoreUnavailable   = errors.New("keychain is unavailable")
	ErrSecretStoreUserCancelled = errors.New("keychain access was cancelled")
)

type SecretRef uint8

const (
	ProviderAccountToken SecretRef = iota + 1
	PlayerBasicAuthPassword
)

const (
	ProviderAccountTokenAccount = "ngrok-authtoken"
	PlayerPasswordAccount       = "player-basic-auth-password"
)

func (ref SecretRef) Valid() bool {
	return ref == ProviderAccountToken || ref == PlayerBasicAuthPassword
}

func (ref SecretRef) Account() string {
	switch ref {
	case ProviderAccountToken:
		return ProviderAccountTokenAccount
	case PlayerBasicAuthPassword:
		return PlayerPasswordAccount
	default:
		return ""
	}
}

type SecretPresence uint8

const (
	SecretAbsent SecretPresence = iota + 1
	SecretPresent
	SecretUnknown
)

func (presence SecretPresence) Valid() bool {
	return presence >= SecretAbsent && presence <= SecretUnknown
}

// SecretUse is passed only to a trusted scoped callback and is cleared immediately afterward.
type SecretUse struct {
	ProviderToken  []byte
	PlayerPassword []byte
}

func (use *SecretUse) Clear() {
	if use == nil {
		return
	}
	clear(use.ProviderToken)
	clear(use.PlayerPassword)
	use.ProviderToken = nil
	use.PlayerPassword = nil
}

// SecretStore deliberately has no Get, export, list, or string-returning method.
type SecretStore interface {
	Presence(context.Context, SecretRef) (SecretPresence, error)
	Replace(context.Context, SecretRef, []byte) error
	Delete(context.Context, SecretRef) error
	WithSecrets(context.Context, []SecretRef, func(*SecretUse) error) error
}

func ValidateProviderToken(value []byte) error {
	if len(value) == 0 || strings.ContainsAny(string(value), "\x00\r\n") {
		return errors.New("provider account token is invalid")
	}
	return nil
}

func ValidatePlayerPassword(value []byte) error {
	if !utf8.Valid(value) || utf8.RuneCount(value) < MinimumPlayerPasswordBytes || strings.ContainsAny(string(value), "\x00\r\n") {
		return errors.New("player password must contain at least eight characters and no newlines")
	}
	return nil
}

func GeneratePlayerPassword(source io.Reader) ([]byte, error) {
	if source == nil {
		source = rand.Reader
	}
	entropy := make([]byte, GeneratedPasswordEntropyBytes)
	if _, err := io.ReadFull(source, entropy); err != nil {
		clear(entropy)
		return nil, errors.New("generate player password: cryptographic random source unavailable")
	}
	generated := make([]byte, base64.RawURLEncoding.EncodedLen(len(entropy)))
	base64.RawURLEncoding.Encode(generated, entropy)
	clear(entropy)
	return generated, nil
}

func ReplaceSecret(ctx context.Context, store SecretStore, ref SecretRef, value []byte) error {
	if store == nil {
		return ErrSecretStoreUnavailable
	}
	if !ref.Valid() {
		return errors.New("invalid secret reference")
	}
	var validationErr error
	switch ref {
	case ProviderAccountToken:
		validationErr = ValidateProviderToken(value)
	case PlayerBasicAuthPassword:
		validationErr = ValidatePlayerPassword(value)
	}
	if validationErr != nil {
		return validationErr
	}
	temporary := append([]byte(nil), value...)
	defer clear(temporary)
	if err := store.Replace(ctx, ref, temporary); err != nil {
		return redactSecretStoreError(err)
	}
	return nil
}

func DeleteSecret(ctx context.Context, store SecretStore, ref SecretRef) error {
	if store == nil {
		return ErrSecretStoreUnavailable
	}
	if !ref.Valid() {
		return errors.New("invalid secret reference")
	}
	if err := store.Delete(ctx, ref); err != nil {
		return redactSecretStoreError(err)
	}
	return nil
}

func WithPublicAccessSecrets(ctx context.Context, store SecretStore, callback func(*SecretUse) error) error {
	if store == nil || callback == nil {
		return ErrSecretStoreUnavailable
	}
	var callbackErr error
	err := store.WithSecrets(ctx, []SecretRef{ProviderAccountToken, PlayerBasicAuthPassword}, func(use *SecretUse) error {
		if use == nil {
			return ErrSecretStoreUnavailable
		}
		defer use.Clear()
		if validationErr := ValidateProviderToken(use.ProviderToken); validationErr != nil {
			return errors.New("stored provider credential is invalid")
		}
		if validationErr := ValidatePlayerPassword(use.PlayerPassword); validationErr != nil {
			return errors.New("stored player credential is invalid")
		}
		callbackErr = callback(use)
		return callbackErr
	})
	if callbackErr != nil && errors.Is(err, callbackErr) {
		return callbackErr
	}
	if err != nil {
		return redactSecretStoreError(err)
	}
	return nil
}

// WithPlayerPassword provides bounded access to the saved player password
// without requiring or exposing the provider credential.
func WithPlayerPassword(ctx context.Context, store SecretStore, callback func([]byte) error) error {
	if store == nil || callback == nil {
		return ErrSecretStoreUnavailable
	}
	var callbackErr error
	err := store.WithSecrets(ctx, []SecretRef{PlayerBasicAuthPassword}, func(use *SecretUse) error {
		if use == nil {
			return ErrSecretStoreUnavailable
		}
		defer use.Clear()
		if validationErr := ValidatePlayerPassword(use.PlayerPassword); validationErr != nil {
			return errors.New("stored player credential is invalid")
		}
		callbackErr = callback(use.PlayerPassword)
		return callbackErr
	})
	if callbackErr != nil && errors.Is(err, callbackErr) {
		return callbackErr
	}
	if err != nil {
		return redactSecretStoreError(err)
	}
	return nil
}

func redactSecretStoreError(err error) error {
	switch {
	case errors.Is(err, ErrSecretStoreLocked):
		return ErrSecretStoreLocked
	case errors.Is(err, ErrSecretStoreDenied):
		return ErrSecretStoreDenied
	case errors.Is(err, ErrSecretStoreUserCancelled):
		return ErrSecretStoreUserCancelled
	default:
		return ErrSecretStoreUnavailable
	}
}
