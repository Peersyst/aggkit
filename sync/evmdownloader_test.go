package sync

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/agglayer/aggkit/log"
	aggkittypes "github.com/agglayer/aggkit/types"
	aggkittypesmocks "github.com/agglayer/aggkit/types/mocks"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	contractAddr   = common.HexToAddress("f00")
	eventSignature = crypto.Keccak256Hash([]byte("foo"))
)

const (
	syncBlockChunck = uint64(10)
)

type testEvent common.Hash

func TestGetEventsByBlockRange(t *testing.T) {
	type testCase struct {
		description        string
		inputLogs          []types.Log
		fromBlock, toBlock uint64
		expectedBlocks     EVMBlocks
		setupMocks         func(*aggkittypesmocks.BaseEthereumClienter)
		contextCancelled   bool
	}
	testCases := []testCase{}
	ctx := context.Background()
	d, clientMock := NewTestDownloader(t, time.Millisecond*100)

	// case 0: single block, no events
	case0 := testCase{
		description:    "case 0: single block, no events",
		inputLogs:      []types.Log{},
		fromBlock:      1,
		toBlock:        3,
		expectedBlocks: EVMBlocks{},
	}
	testCases = append(testCases, case0)

	// case 1: single block, single event
	logC1, updateC1 := generateEvent(3)
	logsC1 := []types.Log{
		*logC1,
	}
	blocksC1 := EVMBlocks{
		{
			EVMBlockHeader: EVMBlockHeader{
				Num:        logC1.BlockNumber,
				Hash:       logC1.BlockHash,
				ParentHash: common.HexToHash("foo"),
			},
			Events: []interface{}{updateC1},
		},
	}
	case1 := testCase{
		description:    "case 1: single block, single event",
		inputLogs:      logsC1,
		fromBlock:      3,
		toBlock:        3,
		expectedBlocks: blocksC1,
	}
	testCases = append(testCases, case1)

	// case 2: single block, multiple events
	logC2_1, updateC2_1 := generateEvent(5)
	logC2_2, updateC2_2 := generateEvent(5)
	logC2_3, updateC2_3 := generateEvent(5)
	logC2_4, updateC2_4 := generateEvent(5)
	logsC2 := []types.Log{
		*logC2_1,
		*logC2_2,
		*logC2_3,
		*logC2_4,
	}
	blocksC2 := []*EVMBlock{
		{
			EVMBlockHeader: EVMBlockHeader{
				Num:        logC2_1.BlockNumber,
				Hash:       logC2_1.BlockHash,
				ParentHash: common.HexToHash("foo"),
			},
			Events: []interface{}{
				updateC2_1,
				updateC2_2,
				updateC2_3,
				updateC2_4,
			},
		},
	}
	case2 := testCase{
		description:    "case 2: single block, multiple events",
		inputLogs:      logsC2,
		fromBlock:      5,
		toBlock:        5,
		expectedBlocks: blocksC2,
	}
	testCases = append(testCases, case2)

	// case 3: multiple blocks, some events
	logC3_1, updateC3_1 := generateEvent(7)
	logC3_2, updateC3_2 := generateEvent(7)
	logC3_3, updateC3_3 := generateEvent(8)
	logC3_4, updateC3_4 := generateEvent(8)
	logsC3 := []types.Log{
		*logC3_1,
		*logC3_2,
		*logC3_3,
		*logC3_4,
	}
	blocksC3 := EVMBlocks{
		{
			EVMBlockHeader: EVMBlockHeader{
				Num:        logC3_1.BlockNumber,
				Hash:       logC3_1.BlockHash,
				ParentHash: common.HexToHash("foo"),
			},
			Events: []interface{}{
				updateC3_1,
				updateC3_2,
			},
		},
		{
			EVMBlockHeader: EVMBlockHeader{
				Num:        logC3_3.BlockNumber,
				Hash:       logC3_3.BlockHash,
				ParentHash: common.HexToHash("foo"),
			},
			Events: []interface{}{
				updateC3_3,
				updateC3_4,
			},
		},
	}
	case3 := testCase{
		description:    "case 3: multiple blocks, some events",
		inputLogs:      logsC3,
		fromBlock:      7,
		toBlock:        8,
		expectedBlocks: blocksC3,
	}
	testCases = append(testCases, case3)

	// case 4: context cancelled
	case4 := testCase{
		description:      "case 4: context cancelled",
		inputLogs:        []types.Log{},
		fromBlock:        1,
		toBlock:          3,
		expectedBlocks:   nil,
		contextCancelled: true,
	}
	testCases = append(testCases, case4)

	// case 5: block hash mismatch with retry success
	logC5, updateC5 := generateEvent(10)
	logsC5 := []types.Log{*logC5}
	blocksC5 := EVMBlocks{
		{
			EVMBlockHeader: EVMBlockHeader{
				Num:        logC5.BlockNumber,
				Hash:       logC5.BlockHash,
				ParentHash: common.HexToHash("foo"),
			},
			Events: []interface{}{updateC5},
		},
	}
	case5 := testCase{
		description:    "case 5: block hash mismatch with retry success",
		inputLogs:      logsC5,
		fromBlock:      10,
		toBlock:        10,
		expectedBlocks: blocksC5,
		setupMocks: func(clientMock *aggkittypesmocks.BaseEthereumClienter) {
			// First call returns different hash (mismatch)
			clientMock.EXPECT().HeaderByNumber(mock.Anything, big.NewInt(10)).
				Return(&types.Header{
					Number:     big.NewInt(10),
					ParentHash: common.HexToHash("foo"),
				}, nil).Once()
			// Second call returns correct hash
			clientMock.EXPECT().HeaderByNumber(mock.Anything, big.NewInt(10)).
				Return(&types.Header{
					Number:     big.NewInt(10),
					ParentHash: common.HexToHash("foo"),
				}, nil).Once()
		},
	}
	testCases = append(testCases, case5)

	// case 6: block hash mismatch with max retries exceeded
	logC6, _ := generateEvent(15)
	logsC6 := []types.Log{*logC6}
	case6 := testCase{
		description:    "case 6: block hash mismatch with max retries exceeded",
		inputLogs:      logsC6,
		fromBlock:      15,
		toBlock:        15,
		expectedBlocks: nil,
		setupMocks: func(clientMock *aggkittypesmocks.BaseEthereumClienter) {
			// Return a different hash than the log's block hash for all retry attempts
			// This will trigger the retry logic and eventually exceed max retries
			for i := 0; i < MaxRetryCountBlockHashMismatch+1; i++ {
				clientMock.EXPECT().HeaderByNumber(mock.Anything, big.NewInt(15)).
					Return(&types.Header{
						Number:     big.NewInt(15),
						ParentHash: common.HexToHash("bar"), // Different parent hash to create different block hash
						// The hash will be different from logC6.BlockHash, causing mismatch
					}, nil).Once()
			}
		},
	}
	testCases = append(testCases, case6)

	// case 7: logs with removed flag should be filtered out
	logC7_1, _ := generateEvent(20)
	logC7_2, updateC7_2 := generateEvent(20)
	logC7_1.Removed = true // This log should be filtered out
	logsC7 := []types.Log{*logC7_1, *logC7_2}
	blocksC7 := EVMBlocks{
		{
			EVMBlockHeader: EVMBlockHeader{
				Num:        logC7_2.BlockNumber,
				Hash:       logC7_2.BlockHash,
				ParentHash: common.HexToHash("foo"),
			},
			Events: []interface{}{updateC7_2}, // Only the non-removed log
		},
	}
	case7 := testCase{
		description:    "case 7: logs with removed flag should be filtered out",
		inputLogs:      logsC7,
		fromBlock:      20,
		toBlock:        20,
		expectedBlocks: blocksC7,
	}
	testCases = append(testCases, case7)

	// case 8: logs with non-matching topics should be filtered out
	logC8_1, updateC8_1 := generateEvent(25)
	logC8_2 := &types.Log{
		Address:     contractAddr,
		BlockNumber: 25,
		Topics: []common.Hash{
			common.HexToHash("0x1234567890abcdef"), // Non-matching topic
			common.HexToHash("0xabcdef1234567890"),
		},
		BlockHash: logC8_1.BlockHash,
		Data:      nil,
	}
	logsC8 := []types.Log{*logC8_1, *logC8_2}
	blocksC8 := EVMBlocks{
		{
			EVMBlockHeader: EVMBlockHeader{
				Num:        logC8_1.BlockNumber,
				Hash:       logC8_1.BlockHash,
				ParentHash: common.HexToHash("foo"),
			},
			Events: []interface{}{updateC8_1}, // Only the matching topic log
		},
	}
	case8 := testCase{
		description:    "case 8: logs with non-matching topics should be filtered out",
		inputLogs:      logsC8,
		fromBlock:      25,
		toBlock:        25,
		expectedBlocks: blocksC8,
	}
	testCases = append(testCases, case8)

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test_case_%d_%s", i, tc.description), func(t *testing.T) {
			// Reset mock for each test case
			clientMock.ExpectedCalls = nil

			query := ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(tc.fromBlock),
				Addresses: []common.Address{contractAddr},
				ToBlock:   new(big.Int).SetUint64(tc.toBlock),
			}

			if tc.contextCancelled {
				// Create a cancelled context
				cancelledCtx, cancel := context.WithCancel(context.Background())
				cancel()
				clientMock.EXPECT().FilterLogs(cancelledCtx, query).Return(tc.inputLogs, nil)
			} else {
				clientMock.EXPECT().FilterLogs(mock.Anything, query).Return(tc.inputLogs, nil)
			}

			// Setup custom mocks if provided
			if tc.setupMocks != nil {
				tc.setupMocks(clientMock)
			} else {
				// Default mock setup for block headers
				for _, b := range tc.expectedBlocks {
					clientMock.EXPECT().HeaderByNumber(mock.Anything, big.NewInt(int64(b.Num))).
						Return(&types.Header{
							Number:     big.NewInt(int64(b.Num)),
							ParentHash: common.HexToHash("foo"),
						}, nil)
				}
			}

			var actualBlocks EVMBlocks
			if tc.contextCancelled {
				cancelledCtx, cancel := context.WithCancel(context.Background())
				cancel()
				actualBlocks = d.GetEventsByBlockRange(cancelledCtx, tc.fromBlock, tc.toBlock)
			} else {
				actualBlocks = d.GetEventsByBlockRange(ctx, tc.fromBlock, tc.toBlock)
			}

			require.Equal(t, tc.expectedBlocks, actualBlocks, tc.description)
		})
	}
}

