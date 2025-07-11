package l1infotreesync

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/agglayer/aggkit/db"
	"github.com/agglayer/aggkit/db/compatibility"
	"github.com/agglayer/aggkit/sync"
	"github.com/agglayer/aggkit/tree"
	"github.com/agglayer/aggkit/tree/types"
	aggkittypes "github.com/agglayer/aggkit/types"
	"github.com/ethereum/go-ethereum/common"
)

const (
	reorgDetectorID    = "l1InfoTreeSyncer"
	downloadBufferSize = 1000
)

type CreationFlags uint64

const (
	FlagNone                     CreationFlags = 1 << iota // Check for correct contracts addresses
	FlagAllowWrongContractsAddrs                           // Allow to set wrong contracts addresses
)

var (
	ErrNotFound = errors.New("l1infotreesync: not found")
)

type L1InfoTreeSync struct {
	processor *processor
	driver    *sync.EVMDriver
}

// New creates a L1 Info tree syncer that syncs the L1 info tree
// and the rollup exit tree
func New(
	ctx context.Context,
	dbPath string,
	globalExitRoot, rollupManager common.Address,
	syncBlockChunkSize uint64,
	blockFinalityType aggkittypes.BlockNumberFinality,
	rd sync.ReorgDetector,
	l1Client aggkittypes.BaseEthereumClienter,
	waitForNewBlocksPeriod time.Duration,
	initialBlock uint64,
	retryAfterErrorPeriod time.Duration,
	maxRetryAttemptsAfterError int,
	flags CreationFlags,
	finalizedBlockType aggkittypes.BlockNumberFinality,
	requireStorageContentCompatibility bool,
) (*L1InfoTreeSync, error) {
	processor, err := newProcessor(dbPath)
	if err != nil {
		return nil, err
	}
	// TODO: get the initialBlock from L1 to simplify config
	lastProcessedBlock, err := processor.GetLastProcessedBlock(ctx)
	if err != nil {
		return nil, err
	}
	if initialBlock > 0 && lastProcessedBlock < initialBlock-1 {
		block, err := l1Client.BlockByNumber(ctx, new(big.Int).SetUint64(initialBlock-1))
		if err != nil {
			return nil, fmt.Errorf("failed to get initial block %d: %w", initialBlock-1, err)
		}

		err = processor.ProcessBlock(ctx, sync.Block{
			Num:  initialBlock - 1,
			Hash: block.Hash(),
		})
		if err != nil {
			return nil, err
		}
	}
	rh := &sync.RetryHandler{
		RetryAfterErrorPeriod:      retryAfterErrorPeriod,
		MaxRetryAttemptsAfterError: maxRetryAttemptsAfterError,
	}

	appender, err := buildAppender(l1Client, globalExitRoot, rollupManager, flags)
	if err != nil {
		return nil, err
	}
	downloader, err := sync.NewEVMDownloader(
		"l1infotreesync",
		l1Client,
		syncBlockChunkSize,
		blockFinalityType,
		waitForNewBlocksPeriod,
		appender,
		[]common.Address{globalExitRoot, rollupManager},
		rh,
		finalizedBlockType,
	)
	if err != nil {
		return nil, err
	}
	compatibilityChecker := compatibility.NewCompatibilityCheck(
		requireStorageContentCompatibility,
		downloader.RuntimeData,
		processor)
	driver, err := sync.NewEVMDriver(rd, processor, downloader, reorgDetectorID,
		downloadBufferSize, rh, compatibilityChecker)
	if err != nil {
		return nil, err
	}

	return &L1InfoTreeSync{
		processor: processor,
		driver:    driver,
	}, nil
}

// Start starts the synchronization process
func (s *L1InfoTreeSync) Start(ctx context.Context) {
	s.driver.Sync(ctx)
}

