//go:build ledger && test_ledger_mock
// +build ledger,test_ledger_mock

package client_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
)

func TestGetClientTxContextLedgerSignMode(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	kr := keyring.NewInMemory(codec.NewProtoCodec(registry))
	_, err := kr.SaveLedgerKey("ledger", hd.Secp256k1, "cosmos", 118, 0, 0)
	require.NoError(t, err)

	testCases := []struct {
		signModeStr string
		expected    string
	}{
		{"", flags.SignModeEIP712},
		{flags.SignModeLegacyAminoJSON, flags.SignModeEIP712},
		{flags.SignModeEIP712, flags.SignModeEIP712},
		{flags.SignModeEIP712V3, flags.SignModeEIP712V3},
	}

	for _, tc := range testCases {
		t.Run(tc.signModeStr, func(t *testing.T) {
			cmd := &cobra.Command{}
			flags.AddTxFlagsToCmd(cmd)
			args := []string{fmt.Sprintf("--%s=ledger", flags.FlagFrom)}
			if tc.signModeStr != "" {
				args = append(args, fmt.Sprintf("--%s=%s", flags.FlagSignMode, tc.signModeStr))
			}
			require.NoError(t, cmd.ParseFlags(args))

			clientCtx := client.Context{}.WithKeyring(kr)
			cmd.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))

			clientCtx, err := client.GetClientTxContext(cmd)
			require.NoError(t, err)
			require.Equal(t, tc.expected, clientCtx.SignModeStr)
		})
	}
}
