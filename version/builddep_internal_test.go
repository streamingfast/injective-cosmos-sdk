package version

import (
	"encoding/json"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildDepJSONRoundTrip(t *testing.T) {
	deps := []buildDep{
		{Module: &debug.Module{Path: "github.com/a/b", Version: "v1.2.3"}},
		{Module: &debug.Module{
			Path:    "github.com/cometbft/cometbft",
			Version: "v1.0.1",
			Replace: &debug.Module{Path: "github.com/InjectiveLabs/cometbft", Version: "v1.0.1-inj.9"},
		}},
	}

	bz, err := json.Marshal(deps)
	require.NoError(t, err)
	require.JSONEq(t, `["github.com/a/b@v1.2.3","github.com/cometbft/cometbft@v1.0.1 => github.com/InjectiveLabs/cometbft@v1.0.1-inj.9"]`, string(bz))

	var got []buildDep
	require.NoError(t, json.Unmarshal(bz, &got))
	require.Equal(t, deps, got)
}
