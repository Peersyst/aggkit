// @title Bridge Service API
// @version 1.0
// @description API documentation for the bridge service

// @contact.name API Support
// @contact.url https://polygon.technology/

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @BasePath /bridge/v1

package bridgeservice

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/agglayer/aggkit"
	_ "github.com/agglayer/aggkit/bridgeservice/docs"
	"github.com/agglayer/aggkit/bridgeservice/types"
	"github.com/agglayer/aggkit/bridgesync"
	aggkitcommon "github.com/agglayer/aggkit/common"
	"github.com/agglayer/aggkit/l1infotreesync"
	"github.com/agglayer/aggkit/log"
	tree "github.com/agglayer/aggkit/tree/types"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginswagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

const (
	// BridgeV1Prefix is the url prefix for the bridge service
	BridgeV1Prefix = "/bridge/v1"
	meterName      = "github.com/agglayer/aggkit/bridgeservice"

	networkIDParam    = "network_id"
	networkIDsParam   = "network_ids"
	pageNumberParam   = "page_number"
	pageSizeParam     = "page_size"
	depositCountParam = "deposit_count"
	fromAddressParam  = "from_address"
	leafIndexParam    = "leaf_index"
	globalIndexParam  = "global_index"
	includeAllFields  = "include_all_fields"

	binarySearchDivider = 2
	mainnetNetworkID    = 0

	errNetworkID         = "unsupported network id: %v"
	errSetupRequest      = "failed to setup request: %v"
	errDepositCountParam = "invalid deposit count parameter: %v"
)

var (
	ErrNotOnL1Info = errors.New("this bridge has not been included on the L1 Info Tree yet")
)

type Config struct {
	Logger       *log.Logger
	Address      string
	WriteTimeout time.Duration
	ReadTimeout  time.Duration
	NetworkID    uint32
}

// BridgeService contains implementations for the bridge service endpoints
type BridgeService struct {
	logger       *log.Logger
	address      string
	meter        metric.Meter
	readTimeout  time.Duration
	writeTimeout time.Duration
	networkID    uint32
	l1InfoTree   L1InfoTreer
	injectedGERs LastGERer
	bridgeL1     Bridger
	bridgeL2     Bridger

	router *gin.Engine
}

// New returns instance of BridgeService
func New(
	cfg *Config,
	l1InfoTree L1InfoTreer,
	injectedGERs LastGERer,
	bridgeL1 Bridger,
	bridgeL2 Bridger,
) *BridgeService {
	meter := otel.Meter(meterName)
	cfg.Logger.Infof("starting bridge service (network id=%d, address=%s)", cfg.NetworkID, cfg.Address)

	// The GIN_MODE environment variable controls the mode of the Gin framework.
	// Valid values are "debug", "release", and "test". If an invalid value is provided,
	// the mode defaults to "release" for safety and performance.
	ginMode := os.Getenv("GIN_MODE")
	switch ginMode {
	case gin.DebugMode, gin.ReleaseMode, gin.TestMode:
		gin.SetMode(ginMode)
	default:
		cfg.Logger.Infof("invalid or missing GIN_MODE value ('%s') provided, defaulting to '%s' mode",
			ginMode, gin.ReleaseMode)
		gin.SetMode(gin.ReleaseMode) // fallback to release mode
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(LoggerHandler(cfg.Logger))

	b := &BridgeService{
		logger:       cfg.Logger,
		address:      cfg.Address,
		meter:        meter,
		readTimeout:  cfg.ReadTimeout,
		writeTimeout: cfg.WriteTimeout,
		networkID:    cfg.NetworkID,
		l1InfoTree:   l1InfoTree,
		injectedGERs: injectedGERs,
		bridgeL1:     bridgeL1,
		bridgeL2:     bridgeL2,
		router:       router,
	}

	b.registerRoutes()
	cfg.Logger.Info("bridge service initialized successfully")

	return b
}

// LoggerHandler returns a Gin middleware that logs HTTP requests using logger at DEBUG level.
func LoggerHandler(logger aggkitcommon.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		if latency > time.Minute {
			latency = latency.Truncate(time.Second)
		}

		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		if raw != "" {
			path += "?" + raw
		}

		logger.Debugf(
			"[GIN] %v | %3d | %13v | %15s | %-7s %#v\n%s",
			start.Format("2006/01/02 - 15:04:05"),
			statusCode,
			latency,
			clientIP,
			method,
			path,
			errorMessage,
		)
	}
}

