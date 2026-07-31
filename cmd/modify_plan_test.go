package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadModifyPlan_StrictJSON(t *testing.T) {
	t.Run("reads configured input", func(t *testing.T) {
		plan, err := readModifyPlan(strings.NewReader(`{"branches":[{"name":"one"}]}`), "-")

		require.NoError(t, err)
		require.Len(t, plan.Branches, 1)
		assert.Equal(t, "one", plan.Branches[0].Name)
	})

	t.Run("rejects unknown fields", func(t *testing.T) {
		_, err := readModifyPlan(strings.NewReader(`{"branches":[{"name":"one","acton":"drop"}]}`), "-")

		assert.ErrorContains(t, err, "unknown field")
	})
}
