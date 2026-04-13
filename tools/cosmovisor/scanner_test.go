package cosmovisor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	upgradetypes "cosmossdk.io/x/upgrade/types"
)

func TestParseUpgradeInfoFile(t *testing.T) {
	cases := []struct {
		filename      string
		expectUpgrade upgradetypes.Plan
		expectErr     bool
	}{
		{
			filename:      "f1-good.json",
			expectUpgrade: upgradetypes.Plan{Name: "upgrade1", Info: "some info", Height: 123},
			expectErr:     false,
		},
		{
			filename:      "f2-normalized-name.json",
			expectUpgrade: upgradetypes.Plan{Name: "upgrade2", Info: "some info", Height: 125},
			expectErr:     false,
		},
		{
			filename:      "f2-bad-type.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
		{
			filename:      "f2-bad-type-2.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
		{
			filename:      "f3-empty.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
		{
			filename:      "f4-empty-obj.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
		{
			filename:      "f5-partial-obj-1.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
		{
			filename:      "f5-partial-obj-2.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
		{
			filename:      "unknown.json",
			expectUpgrade: upgradetypes.Plan{},
			expectErr:     true,
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.filename, func(t *testing.T) {
			require := require.New(t)
			ui, err := parseUpgradeInfoFile(filepath.Join(".", "testdata", "upgrade-files", tc.filename))
			if tc.expectErr {
				require.Error(err)
			} else {
				require.NoError(err)
				require.Equal(tc.expectUpgrade, ui)
			}
		})
	}
}

func TestFileWatcherCheckUpdateRetriesAfterTransientParseError(t *testing.T) {
	home := t.TempDir()
	filename := filepath.Join(home, "upgrade-info.json")

	fw, err := newUpgradeFileWatcher(log.NewTestLogger(t), filename, time.Millisecond)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filename, []byte(`{"name":"upgrade1"`), 0o644))
	require.False(t, fw.CheckUpdate(upgradetypes.Plan{}))
	require.False(t, fw.initialized)
	require.True(t, fw.lastModTime.IsZero())

	require.NoError(t, os.WriteFile(filename, []byte(`{"name":"upgrade1","height":123,"info":"some info"}`), 0o644))
	require.True(t, fw.CheckUpdate(upgradetypes.Plan{}))
	require.True(t, fw.initialized)
	require.Equal(t, upgradetypes.Plan{Name: "upgrade1", Height: 123, Info: "some info"}, fw.currentInfo)
}