func generateEvent(blockNum uint32) (*types.Log, testEvent) {
	h := common.HexToHash(strconv.Itoa(int(blockNum)))
	header := types.Header{
		Number:     big.NewInt(int64(blockNum)),
		ParentHash: common.HexToHash("foo"),
	}
	blockHash := header.Hash()
	log := &types.Log{
		Address:     contractAddr,
		BlockNumber: uint64(blockNum),
		Topics: []common.Hash{
			eventSignature,
			h,
		},
		BlockHash: blockHash,
		Data:      nil,
	}
	return log, testEvent(h)
}

func TestWaitForNewBlocks(t *testing.T) {
	ctx := context.Background()
	d, clientMock := NewTestDownloader(t, time.Millisecond*100)

	// at first attempt
	currentBlock := uint64(5)
	expectedBlock := uint64(6)
	clientMock.On("HeaderByNumber", ctx, mock.Anything).Return(&types.Header{
		Number: big.NewInt(6),
	}, nil).Once()
	actualBlock := d.WaitForNewBlocks(ctx, currentBlock)
	assert.Equal(t, expectedBlock, actualBlock)

	// 2 iterations
	clientMock.On("HeaderByNumber", ctx, mock.Anything).Return(&types.Header{
		Number: big.NewInt(5),
	}, nil).Once()
	clientMock.On("HeaderByNumber", ctx, mock.Anything).Return(&types.Header{
		Number: big.NewInt(6),
	}, nil).Once()
	actualBlock = d.WaitForNewBlocks(ctx, currentBlock)
	assert.Equal(t, expectedBlock, actualBlock)

	// after error from client
	clientMock.On("HeaderByNumber", ctx, mock.Anything).Return(nil, errors.New("foo")).Once()
	clientMock.On("HeaderByNumber", ctx, mock.Anything).Return(&types.Header{
		Number: big.NewInt(6),
	}, nil).Once()
	actualBlock = d.WaitForNewBlocks(ctx, currentBlock)
	assert.Equal(t, expectedBlock, actualBlock)
}

