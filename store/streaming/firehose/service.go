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
	"github.com/cometbft/cometbft/proto/tendermint/crypto"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/store/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
	sfpb "github.com/streamingfast/firehose-cosmos/pb/sf/cometbft/type/v38"
	"google.golang.org/protobuf/types/known/durationpb"
)

var _ baseapp.StreamingService = &Service{}

type Service struct {
	storeListeners []*types.MemoryListener // a series of KVStore listeners for each KVStore
	logger         log.Logger
	stopNodeOnErr  bool

	block sfpb.Block
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
	fmt.Println("FIRE INIT 3.0", "cosmos.firehose.v1.Block")

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
	events := convertEvents(res.Events)
	s.block.Events = append(s.block.Events, events...)
	return nil
}

func (s *Service) ListenDeliverTx(ctx context.Context, req abci.RequestDeliverTx, res abci.ResponseDeliverTx) error {
	events := convertEvents(res.Events)
	sfTx := sfpb.ExecTxResult{
		Code:      res.Code,
		Data:      res.Data,
		Log:       res.Log,
		Info:      res.Info,
		GasWanted: res.GasWanted,
		GasUsed:   res.GasUsed,
		Events:    events,
		Codespace: res.Codespace,
	}
	s.block.TxResults = append(s.block.TxResults, &sfTx)

	return nil
}

func (s *Service) ListenEndBlock(ctx context.Context, req abci.RequestEndBlock, res abci.ResponseEndBlock) error {
	//todo: what about AppHash

	events := convertEvents(res.Events)
	s.block.Events = append(s.block.Events, events...)

	validatorUpdates := make([]*sfpb.ValidatorUpdate, len(res.ValidatorUpdates))
	for i, update := range res.ValidatorUpdates {
		var k *sfpb.PublicKey
		switch update.PubKey.Sum.(type) {
		case *crypto.PublicKey_Ed25519:
			k = &sfpb.PublicKey{
				Sum: &sfpb.PublicKey_Ed25519{
					Ed25519: update.PubKey.GetEd25519(),
				},
			}
		case *crypto.PublicKey_Secp256K1:
			k = &sfpb.PublicKey{
				Sum: &sfpb.PublicKey_Secp256K1{
					Secp256K1: update.PubKey.GetSecp256K1(),
				},
			}
		default:
			panic("unknown public key type")
		}
		validatorUpdates[i] = &sfpb.ValidatorUpdate{
			PubKey: k,
			Power:  update.Power,
		}
	}
	s.block.ValidatorUpdates = validatorUpdates

	s.block.ConsensusParamUpdates = &sfpb.ConsensusParams{
		Block: &sfpb.BlockParams{
			MaxBytes: res.ConsensusParamUpdates.Block.MaxBytes,
			MaxGas:   res.ConsensusParamUpdates.Block.MaxGas,
		},
		Evidence: &sfpb.EvidenceParams{
			MaxAgeNumBlocks: res.ConsensusParamUpdates.Evidence.MaxAgeNumBlocks,
			MaxAgeDuration:  durationpb.New(res.ConsensusParamUpdates.Evidence.MaxAgeDuration),
			MaxBytes:        res.ConsensusParamUpdates.Evidence.MaxBytes,
		},
		Validator: &sfpb.ValidatorParams{
			PubKeyTypes: res.ConsensusParamUpdates.Validator.PubKeyTypes,
		},
		Version: &sfpb.VersionParams{
			App: res.ConsensusParamUpdates.Version.App,
		},
		//todo: does not exist in 0.37.5 need to update this code when switching to 0.38.0 +
		//Abci: &sfpb.ABCIParams{
		//	VoteExtensionsEnableHeight: res.ConsensusParamUpdates.
		//},
	}

	return nil
}

func (s *Service) ListenCommit(ctx context.Context, res abci.ResponseCommit) error {
	if err := s.listenCommit(res); err != nil {
		s.logger.Error("Listen commit failed", "height", s.block.Meta.Header.Height, "err", err)
		if s.stopNodeOnErr {
			return err
		}
	}

	return nil
}

func (s *Service) listenCommit(res abci.ResponseCommit) error {

	payload, err := gogoproto.Marshal(&s.block)
	if err != nil {
		return fmt.Errorf("mashalling block: %w", err)
	}

	blockLine := fmt.Sprintf(
		"FIRE BLOCK %d %s %d %s %d %d %s",
		s.block.Meta.Header.Height,
		hex.EncodeToString(s.block.Meta.BlockId.Hash),
		s.block.Meta.Header.Height-1,
		s.block.Meta.Header.LastCommitHash,
		s.block.Meta.Header.Height,
		s.block.Meta.Header.Time.AsTime().UnixNano(),
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

func convertEvents(in []abci.Event) []*sfpb.Event {
	events := make([]*sfpb.Event, len(in))
	for _, event := range in {
		attributes := make([]*sfpb.EventAttribute, len(event.Attributes))
		for i, attr := range event.Attributes {
			attributes[i] = &sfpb.EventAttribute{
				Key:   attr.Key,
				Value: attr.Value,
			}
		}
		events = append(events, &sfpb.Event{
			Type:       event.Type,
			Attributes: attributes,
		})
	}
	return events
}
