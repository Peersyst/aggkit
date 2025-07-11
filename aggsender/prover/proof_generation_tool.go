package prover

import (
	"context"
	"fmt"
	"time"

	"github.com/0xPolygon/cdk-rpc/rpc"
	"github.com/agglayer/aggkit/aggoracle/chaingerreader"
	"github.com/agglayer/aggkit/aggsender/aggchainproofclient"
	"github.com/agglayer/aggkit/aggsender/flows"
	"github.com/agglayer/aggkit/aggsender/query"
	"github.com/agglayer/aggkit/aggsender/types"
	aggkitgrpc "github.com/agglayer/aggkit/grpc"
	"github.com/agglayer/aggkit/log"
	treetypes "github.com/agglayer/aggkit/tree/types"
	aggkittypes "github.com/agglayer/aggkit/types"
	"github.com/ethereum/go-ethereum/common"
)

// ProofGeneration is the interface for generating Aggchain proofs
type AggchainProofGeneration interface {
	GenerateAggchainProof(ctx context.Context, fromBlock, toBlock uint64) (*types.SP1StarkProof, error)
}

// AggchainProofFlow is the interface for the Aggchain proof flow
type AggchainProofFlow interface {
	// GenerateAggchainProof generates an Aggchain proof
	GenerateAggchainProof(
		ctx context.Context,
		lastProvenBlock, toBlock uint64,
		certBuildParams *types.CertificateBuildParams) (*types.AggchainProof, *treetypes.Root, error)
}

// Config is the configuration for the AggchainProofGenerationTool
type Config struct {
	// AggkitProverClient is the AggkitProver client configuration
	AggkitProverClient *aggkitgrpc.ClientConfig `mapstructure:"AggkitProverClient"`

	// GlobalExitRootL2Addr is the address of the GlobalExitRootManager contract on l2 sovereign chain
	// this address is needed for the AggchainProof mode of the AggSender
	GlobalExitRootL2Addr common.Address `mapstructure:"GlobalExitRootL2"`

	// SovereignRollupAddr is the address of the sovereign rollup contract on L1
	SovereignRollupAddr common.Address `mapstructure:"SovereignRollupAddr"`
}

// AggchainProofGenerationTool is a tool to generate Aggchain proofs
type AggchainProofGenerationTool struct {
	cfg Config

	logger   *log.Logger
	l2Syncer types.L2BridgeSyncer

	aggchainProofClient types.AggchainProofClientInterface
	flow                AggchainProofFlow
}

type OptimisticModeQuerierAlwaysOff struct{}

func (o *OptimisticModeQuerierAlwaysOff) IsOptimisticModeOn() (bool, error) {
	return false, nil
}

// NewAggchainProofGenerationTool creates a new AggchainProofGenerationTool
func NewAggchainProofGenerationTool(
	ctx context.Context,
	logger *log.Logger,
	cfg Config,
	l2Syncer types.L2BridgeSyncer,
	l1InfoTreeSyncer types.L1InfoTreeSyncer,
	l1Client aggkittypes.BaseEthereumClienter,
	l2Client aggkittypes.BaseEthereumClienter) (*AggchainProofGenerationTool, error) {
	if err := cfg.AggkitProverClient.Validate(); err != nil {
		return nil, fmt.Errorf("invalid aggkit prover client config: %w", err)
	}

	aggchainProofClient, err := aggchainproofclient.NewAggchainProofClient(cfg.AggkitProverClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create AggchainProofClient: %w", err)
	}

	chainGERReader, err := chaingerreader.NewEVMChainGERReader(cfg.GlobalExitRootL2Addr, l2Client)
	if err != nil {
		return nil, fmt.Errorf("error creating chain GER reader: %w", err)
	}

	l1InfoTreeQuerier := query.NewL1InfoTreeDataQuerier(l1Client, l1InfoTreeSyncer)
	l2BridgeQuerier := query.NewBridgeDataQuerier(logger, l2Syncer, time.Second)

	baseFlow := flows.NewBaseFlow(
		logger,
		l2BridgeQuerier,
		nil, // storage
		l1InfoTreeQuerier,
		nil, // lerQuerier
		flows.NewBaseFlowConfigDefault(),
	)
	aggchainProverFlow := flows.NewAggchainProverFlow(
		logger,
		flows.NewAggchainProverFlowConfigDefault(),
		baseFlow,
		aggchainProofClient,
		nil, // storage
		l1InfoTreeQuerier,
		l2BridgeQuerier,
		query.NewGERDataQuerier(l1InfoTreeQuerier, chainGERReader),
		l1Client,
		nil,                               // signer
		&OptimisticModeQuerierAlwaysOff{}, // For tools is always no optimistic mode,
		nil,                               // optimisticSigner
	)

	return &AggchainProofGenerationTool{
		cfg:                 cfg,
		logger:              logger,
		l2Syncer:            l2Syncer,
		flow:                aggchainProverFlow,
		aggchainProofClient: aggchainProofClient,
	}, nil
}

// GetRPCServices returns the list of services that the RPC provider exposes
func (a *AggchainProofGenerationTool) GetRPCServices() []rpc.Service {
	return []rpc.Service{
		{
			Name:    "aggkit",
			Service: NewAggchainProofGenerationToolRPC(a),
		},
	}
}

// GenerateAggchainProof generates an Aggchain proof
func (a *AggchainProofGenerationTool) GenerateAggchainProof(
	ctx context.Context,
	lastProvenBlock, maxEndBlock uint64) (*types.SP1StarkProof, error) {
	a.logger.Infof("Generating Aggchain proof. Last proven block: %d. "+
		"Max end block: %d", lastProvenBlock, maxEndBlock)

	// get last L2 block synced
	lastL2BlockSynced, err := a.l2Syncer.GetLastProcessedBlock(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting last processed block from l2: %w", err)
	}

	a.logger.Debugf("Last L2 block synced: %d", lastL2BlockSynced)

	// check if last L2 block synced is less than last proven block
	if lastL2BlockSynced < lastProvenBlock {
		a.logger.Errorf("last L2 block synced %d is less than last proven block %d",
			lastL2BlockSynced, lastProvenBlock)

		return nil, fmt.Errorf("the last L2 block synced %d is less than the last proven block %d",
			lastL2BlockSynced, lastProvenBlock)
	}

	fromBlock := lastProvenBlock + 1

	// get claims for the block range
	a.logger.Debugf("Getting claims for block range [%d : %d]", fromBlock, maxEndBlock)

	claims, err := a.l2Syncer.GetClaims(ctx, fromBlock, maxEndBlock)
	if err != nil {
		return nil, fmt.Errorf("error getting claims (imported bridge exits): %w", err)
	}

	a.logger.Debugf("Got %d claims for block range [%d : %d]", len(claims), fromBlock, maxEndBlock)

	// call the prover to generate the proof
	a.logger.Debugf("Calling AggchainProofClient to generate proof for block range [%d : %d]",
		fromBlock, maxEndBlock)

	certBuildParams := &types.CertificateBuildParams{
		Claims: claims,
	}
	aggchainProof, _, err := a.flow.GenerateAggchainProof(
		ctx,
		lastProvenBlock,
		maxEndBlock,
		certBuildParams,
	)
	if err != nil {
		return nil, fmt.Errorf("error generating Aggchain proof: %w", err)
	}

	a.logger.Infof("Generated Aggchain proof for block range [%d : %d]", fromBlock, maxEndBlock)

	return aggchainProof.SP1StarkProof, nil
}
