package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCurrentDefaultsToExplicitDevelopmentIdentity(t *testing.T) {
	assert.Equal(t, "development", Current())
	assert.False(t, IsRelease(), "the development default must never be treated as a tagged release")
}

func TestCurrentReturnsLinkerInjectedReleaseIdentity(t *testing.T) {
	original := value
	t.Cleanup(func() {
		value = original
	})

	value = "2.0.0-rc.1"

	assert.Equal(t, "2.0.0-rc.1", Current())
	assert.True(t, IsRelease())
}
