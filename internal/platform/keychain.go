package platform

import (
	"context"
	"errors"

	"github.com/obalunenko/Fallout-Terminal/internal/tunnel"
)

const credentialServiceBase = "com.vaulttec.fallout-terminal"

var (
	errCredentialNotFound         = errors.New("credential item not found")
	errCredentialLocked           = errors.New("credential store locked")
	errCredentialDenied           = errors.New("credential store access denied")
	errCredentialUnavailable      = errors.New("credential store unavailable")
	errCredentialUserCancelled    = errors.New("credential store interaction cancelled")
	errCredentialContextRequired  = errors.New("credential store context is required")
	errCredentialReferenceInvalid = errors.New("invalid secret reference")
)

type credentialBackend interface {
	Presence(context.Context, string, string) (bool, error)
	Update(context.Context, string, string, []byte) error
	Add(context.Context, string, string, []byte) error
	Delete(context.Context, string, string) error
	Read(context.Context, string, string) ([]byte, error)
}

// SecureCredentialStore adapts an operating-system credential provider to the
// platform-neutral tunnel.SecretStore contract. KeychainSecretStore remains an
// alias for source compatibility with the original Darwin-only implementation.
type SecureCredentialStore struct {
	service string
	backend credentialBackend
}

// KeychainSecretStore is the compatibility name for SecureCredentialStore.
type KeychainSecretStore = SecureCredentialStore

// CredentialServiceName returns the stable native-store service namespace.
func CredentialServiceName(production bool) string {
	if production {
		return credentialServiceBase + ".public-access"
	}
	return credentialServiceBase + ".dev.public-access"
}

func KeychainServiceName(production bool) string {
	return CredentialServiceName(production)
}

func KeychainServiceNameForSigning(production bool, _ string) string {
	return KeychainServiceName(production)
}

func NewKeychainSecretStore(production bool, backend credentialBackend) *KeychainSecretStore {
	return newSecureCredentialStore(production, backend)
}

func newSecureCredentialStore(production bool, backend credentialBackend) *SecureCredentialStore {
	if backend == nil {
		backend = defaultCredentialBackend()
	}
	return &SecureCredentialStore{service: CredentialServiceName(production), backend: backend}
}

func NewPlatformKeychainSecretStore(production bool) tunnel.SecretStore {
	return NewPlatformSecureCredentialStore(production)
}

// NewPlatformSecureCredentialStore selects the native credential provider for
// the current operating system.
func NewPlatformSecureCredentialStore(production bool) tunnel.SecretStore {
	return newSecureCredentialStore(production, nil)
}

func (store *SecureCredentialStore) Presence(ctx context.Context, ref tunnel.SecretRef) (tunnel.SecretPresence, error) {
	if err := credentialContextError(ctx); err != nil {
		return tunnel.SecretUnknown, err
	}
	account, err := credentialAccount(ref)
	if err != nil {
		return tunnel.SecretUnknown, err
	}
	if store == nil || store.backend == nil {
		return tunnel.SecretUnknown, tunnel.ErrSecretStoreUnavailable
	}
	present, err := store.backend.Presence(ctx, store.service, account)
	if err != nil {
		return tunnel.SecretUnknown, mapCredentialError(err)
	}
	if present {
		return tunnel.SecretPresent, nil
	}
	return tunnel.SecretAbsent, nil
}

func (store *SecureCredentialStore) Replace(ctx context.Context, ref tunnel.SecretRef, value []byte) error {
	if err := credentialContextError(ctx); err != nil {
		return err
	}
	account, err := credentialAccount(ref)
	if err != nil {
		return err
	}
	if store == nil || store.backend == nil {
		return tunnel.ErrSecretStoreUnavailable
	}
	temporary := append([]byte(nil), value...)
	defer clear(temporary)
	err = store.backend.Update(ctx, store.service, account, temporary)
	if errors.Is(err, errCredentialNotFound) {
		err = store.backend.Add(ctx, store.service, account, temporary)
	}
	if err != nil {
		return mapCredentialError(err)
	}
	return nil
}

func (store *SecureCredentialStore) Delete(ctx context.Context, ref tunnel.SecretRef) error {
	if err := credentialContextError(ctx); err != nil {
		return err
	}
	account, err := credentialAccount(ref)
	if err != nil {
		return err
	}
	if store == nil || store.backend == nil {
		return tunnel.ErrSecretStoreUnavailable
	}
	err = store.backend.Delete(ctx, store.service, account)
	if errors.Is(err, errCredentialNotFound) {
		return nil
	}
	if err != nil {
		return mapCredentialError(err)
	}
	return nil
}

func (store *SecureCredentialStore) WithSecrets(ctx context.Context, refs []tunnel.SecretRef, callback func(*tunnel.SecretUse) error) error {
	if err := credentialContextError(ctx); err != nil {
		return err
	}
	if store == nil || store.backend == nil || callback == nil {
		return tunnel.ErrSecretStoreUnavailable
	}
	use := &tunnel.SecretUse{}
	defer use.Clear()
	for _, ref := range refs {
		account, err := credentialAccount(ref)
		if err != nil {
			return err
		}
		value, err := store.backend.Read(ctx, store.service, account)
		if err != nil {
			clear(value)
			return mapCredentialError(err)
		}
		switch ref {
		case tunnel.ProviderAccountToken:
			clear(use.ProviderToken)
			use.ProviderToken = value
		case tunnel.PlayerBasicAuthPassword:
			clear(use.PlayerPassword)
			use.PlayerPassword = value
		}
	}
	return callback(use)
}

func credentialContextError(ctx context.Context) error {
	if ctx == nil {
		return errCredentialContextRequired
	}
	return ctx.Err()
}

func credentialAccount(ref tunnel.SecretRef) (string, error) {
	switch ref {
	case tunnel.ProviderAccountToken:
		return tunnel.ProviderAccountTokenAccount, nil
	case tunnel.PlayerBasicAuthPassword:
		return tunnel.PlayerPasswordAccount, nil
	default:
		return "", errCredentialReferenceInvalid
	}
}

func mapCredentialError(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, errCredentialNotFound):
		return tunnel.ErrSecretStoreUnavailable
	case errors.Is(err, errCredentialLocked):
		return tunnel.ErrSecretStoreLocked
	case errors.Is(err, errCredentialDenied):
		return tunnel.ErrSecretStoreDenied
	case errors.Is(err, errCredentialUserCancelled):
		return tunnel.ErrSecretStoreUserCancelled
	default:
		return tunnel.ErrSecretStoreUnavailable
	}
}

var _ tunnel.SecretStore = (*SecureCredentialStore)(nil)
