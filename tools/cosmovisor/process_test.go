//go:build linux
// +build linux

package cosmovisor_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/log"
	"cosmossdk.io/tools/cosmovisor"
	upgradetypes "cosmossdk.io/x/upgrade/types"
)

type processTestSuite struct {
	suite.Suite
}

func TestProcessTestSuite(t *testing.T) {
	suite.Run(t, new(processTestSuite))
}

// TestLaunchProcess will try running the script a few times and watch upgrades work properly
// and args are passed through
func (s *processTestSuite) TestLaunchProcess() {
	// binaries from testdata/validate directory
	require := s.Require()
	home := copyTestData(s.T(), "validate")
	cfg := &cosmovisor.Config{Home: home, Name: "dummyd", PollInterval: 20, UnsafeSkipBackup: true}
	logger := log.NewTestLogger(s.T()).With(log.ModuleKey, "cosmosvisor")

	// should run the genesis binary and produce expected output
	stdout, stderr := newBuffer(), newBuffer()
	currentBin, err := cfg.CurrentBin()
	require.NoError(err)
	require.Equal(cfg.GenesisBin(), currentBin)

	launcher, err := cosmovisor.NewLauncher(logger, cfg)
	require.NoError(err)

	upgradeFile := cfg.UpgradeInfoFilePath()

	args := []string{"foo", "bar", "1234", upgradeFile}
	doUpgrade, err := launcher.Run(args, stdout, stderr)
	require.NoError(err)
	require.True(doUpgrade)
	require.Equal("", stderr.String())
	require.Equal(fmt.Sprintf("Genesis foo bar 1234 %s\nUPGRADE \"chain2\" NEEDED at height: 49: {}\n", upgradeFile), stdout.String())

	// ensure this is upgraded now and produces new output
	currentBin, err = cfg.CurrentBin()
	require.NoError(err)

	require.Equal(cfg.UpgradeBin("chain2"), currentBin)
	args = []string{"second", "run", "--verbose"}
	stdout.Reset()
	stderr.Reset()

	doUpgrade, err = launcher.Run(args, stdout, stderr)
	require.NoError(err)
	require.False(doUpgrade)
	require.Equal("", stderr.String())
	require.Equal("Chain 2 is live!\nArgs: second run --verbose\nFinished successfully\n", stdout.String())

	// ended without other upgrade
	require.Equal(cfg.UpgradeBin("chain2"), currentBin)
}

func (s *processTestSuite) TestLaunchProcessWithRestartDelay() {
	// binaries from testdata/validate directory
	require := s.Require()
	home := copyTestData(s.T(), "validate")
	cfg := &cosmovisor.Config{Home: home, Name: "dummyd", RestartDelay: 5 * time.Second, PollInterval: 20, UnsafeSkipBackup: true}
	logger := log.NewTestLogger(s.T()).With(log.ModuleKey, "cosmosvisor")

	// should run the genesis binary and produce expected output
	stdout, stderr := newBuffer(), newBuffer()
	currentBin, err := cfg.CurrentBin()
	require.NoError(err)
	require.Equal(cfg.GenesisBin(), currentBin)

	launcher, err := cosmovisor.NewLauncher(logger, cfg)
	require.NoError(err)

	upgradeFile := cfg.UpgradeInfoFilePath()

	start := time.Now()

	doUpgrade, err := launcher.Run([]string{"foo", "bar", "1234", upgradeFile}, stdout, stderr)
	require.NoError(err)
	require.True(doUpgrade)

	// may not be the best way but the fastest way to check we meet the delay
	// in addition to comparing both the runtime of this test and TestLaunchProcess in addition
	if time.Since(start) < cfg.RestartDelay {
		require.FailNow("restart delay not met")
	}
}

