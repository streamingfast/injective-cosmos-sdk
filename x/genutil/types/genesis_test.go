package types_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmttypes "github.com/cometbft/cometbft/types"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"

	"github.com/cosmos/cosmos-sdk/x/genutil/types"
)

func TestAppGenesis_Marshal(t *testing.T) {
	genesis := types.AppGenesis{
		AppName:    "simapp",
		AppVersion: "0.1.0",
		ChainID:    "test",
	}

	out, err := json.Marshal(&genesis)
	assert.NilError(t, err)
	assert.Equal(t, string(out), `{"app_name":"simapp","app_version":"0.1.0","genesis_time":"0001-01-01T00:00:00Z","chain_id":"test","initial_height":0,"app_hash":null}`)
}

func TestAppGenesis_Unmarshal(t *testing.T) {
	jsonBlob, err := os.ReadFile("testdata/app_genesis.json")
	assert.NilError(t, err)

	var genesis types.AppGenesis
	err = json.Unmarshal(jsonBlob, &genesis)
	assert.NilError(t, err)

	assert.DeepEqual(t, genesis.ChainID, "demo")
	assert.DeepEqual(t, genesis.Consensus.Params.Block.MaxBytes, int64(22020096))
}

func TestAppGenesis_ValidGenesis(t *testing.T) {
	// validate can read cometbft genesis file
	genesis, err := types.AppGenesisFromFile("testdata/cmt_genesis.json")
	assert.NilError(t, err)

	assert.DeepEqual(t, genesis.ChainID, "demo")
	assert.DeepEqual(t, genesis.Consensus.Validators[0].Name, "test")

	// validate the app genesis can be translated properly to cometbft genesis
	cmtGenesis, err := genesis.ToGenesisDoc()
	assert.NilError(t, err)
	rawCmtGenesis, err := cmttypes.GenesisDocFromFile("testdata/cmt_genesis.json")
	assert.NilError(t, err)
	assert.DeepEqual(t, cmtGenesis, rawCmtGenesis)

	// validate can properly marshal to app genesis file
	rawAppGenesis, err := json.Marshal(&genesis)
	assert.NilError(t, err)
	golden.Assert(t, string(rawAppGenesis), "app_genesis.json")

	// validate the app genesis can be unmarshalled properly
	var appGenesis types.AppGenesis
	err = json.Unmarshal(rawAppGenesis, &appGenesis)
	assert.NilError(t, err)
	assert.DeepEqual(t, appGenesis.Consensus.Params, genesis.Consensus.Params)

	// validate marshaling of app genesis
	rawAppGenesis, err = json.Marshal(&appGenesis)
	assert.NilError(t, err)
	golden.Assert(t, string(rawAppGenesis), "app_genesis.json")
}

func TestAppGenesis_SaveAsRoundTrip(t *testing.T) {
	genesis, err := types.AppGenesisFromFile("testdata/cmt_genesis.json")
	assert.NilError(t, err)
	genesis.AppName = "simapp"
	genesis.AppVersion = "0.1.0"
	genesis.InitialHeight = 5

	file := filepath.Join(t.TempDir(), "genesis.json")
	assert.NilError(t, genesis.SaveAs(file))

	// SaveAs uses cmtjson, which writes 64-bit integers as strings.
	raw, err := os.ReadFile(file)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(raw), `"initial_height": "5"`), string(raw))

	loaded, err := types.AppGenesisFromFile(file)
	assert.NilError(t, err)
	assert.Equal(t, loaded.AppName, genesis.AppName)
	assert.Equal(t, loaded.AppVersion, genesis.AppVersion)
	assert.Equal(t, loaded.ChainID, genesis.ChainID)
	assert.Equal(t, loaded.InitialHeight, genesis.InitialHeight)
	assert.Assert(t, loaded.GenesisTime.Equal(genesis.GenesisTime))
	assert.DeepEqual(t, loaded.Consensus, genesis.Consensus)

	// SaveAs indents app_state, so compare it compacted.
	var loadedState, wantState bytes.Buffer
	assert.NilError(t, json.Compact(&loadedState, loaded.AppState))
	assert.NilError(t, json.Compact(&wantState, genesis.AppState))
	assert.Equal(t, loadedState.String(), wantState.String())
}

// Genesis files on existing networks were written by encoding/json SaveAs
// (numeric initial_height) or are CometBFT genesis files. Both must still load
// unchanged, and app_state must come back byte for byte: the genesis checksum
// that CometBFT compares against the stored genesisDocHash is taken over it.
func TestAppGenesis_LegacyFilesLoad(t *testing.T) {
	cmtGenesis, err := types.AppGenesisFromFile("testdata/cmt_genesis.json")
	assert.NilError(t, err)
	cmtAppName := cmtGenesis.AppName
	cmtGenesis.AppName = "simapp"
	cmtGenesis.AppVersion = "0.1.0"

	// Byte layout of SaveAs before it switched to cmtjson.
	jsonGenesis, err := json.MarshalIndent(cmtGenesis, "", "  ")
	assert.NilError(t, err)
	jsonFile := filepath.Join(t.TempDir(), "genesis.json")
	assert.NilError(t, os.WriteFile(jsonFile, jsonGenesis, 0o600))

	for name, tc := range map[string]struct {
		file    string
		appName string
	}{
		"encoding/json SaveAs": {file: jsonFile, appName: cmtGenesis.AppName},
		"CometBFT genesis":     {file: "testdata/cmt_genesis.json", appName: cmtAppName},
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(tc.file)
			assert.NilError(t, err)
			var fields map[string]json.RawMessage
			assert.NilError(t, json.Unmarshal(raw, &fields))

			loaded, err := types.AppGenesisFromFile(tc.file)
			assert.NilError(t, err)
			assert.Equal(t, loaded.AppName, tc.appName)
			assert.Equal(t, loaded.ChainID, "demo")
			assert.Equal(t, loaded.InitialHeight, int64(48))
			assert.DeepEqual(t, loaded.Consensus, cmtGenesis.Consensus)
			assert.Equal(t, string(loaded.AppState), string(fields["app_state"]))
		})
	}
}