// registerRoutes registers the routes for the bridge service
func (b *BridgeService) registerRoutes() {
	// Health check endpoint at root path
	b.router.GET("/", b.HealthCheckHandler)

	bridgeGroup := b.router.Group(BridgeV1Prefix)
	{
		bridgeGroup.GET("/bridges", b.GetBridgesHandler)
		bridgeGroup.GET("/claims", b.GetClaimsHandler)
		bridgeGroup.GET("/token-mappings", b.GetTokenMappingsHandler)
		bridgeGroup.GET("/legacy-token-migrations", b.GetLegacyTokenMigrationsHandler)
		bridgeGroup.GET("/l1-info-tree-index", b.L1InfoTreeIndexForBridgeHandler)
		bridgeGroup.GET("/injected-l1-info-leaf", b.InjectedL1InfoLeafHandler)
		bridgeGroup.GET("/claim-proof", b.ClaimProofHandler)
		bridgeGroup.GET("/last-reorg-event", b.GetLastReorgEventHandler)
		bridgeGroup.GET("/sync-status", b.GetSyncStatusHandler)

		// Swagger docs endpoint
		bridgeGroup.GET("/swagger/*any", ginswagger.WrapHandler(swaggerfiles.Handler))

		// Redirect to the Swagger UI
		bridgeGroup.GET("/swagger", func(ctx *gin.Context) {
			ctx.Redirect(http.StatusFound, BridgeV1Prefix+"/swagger/index.html")
		})
	}
}

// Start starts the HTTP bridge service
func (b *BridgeService) Start(ctx context.Context) {
	srv := &http.Server{
		Addr:         b.address,
		Handler:      b.router,
		ReadTimeout:  b.readTimeout,
		WriteTimeout: b.writeTimeout,
	}

	b.logger.Infof("Bridge service listening on %s...", b.address)
	err := srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		b.logger.Panicf("failed to start bridge service: %v", err)
	}

	<-ctx.Done()

	b.logger.Info("Shutting down bridge service...")

	var parentCtx context.Context
	if ctx.Err() == nil {
		parentCtx = ctx
	} else {
		parentCtx = context.Background()
	}

	ctx, cancel := context.WithTimeout(parentCtx, b.readTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		b.logger.Panicf("Server shutdown error: %v", err)
	}

	b.logger.Info("Bridge service exited gracefully")
}

// HealthCheckHandler returns the health status and version information of the bridge service.
//
// @Summary Get health status
// @Description Returns the health status and version information of the bridge service
// @Tags health
// @Produce json
// @Success 200 {object} types.HealthCheckResponse "Health status and version information"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router / [get]
func (b *BridgeService) HealthCheckHandler(c *gin.Context) {
	version := aggkit.GetVersion()
	c.JSON(http.StatusOK, types.HealthCheckResponse{
		Status:  "ok",
		Time:    time.Now().UTC(),
		Version: version.Version,
	})
}

// GetBridgesHandler retrieves paginated bridge data for the specified network.
//
// @Summary Get bridges
// @Description Returns a paginated list of bridge events for the specified network.
// @Tags bridges
// @Param network_id query uint32 true "Target network ID"
// @Param page_number query uint32 false "Page number (default 1)"
// @Param page_size query uint32 false "Page size (default 100)"
// @Param deposit_count query uint64 false "Filter by deposit count"
// @Param from_address query string false "Filter by from address"
// @Param network_ids query []uint32 false "Filter by one or more network IDs"
// @Produce json
// @Success 200 {object} types.BridgesResult
// @Failure 400 {object} types.ErrorResponse "Bad Request"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /bridges [get]
func (b *BridgeService) GetBridgesHandler(c *gin.Context) {
	b.logger.Debugf("GetBridges request received (network id=%s, page number=%s, page size=%s)",
		c.Query(networkIDParam), c.Query(pageNumberParam), c.Query(pageSizeParam))

	networkID, err := parseUintQuery(c, networkIDParam, true, uint32(0))
	if err != nil {
		b.logger.Warnf(errNetworkID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	depositCount, err := parseUintQuery(c, depositCountParam, false, uint64(math.MaxUint64))
	if err != nil {
		b.logger.Warnf(errDepositCountParam, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var depositCountPtr *uint64
	if depositCount != math.MaxUint64 {
		depositCountPtr = &depositCount
	}

	fromAddress := c.Query(fromAddressParam)

	networkIDs, err := parseUint32SliceParam(c, networkIDsParam)
	if err != nil {
		b.logger.Warnf("invalid network IDs parameter: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid network_ids: %s", err)})
		return
	}

	ctx, cancel, pageNumber, pageSize, err := b.setupRequest(c, "get_bridges")
	if err != nil {
		b.logger.Warnf(errSetupRequest, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer cancel()

	b.logger.Debugf(
		"fetching bridges (network id=%d, page=%d, size=%d, deposit_count=%v, network_ids=%v, from_address=%s)",
		networkID, pageNumber, pageSize, depositCountPtr, networkIDs, fromAddress)

	var (
		bridges []*bridgesync.Bridge
		count   int
	)

	switch {
	case networkID == mainnetNetworkID:
		bridges, count, err = b.bridgeL1.GetBridgesPaged(ctx, pageNumber, pageSize, depositCountPtr, networkIDs, fromAddress)
		if err != nil {
			b.logger.Errorf("failed to get bridges for L1 network: %v", err)
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("failed to get bridges for the L1 network, error: %s", err)})
			return
		}
	case networkID == b.networkID:
		bridges, count, err = b.bridgeL2.GetBridgesPaged(ctx, pageNumber, pageSize, depositCountPtr, networkIDs, fromAddress)
		if err != nil {
			b.logger.Errorf("failed to get bridges for L2 network (ID=%d): %v", networkID, err)
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("failed to get bridges for the L2 network (ID=%d), error: %s", networkID, err)})
			return
		}
	default:
		b.logger.Warnf(errNetworkID, networkID)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf(errNetworkID, networkID)})
		return
	}

	b.logger.Debugf("successfully retrieved %d bridges for network %d", count, networkID)
	bridgeResponses := aggkitcommon.MapSlice(bridges, NewBridgeResponse)

	c.JSON(http.StatusOK,
		types.BridgesResult{
			Bridges: bridgeResponses,
			Count:   count,
		})
}