// TestLaunchProcess will try running the script a few times and watch upgrades work properly
// and args are passed through
func (s *processTestSuite) TestLaunchProcessWithDownloads() {
	// test case upgrade path (binaries from testdata/download directory):
	// genesis -> chain2-zip_bin
	// chain2-zip_bin -> ref_to_chain3-zip_dir.json = (json for the next download instructions) -> chain3-zip_dir
	// chain3-zip_dir - doesn't upgrade
	require := s.Require()
	home := copyTestData(s.T(), "download")
	cfg := &cosmovisor.Config{Home: home, Name: "autod", AllowDownloadBinaries: true, PollInterval: 100, UnsafeSkipBackup: true}
	logger := log.NewLogger(os.Stdout).With(log.ModuleKey, "cosmovisor")
	upgradeFilename := cfg.UpgradeInfoFilePath()
	localRefPath := rewriteLocalDownloadFixtures(s.T(), home, cfg)

	// should run the genesis binary and produce expected output
	currentBin, err := cfg.CurrentBin()
	require.NoError(err)
	require.Equal(cfg.GenesisBin(), currentBin)

	launcher, err := cosmovisor.NewLauncher(logger, cfg)
	require.NoError(err)

	stdout, stderr := newBuffer(), newBuffer()
	args := []string{"some", "args", upgradeFilename}
	doUpgrade, err := launcher.Run(args, stdout, stderr)

	require.NoError(err)
	require.True(doUpgrade)
	require.Equal("", stderr.String())
	require.Equal("Genesis autod. Args: some args "+upgradeFilename+"\n"+`ERROR: UPGRADE "chain2" NEEDED at height: 49: zip_binary`+"\n", stdout.String())
	currentBin, err = cfg.CurrentBin()
	require.NoError(err)
	require.Equal(cfg.UpgradeBin("chain2"), currentBin)
	rewriteDownloadedChain2Binary(s.T(), cfg.UpgradeBin("chain2"), localRefPath)

	// start chain2
	stdout.Reset()
	stderr.Reset()
	args = []string{"run", "--fast", upgradeFilename}
	doUpgrade, err = launcher.Run(args, stdout, stderr)
	require.NoError(err)

	require.Equal("", stderr.String())
	require.Equal("Chain 2 from zipped binary\nArgs: run --fast "+upgradeFilename+"\n"+`ERROR: UPGRADE "chain3" NEEDED at height: 936: ref_to_chain3-zip_dir.json module=main`+"\n", stdout.String())
	// ended with one more upgrade
	require.True(doUpgrade)
	currentBin, err = cfg.CurrentBin()
	require.NoError(err)
	require.Equal(cfg.UpgradeBin("chain3"), currentBin)

	// run the last chain
	args = []string{"end", "--halt", upgradeFilename}
	stdout.Reset()
	stderr.Reset()
	doUpgrade, err = launcher.Run(args, stdout, stderr)
	require.NoError(err)
	require.False(doUpgrade)
	require.Equal("", stderr.String())
	require.Equal("Chain 3 from zipped directory\nArgs: end --halt "+upgradeFilename+"\n", stdout.String())

	// and this doesn't upgrade
	currentBin, err = cfg.CurrentBin()
	require.NoError(err)
	require.Equal(cfg.UpgradeBin("chain3"), currentBin)
}

func rewriteLocalDownloadFixtures(t *testing.T, home string, cfg *cosmovisor.Config) string {
	t.Helper()

	chain2Zip := absTestRepoPath(t, "chain2-zip_bin", "autod.zip")
	chain3Zip := absTestRepoPath(t, "chain3-zip_dir", "autod.zip")

	localRefPath := filepath.Join(home, "ref_to_chain3-zip_dir.json")
	localRefContents := fmt.Sprintf(`{
  "binaries": {
    "linux/amd64": "%s?checksum=sha256:%s"
  }
}
`, chain3Zip, fileSHA256(t, chain3Zip))
	require.NoError(t, os.WriteFile(localRefPath, []byte(localRefContents), 0o644))

	genesisScript := fmt.Sprintf(`#!/bin/sh

echo Genesis autod. Args: $@
sleep 0.1
echo 'ERROR: UPGRADE "chain2" NEEDED at height: 49: zip_binary'

# create upgrade info
# this info contains directly information about binaries (in chain2->chain3 update we test with info containing a link to the file with an address for the new chain binary)
echo '{"name":"chain2","height":49,"info":"{\"binaries\":{\"linux/amd64\":\"%s?checksum=sha256:%s\"}}"}' >$3

sleep 0.1
echo Never should be printed!!!
`, chain2Zip, fileSHA256(t, chain2Zip))
	require.NoError(t, os.WriteFile(cfg.GenesisBin(), []byte(genesisScript), 0o755))

	return localRefPath
}

