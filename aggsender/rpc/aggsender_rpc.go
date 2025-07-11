package aggsenderrpc

import (
	"fmt"

	"github.com/0xPolygon/cdk-rpc/rpc"
	"github.com/agglayer/aggkit/aggsender/types"
	"github.com/agglayer/aggkit/log"
)

type AggsenderStorer interface {
	GetCertificateByHeight(height uint64) (*types.Certificate, error)
	GetLastSentCertificate() (*types.Certificate, error)
}

type AggsenderInterface interface {
	Info() types.AggsenderInfo
}

// AggsenderRPC is the RPC interface for the aggsender
type AggsenderRPC struct {
	logger    *log.Logger
	storage   AggsenderStorer
	aggsender AggsenderInterface
}

func NewAggsenderRPC(
	logger *log.Logger,
	storage AggsenderStorer,
	aggsender AggsenderInterface,
) *AggsenderRPC {
	return &AggsenderRPC{
		logger:    logger,
		storage:   storage,
		aggsender: aggsender,
	}
}

// Status returns the status of the aggsender
// curl -X POST http://localhost:5576/ "Content-Type: application/json" \
// -d '{"method":"aggsender_status", "params":[], "id":1}'
func (b *AggsenderRPC) Status() (interface{}, rpc.Error) {
	info := b.aggsender.Info()
	return info, nil
}

// GetCertificateHeaderPerHeight returns the certificate header for the given height
// if param is `nil` it returns the last sent certificate
// latest:
//
//	curl -X POST http://localhost:5576/ -H "Content-Type: application/json" \
//	 -d '{"method":"aggsender_getCertificateHeaderPerHeight", "params":[], "id":1}'
//
// specific height:
//
// curl -X POST http://localhost:5576/ -H "Content-Type: application/json" \
// -d '{"method":"aggsender_getCertificateHeaderPerHeight", "params":[$height], "id":1}'
func (b *AggsenderRPC) GetCertificateHeaderPerHeight(height *uint64) (interface{}, rpc.Error) {
	var (
		cert *types.Certificate
		err  error
	)
	if height == nil {
		cert, err = b.storage.GetLastSentCertificate()
	} else {
		cert, err = b.storage.GetCertificateByHeight(*height)
	}
	if err != nil {
		return nil, rpc.NewRPCError(rpc.DefaultErrorCode, fmt.Sprintf("error getting certificate by height: %v", err))
	}
	if cert == nil {
		return nil, rpc.NewRPCError(rpc.NotFoundErrorCode, "certificate not found")
	}

	return cert, nil
}