// GetClaimsHandler retrieves paginated claims for a given network.
//
// @Summary Get claims
// @Description Returns a paginated list of claims for the specified network.
// @Tags claims
// @Param network_id query uint32 true "Target network ID"
// @Param page_number query uint32 false "Page number (default 1)"
// @Param page_size query uint32 false "Page size (default 100)"
// @Param network_ids query []uint32 false "Filter by one or more network IDs"
// @Param from_address query string false "Filter by from address"
// @Param include_all_fields query bool false "Whether to include full response fields (default false)"
// @Produce json
// @Success 200 {object} types.ClaimsResult
// @Failure 400 {object} types.ErrorResponse "Bad Request"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /claims [get]
func (b *BridgeService) GetClaimsHandler(c *gin.Context) {
	b.logger.Debugf("GetClaims request received (network id=%s, page number=%s, page size=%s, include_all_fields=%s)",
		c.Query(networkIDParam), c.Query(pageNumberParam), c.Query(pageSizeParam), c.Query(includeAllFields))

	networkID, err := parseUintQuery(c, networkIDParam, true, uint32(0))
	if err != nil {
		b.logger.Warnf(errNetworkID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	networkIDs, err := parseUint32SliceParam(c, networkIDsParam)
	if err != nil {
		b.logger.Warnf("invalid network IDs parameter: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fromAddress := c.Query(fromAddressParam)

	// Parse include_all_fields parameter (default to false)
	includeAllFieldsFlag := false
	if includeAllFieldsStr := c.Query(includeAllFields); includeAllFieldsStr != "" {
		includeAllFieldsFlag, err = strconv.ParseBool(includeAllFieldsStr)
		if err != nil {
			b.logger.Warnf("invalid include_all_fields parameter: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid include_all_fields parameter"})
			return
		}
	}

	ctx, cancel, pageNumber, pageSize, err := b.setupRequest(c, "get_claims")
	if err != nil {
		b.logger.Warnf(errSetupRequest, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer cancel()

	b.logger.Debugf(
		"fetching claims (network id=%d, page=%d, size=%d, network_ids=%v, from_address=%s, include_all_fields=%t)",
		networkID, pageNumber, pageSize, networkIDs, fromAddress, includeAllFieldsFlag)

	var (
		claims []*bridgesync.Claim
		count  int
	)

	switch {
	case networkID == mainnetNetworkID:
		claims, count, err = b.bridgeL1.GetClaimsPaged(ctx, pageNumber, pageSize, networkIDs, fromAddress)
		if err != nil {
			b.logger.Warnf("failed to get claims for L1 network: %v", err)
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("failed to get claims for the L1 network, error: %s", err)})
			return
		}
	case networkID == b.networkID:
		claims, count, err = b.bridgeL2.GetClaimsPaged(ctx, pageNumber, pageSize, networkIDs, fromAddress)
		if err != nil {
			b.logger.Warnf("failed to get claims for L2 network (ID=%d): %v", networkID, err)
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("failed to get claims for the L2 network (ID=%d), error: %s", networkID, err)})
			return
		}
	default:
		b.logger.Warnf(errNetworkID, networkID)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf(errNetworkID, networkID)})
		return
	}

	// Use conditional function to create claim responses
	claimResponses := make([]*types.ClaimResponse, len(claims))
	for i, claim := range claims {
		claimResponses[i] = NewClaimResponse(claim, includeAllFieldsFlag)
	}

	c.JSON(http.StatusOK,
		types.ClaimsResult{
			Claims: claimResponses,
			Count:  count,
		})
}

