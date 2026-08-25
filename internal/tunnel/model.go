package tunnel

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
)

const (
	PublicAccessSettingsVersion uint32 = 1
	DefaultPlayerUsername              = "players"
)

var dnsLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// PublicAccessPreferences is the native, reusable, secret-free settings model.
type PublicAccessPreferences struct {
	Version                   uint32
	EnabledPreference         bool
	ReservedDomain            string
	Username                  string
	ProviderTokenPresentHint  bool
	PlayerPasswordPresentHint bool
	Revision                  uint64
}

func DefaultPublicAccessPreferences() PublicAccessPreferences {
	return PublicAccessPreferences{Version: PublicAccessSettingsVersion, Username: DefaultPlayerUsername}
}

func (preferences PublicAccessPreferences) Normalized() (PublicAccessPreferences, error) {
	if preferences.Version != PublicAccessSettingsVersion {
		return PublicAccessPreferences{}, errors.New("unsupported public-access settings version")
	}
	preferences.Username = strings.TrimSpace(preferences.Username)
	if preferences.Username == "" || strings.ContainsAny(preferences.Username, "\r\n") {
		return PublicAccessPreferences{}, errors.New("public-access username is invalid")
	}
	domain, err := NormalizeReservedDomain(preferences.ReservedDomain)
	if err != nil {
		return PublicAccessPreferences{}, err
	}
	preferences.ReservedDomain = domain
	return preferences, nil
}

func NormalizeReservedDomain(raw string) (string, error) {
	domain := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
	if domain == "" {
		return "", nil
	}
	if len(domain) > 253 || strings.ContainsAny(domain, "/:@?#[]%\\\r\n\t ") || net.ParseIP(domain) != nil {
		return "", errors.New("reserved domain is invalid")
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", errors.New("reserved domain is invalid")
	}
	for _, label := range labels {
		if !dnsLabelPattern.MatchString(label) {
			return "", errors.New("reserved domain is invalid")
		}
	}
	return domain, nil
}

type LifecycleState uint8

const (
	LifecycleDisabled LifecycleState = iota + 1
	LifecycleStarting
	LifecycleReady
	LifecycleStopping
	LifecycleFailed
)

func (state LifecycleState) Valid() bool {
	return state >= LifecycleDisabled && state <= LifecycleFailed
}

type ErrorCategory uint8

const (
	ErrorValidation ErrorCategory = iota + 1
	ErrorSettingsCorrupt
	ErrorSecretStoreLocked
	ErrorSecretStoreDenied
	ErrorSecretStoreUnavailable
	ErrorCredentialMissing
	ErrorProviderAuthentication
	ErrorDomainUnavailable
	ErrorNetworkUnavailable
	ErrorTimeout
	ErrorProviderFailure
	ErrorShutdownTimeout
	ErrorConflict
)

type PublicAccessDiagnosticCode uint8

const (
	DiagnosticPublicIngressListenFailed PublicAccessDiagnosticCode = iota + 1
)

func (code PublicAccessDiagnosticCode) Valid() bool {
	return code == DiagnosticPublicIngressListenFailed
}

func (code PublicAccessDiagnosticCode) String() string {
	if code == DiagnosticPublicIngressListenFailed {
		return "public_ingress_listen_failed"
	}
	return ""
}

func (category ErrorCategory) Valid() bool {
	return category >= ErrorValidation && category <= ErrorConflict
}

func (category ErrorCategory) SafeMessage() string {
	switch category {
	case ErrorValidation:
		return "Check the public-access settings and try again."
	case ErrorSettingsCorrupt:
		return "Saved public-access settings were reset safely."
	case ErrorSecretStoreLocked:
		return "Unlock Keychain and try again."
	case ErrorSecretStoreDenied:
		return "Allow Keychain access and try again."
	case ErrorSecretStoreUnavailable:
		return "Keychain is unavailable; local access remains available."
	case ErrorCredentialMissing:
		return "Add the required credentials before starting public access."
	case ErrorProviderAuthentication:
		return "The provider rejected the account credential."
	case ErrorDomainUnavailable:
		return "The reserved domain is unavailable for this account."
	case ErrorNetworkUnavailable:
		return "The network is unavailable; local access remains available."
	case ErrorTimeout:
		return "Public access did not become ready in time."
	case ErrorProviderFailure:
		return "The public-access provider stopped unexpectedly."
	case ErrorShutdownTimeout:
		return "Public-access cleanup exceeded the shutdown deadline."
	case ErrorConflict:
		return "Settings changed; reload the current public-access state."
	default:
		return "Public access is unavailable."
	}
}

type PublicAccessStatus struct {
	State            LifecycleState
	Generation       uint64
	SettingsRevision uint64
	PublicURL        string
	ErrorCategory    ErrorCategory
	ErrorMessage     string
}

func (status PublicAccessStatus) Validate() error {
	if !status.State.Valid() {
		return errors.New("public-access lifecycle state is invalid")
	}
	if status.State == LifecycleReady {
		if status.PublicURL == "" {
			return errors.New("ready public access requires a public URL")
		}
		if _, _, err := NormalizeEndpointURL(status.PublicURL, ""); err != nil {
			return errors.New("ready public access has an invalid public URL")
		}
	} else if status.PublicURL != "" {
		return errors.New("public URL is forbidden outside ready state")
	}
	if status.State == LifecycleFailed {
		if !status.ErrorCategory.Valid() {
			return errors.New("failed public access requires an error category")
		}
	} else if status.ErrorCategory != 0 || status.ErrorMessage != "" {
		return errors.New("public-access error is forbidden outside failed state")
	}
	return nil
}

type PublicAccessSnapshot struct {
	Preferences            PublicAccessPreferences
	ProviderTokenPresence  SecretPresence
	PlayerPasswordPresence SecretPresence
	Status                 PublicAccessStatus
}

type IntentVersion struct {
	Generation       uint64
	SettingsRevision uint64
}

func (version IntentVersion) Matches(other IntentVersion) bool {
	return version == other
}

func (version IntentVersion) NewerThan(other IntentVersion) bool {
	return version.Generation > other.Generation
}

func NormalizeEndpointURL(raw, reservedDomain string) (canonicalURL string, host string, err error) {
	parsed, parseErr := url.Parse(raw)
	if parseErr != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.EscapedPath() != "" && parsed.EscapedPath() != "/") {
		return "", "", errors.New("provider returned an invalid HTTPS URL")
	}
	if parsed.Port() != "" || parsed.Host != parsed.Hostname() && parsed.Host != parsed.Hostname()+"." {
		return "", "", errors.New("provider returned an invalid HTTPS authority")
	}
	host, err = NormalizeReservedDomain(parsed.Hostname())
	if err != nil {
		return "", "", errors.New("provider returned an invalid HTTPS host")
	}
	if reservedDomain != "" {
		expected, normalizeErr := NormalizeReservedDomain(reservedDomain)
		if normalizeErr != nil || host != expected {
			return "", "", errors.New("provider did not return the requested reserved domain")
		}
	}
	return (&url.URL{Scheme: "https", Host: host}).String(), host, nil
}
