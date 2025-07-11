package types

import (
	"context"
	"math/big"

	"github.com/0xPolygon/cdk-contracts-tooling/contracts/pp/l2-sovereign-chain/globalexitrootmanagerl2sovereignchain"
	ethtxtypes "github.com/0xPolygon/zkevm-ethtx-manager/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// EthTxManager is an interface to interact with the EthTxManager
type EthTxManager interface {
	Remove(ctx context.Context, id common.Hash) error
	ResultsByStatus(ctx context.Context,
		statuses []ethtxtypes.MonitoredTxStatus,
	) ([]ethtxtypes.MonitoredTxResult, error)
	Result(ctx context.Context, id common.Hash) (ethtxtypes.MonitoredTxResult, error)
	Add(ctx context.Context,
		to *common.Address,
		value *big.Int,
		data []byte,
		gasOffset uint64,
		sidecar *types.BlobTxSidecar,
	) (common.Hash, error)
	From() common.Address
}

// L2GERManagerContract is an interface to interact with the GlobalExitRootManager contract
type L2GERManagerContract interface {
	GlobalExitRootMap(opts *bind.CallOpts, ger [common.HashLength]byte) (*big.Int, error)
	BridgeAddress(*bind.CallOpts) (common.Address, error)
	FilterUpdateHashChainValue(opts *bind.FilterOpts, newGlobalExitRoot [][32]byte, newHashChainValue [][32]byte) (
		*globalexitrootmanagerl2sovereignchain.Globalexitrootmanagerl2sovereignchainUpdateHashChainValueIterator, error)
	FilterUpdateRemovalHashChainValue(
		opts *bind.FilterOpts,
		removedGlobalExitRoot [][32]byte,
		newRemovalHashChainValue [][32]byte) (
		*globalexitrootmanagerl2sovereignchain.Globalexitrootmanagerl2sovereignchainUpdateRemovalHashChainValueIterator,
		error,
	)
	GlobalExitRootUpdater(opts *bind.CallOpts) (common.Address, error)
}