// @Summary Get token mappings
// @Description Returns token mappings for the given network, paginated
// @Tags token-mappings
// @Param network_id query int true "Network ID"
// @Param page_number query int false "Page number"
// @Param page_size query int false "Page size"
// @Produce json
// @Success 200 {object} types.TokenMappingsResult
// @Failure 400 {object} types.ErrorResponse "Bad Request"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /token-mappings [get]
//
//nolint:dupl
func (b *BridgeService) GetTokenMappingsHandler(c *gin.Context) {
	b.logger.Debugf("GetTokenMappings request received (network id=%s, page number=%s, page size=%s)",
		c.Query(networkIDParam), c.Query(pageNumberParam), c.Query(pageSizeParam))

	networkID, err := parseUintQuery(c, networkIDParam, true, uint32(0))
	if err != nil {
		b.logger.Warnf(errNetworkID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel, pageNumber, pageSize, err := b.setupRequest(c, "get_token_mappings")
	if err != nil {
		b.logger.Warnf(errSetupRequest, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer cancel()

	var (
		tokenMappings      []*bridgesync.TokenMapping
		tokenMappingsCount int
	)

	switch {
	case networkID == mainnetNetworkID:
		tokenMappings, tokenMappingsCount, err = b.bridgeL1.GetTokenMappings(ctx, pageNumber, pageSize)
	case b.networkID == networkID:
		tokenMappings, tokenMappingsCount, err = b.bridgeL2.GetTokenMappings(ctx, pageNumber, pageSize)
	default:
		b.logger.Warnf(errNetworkID, networkID)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf(errNetworkID, networkID)})
		return
	}

	if err != nil {
		b.logger.Errorf("failed to fetch token mappings: %v", err)
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": fmt.Sprintf("failed to fetch token mappings: %s", err.Error())})
		return
	}

	tokenMappingResponses := aggkitcommon.MapSlice(tokenMappings, NewTokenMappingResponse)

	c.JSON(http.StatusOK,
		types.TokenMappingsResult{
			TokenMappings: tokenMappingResponses,
			Count:         tokenMappingsCount,
		})
}

// @Summary Get legacy token migrations
// @Description Returns legacy token migrations for the given network, paginated
// @Tags legacy-token-migrations
// @Param network_id query int true "Network ID"
// @Param page_number query int false "Page number"
// @Param page_size query int false "Page size"
// @Produce json
// @Success 200 {object} types.LegacyTokenMigrationsResult
// @Failure 400 {object} types.ErrorResponse "Bad Request"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /legacy-token-migrations [get]
//
//nolint:dupl
func (b *BridgeService) GetLegacyTokenMigrationsHandler(c *gin.Context) {
	b.logger.Debugf("GetLegacyTokenMigrations request received (network id=%s, page number=%s, page size=%s)",
		c.Query(networkIDParam), c.Query(pageNumberParam), c.Query(pageSizeParam))

	networkID, err := parseUintQuery(c, networkIDParam, true, uint32(0))
	if err != nil {
		b.logger.Warnf(errNetworkID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel, pageNumber, pageSize, err := b.setupRequest(c, "get_legacy_token_migrations")
	if err != nil {
		b.logger.Warnf(errSetupRequest, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer cancel()

	var (
		tokenMigrations      []*bridgesync.LegacyTokenMigration
		tokenMigrationsCount int
	)

	switch {
	case networkID == mainnetNetworkID:
		tokenMigrations, tokenMigrationsCount, err = b.bridgeL1.GetLegacyTokenMigrations(ctx, pageNumber, pageSize)
	case b.networkID == networkID:
		tokenMigrations, tokenMigrationsCount, err = b.bridgeL2.GetLegacyTokenMigrations(ctx, pageNumber, pageSize)
	default:
		b.logger.Warnf(errNetworkID, networkID)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf(errNetworkID, networkID)})
		return
	}

	if err != nil {
		b.logger.Errorf("failed to fetch legacy token migrations: %v", err)
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": fmt.Sprintf("failed to fetch legacy token migrations: %s", err.Error())})
		return
	}

	tokenMigrationResponses := aggkitcommon.MapSlice(tokenMigrations, NewTokenMigrationResponse)

	c.JSON(http.StatusOK,
		types.LegacyTokenMigrationsResult{
			TokenMigrations: tokenMigrationResponses,
			Count:           tokenMigrationsCount,
		})
}

// @Summary Get L1 Info Tree index for a bridge
// @Description Returns the first L1 Info Tree index after a given deposit count for the specified network
// @Tags l1-info-tree-leaf
// @Param network_id query int true "Network ID"
// @Param deposit_count query int true "Deposit count"
// @Produce json
// @Success 200 {object} uint32
// @Failure 400 {object} types.ErrorResponse "Bad Request"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /l1-info-tree-index [get]
func (b *BridgeService) L1InfoTreeIndexForBridgeHandler(c *gin.Context) {
	b.logger.Debugf("L1InfoTreeIndexForBridge request received (network id=%s, deposit count=%s)",
		c.Query(networkIDParam), c.Query(depositCountParam))

	networkID, err := parseUintQuery(c, networkIDParam, true, uint32(0))
	if err != nil {
		b.logger.Warnf(errNetworkID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	depositCount, err := parseUintQuery(c, depositCountParam, true, uint32(0))
	if err != nil {
		b.logger.Warnf(errDepositCountParam, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c, b.readTimeout)
	defer cancel()

	cnt, merr := b.meter.Int64Counter("l1_info_tree_index_for_bridge")
	if merr != nil {
		b.logger.Warnf("failed to create l1_info_tree_index_for_bridge counter: %s", merr)
	}
	cnt.Add(ctx, 1)

	var l1InfoTreeIndex uint32

	switch {
	case networkID == mainnetNetworkID:
		l1InfoTreeIndex, err = b.getFirstL1InfoTreeIndexForL1Bridge(ctx, depositCount)
	case b.networkID == networkID:
		l1InfoTreeIndex, err = b.getFirstL1InfoTreeIndexForL2Bridge(ctx, depositCount)
	default:
		b.logger.Warnf(errNetworkID, networkID)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf(errNetworkID, networkID)})
		return
	}

	if err != nil {
		b.logger.Errorf(
			"failed to get L1 info tree index (network id=%d, deposit count=%d): %v",
			networkID,
			depositCount,
			err,
		)
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": fmt.Sprintf("failed to get l1 info tree index for network id %d and deposit count %d, error: %s",
				networkID, depositCount, err)})
		return
	}

	c.JSON(http.StatusOK, l1InfoTreeIndex)
}

// @Summary Get injected L1 info tree leaf after a given L1 info tree index
// @Description Returns the L1 info tree leaf either at the given index (for L1)
// @Description or the first injected global exit root after the given index (for L2).
// @Tags l1-info-tree-leaf
// @Param network_id query int true "Network ID"
// @Param leaf_index query int true "L1 Info Tree Index"
// @Produce json
// @Success 200 {object} types.L1InfoTreeLeafResponse
// @Failure 400 {object} types.ErrorResponse "Bad Request"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /injected-l1-info-leaf [get]
func (b *BridgeService) InjectedL1InfoLeafHandler(c *gin.Context) {
	b.logger.Debugf("InjectedInfoAfterIndex request received (network id=%s, leaf index=%s)",
		c.Query(networkIDParam), c.Query(leafIndexParam))

	networkID, err := parseUintQuery(c, networkIDParam, true, uint32(0))
	if err != nil {
		b.logger.Warnf(errNetworkID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	l1InfoTreeIndex, err := parseUintQuery(c, leafIndexParam, true, uint32(0))
	if err != nil {
		b.logger.Warnf("invalid L1 info tree index parameter: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c, b.readTimeout)
	defer cancel()

	cnt, merr := b.meter.Int64Counter("injected_info_after_index")
	if merr != nil {
		b.logger.Warnf("failed to create injected_info_after_index counter: %s", merr)
	}
	cnt.Add(ctx, 1)

	var l1InfoLeaf *l1infotreesync.L1InfoTreeLeaf

	switch {
	case networkID == mainnetNetworkID:
		l1InfoLeaf, err = b.l1InfoTree.GetInfoByIndex(ctx, l1InfoTreeIndex)
	case b.networkID == networkID:
		e, err := b.injectedGERs.GetFirstGERAfterL1InfoTreeIndex(ctx, l1InfoTreeIndex)
		if err != nil {
			b.logger.Errorf("failed to get injected global exit root for leaf index=%d: %v", l1InfoTreeIndex, err)
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("failed to get injected global exit root for leaf index=%d, error: %s",
					l1InfoTreeIndex, err)})
			return
		}

		l1InfoLeaf, err = b.l1InfoTree.GetInfoByIndex(ctx, e.L1InfoTreeIndex)
		if err != nil {
			b.logger.Errorf("failed to get L1 info tree leaf (leaf index=%d): %v", e.L1InfoTreeIndex, err)
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("failed to get L1 info tree leaf (leaf index=%d), error: %s",
					e.L1InfoTreeIndex, err)})
			return
		}
	default:
		b.logger.Warnf(errNetworkID, networkID)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf(errNetworkID, networkID)})
		return
	}

	if err != nil {
		b.logger.Errorf("failed to get L1 info tree leaf (network id=%d, leaf index=%d): %v", networkID, l1InfoTreeIndex, err)
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": fmt.Sprintf("failed to get L1 info tree leaf (network id=%d, leaf index=%d), error: %s",
				networkID, l1InfoTreeIndex, err)})
		return
	}

	l1InfoLeafResponse := NewL1InfoTreeLeafResponse(l1InfoLeaf)
	c.JSON(http.StatusOK, l1InfoLeafResponse)
}

