// Code generated by mockery. DO NOT EDIT.

package mocks

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"

	proverv1 "buf.build/gen/go/agglayer/provers/protocolbuffers/go/aggkit/prover/v1"
)

// AggchainProofServiceClient is an autogenerated mock type for the AggchainProofServiceClient type
type AggchainProofServiceClient struct {
	mock.Mock
}

type AggchainProofServiceClient_Expecter struct {
	mock *mock.Mock
}

func (_m *AggchainProofServiceClient) EXPECT() *AggchainProofServiceClient_Expecter {
	return &AggchainProofServiceClient_Expecter{mock: &_m.Mock}
}

// GenerateAggchainProof provides a mock function with given fields: ctx, in, opts
func (_m *AggchainProofServiceClient) GenerateAggchainProof(ctx context.Context, in *proverv1.GenerateAggchainProofRequest, opts ...grpc.CallOption) (*proverv1.GenerateAggchainProofResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for GenerateAggchainProof")
	}

	var r0 *proverv1.GenerateAggchainProofResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *proverv1.GenerateAggchainProofRequest, ...grpc.CallOption) (*proverv1.GenerateAggchainProofResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *proverv1.GenerateAggchainProofRequest, ...grpc.CallOption) *proverv1.GenerateAggchainProofResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*proverv1.GenerateAggchainProofResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *proverv1.GenerateAggchainProofRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// AggchainProofServiceClient_GenerateAggchainProof_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GenerateAggchainProof'
type AggchainProofServiceClient_GenerateAggchainProof_Call struct {
	*mock.Call
}

// GenerateAggchainProof is a helper method to define mock.On call
//   - ctx context.Context
//   - in *proverv1.GenerateAggchainProofRequest
//   - opts ...grpc.CallOption
func (_e *AggchainProofServiceClient_Expecter) GenerateAggchainProof(ctx interface{}, in interface{}, opts ...interface{}) *AggchainProofServiceClient_GenerateAggchainProof_Call {
	return &AggchainProofServiceClient_GenerateAggchainProof_Call{Call: _e.mock.On("GenerateAggchainProof",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *AggchainProofServiceClient_GenerateAggchainProof_Call) Run(run func(ctx context.Context, in *proverv1.GenerateAggchainProofRequest, opts ...grpc.CallOption)) *AggchainProofServiceClient_GenerateAggchainProof_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*proverv1.GenerateAggchainProofRequest), variadicArgs...)
	})
	return _c
}

func (_c *AggchainProofServiceClient_GenerateAggchainProof_Call) Return(_a0 *proverv1.GenerateAggchainProofResponse, _a1 error) *AggchainProofServiceClient_GenerateAggchainProof_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *AggchainProofServiceClient_GenerateAggchainProof_Call) RunAndReturn(run func(context.Context, *proverv1.GenerateAggchainProofRequest, ...grpc.CallOption) (*proverv1.GenerateAggchainProofResponse, error)) *AggchainProofServiceClient_GenerateAggchainProof_Call {
	_c.Call.Return(run)
	return _c
}

// GenerateOptimisticAggchainProof provides a mock function with given fields: ctx, in, opts
func (_m *AggchainProofServiceClient) GenerateOptimisticAggchainProof(ctx context.Context, in *proverv1.GenerateOptimisticAggchainProofRequest, opts ...grpc.CallOption) (*proverv1.GenerateOptimisticAggchainProofResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for GenerateOptimisticAggchainProof")
	}

	var r0 *proverv1.GenerateOptimisticAggchainProofResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *proverv1.GenerateOptimisticAggchainProofRequest, ...grpc.CallOption) (*proverv1.GenerateOptimisticAggchainProofResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *proverv1.GenerateOptimisticAggchainProofRequest, ...grpc.CallOption) *proverv1.GenerateOptimisticAggchainProofResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*proverv1.GenerateOptimisticAggchainProofResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *proverv1.GenerateOptimisticAggchainProofRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// AggchainProofServiceClient_GenerateOptimisticAggchainProof_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GenerateOptimisticAggchainProof'
type AggchainProofServiceClient_GenerateOptimisticAggchainProof_Call struct {
	*mock.Call
}

// GenerateOptimisticAggchainProof is a helper method to define mock.On call
//   - ctx context.Context
//   - in *proverv1.GenerateOptimisticAggchainProofRequest
//   - opts ...grpc.CallOption
func (_e *AggchainProofServiceClient_Expecter) GenerateOptimisticAggchainProof(ctx interface{}, in interface{}, opts ...interface{}) *AggchainProofServiceClient_GenerateOptimisticAggchainProof_Call {
	return &AggchainProofServiceClient_GenerateOptimisticAggchainProof_Call{Call: _e.mock.On("GenerateOptimisticAggchainProof",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *AggchainProofServiceClient_GenerateOptimisticAggchainProof_Call) Run(run func(ctx context.Context, in *proverv1.GenerateOptimisticAggchainProofRequest, opts ...grpc.CallOption)) *AggchainProofServiceClient_GenerateOptimisticAggchainProof_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*proverv1.GenerateOptimisticAggchainProofRequest), variadicArgs...)
	})
	return _c
}

func (_c *AggchainProofServiceClient_GenerateOptimisticAggchainProof_Call) Return(_a0 *proverv1.GenerateOptimisticAggchainProofResponse, _a1 error) *AggchainProofServiceClient_GenerateOptimisticAggchainProof_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *AggchainProofServiceClient_GenerateOptimisticAggchainProof_Call) RunAndReturn(run func(context.Context, *proverv1.GenerateOptimisticAggchainProofRequest, ...grpc.CallOption) (*proverv1.GenerateOptimisticAggchainProofResponse, error)) *AggchainProofServiceClient_GenerateOptimisticAggchainProof_Call {
	_c.Call.Return(run)
	return _c
}

// NewAggchainProofServiceClient creates a new instance of AggchainProofServiceClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewAggchainProofServiceClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *AggchainProofServiceClient {
	mock := &AggchainProofServiceClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
