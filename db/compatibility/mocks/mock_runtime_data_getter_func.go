// Code generated by mockery. DO NOT EDIT.

package mocks

import (
	context "context"

	types "github.com/agglayer/aggkit/db/compatibility/types"
	mock "github.com/stretchr/testify/mock"
)

// RuntimeDataGetterFunc is an autogenerated mock type for the RuntimeDataGetterFunc type
type RuntimeDataGetterFunc[T types.CompatibilityComparer[T]] struct {
	mock.Mock
}

type RuntimeDataGetterFunc_Expecter[T types.CompatibilityComparer[T]] struct {
	mock *mock.Mock
}

func (_m *RuntimeDataGetterFunc[T]) EXPECT() *RuntimeDataGetterFunc_Expecter[T] {
	return &RuntimeDataGetterFunc_Expecter[T]{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: ctx
func (_m *RuntimeDataGetterFunc[T]) Execute(ctx context.Context) (T, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 T
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (T, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) T); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(T)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RuntimeDataGetterFunc_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type RuntimeDataGetterFunc_Execute_Call[T types.CompatibilityComparer[T]] struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - ctx context.Context
func (_e *RuntimeDataGetterFunc_Expecter[T]) Execute(ctx interface{}) *RuntimeDataGetterFunc_Execute_Call[T] {
	return &RuntimeDataGetterFunc_Execute_Call[T]{Call: _e.mock.On("Execute", ctx)}
}

func (_c *RuntimeDataGetterFunc_Execute_Call[T]) Run(run func(ctx context.Context)) *RuntimeDataGetterFunc_Execute_Call[T] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *RuntimeDataGetterFunc_Execute_Call[T]) Return(_a0 T, _a1 error) *RuntimeDataGetterFunc_Execute_Call[T] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *RuntimeDataGetterFunc_Execute_Call[T]) RunAndReturn(run func(context.Context) (T, error)) *RuntimeDataGetterFunc_Execute_Call[T] {
	_c.Call.Return(run)
	return _c
}

// NewRuntimeDataGetterFunc creates a new instance of RuntimeDataGetterFunc. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewRuntimeDataGetterFunc[T types.CompatibilityComparer[T]](t interface {
	mock.TestingT
	Cleanup(func())
}) *RuntimeDataGetterFunc[T] {
	mock := &RuntimeDataGetterFunc[T]{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