// ClaimProofHandler returns the Merkle proofs required to verify a claim on the target network.
//
// @Summary Get claim proof
// @Description Returns the Merkle proofs (local and rollup exit root) and
// @Description the corresponding L1 info tree leaf needed to verify a claim.
// @Tags claims
// @Param network_id query uint32 true "Target network ID"
// @Param leaf_index query uint32 true "Index in the L1 info tree"
// @Param deposit_count query uint32 true "Number of deposits in the bridge"
// @Produce json
// @Success 200 {object} types.ClaimProof "Merkle proofs and L1 info tree leaf"
// @Failure 400 {object} types.ErrorResponse "Bad Request"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /claim-proof [get]
func (b *BridgeService) ClaimProofHandler(c *gin.Context) {
	b.logger.Debugf("ClaimProof request received (network id=%s, l1 info tree index=%s, deposit count=%s)",
		c.Query(networkIDParam), c.Query(leafIndexParam), c.Query(depositCountParam))
	ctx, cancel := context.WithTimeout(c, b.readTimeout)
	defer cancel()

	cnt, merr := b.meter.Int64Counter("claim_proof")
	if merr != nil {
		b.logger.Warnf("failed to create claim_proof counter: %s", merr)
	}
	cnt.Add(ctx, 1)

	networkID, err := parseUintQuery(c, networkIDParam, true, uint32(0))
	if err != nil {
		b.logger.Warnf(errNetworkID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	l1InfoTreeIndex, err := parseUintQuery(c, leafIndexParam, true, uint32(0))
	if err != nil {
		b.logger.Warnf("invalid L1 info tree index parameter: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	depositCount, err := parseUintQuery(c, depositCountParam, true, uint32(0))
	if err != nil {
		b.logger.Warnf(errDepositCountParam, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	info, err := b.l1InfoTree.GetInfoByIndex(ctx, l1InfoTreeIndex)
	if err != nil {
		b.logger.Errorf("failed to get L1 info tree leaf for index %d: %v", l1InfoTreeIndex, err)
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": fmt.Sprintf("failed to get l1 info tree leaf for index %d: %s", l1InfoTreeIndex, err)})
		return
	}

	var proofLocalExitRoot tree.Proof
	switch {
	case networkID == mainnetNetworkID:
		proofLocalExitRoot, err = b.bridgeL1.GetProof(ctx, depositCount, info.MainnetExitRoot)
		if err != nil {
			b.logger.Errorf("failed to get local exit proof for L1: %v", err)
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("failed to get local exit proof, error: %s", err)})
			return
		}

	case networkID == b.networkID:
		localExitRoot, err := b.l1InfoTree.GetLocalExitRoot(ctx, networkID, info.RollupExitRoot)
		if err != nil {
			b.logger.Errorf("failed to get local exit root from rollup exit tree: %v", err)
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("failed to get local exit root from rollup exit tree, error: %s", err)})
			return
		}
		proofLocalExitRoot, err = b.bridgeL2.GetProof(ctx, depositCount, localExitRoot)
		if err != nil {
			b.logger.Errorf("failed to get local exit proof for L2: %v", err)
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("failed to get local exit proof, error: %s", err)})
			return
		}

	default:
		b.logger.Warnf("unsupported network id for claim proof: %d", networkID)
		c.JSON(http.StatusBadRequest,
			gin.H{"error": fmt.Sprintf("failed to get claim proof, unsupported network %d", networkID)})
		return
	}

	proofRollupExitRoot, err := b.l1InfoTree.GetRollupExitTreeMerkleProof(ctx, networkID, info.RollupExitRoot)
	if err != nil {
		b.logger.Errorf("failed to get rollup exit proof (network id=%d, leaf index=%d, deposit count=%d): %v",
			networkID, l1InfoTreeIndex, depositCount, err)
		c.JSON(http.StatusInternalServerError,
			gin.H{
				"error": fmt.Sprintf("failed to get rollup exit proof (network id=%d, leaf index=%d, deposit count=%d), error: %s",
					networkID, l1InfoTreeIndex, depositCount, err)})
		return
	}

	infoResponse := NewL1InfoTreeLeafResponse(info)

	c.JSON(http.StatusOK, types.ClaimProof{
		ProofLocalExitRoot:  types.ConvertToProofResponse(proofLocalExitRoot),
		ProofRollupExitRoot: types.ConvertToProofResponse(proofRollupExitRoot),
		L1InfoTreeLeaf:      *infoResponse,
	})
}