func TestGetBlockHeader(t *testing.T) {
	ctx := context.Background()
	d, clientMock := NewTestDownloader(t, time.Millisecond)

	blockNum := uint64(5)
	blockNumBig := big.NewInt(5)
	returnedBlock := &types.Header{
		Number: blockNumBig,
	}
	expectedBlock := EVMBlockHeader{
		Num:  5,
		Hash: returnedBlock.Hash(),
	}

	// at first attempt
	clientMock.On("HeaderByNumber", ctx, blockNumBig).Return(returnedBlock, nil).Once()
	actualBlock, isCanceled := d.GetBlockHeader(ctx, blockNum)
	assert.Equal(t, expectedBlock, actualBlock)
	assert.False(t, isCanceled)

	// after error from client
	clientMock.On("HeaderByNumber", ctx, blockNumBig).Return(nil, errors.New("foo")).Once()
	clientMock.On("HeaderByNumber", ctx, blockNumBig).Return(returnedBlock, nil).Once()
	actualBlock, isCanceled = d.GetBlockHeader(ctx, blockNum)
	assert.Equal(t, expectedBlock, actualBlock)
	assert.False(t, isCanceled)

	// header not found default
	clientMock.On("HeaderByNumber", ctx, blockNumBig).Return(nil, ethereum.NotFound).Once()
	clientMock.On("HeaderByNumber", ctx, blockNumBig).Return(returnedBlock, nil).Once()
	actualBlock, isCanceled = d.GetBlockHeader(ctx, 5)
	assert.Equal(t, expectedBlock, actualBlock)
	assert.False(t, isCanceled)

	// header not found default TO
	d, clientMock = NewTestDownloader(t, 0)
	clientMock.On("HeaderByNumber", ctx, blockNumBig).Return(nil, ethereum.NotFound).Once()
	clientMock.On("HeaderByNumber", ctx, blockNumBig).Return(returnedBlock, nil).Once()
	actualBlock, isCanceled = d.GetBlockHeader(ctx, 5)
	assert.Equal(t, expectedBlock, actualBlock)
	assert.False(t, isCanceled)
}

