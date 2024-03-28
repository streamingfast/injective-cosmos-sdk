package firehose

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
)

var _ baseapp.StreamingService = &Service{}

type Service struct {
	storeListeners []*types.MemoryListener // a series of KVStore listeners for each KVStore
	logger         log.Logger
	stopNodeOnErr  bool

	block Block
}

func NewFirehoseService(storeKeys []types.StoreKey, logger log.Logger, stopNodeOnErr bool) (*Service, error) {
	// sort storeKeys for deterministic output
	sort.SliceStable(storeKeys, func(i, j int) bool {
		return storeKeys[i].Name() < storeKeys[j].Name()
	})

	// NOTE: We use the same listener for each store.
	listeners := make([]*types.MemoryListener, len(storeKeys))
	for i, key := range storeKeys {
		listeners[i] = types.NewMemoryListener(key)
	}

	//Emitting log line to inform the firehose console reader that the firehose service has been initialized
	fmt.Println("FIRE INIT 3.0", "sf.cosmos.type.v2.Block")

	return &Service{
		storeListeners: listeners,
		logger:         logger,
		stopNodeOnErr:  stopNodeOnErr,
	}, nil
}

func (s *Service) Listeners() map[types.StoreKey][]types.WriteListener {
	listeners := make(map[types.StoreKey][]types.WriteListener, len(s.storeListeners))
	for _, listener := range s.storeListeners {
		listeners[listener.StoreKey()] = []types.WriteListener{listener}
	}

	return listeners
}

func (s *Service) ListenBeginBlock(ctx context.Context, req abci.RequestBeginBlock, res abci.ResponseBeginBlock) error {
	sdkCtx := ctx.(sdk.Context)
	s.block = Block{}
	s.block.Header = &req.Header

	s.block.Req = &RequestFinalizeBlock{
		DecidedLastCommit:  req.LastCommitInfo,
		Misbehavior:        req.ByzantineValidators,
		Hash:               req.Hash,
		Height:             req.Header.Height,
		Time:               req.Header.Time,
		NextValidatorsHash: sdkCtx.BlockHeader().NextValidatorsHash,
		ProposerAddress:    sdkCtx.BlockHeader().ProposerAddress,
	}

	s.block.Res = &ResponseFinalizeBlock{}
	s.block.Res.Events = res.Events //todo: not sure about this

	return nil
}

func (s *Service) ListenDeliverTx(ctx context.Context, req abci.RequestDeliverTx, res abci.ResponseDeliverTx) error {
	s.block.Req.Txs = append(s.block.Req.Txs, req.Tx)
	s.block.Res.TxResults = append(s.block.Res.TxResults, &res)

	return nil
}

func (s *Service) ListenEndBlock(ctx context.Context, req abci.RequestEndBlock, res abci.ResponseEndBlock) error {
	s.block.Res.Events = append(s.block.Res.Events, res.Events...) //double check if we also want events from begin block
	s.block.Res.ValidatorUpdates = res.ValidatorUpdates
	s.block.Res.ConsensusParamUpdates = res.ConsensusParamUpdates
	//s.block.Res.AppHash = sdkCtx.BlockHeader().AppHash

	return nil
}

func (s *Service) ListenCommit(ctx context.Context, res abci.ResponseCommit) error {
	if err := s.listenCommit(ctx, res); err != nil {
		s.logger.Error("Listen commit failed", "height", s.block.Header.Height, "err", err)
		if s.stopNodeOnErr {
			return err
		}
	}

	return nil
}

func (s *Service) listenCommit(ctx context.Context, res abci.ResponseCommit) error {
	payload, err := gogoproto.Marshal(&s.block)
	if err != nil {
		return fmt.Errorf("mashalling block: %w", err)
	}

	// [block_num:342342342] [block_hash] [parent_num] [parent_hash] [lib:123123123] [timestamp:unix_nano] B64ENCODED_any
	blockLine := fmt.Sprintf(
		"FIRE BLOCK %d %s %d %s %d %d %s",
		s.block.Header.Height,
		hex.EncodeToString(s.block.Req.Hash),
		s.block.Header.Height-1,
		hex.EncodeToString(s.block.Header.LastBlockId.Hash),
		s.block.Header.Height-1,
		s.block.Header.Time.UnixNano(),
		base64.StdEncoding.EncodeToString(payload),
	)

	//Emitting log line to inform the firehose console reader that a block has been committed and is ready to be read
	fmt.Println(blockLine)

	return nil
}

// Stream satisfies the StreamingService interface. It performs a no-op.
func (s *Service) Stream(wg *sync.WaitGroup) error { return nil }

// Close satisfies the StreamingService interface. It performs a no-op.
func (s *Service) Close() error { return nil }