// GetLastReorgEventHandler returns the most recent reorganization event for the specified network.
//
// @Summary Get last reorg event
// @Description Retrieves the last known reorg event for either L1 or L2, based on the provided network ID.
// @Tags reorgs
// @Param network_id query int true "Network ID (e.g., 0 for L1, or the ID of the L2 network)"
// @Produce json
// @Success 200 {object} bridgesync.LastReorg "Details of the last reorg event"
// @Failure 400 {object} types.ErrorResponse "Bad Request"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /last-reorg-event [get]
func (b *BridgeService) GetLastReorgEventHandler(c *gin.Context) {
	b.logger.Debugf("GetLastReorgEvent request received (network id=%s)", c.Query(networkIDParam))
	ctx, cancel := context.WithTimeout(c, b.readTimeout)
	defer cancel()

	cnt, merr := b.meter.Int64Counter("last_reorg_event")
	if merr != nil {
		b.logger.Warnf("Failed to create last_reorg_event counter: %s", merr)
	}
	cnt.Add(ctx, 1)

	networkID, err := parseUintQuery(c, networkIDParam, true, uint32(0))
	if err != nil {
		b.logger.Warnf(errNetworkID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var reorgEvent *bridgesync.LastReorg

	switch {
	case networkID == mainnetNetworkID:
		reorgEvent, err = b.bridgeL1.GetLastReorgEvent(ctx)
		if err != nil {
			b.logger.Errorf("failed to get last reorg event for L1 network: %v", err)
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("failed to get last reorg event for the L1 network, error: %s", err)})
			return
		}
	case networkID == b.networkID:
		reorgEvent, err = b.bridgeL2.GetLastReorgEvent(ctx)
		if err != nil {
			b.logger.Errorf("failed to get last reorg event for L2 network (ID=%d): %v", networkID, err)
			c.JSON(http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("failed to get last reorg event for the L2 network (ID=%d), error: %s", networkID, err)})
			return
		}
	default:
		b.logger.Warnf(errNetworkID, networkID)
		c.JSON(http.StatusBadRequest,
			gin.H{"error": fmt.Sprintf("failed to get last reorg event, unsupported network %d", networkID)})
		return
	}

	c.JSON(http.StatusOK, reorgEvent)
}