func TestFilterQueryToString(t *testing.T) {
	addr1 := common.HexToAddress("0xf000")
	addr2 := common.HexToAddress("0xabcd")
	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(1000),
		Addresses: []common.Address{addr1, addr2},
		ToBlock:   new(big.Int).SetUint64(1100),
	}

	assert.Equal(t, "FromBlock: 1000, ToBlock: 1100, Addresses: [0x000000000000000000000000000000000000f000 0x000000000000000000000000000000000000ABcD], Topics: []", filterQueryToString(query))

	query = ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(1000),
		Addresses: []common.Address{addr1, addr2},
		ToBlock:   new(big.Int).SetUint64(1100),
		Topics:    [][]common.Hash{{common.HexToHash("0x1234"), common.HexToHash("0x5678")}},
	}
	assert.Equal(t, "FromBlock: 1000, ToBlock: 1100, Addresses: [0x000000000000000000000000000000000000f000 0x000000000000000000000000000000000000ABcD], Topics: [[0x0000000000000000000000000000000000000000000000000000000000001234 0x0000000000000000000000000000000000000000000000000000000000005678]]", filterQueryToString(query))
}

func TestGetLogs(t *testing.T) {
	mockEthClient := aggkittypesmocks.NewBaseEthereumClienter(t)
	sut := EVMDownloaderImplementation{
		ethClient:        mockEthClient,
		addressesToQuery: []common.Address{contractAddr},
		log:              log.WithFields("test", "EVMDownloaderImplementation"),
		rh: &RetryHandler{
			RetryAfterErrorPeriod:      time.Millisecond,
			MaxRetryAttemptsAfterError: 5,
		},
	}
	ctx := context.TODO()
	mockEthClient.EXPECT().FilterLogs(ctx, mock.Anything).Return(nil, errors.New("foo")).Once()
	mockEthClient.EXPECT().FilterLogs(ctx, mock.Anything).Return(nil, nil).Once()
	logs := sut.GetLogs(ctx, 0, 1)
	require.Equal(t, []types.Log{}, logs)
}

