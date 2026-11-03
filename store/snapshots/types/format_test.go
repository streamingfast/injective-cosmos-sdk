package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/store/snapshots/types"
)

func TestIsRestorableFormat(t *testing.T) {
	require.EqualValues(t, 4, types.CurrentFormat)
	require.True(t, types.IsRestorableFormat(types.CurrentFormat))
	// Format 3 (Go <= 1.26 zlib output) restores identically.
	require.True(t, types.IsRestorableFormat(3))
	for _, f := range []uint32{0, 1, 2, 5, 100} {
		require.False(t, types.IsRestorableFormat(f), "format %d", f)
	}
}
