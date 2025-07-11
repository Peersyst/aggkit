package rpcclient

import (
	"encoding/json"
	"testing"

	"github.com/0xPolygon/cdk-rpc/rpc"
	"github.com/agglayer/aggkit/aggsender/types"
	"github.com/stretchr/testify/require"
)

func TestGetCertificateHeaderPerHeight(t *testing.T) {
	sut := NewClient("url")
	height := uint64(1)
	responseCert := types.Certificate{Header: &types.CertificateHeader{}}
	responseCertJSON, err := json.Marshal(responseCert)
	require.NoError(t, err)
	response := rpc.Response{
		Result: responseCertJSON,
	}
	jSONRPCCall = func(_, _ string, _ ...interface{}) (rpc.Response, error) {
		return response, nil
	}
	cert, err := sut.GetCertificateHeaderPerHeight(&height)
	require.NoError(t, err)
	require.NotNil(t, cert)
	require.Equal(t, responseCert, *cert)
}

func TestGetStatus(t *testing.T) {
	sut := NewClient("url")
	responseData := types.AggsenderInfo{}
	responseDataJSON, err := json.Marshal(responseData)
	require.NoError(t, err)
	response := rpc.Response{
		Result: responseDataJSON,
	}
	jSONRPCCall = func(_, _ string, _ ...interface{}) (rpc.Response, error) {
		return response, nil
	}
	result, err := sut.GetStatus()
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, responseData, *result)
}
