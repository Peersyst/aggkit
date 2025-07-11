package aggsender

import (
	"context"
	"fmt"

	"github.com/agglayer/aggkit/agglayer"
	"github.com/agglayer/aggkit/aggsender/types"
	aggkitcommon "github.com/agglayer/aggkit/common"
)

const (
	maxPercent = 100.0
)

type ExtraInfoEventEpoch struct {
	PendingBlocks int
}

func (e *ExtraInfoEventEpoch) String() string {
	return fmt.Sprintf("ExtraInfoEventEpoch: pendingBlocks=%d", e.PendingBlocks)
}

type ConfigEpochNotifierPerBlock struct {
	StartingEpochBlock uint64
	NumBlockPerEpoch   uint

	// EpochNotificationPercentage
	// 0 -> begin new Epoch
	// 50 -> middle of epoch
	// 100 -> end of epoch (same as 0)
	EpochNotificationPercentage uint
}

func (c *ConfigEpochNotifierPerBlock) String() string {
	if c == nil {
		return "nil"
	}
	return fmt.Sprintf("{startEpochBlock=%d, sizeEpoch=%d, threshold=%d%%}",
		c.StartingEpochBlock, c.NumBlockPerEpoch, c.EpochNotificationPercentage)
}

func NewConfigEpochNotifierPerBlock(ctx context.Context,
	agglayerClient agglayer.AggLayerClientGetEpochConfiguration,
	epochNotificationPercentage uint) (*ConfigEpochNotifierPerBlock, error) {
	if agglayerClient == nil {
		return nil, fmt.Errorf("newConfigEpochNotifierPerBlock: agglayerClient is required")
	}
	clockConfig, err := agglayerClient.GetEpochConfiguration(ctx)
	if err != nil {
		return nil, fmt.Errorf("newConfigEpochNotifierPerBlock: error getting clock configuration from AggLayer: %w", err)
	}
	return &ConfigEpochNotifierPerBlock{
		StartingEpochBlock:          clockConfig.GenesisBlock,
		NumBlockPerEpoch:            uint(clockConfig.EpochDuration),
		EpochNotificationPercentage: epochNotificationPercentage,
	}, nil
}

func (c *ConfigEpochNotifierPerBlock) Validate() error {
	if c.NumBlockPerEpoch == 0 {
		return fmt.Errorf("num block per epoch should be greater than 0")
	}
	if c.EpochNotificationPercentage >= maxPercent {
		return fmt.Errorf("epoch notification percentage must be between 0 and 99")
	}
	return nil
}

type EpochNotifierPerBlock struct {
	blockNotifier types.BlockNotifier
	logger        aggkitcommon.Logger

	lastStartingEpochBlock uint64

	Config ConfigEpochNotifierPerBlock
	types.GenericSubscriber[types.EpochEvent]
}

func NewEpochNotifierPerBlock(blockNotifier types.BlockNotifier,
	logger aggkitcommon.Logger,
	config ConfigEpochNotifierPerBlock,
	subscriber types.GenericSubscriber[types.EpochEvent]) (*EpochNotifierPerBlock, error) {
	if subscriber == nil {
		subscriber = NewGenericSubscriberImpl[types.EpochEvent]()
	}

	err := config.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &EpochNotifierPerBlock{
		blockNotifier:          blockNotifier,
		logger:                 logger,
		lastStartingEpochBlock: config.StartingEpochBlock,
		Config:                 config,
		GenericSubscriber:      subscriber,
	}, nil
}

func (e *EpochNotifierPerBlock) String() string {
	return fmt.Sprintf("EpochNotifierPerBlock: config: %s", e.Config.String())
}

// StartAsync starts the notifier in a goroutine
func (e *EpochNotifierPerBlock) StartAsync(ctx context.Context) {
	eventNewBlockChannel := e.blockNotifier.Subscribe("EpochNotifierPerBlock")
	go e.startInternal(ctx, eventNewBlockChannel)
}

// Start starts the notifier synchronously
func (e *EpochNotifierPerBlock) Start(ctx context.Context) {
	eventNewBlockChannel := e.blockNotifier.Subscribe("EpochNotifierPerBlock")
	e.startInternal(ctx, eventNewBlockChannel)
}

// GetCurrentStatus returns the current status of the epoch
func (e *EpochNotifierPerBlock) GetEpochStatus() types.EpochStatus {
	currentBlock := e.blockNotifier.GetCurrentBlockNumber()
	return types.EpochStatus{
		Epoch:        e.epochNumber(currentBlock),
		PercentEpoch: e.percentEpoch(currentBlock),
	}
}

