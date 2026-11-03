package baseapp

import (
	"context"
	"sync"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func newQueryContextTestApp(t *testing.T) *BaseApp {
	t.Helper()
	app := NewBaseApp(t.Name(), log.NewNopLogger(), dbm.NewMemDB(), nil)
	_, err := app.FinalizeBlock(&abci.FinalizeBlockRequest{Height: 1})
	require.NoError(t, err)
	_, err = app.Commit()
	require.NoError(t, err)
	return app
}

func TestCreateQueryContextConcurrentWithCommit(t *testing.T) {
	app := newQueryContextTestApp(t)
	stop := make(chan struct{})
	var readers sync.WaitGroup
	t.Cleanup(func() {
		close(stop)
		readers.Wait()
	})
	for i := 0; i < 4; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if _, err := app.CreateQueryContext(1, false); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	for height := int64(2); height <= 100; height++ {
		_, err := app.FinalizeBlock(&abci.FinalizeBlockRequest{Height: height})
		require.NoError(t, err)
		_, err = app.Commit()
		require.NoError(t, err)
	}
}

func TestGRPCQueryAllowsNestedQuery(t *testing.T) {
	app := newQueryContextTestApp(t)
	app.SetVersion("nested-query")
	app.grpcQueryRouter.routes["/test/nested"] = func(_ sdk.Context, _ *abci.QueryRequest) (*abci.QueryResponse, error) {
		return app.Query(context.Background(), &abci.QueryRequest{Path: "/app/version"})
	}

	responses := make(chan *abci.QueryResponse, 1)
	go func() {
		resp, err := app.Query(context.Background(), &abci.QueryRequest{Path: "/test/nested"})
		if err != nil {
			t.Error(err)
		}
		responses <- resp
	}()
	select {
	case resp := <-responses:
		require.NotNil(t, resp)
		require.Zero(t, resp.Code, resp.Log)
		require.Equal(t, []byte("nested-query"), resp.Value)
	case <-time.After(5 * time.Second):
		t.Fatal("gRPC handler could not complete a nested query")
	}
}