func TestDownloadBeforeFinalized(t *testing.T) {
	steps := []evmTestStep{
		{finalizedBlock: 33, fromBlock: 1, toBlock: 11, waitForNewBlocks: true, waitForNewBlocksRequest: 0, waitForNewBlockReply: 35, getBlockHeader: &EVMBlockHeader{Num: 11}},
		{finalizedBlock: 33, fromBlock: 12, toBlock: 22, eventsReponse: EVMBlocks{createEVMBlock(t, 14, true)}, getBlockHeader: &EVMBlockHeader{Num: 22}},
		// It returns the last block of range, so it don't need to create a empty one
		{finalizedBlock: 33, fromBlock: 23, toBlock: 33, eventsReponse: EVMBlocks{createEVMBlock(t, 33, true)}},
		// It reach the top of chain (block 35)
		{finalizedBlock: 33, fromBlock: 34, toBlock: 35},
		// Previous iteration we reach top of chain so we need update the latest block
		{finalizedBlock: 33, fromBlock: 34, toBlock: 54, waitForNewBlocks: true, waitForNewBlocksRequest: 35, waitForNewBlockReply: 60},
		// finalized block is 35, so we can reduce emit an emptyBlock and reduce the range
		{finalizedBlock: 35, fromBlock: 34, toBlock: 60, getBlockHeader: &EVMBlockHeader{Num: 35}},
		{finalizedBlock: 35, fromBlock: 36, toBlock: 46},
		{finalizedBlock: 35, fromBlock: 36, toBlock: 56, eventsReponse: EVMBlocks{createEVMBlock(t, 36, false)}},
		// Block 36 is the new last block,so it reduce the range again to [37-47]
		{finalizedBlock: 35, fromBlock: 37, toBlock: 47},
		{finalizedBlock: 57, fromBlock: 37, toBlock: 57, eventsReponse: EVMBlocks{createEVMBlock(t, 57, false)}},
		{finalizedBlock: 61, fromBlock: 58, toBlock: 60, eventsReponse: EVMBlocks{createEVMBlock(t, 60, false)}},
		{finalizedBlock: 61, fromBlock: 61, toBlock: 61, waitForNewBlocks: true, waitForNewBlocksRequest: 60, waitForNewBlockReply: 61, getBlockHeader: &EVMBlockHeader{Num: 61}},
		{finalizedBlock: 61, fromBlock: 62, toBlock: 62, waitForNewBlocks: true, waitForNewBlocksRequest: 61, waitForNewBlockReply: 62},
	}
	runSteps(t, 1, steps)
}

func TestCaseAskLastBlockIfFinalitySameAsTargetBlock(t *testing.T) {
	steps := []evmTestStep{
		{finalizedBlock: 105, fromBlock: 99, toBlock: 105, waitForNewBlocks: true, waitForNewBlocksRequest: 0, waitForNewBlockReply: 105, getBlockHeader: &EVMBlockHeader{Num: 105}},
		{finalizedBlock: 110, fromBlock: 106, toBlock: 110, waitForNewBlocks: true, waitForNewBlocksRequest: 105, waitForNewBlockReply: 110, getBlockHeader: &EVMBlockHeader{Num: 110}},
		// Here is the bug:
		// - the range 111-115 returns block: 106. So the code must emit the block 106 and also the block 115 as empty (last block)
		{finalizedBlock: 115, fromBlock: 111, toBlock: 115, waitForNewBlocks: true, waitForNewBlocksRequest: 110, waitForNewBlockReply: 115, eventsReponse: EVMBlocks{createEVMBlock(t, 106, false)}, getBlockHeader: &EVMBlockHeader{Num: 115}},
	}
	runSteps(t, 99, steps)
}

func buildAppender() LogAppenderMap {
	appender := make(LogAppenderMap)
	appender[eventSignature] = func(b *EVMBlock, l types.Log) error {
		b.Events = append(b.Events, testEvent(l.Topics[1]))
		return nil
	}
	return appender
}

