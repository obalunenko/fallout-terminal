package control

import (
	"context"
	"errors"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
)

var (
	// ErrCommandExecutionStale identifies a decision that no longer matches
	// the current pending command execution.
	ErrCommandExecutionStale = errors.New("command execution decision is stale")
	// ErrCommandExecutionPersistence identifies a failure to prove and install
	// the durable result of a state-changing command.
	ErrCommandExecutionPersistence = errors.New("command execution could not be persisted")

	// ErrFacilityMissingReference identifies a facility action whose stable
	// reference is absent from the current authored facility.
	ErrFacilityMissingReference = facilityError{
		code: domain.FacilityFailureMissingReference,
	}
	// ErrFacilityInvalidTransition identifies a transition that cannot be
	// applied to its requested device.
	ErrFacilityInvalidTransition = facilityError{
		code: domain.FacilityFailureInvalidTransition,
	}
	// ErrFacilityPreconditionFailed identifies an authored equality that is
	// false in the authoritative facility pre-state.
	ErrFacilityPreconditionFailed = facilityError{
		code: domain.FacilityFailurePreconditionFailed,
	}
	// ErrFacilityStaleRevision identifies an approval made against an older
	// facility revision.
	ErrFacilityStaleRevision = facilityError{
		code: domain.FacilityFailureStaleRevision,
	}
	// ErrFacilityConflict identifies a pending action that no longer matches
	// the current authored action.
	ErrFacilityConflict = facilityError{
		code: domain.FacilityFailureConflict,
	}
	// ErrFacilityDuplicate identifies an already resolved facility request.
	ErrFacilityDuplicate = facilityError{
		code: domain.FacilityFailureDuplicate,
	}
	// ErrFacilityInvalidConfiguration identifies a facility action that cannot
	// be resolved into one bounded world action.
	ErrFacilityInvalidConfiguration = facilityError{
		code: domain.FacilityFailureInvalidConfiguration,
	}
	// ErrFacilityPersistence identifies a failure to prove and install the
	// durable result of a facility action.
	ErrFacilityPersistence = facilityError{
		code: domain.FacilityFailurePersistenceFailed,
	}
	// ErrFacilityRuntimeContextEnded identifies an action canceled before the
	// durable owner returned an authoritative result.
	ErrFacilityRuntimeContextEnded = facilityError{
		code: domain.FacilityFailureRuntimeContextEnded,
	}
)

type facilityError struct {
	code domain.FacilityFailureCode
}

func (err facilityError) Error() string {
	return "facility operation failed: " + string(err.code)
}

// facilityFailureCode returns the stable outcome for a typed coordinator
// error. Unknown errors intentionally remain unspecified so callers cannot
// infer outcomes from private error text.
func facilityFailureCode(err error) domain.FacilityFailureCode {
	if err == nil {
		return domain.FacilityFailureUnspecified
	}
	if failure, ok := errors.AsType[facilityError](err); ok {
		return failure.code
	}
	switch {
	case errors.Is(err, ErrCommandExecutionStale):
		return domain.FacilityFailureStaleRevision
	case errors.Is(err, ErrCommandExecutionPersistence):
		return domain.FacilityFailurePersistenceFailed
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return domain.FacilityFailureRuntimeContextEnded
	default:
		return domain.FacilityFailureUnspecified
	}
}

// facilityFailureError returns the stable coordinator error for a structured
// facility failure. Rejection is a successful approval decision with an
// explicit result, so it does not produce an error.
func facilityFailureError(code domain.FacilityFailureCode) error {
	switch code {
	case "", domain.FacilityFailureUnspecified, domain.FacilityFailureRejected:
		return nil
	case domain.FacilityFailureMissingReference:
		return ErrFacilityMissingReference
	case domain.FacilityFailureInvalidTransition:
		return ErrFacilityInvalidTransition
	case domain.FacilityFailurePreconditionFailed:
		return ErrFacilityPreconditionFailed
	case domain.FacilityFailureStaleRevision:
		return ErrFacilityStaleRevision
	case domain.FacilityFailureConflict:
		return ErrFacilityConflict
	case domain.FacilityFailureDuplicate:
		return ErrFacilityDuplicate
	case domain.FacilityFailureInvalidConfiguration:
		return ErrFacilityInvalidConfiguration
	case domain.FacilityFailurePersistenceFailed:
		return ErrFacilityPersistence
	case domain.FacilityFailureRuntimeContextEnded:
		return ErrFacilityRuntimeContextEnded
	default:
		return ErrFacilityPersistence
	}
}

type commandExecutionFailure struct {
	message  string
	identity error
}

func (failure commandExecutionFailure) Error() string {
	return failure.message
}

func (failure commandExecutionFailure) Unwrap() error {
	return failure.identity
}

func commandExecutionPersistenceFailure(message string) error {
	if message == ErrCommandExecutionPersistence.Error() {
		return ErrCommandExecutionPersistence
	}
	return commandExecutionFailure{message: message, identity: ErrCommandExecutionPersistence}
}