func rewriteDownloadedChain2Binary(t *testing.T, chain2BinPath, localRefPath string) {
	t.Helper()

	chain2Script := fmt.Sprintf(`#!/bin/sh

echo Chain 2 from zipped binary
echo Args: $@
# note that we just have a url (follow the ref), not a full link
echo 'ERROR: UPGRADE "chain3" NEEDED at height: 936: ref_to_chain3-zip_dir.json module=main'

# this update info doesn't contain binaries, instead it is a reference for further download instructions.
echo '{"name":"chain3","height":936,"info":"%s?checksum=sha256:%s"}' >$3

sleep 1
echo 'Do not print'
`, localRefPath, fileSHA256(t, localRefPath))
	require.NoError(t, os.WriteFile(chain2BinPath, []byte(chain2Script), 0o755))
}

func absTestRepoPath(t *testing.T, parts ...string) string {
	t.Helper()

	pathParts := append([]string{"testdata", "repo"}, parts...)
	path, err := filepath.Abs(filepath.Join(pathParts...))
	require.NoError(t, err)

	return path
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()

	bz, err := os.ReadFile(path)
	require.NoError(t, err)

	sum := sha256.Sum256(bz)
	return fmt.Sprintf("%x", sum)
}

// TestSkipUpgrade tests heights that are identified to be skipped and return if upgrade height matches the skip heights
func TestSkipUpgrade(t *testing.T) {
	cases := []struct {
		args        []string
		upgradeInfo upgradetypes.Plan
		expectRes   bool
	}{{
		args:        []string{"appb", "start", "--unsafe-skip-upgrades"},
		upgradeInfo: upgradetypes.Plan{Name: "upgrade1", Info: "some info", Height: 123},
		expectRes:   false,
	}, {
		args:        []string{"appb", "start", "--unsafe-skip-upgrades", "--abcd"},
		upgradeInfo: upgradetypes.Plan{Name: "upgrade1", Info: "some info", Height: 123},
		expectRes:   false,
	}, {
		args:        []string{"appb", "start", "--unsafe-skip-upgrades", "10", "--abcd"},
		upgradeInfo: upgradetypes.Plan{Name: "upgrade1", Info: "some info", Height: 11},
		expectRes:   false,
	}, {
		args:        []string{"appb", "start", "--unsafe-skip-upgrades", "10", "20", "--abcd"},
		upgradeInfo: upgradetypes.Plan{Name: "upgrade1", Info: "some info", Height: 20},
		expectRes:   true,
	}, {
		args:        []string{"appb", "start", "--unsafe-skip-upgrades", "10", "20", "--abcd", "34"},
		upgradeInfo: upgradetypes.Plan{Name: "upgrade1", Info: "some info", Height: 34},
		expectRes:   false,
	}}

	for i := range cases {
		tc := cases[i]
		require := require.New(t)
		h := cosmovisor.IsSkipUpgradeHeight(tc.args, tc.upgradeInfo)
		require.Equal(h, tc.expectRes)
	}
}

// TestUpgradeSkipHeights tests if correct skip upgrade heights are identified from the cli args
func TestUpgradeSkipHeights(t *testing.T) {
	cases := []struct {
		args      []string
		expectRes []int
	}{{
		args:      []string{},
		expectRes: nil,
	}, {
		args:      []string{"appb", "start"},
		expectRes: nil,
	}, {
		args:      []string{"appb", "start", "--unsafe-skip-upgrades"},
		expectRes: nil,
	}, {
		args:      []string{"appb", "start", "--unsafe-skip-upgrades", "--abcd"},
		expectRes: nil,
	}, {
		args:      []string{"appb", "start", "--unsafe-skip-upgrades", "10", "--abcd"},
		expectRes: []int{10},
	}, {
		args:      []string{"appb", "start", "--unsafe-skip-upgrades", "10", "20", "--abcd"},
		expectRes: []int{10, 20},
	}, {
		args:      []string{"appb", "start", "--unsafe-skip-upgrades", "10", "20", "--abcd", "34"},
		expectRes: []int{10, 20},
	}, {
		args:      []string{"appb", "start", "--unsafe-skip-upgrades", "10", "as", "20", "--abcd"},
		expectRes: []int{10, 20},
	}}

	for i := range cases {
		tc := cases[i]
		require := require.New(t)
		h := cosmovisor.UpgradeSkipHeights(tc.args)
		require.Equal(h, tc.expectRes)
	}
}

// buffer is a thread safe bytes buffer
type buffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func newBuffer() *buffer {
	return &buffer{}
}

func (b *buffer) Write(bz []byte) (int, error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(bz)
}

func (b *buffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}

func (b *buffer) Reset() {
	b.m.Lock()
	defer b.m.Unlock()
	b.b.Reset()
}