func (e *EpochNotifierPerBlock) startInternal(ctx context.Context, eventNewBlockChannel <-chan types.EventNewBlock) {
	status := internalStatus{
		lastBlockSeen:   e.Config.StartingEpochBlock,
		waitingForEpoch: e.epochNumber(e.Config.StartingEpochBlock),
	}
	for {
		select {
		case <-ctx.Done():
			return
		case newBlock := <-eventNewBlockChannel:
			var event *types.EpochEvent
			status, event = e.step(status, newBlock)
			if event != nil {
				e.logger.Debugf("new Epoch Event: %s", event.String())
				e.GenericSubscriber.Publish(*event)
			}
		}
	}
}

type internalStatus struct {
	lastBlockSeen   uint64
	waitingForEpoch uint64
}

func (e *EpochNotifierPerBlock) step(status internalStatus,
	newBlock types.EventNewBlock) (internalStatus, *types.EpochEvent) {
	currentBlock := newBlock.BlockNumber
	if currentBlock < e.Config.StartingEpochBlock {
		// This is a bit strange, the first epoch is in the future
		e.logger.Warnf("Block number %d is before the starting first epoch block %d."+
			" Please check your config", currentBlock, e.Config.StartingEpochBlock)
		return status, nil
	}
	// No new block
	if currentBlock <= status.lastBlockSeen {
		return status, nil
	}
	status.lastBlockSeen = currentBlock

	needNotify, closingEpoch := e.isNotificationRequired(currentBlock, status.waitingForEpoch)
	percentEpoch := e.percentEpoch(currentBlock)
	logFunc := e.logger.Debugf
	if needNotify {
		logFunc = e.logger.Infof
	}
	logFunc("New block seen [finality:%s]: %d. blockRate:%s Epoch:%d Percent:%.2f%% notify:%v config:%s",
		newBlock.BlockFinalityType, newBlock.BlockNumber, newBlock.BlockRate, closingEpoch,
		percentEpoch*maxPercent, needNotify, e.Config.String())
	if needNotify {
		// Notify the epoch has started
		info := e.infoEpoch(currentBlock, closingEpoch)
		status.waitingForEpoch = closingEpoch + 1
		return status, &types.EpochEvent{
			Epoch:     closingEpoch,
			ExtraInfo: info,
		}
	}
	return status, nil
}

func (e *EpochNotifierPerBlock) infoEpoch(currentBlock, newEpochNotified uint64) *ExtraInfoEventEpoch {
	nextBlockStartingEpoch := e.endBlockEpoch(newEpochNotified)
	return &ExtraInfoEventEpoch{
		PendingBlocks: int(nextBlockStartingEpoch - currentBlock),
	}
}
func (e *EpochNotifierPerBlock) percentEpoch(currentBlock uint64) float64 {
	epoch := e.epochNumber(currentBlock)
	startingBlock := e.startingBlockEpoch(epoch)
	elapsedBlocks := currentBlock - startingBlock
	return float64(elapsedBlocks) / float64(e.Config.NumBlockPerEpoch)
}
func (e *EpochNotifierPerBlock) isNotificationRequired(currentBlock, lastEpochNotified uint64) (bool, uint64) {
	percentEpoch := e.percentEpoch(currentBlock)
	thresholdPercent := float64(e.Config.EpochNotificationPercentage) / maxPercent
	maxTresholdPercent := float64(e.Config.NumBlockPerEpoch-1) / float64(e.Config.NumBlockPerEpoch)
	if thresholdPercent > maxTresholdPercent {
		thresholdPercent = maxTresholdPercent
	}
	if percentEpoch < thresholdPercent {
		return false, e.epochNumber(currentBlock)
	}
	nextEpoch := e.epochNumber(currentBlock) + 1
	return nextEpoch > lastEpochNotified, e.epochNumber(currentBlock)
}

func (e *EpochNotifierPerBlock) startingBlockEpoch(epoch uint64) uint64 {
	if epoch == 0 {
		return e.Config.StartingEpochBlock - 1
	}
	return e.Config.StartingEpochBlock + ((epoch - 1) * uint64(e.Config.NumBlockPerEpoch))
}

func (e *EpochNotifierPerBlock) endBlockEpoch(epoch uint64) uint64 {
	return e.startingBlockEpoch(epoch + 1)
}
func (e *EpochNotifierPerBlock) epochNumber(currentBlock uint64) uint64 {
	if currentBlock < e.Config.StartingEpochBlock {
		return 0
	}
	return 1 + ((currentBlock - e.Config.StartingEpochBlock) / uint64(e.Config.NumBlockPerEpoch))
}