// GetSyncStatusHandler returns the sync status of the bridge service.
//
// @Summary Get bridge sync status
// @Description Returns the sync status by comparing the deposit count
// @Description from the bridge contract with the deposit count in the bridge sync database for both L1 and L2 networks.
// @Tags sync
// @Produce json
// @Success 200 {object} types.SyncStatus "Bridge sync status for both L1 and L2 networks"
// @Failure 500 {object} types.ErrorResponse "Internal Server Error"
// @Router /sync-status [get]
func (b *BridgeService) GetSyncStatusHandler(c *gin.Context) {
	b.logger.Debugf("GetSyncStatus request received")

	ctx, cancel := context.WithTimeout(c, b.readTimeout)
	defer cancel()

	cnt, merr := b.meter.Int64Counter("get_sync_status")
	if merr != nil {
		b.logger.Warnf("failed to create get_sync_status counter: %s", merr)
	}
	cnt.Add(ctx, 1)

	var syncStatus types.SyncStatus
	syncStatus.L1Info = &types.NetworkSyncInfo{}
	syncStatus.L2Info = &types.NetworkSyncInfo{}

	// Check L1 sync status
	l1ContractDepositCount, err := b.bridgeL1.GetContractDepositCount(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": fmt.Sprintf("failed to get deposit count from L1 bridge contract: %s", err)})
		return
	}

	// Get the last bridge from L1 database
	_, bridgesCount, err := b.bridgeL1.GetBridgesPaged(ctx, 1, 1, nil, nil, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": fmt.Sprintf("failed to get bridges from L1 database: %s", err)})
		return
	}

	syncStatus.L1Info.BridgeDepositCount = uint32(bridgesCount)
	syncStatus.L1Info.ContractDepositCount = l1ContractDepositCount
	syncStatus.L1Info.IsSynced = syncStatus.L1Info.ContractDepositCount == syncStatus.L1Info.BridgeDepositCount

	// Check L2 sync status
	l2ContractDepositCount, err := b.bridgeL2.GetContractDepositCount(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": fmt.Sprintf("failed to get deposit count from L2 bridge contract: %s", err)})
		return
	}

	// Get the last bridge from L2 database
	_, bridgesCount, err = b.bridgeL2.GetBridgesPaged(ctx, 1, 1, nil, nil, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": fmt.Sprintf("failed to get bridges from L2 database: %s", err)})
		return
	}

	syncStatus.L2Info.BridgeDepositCount = uint32(bridgesCount)
	syncStatus.L2Info.ContractDepositCount = l2ContractDepositCount
	syncStatus.L2Info.IsSynced = syncStatus.L2Info.ContractDepositCount == syncStatus.L2Info.BridgeDepositCount

	c.JSON(http.StatusOK, syncStatus)
}

