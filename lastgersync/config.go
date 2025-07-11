package lastgersync

import (
	"github.com/agglayer/aggkit/config/types"
	"github.com/ethereum/go-ethereum/common"
)

type Config struct {
	// DBPath path of the DB
	DBPath string `mapstructure:"DBPath"`
	// BlockFinality indicates the status of the blocks that will be queried in order to sync
	BlockFinality string `jsonschema:"enum=LatestBlock, enum=SafeBlock, enum=PendingBlock, enum=FinalizedBlock, enum=EarliestBlock" mapstructure:"BlockFinality"` //nolint:lll
	// InitialBlockNum is the first block that will be queried when starting the synchronization from scratch.
	// It should be a number equal or bellow the creation of the bridge contract
	InitialBlockNum uint64 `mapstructure:"InitialBlockNum"`
	// GlobalExitRootL2Addr is the address of the GER smart contract on L2
	GlobalExitRootL2Addr common.Address `mapstructure:"GlobalExitRootL2Addr"`
	// RetryAfterErrorPeriod is the time that will be waited when an unexpected error happens before retry
	RetryAfterErrorPeriod types.Duration `mapstructure:"RetryAfterErrorPeriod"`
	// MaxRetryAttemptsAfterError is the maximum number of consecutive attempts that will happen before panicing.
	// Any number smaller than zero will be considered as unlimited retries
	MaxRetryAttemptsAfterError int `mapstructure:"MaxRetryAttemptsAfterError"`
	// WaitForNewBlocksPeriod time that will be waited when the synchronizer has reached the latest block
	WaitForNewBlocksPeriod types.Duration `mapstructure:"WaitForNewBlocksPeriod"`
	// DownloadBufferSize buffer size of events to be processed. When the buffer limit is reached,
	// downloading will stop until the processing catches up.
	DownloadBufferSize int `mapstructure:"DownloadBufferSize"`
	// RequireStorageContentCompatibility is true it's mandatory that data stored in the database
	// is compatible with the running environment
	RequireStorageContentCompatibility bool `mapstructure:"RequireStorageContentCompatibility"`
	// SyncMode denotes should the latest global exit root be determined
	// by querying the global exit root map (which is common way for FEP chains)
	// or the events emitted by sovereign chains (which is a common way for PP chains)
	SyncMode SyncMode `jsonschema:"enum=FEP, enum=PP" mapstructure:"SyncMode"`
}