func NewTestDownloader(t *testing.T, retryPeriod time.Duration) (*EVMDownloader, *aggkittypesmocks.BaseEthereumClienter) {
	t.Helper()

	rh := &RetryHandler{
		MaxRetryAttemptsAfterError: 5,
		RetryAfterErrorPeriod:      retryPeriod,
	}
	clientMock := aggkittypesmocks.NewBaseEthereumClienter(t)
	d, err := NewEVMDownloader("test",
		clientMock, syncBlockChunck, aggkittypes.LatestBlock, time.Millisecond,
		buildAppender(), []common.Address{contractAddr}, rh,
		aggkittypes.FinalizedBlock,
	)
	require.NoError(t, err)
	return d, clientMock
}

func createEVMBlock(t *testing.T, num uint64, isSafeBlock bool) *EVMBlock {
	t.Helper()
	return &EVMBlock{
		IsFinalizedBlock: isSafeBlock,
		EVMBlockHeader: EVMBlockHeader{
			Num:        num,
			Hash:       common.HexToHash(fmt.Sprintf("0x%.2X", num)),
			ParentHash: common.HexToHash(fmt.Sprintf("0x%.2X", num-1)),
			Timestamp:  uint64(time.Now().Unix()),
		},
	}
}

type evmTestStep struct {
	finalizedBlock          uint64
	fromBlock, toBlock      uint64
	eventsReponse           EVMBlocks
	waitForNewBlocks        bool
	waitForNewBlocksRequest uint64
	waitForNewBlockReply    uint64
	getBlockHeader          *EVMBlockHeader
}

func runSteps(t *testing.T, fromBlock uint64, steps []evmTestStep) {
	t.Helper()
	mockEthDownloader := NewEVMDownloaderMock(t)

	ctx := context.Background()
	ctx1, cancel := context.WithCancel(ctx)
	defer cancel()

	downloader, _ := NewTestDownloader(t, time.Millisecond)
	downloader.EVMDownloaderInterface = mockEthDownloader

	for i := 0; i < len(steps); i++ {
		log.Info("iteration: ", i, "------------------------------------------------")
		downloadCh := make(chan EVMBlock, 100)
		downloader, _ := NewTestDownloader(t, time.Millisecond)
		downloader.EVMDownloaderInterface = mockEthDownloader
		downloader.setStopDownloaderOnIterationN(i + 1)
		expectedBlocks := EVMBlocks{}
		for _, step := range steps[:i+1] {
			mockEthDownloader.On("GetLastFinalizedBlock", mock.Anything).Return(&types.Header{Number: big.NewInt(int64(step.finalizedBlock))}, nil).Once()
			if step.waitForNewBlocks {
				mockEthDownloader.On("WaitForNewBlocks", mock.Anything, step.waitForNewBlocksRequest).Return(step.waitForNewBlockReply).Once()
			}
			mockEthDownloader.On("GetEventsByBlockRange", mock.Anything, step.fromBlock, step.toBlock).
				Return(step.eventsReponse, false).Once()
			expectedBlocks = append(expectedBlocks, step.eventsReponse...)
			if step.getBlockHeader != nil {
				log.Infof("iteration:%d : GetBlockHeader(%d) ", i, step.getBlockHeader.Num)
				mockEthDownloader.On("GetBlockHeader", mock.Anything, step.getBlockHeader.Num).Return(*step.getBlockHeader, false).Once()
				expectedBlocks = append(expectedBlocks, &EVMBlock{
					EVMBlockHeader:   *step.getBlockHeader,
					IsFinalizedBlock: step.getBlockHeader.Num <= step.finalizedBlock,
				})
			}
		}
		downloader.Download(ctx1, fromBlock, downloadCh)
		mockEthDownloader.AssertExpectations(t)
		for _, expectedBlock := range expectedBlocks {
			log.Debugf("waiting block %d ", expectedBlock.Num)
			actualBlock := <-downloadCh
			log.Debugf("block %d received!", actualBlock.Num)
			require.Equal(t, *expectedBlock, actualBlock)
		}
	}
}
