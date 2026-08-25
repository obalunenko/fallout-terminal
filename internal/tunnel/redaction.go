package tunnel

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
)

const maximumPublicAccessDiagnosticBytes = 512

type publicAccessCategorizedError struct {
	category       ErrorCategory
	providerCode   string
	diagnosticCode PublicAccessDiagnosticCode
}

func (failure publicAccessCategorizedError) Error() string {
	return safePublicAccessMessage(failure.category, failure.providerCode)
}

func (failure publicAccessCategorizedError) PublicAccessCategory() ErrorCategory {
	return failure.category
}

func (failure publicAccessCategorizedError) Code() string {
	return failure.providerCode
}

func (failure publicAccessCategorizedError) DiagnosticCode() PublicAccessDiagnosticCode {
	return failure.diagnosticCode
}

type publicAccessErrorCategory interface {
	PublicAccessCategory() ErrorCategory
}

type providerErrorCode interface {
	Code() string
}

type publicAccessDiagnosticCode interface {
	DiagnosticCode() PublicAccessDiagnosticCode
}

func newRedactedPublicAccessError(err error) error {
	if err == nil {
		return nil
	}
	category, _ := redactedPublicAccessFailure(err)
	return publicAccessCategorizedError{category: category, providerCode: safeProviderErrorCode(err)}
}

func safePublicAccessDiagnosticCode(err error) PublicAccessDiagnosticCode {
	var diagnostic publicAccessDiagnosticCode
	if !errors.As(err, &diagnostic) {
		return 0
	}
	code := diagnostic.DiagnosticCode()
	if !code.Valid() {
		return 0
	}
	return code
}

// redactedPublicAccessFailure returns only a stable category and its fixed
// corrective message. Raw SDK, network, account, domain, and credential text
// is neither copied nor retained.
func redactedPublicAccessFailure(err error) (ErrorCategory, string) {
	category := ErrorProviderFailure
	providerCode := safeProviderErrorCode(err)
	var categorized publicAccessErrorCategory
	var coded providerErrorCode
	var networkError net.Error
	switch {
	case errors.As(err, &categorized) && categorized.PublicAccessCategory().Valid():
		category = categorized.PublicAccessCategory()
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		category = ErrorTimeout
	case errors.Is(err, ErrSecretStoreLocked):
		category = ErrorSecretStoreLocked
	case errors.Is(err, ErrSecretStoreDenied), errors.Is(err, ErrSecretStoreUserCancelled):
		category = ErrorSecretStoreDenied
	case errors.Is(err, ErrSecretStoreUnavailable):
		category = ErrorSecretStoreUnavailable
	case errors.As(err, &coded):
		category = ngrokErrorCategory(coded.Code())
	case errors.As(err, &networkError):
		category = ErrorNetworkUnavailable
	}
	return category, safePublicAccessMessage(category, providerCode)
}

func ngrokErrorCategory(code string) ErrorCategory {
	switch code {
	case "ERR_NGROK_102", "ERR_NGROK_103", "ERR_NGROK_105", "ERR_NGROK_106", "ERR_NGROK_107", "ERR_NGROK_108", "ERR_NGROK_109", "ERR_NGROK_115", "ERR_NGROK_116", "ERR_NGROK_118", "ERR_NGROK_121", "ERR_NGROK_122", "ERR_NGROK_123", "ERR_NGROK_127", "ERR_NGROK_200", "ERR_NGROK_201", "ERR_NGROK_202", "ERR_NGROK_203", "ERR_NGROK_300":
		return ErrorProviderAuthentication
	case "ERR_NGROK_307", "ERR_NGROK_308", "ERR_NGROK_309", "ERR_NGROK_310", "ERR_NGROK_311", "ERR_NGROK_312", "ERR_NGROK_313", "ERR_NGROK_314", "ERR_NGROK_315", "ERR_NGROK_316", "ERR_NGROK_317", "ERR_NGROK_318", "ERR_NGROK_319", "ERR_NGROK_320", "ERR_NGROK_321", "ERR_NGROK_322", "ERR_NGROK_401", "ERR_NGROK_415", "ERR_NGROK_417", "ERR_NGROK_430":
		return ErrorDomainUnavailable
	case "ERR_NGROK_2257", "ERR_NGROK_2258", "ERR_NGROK_2261", "ERR_NGROK_9025", "ERR_NGROK_9026", "ERR_NGROK_15008", "ERR_NGROK_15009", "ERR_NGROK_15011":
		return ErrorValidation
	default:
		if number, ok := ngrokErrorNumber(code); ok && number >= 8000 && number <= 8010 {
			return ErrorNetworkUnavailable
		}
		return ErrorProviderFailure
	}
}

func safeProviderErrorCode(err error) string {
	var coded providerErrorCode
	if !errors.As(err, &coded) {
		return ""
	}
	code := coded.Code()
	if _, ok := ngrokErrorNumber(code); !ok {
		return ""
	}
	return code
}

func ngrokErrorNumber(code string) (int, bool) {
	const prefix = "ERR_NGROK_"
	if !strings.HasPrefix(code, prefix) || len(code) <= len(prefix) || len(code) > len(prefix)+8 {
		return 0, false
	}
	for _, character := range code[len(prefix):] {
		if character < '0' || character > '9' {
			return 0, false
		}
	}
	number, err := strconv.Atoi(code[len(prefix):])
	return number, err == nil
}

func safePublicAccessMessage(category ErrorCategory, providerCode string) string {
	message := category.SafeMessage()
	if _, ok := ngrokErrorNumber(providerCode); !ok {
		return message
	}
	return strings.TrimSuffix(message, ".") + " (" + providerCode + ")."
}