func (b *BridgeService) getFirstL1InfoTreeIndexForL1Bridge(ctx context.Context, depositCount uint32) (uint32, error) {
	lastInfo, err := b.l1InfoTree.GetLastInfo()
	if err != nil {
		return 0, err
	}

	root, err := b.bridgeL1.GetRootByLER(ctx, lastInfo.MainnetExitRoot)
	if err != nil {
		return 0, err
	}
	if root.Index < depositCount {
		return 0, ErrNotOnL1Info
	}

	firstInfo, err := b.l1InfoTree.GetFirstInfo()
	if err != nil {
		return 0, err
	}

	// Binary search between the first and last blocks where L1 info tree was updated.
	// Find the smallest l1 info tree index that is greater than depositCount and matches with
	// a MER that is included on the l1 info tree
	bestResult := lastInfo
	lowerLimit := firstInfo.BlockNumber
	upperLimit := lastInfo.BlockNumber
	for lowerLimit <= upperLimit {
		targetBlock := lowerLimit + ((upperLimit - lowerLimit) / binarySearchDivider)
		targetInfo, err := b.l1InfoTree.GetFirstInfoAfterBlock(targetBlock)
		if err != nil {
			return 0, err
		}
		root, err := b.bridgeL1.GetRootByLER(ctx, targetInfo.MainnetExitRoot)
		if err != nil {
			return 0, err
		}
		if root.Index < depositCount {
			lowerLimit = targetBlock + 1
		} else if root.Index == depositCount {
			bestResult = targetInfo
			break
		} else {
			bestResult = targetInfo
			upperLimit = targetBlock - 1
		}
	}

	return bestResult.L1InfoTreeIndex, nil
}

func (b *BridgeService) getFirstL1InfoTreeIndexForL2Bridge(ctx context.Context, depositCount uint32) (uint32, error) {
	// NOTE: this code assumes that all the rollup exit roots
	// (produced by the smart contract call verifyBatches / verifyBatchesTrustedAggregator)
	// are included in the L1 info tree. As per the current implementation (smart contracts) of the protocol
	// this is true. This could change in the future
	lastVerified, err := b.l1InfoTree.GetLastVerifiedBatches(b.networkID)
	if err != nil {
		return 0, err
	}

	root, err := b.bridgeL2.GetRootByLER(ctx, lastVerified.ExitRoot)
	if err != nil {
		return 0, err
	}
	if root.Index < depositCount {
		return 0, ErrNotOnL1Info
	}

	firstVerified, err := b.l1InfoTree.GetFirstVerifiedBatches(b.networkID)
	if err != nil {
		return 0, err
	}

	// Binary search between the first and last blocks where batches were verified.
	// Find the smallest deposit count that is greater than depositCount and matches with
	// a LER that is verified
	bestResult := lastVerified
	lowerLimit := firstVerified.BlockNumber
	upperLimit := lastVerified.BlockNumber
	for lowerLimit <= upperLimit {
		targetBlock := lowerLimit + ((upperLimit - lowerLimit) / binarySearchDivider)
		targetVerified, err := b.l1InfoTree.GetFirstVerifiedBatchesAfterBlock(b.networkID, targetBlock)
		if err != nil {
			return 0, err
		}
		root, err = b.bridgeL2.GetRootByLER(ctx, targetVerified.ExitRoot)
		if err != nil {
			return 0, err
		}
		if root.Index < depositCount {
			lowerLimit = targetBlock + 1
		} else if root.Index == depositCount {
			bestResult = targetVerified
			break
		} else {
			bestResult = targetVerified
			upperLimit = targetBlock - 1
		}
	}

	info, err := b.l1InfoTree.GetFirstL1InfoWithRollupExitRoot(bestResult.RollupExitRoot)
	if err != nil {
		return 0, err
	}
	return info.L1InfoTreeIndex, nil
}

// setupRequest parses the pagination parameters from the request context
func (b *BridgeService) setupRequest(
	c *gin.Context,
	counterName string) (context.Context, context.CancelFunc, uint32, uint32, error) {
	pageNumber, err := parseUintQuery(c, pageNumberParam, false, DefaultPage)
	if err != nil {
		return nil, nil, 0, 0, err
	}

	pageSize, err := parseUintQuery(c, pageSizeParam, false, DefaultPageSize)
	if err != nil {
		return nil, nil, 0, 0, err
	}

	err = validatePaginationParams(pageNumber, pageSize)
	if err != nil {
		return nil, nil, 0, 0, err
	}

	ctx, cancel := context.WithTimeout(c, b.readTimeout)
	counter, merr := b.meter.Int64Counter(counterName)
	if merr != nil {
		b.logger.Warnf("failed to create %s counter: %s", counterName, merr)
	}
	counter.Add(ctx, 1)

	return ctx, cancel, pageNumber, pageSize, nil
}
