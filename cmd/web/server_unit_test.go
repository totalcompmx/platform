package main

import (
	"testing"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
)

func TestServerConfiguration(t *testing.T) {
	t.Run("Default timeouts are reasonable", func(t *testing.T) {
		assert.True(t, defaultIdleTimeout > 0)
		assert.True(t, defaultReadTimeout > 0)
		assert.True(t, defaultWriteTimeout > defaultReadTimeout)

		if defaultShutdownPeriod <= defaultWriteTimeout {
			t.Errorf("default shutdown period %s must be greater than default write timeout %s", defaultShutdownPeriod, defaultWriteTimeout)
		}
	})
}