// GetL1InfoTreeMerkleProof creates a merkle proof for the L1 Info tree
func (s *L1InfoTreeSync) GetL1InfoTreeMerkleProof(ctx context.Context, index uint32) (types.Proof, types.Root, error) {
	if s.processor.isHalted() {
		return types.Proof{}, types.Root{}, sync.ErrInconsistentState
	}
	return s.processor.GetL1InfoTreeMerkleProof(ctx, index)
}

// GetRollupExitTreeMerkleProof creates a merkle proof for the rollup exit tree
func (s *L1InfoTreeSync) GetRollupExitTreeMerkleProof(
	ctx context.Context,
	networkID uint32,
	root common.Hash,
) (types.Proof, error) {
	if s.processor.isHalted() {
		return types.Proof{}, sync.ErrInconsistentState
	}
	if networkID == 0 {
		return tree.EmptyProof, nil
	}

	return s.processor.rollupExitTree.GetProof(ctx, networkID-1, root)
}

func translateError(err error) error {
	if errors.Is(err, db.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// GetLatestInfoUntilBlock returns the most recent L1InfoTreeLeaf that occurred before or at blockNum.
// If the blockNum has not been processed yet the error ErrBlockNotProcessed will be returned
// It can returns next errors:
// - ErrBlockNotProcessed,
// - ErrNotFound
func (s *L1InfoTreeSync) GetLatestInfoUntilBlock(ctx context.Context, blockNum uint64) (*L1InfoTreeLeaf, error) {
	if s.processor.isHalted() {
		return nil, sync.ErrInconsistentState
	}
	leaf, err := s.processor.GetLatestInfoUntilBlock(ctx, blockNum)
	return leaf, translateError(err)
}

// GetInfoByIndex returns the value of a leaf (not the hash) of the L1 info tree
func (s *L1InfoTreeSync) GetInfoByIndex(ctx context.Context, index uint32) (*L1InfoTreeLeaf, error) {
	if s.processor.isHalted() {
		return nil, sync.ErrInconsistentState
	}
	return s.processor.GetInfoByIndex(ctx, index)
}

// GetL1InfoTreeRootByIndex returns the root of the L1 info tree at the moment the leaf with the given index was added
func (s *L1InfoTreeSync) GetL1InfoTreeRootByIndex(ctx context.Context, index uint32) (types.Root, error) {
	if s.processor.isHalted() {
		return types.Root{}, sync.ErrInconsistentState
	}
	return s.processor.l1InfoTree.GetRootByIndex(ctx, index)
}

// GetLastRollupExitRoot return the last rollup exit root processed
func (s *L1InfoTreeSync) GetLastRollupExitRoot(ctx context.Context) (types.Root, error) {
	if s.processor.isHalted() {
		return types.Root{}, sync.ErrInconsistentState
	}
	return s.processor.rollupExitTree.GetLastRoot(s.processor.db)
}

// GetLastL1InfoTreeRoot return the last root and index processed from the L1 Info tree
func (s *L1InfoTreeSync) GetLastL1InfoTreeRoot(ctx context.Context) (types.Root, error) {
	if s.processor.isHalted() {
		return types.Root{}, sync.ErrInconsistentState
	}
	return s.processor.l1InfoTree.GetLastRoot(s.processor.db)
}

// GetLastProcessedBlock return the last processed block
func (s *L1InfoTreeSync) GetLastProcessedBlock(ctx context.Context) (uint64, error) {
	if s.processor.isHalted() {
		return 0, sync.ErrInconsistentState
	}
	return s.processor.GetLastProcessedBlock(ctx)
}

func (s *L1InfoTreeSync) GetLocalExitRoot(
	ctx context.Context, networkID uint32, rollupExitRoot common.Hash,
) (common.Hash, error) {
	if s.processor.isHalted() {
		return common.Hash{}, sync.ErrInconsistentState
	}
	if networkID == 0 {
		return common.Hash{}, errors.New("network 0 is not a rollup, and it's not part of the rollup exit tree")
	}

	return s.processor.rollupExitTree.GetLeaf(s.processor.db, networkID-1, rollupExitRoot)
}

func (s *L1InfoTreeSync) GetLastVerifiedBatches(rollupID uint32) (*VerifyBatches, error) {
	if s.processor.isHalted() {
		return nil, sync.ErrInconsistentState
	}
	return s.processor.GetLastVerifiedBatches(rollupID)
}

func (s *L1InfoTreeSync) GetFirstVerifiedBatches(rollupID uint32) (*VerifyBatches, error) {
	if s.processor.isHalted() {
		return nil, sync.ErrInconsistentState
	}
	return s.processor.GetFirstVerifiedBatches(rollupID)
}

func (s *L1InfoTreeSync) GetFirstVerifiedBatchesAfterBlock(rollupID uint32, blockNum uint64) (*VerifyBatches, error) {
	if s.processor.isHalted() {
		return nil, sync.ErrInconsistentState
	}
	return s.processor.GetFirstVerifiedBatchesAfterBlock(rollupID, blockNum)
}

func (s *L1InfoTreeSync) GetFirstL1InfoWithRollupExitRoot(rollupExitRoot common.Hash) (*L1InfoTreeLeaf, error) {
	if s.processor.isHalted() {
		return nil, sync.ErrInconsistentState
	}
	return s.processor.GetFirstL1InfoWithRollupExitRoot(rollupExitRoot)
}

func (s *L1InfoTreeSync) GetLastInfo() (*L1InfoTreeLeaf, error) {
	if s.processor.isHalted() {
		return nil, sync.ErrInconsistentState
	}
	return s.processor.GetLastInfo()
}

func (s *L1InfoTreeSync) GetFirstInfo() (*L1InfoTreeLeaf, error) {
	if s.processor.isHalted() {
		return nil, sync.ErrInconsistentState
	}
	return s.processor.GetFirstInfo()
}

func (s *L1InfoTreeSync) GetFirstInfoAfterBlock(blockNum uint64) (*L1InfoTreeLeaf, error) {
	if s.processor.isHalted() {
		return nil, sync.ErrInconsistentState
	}
	return s.processor.GetFirstInfoAfterBlock(blockNum)
}

func (s *L1InfoTreeSync) GetInfoByGlobalExitRoot(ger common.Hash) (*L1InfoTreeLeaf, error) {
	if s.processor.isHalted() {
		return nil, sync.ErrInconsistentState
	}
	return s.processor.GetInfoByGlobalExitRoot(ger)
}

// GetL1InfoTreeMerkleProofFromIndexToRoot creates a merkle proof for the L1 Info tree
func (s *L1InfoTreeSync) GetL1InfoTreeMerkleProofFromIndexToRoot(
	ctx context.Context, index uint32, root common.Hash,
) (types.Proof, error) {
	if s.processor.isHalted() {
		return types.Proof{}, sync.ErrInconsistentState
	}
	return s.processor.l1InfoTree.GetProof(ctx, index, root)
}

// GetInitL1InfoRootMap returns the initial L1 info root map, nil if no root map has been set
func (s *L1InfoTreeSync) GetInitL1InfoRootMap(ctx context.Context) (*L1InfoTreeInitial, error) {
	if s.processor.isHalted() {
		return nil, sync.ErrInconsistentState
	}
	return s.processor.GetInitL1InfoRootMap(nil)
}

// GetProcessedBlockUntil returns the last block processed before the given block number or
// the exact block num and hash associated to given block if it was processed
func (s *L1InfoTreeSync) GetProcessedBlockUntil(ctx context.Context, blockNum uint64) (uint64, common.Hash, error) {
	if s.processor.isHalted() {
		return 0, common.Hash{}, sync.ErrInconsistentState
	}
	return s.processor.GetProcessedBlockUntil(ctx, blockNum)
}
