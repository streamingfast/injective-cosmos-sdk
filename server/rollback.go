package server

import (
	"fmt"
	"os"

	cmtcmd "github.com/cometbft/cometbft/cmd/cometbft/commands"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server/types"
)

// NewRollbackCmd creates a command to rollback CometBFT and multistore state by one or more heights.
func NewRollbackCmd(appCreator types.AppCreator, defaultNodeHome string) *cobra.Command {
	var (
		removeBlock = true
		numBlocks   = int64(1)
	)

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "rollback Cosmos SDK and CometBFT state by one or more heights",
		Long: `
A state rollback is performed to recover from an incorrect application state transition,
when CometBFT has persisted an incorrect app hash and is thus unable to make
progress. Rollback overwrites a state at height n with the state at height n - num.
The application also rolls back to the same height. By default, rolled-back blocks
are removed from the blockstore so they must be fetched or produced again. For
file private validators, hard rollback also rewinds the local signing state to
the rollback height. Use --hard=false to keep local blocks and replay them
against the rolled-back application state.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := GetServerContextFromCmd(cmd)
			cfg := ctx.Config
			home := cfg.RootDir
			// rollback CometBFT state
			height, hash, err := cmtcmd.RollbackStateN(ctx.Config, removeBlock, numBlocks)
			if err != nil {
				return fmt.Errorf("failed to rollback CometBFT state: %w", err)
			}

			db, err := openDB(home, GetAppDBBackend(ctx.Viper))
			if err != nil {
				return err
			}

			prevSkipLoad, hadSkipLoad := os.LookupEnv("COSMOS_SDK_ROLLBACK_SKIP_LOAD_LATEST")
			if err := os.Setenv("COSMOS_SDK_ROLLBACK_SKIP_LOAD_LATEST", "true"); err != nil {
				return err
			}
			defer func() {
				if hadSkipLoad {
					_ = os.Setenv("COSMOS_SDK_ROLLBACK_SKIP_LOAD_LATEST", prevSkipLoad)
				} else {
					_ = os.Unsetenv("COSMOS_SDK_ROLLBACK_SKIP_LOAD_LATEST")
				}
			}()

			app := appCreator(ctx.Logger, db, nil, ctx.Viper)
			if err := app.CommitMultiStore().LoadVersion(height); err != nil {
				return fmt.Errorf("failed to load version: %w", err)
			}
			// rollback the multistore

			if err := app.CommitMultiStore().RollbackToVersion(height); err != nil {
				return fmt.Errorf("failed to rollback to version: %w", err)
			}

			fmt.Printf("Rolled back state to height %d and hash %X", height, hash)
			return nil
		},
	}

	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "The application home directory")
	cmd.Flags().BoolVar(&removeBlock, "hard", true, "remove rolled-back blocks as well as state")
	cmd.Flags().Int64VarP(&numBlocks, "num", "n", 1, "number of blocks to rollback")
	return cmd
}
