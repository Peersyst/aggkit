// Code generated by mockery. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	treetypes "github.com/agglayer/aggkit/tree/types"

	types "github.com/agglayer/aggkit/aggsender/types"
)

// AggchainProofFlow is an autogenerated mock type for the AggchainProofFlow type
type AggchainProofFlow struct {
	mock.Mock
}

type AggchainProofFlow_Expecter struct {
	mock *mock.Mock
}

func (_m *AggchainProofFlow) EXPECT() *AggchainProofFlow_Expecter {
	return &AggchainProofFlow_Expecter{mock: &_m.Mock}
}

// GenerateAggchainProof provides a mock function with given fields: ctx, lastProvenBlock, toBlock, certBuildParams
func (_m *AggchainProofFlow) GenerateAggchainProof(ctx context.Context, lastProvenBlock uint64, toBlock uint64, certBuildParams *types.CertificateBuildParams) (*types.AggchainProof, *treetypes.Root, error) {
	ret := _m.Called(ctx, lastProvenBlock, toBlock, certBuildParams)

	if len(ret) == 0 {
		panic("no return value specified for GenerateAggchainProof")
	}

	var r0 *types.AggchainProof
	var r1 *treetypes.Root
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, uint64, uint64, *types.CertificateBuildParams) (*types.AggchainProof, *treetypes.Root, error)); ok {
		return rf(ctx, lastProvenBlock, toBlock, certBuildParams)
	}
	if rf, ok := ret.Get(0).(func(context.Context, uint64, uint64, *types.CertificateBuildParams) *types.AggchainProof); ok {
		r0 = rf(ctx, lastProvenBlock, toBlock, certBuildParams)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.AggchainProof)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, uint64, uint64, *types.CertificateBuildParams) *treetypes.Root); ok {
		r1 = rf(ctx, lastProvenBlock, toBlock, certBuildParams)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*treetypes.Root)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, uint64, uint64, *types.CertificateBuildParams) error); ok {
		r2 = rf(ctx, lastProvenBlock, toBlock, certBuildParams)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// AggchainProofFlow_GenerateAggchainProof_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GenerateAggchainProof'
type AggchainProofFlow_GenerateAggchainProof_Call struct {
	*mock.Call
}

// GenerateAggchainProof is a helper method to define mock.On call
//   - ctx context.Context
//   - lastProvenBlock uint64
//   - toBlock uint64
//   - certBuildParams *types.CertificateBuildParams
func (_e *AggchainProofFlow_Expecter) GenerateAggchainProof(ctx interface{}, lastProvenBlock interface{}, toBlock interface{}, certBuildParams interface{}) *AggchainProofFlow_GenerateAggchainProof_Call {
	return &AggchainProofFlow_GenerateAggchainProof_Call{Call: _e.mock.On("GenerateAggchainProof", ctx, lastProvenBlock, toBlock, certBuildParams)}
}

func (_c *AggchainProofFlow_GenerateAggchainProof_Call) Run(run func(ctx context.Context, lastProvenBlock uint64, toBlock uint64, certBuildParams *types.CertificateBuildParams)) *AggchainProofFlow_GenerateAggchainProof_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(uint64), args[2].(uint64), args[3].(*types.CertificateBuildParams))
	})
	return _c
}

func (_c *AggchainProofFlow_GenerateAggchainProof_Call) Return(_a0 *types.AggchainProof, _a1 *treetypes.Root, _a2 error) *AggchainProofFlow_GenerateAggchainProof_Call {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *AggchainProofFlow_GenerateAggchainProof_Call) RunAndReturn(run func(context.Context, uint64, uint64, *types.CertificateBuildParams) (*types.AggchainProof, *treetypes.Root, error)) *AggchainProofFlow_GenerateAggchainProof_Call {
	_c.Call.Return(run)
	return _c
}

// NewAggchainProofFlow creates a new instance of AggchainProofFlow. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewAggchainProofFlow(t interface {
	mock.TestingT
	Cleanup(func())
}) *AggchainProofFlow {
	mock := &AggchainProofFlow{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
