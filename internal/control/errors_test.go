package control

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFacilityFailureErrorRoundTrip(t *testing.T) {
	t.Parallel()

	codes := []domain.FacilityFailureCode{
		domain.FacilityFailureMissingReference,
		domain.FacilityFailureInvalidTransition,
		domain.FacilityFailurePreconditionFailed,
		domain.FacilityFailureStaleRevision,
		domain.FacilityFailureConflict,
		domain.FacilityFailureDuplicate,
		domain.FacilityFailureInvalidConfiguration,
		domain.FacilityFailurePersistenceFailed,
		domain.FacilityFailureRuntimeContextEnded,
	}
	for _, code := range codes {
		t.Run(string(code), func(t *testing.T) {
			err := facilityFailureError(code)
			require.Error(t, err)
			assert.Equal(t, code, facilityFailureCode(fmt.Errorf("approval boundary: %w", err)))
		})
	}
}

func TestFacilityFailureCodeUsesTypedIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want domain.FacilityFailureCode
	}{
		{name: "nil", want: domain.FacilityFailureUnspecified},
		{name: "legacy stale", err: ErrCommandExecutionStale, want: domain.FacilityFailureStaleRevision},
		{name: "legacy persistence", err: ErrCommandExecutionPersistence, want: domain.FacilityFailurePersistenceFailed},
		{name: "canceled context", err: context.Canceled, want: domain.FacilityFailureRuntimeContextEnded},
		{name: "expired context", err: context.DeadlineExceeded, want: domain.FacilityFailureRuntimeContextEnded},
		{
			name: "matching text without identity",
			err:  errors.New(ErrFacilityConflict.Error()),
			want: domain.FacilityFailureUnspecified,
		},
		{name: "unknown", err: errors.New("private storage detail"), want: domain.FacilityFailureUnspecified},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, facilityFailureCode(test.err))
		})
	}
}

func TestFacilityFailureErrorTreatsNonFailuresAsSuccess(t *testing.T) {
	t.Parallel()

	assert.NoError(t, facilityFailureError(domain.FacilityFailureUnspecified))
	assert.NoError(t, facilityFailureError(domain.FacilityFailureRejected))
	assert.NoError(t, facilityFailureError(""))
	assert.ErrorIs(t, facilityFailureError(domain.FacilityFailureCode("future-code")), ErrFacilityPersistence)
}
