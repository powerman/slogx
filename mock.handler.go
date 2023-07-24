// Code generated by MockGen. DO NOT EDIT.
// Source: XXX/log/slog/handler.go

// Package slogx is a generated GoMock package.
package slogx

import (
	context "context"
	reflect "reflect"
	slog "log/slog"

	gomock "github.com/golang/mock/gomock"
)

// MockHandler is a mock of Handler interface.
type MockHandler struct {
	ctrl     *gomock.Controller
	recorder *MockHandlerMockRecorder
}

// MockHandlerMockRecorder is the mock recorder for MockHandler.
type MockHandlerMockRecorder struct {
	mock *MockHandler
}

// NewMockHandler creates a new mock instance.
func NewMockHandler(ctrl *gomock.Controller) *MockHandler {
	mock := &MockHandler{ctrl: ctrl}
	mock.recorder = &MockHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHandler) EXPECT() *MockHandlerMockRecorder {
	return m.recorder
}

// Enabled mocks base method.
func (m *MockHandler) Enabled(arg0 context.Context, arg1 slog.Level) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Enabled", arg0, arg1)
	ret0, _ := ret[0].(bool)
	return ret0
}

// Enabled indicates an expected call of Enabled.
func (mr *MockHandlerMockRecorder) Enabled(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Enabled", reflect.TypeOf((*MockHandler)(nil).Enabled), arg0, arg1)
}

// Handle mocks base method.
func (m *MockHandler) Handle(arg0 context.Context, arg1 slog.Record) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Handle", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Handle indicates an expected call of Handle.
func (mr *MockHandlerMockRecorder) Handle(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Handle", reflect.TypeOf((*MockHandler)(nil).Handle), arg0, arg1)
}

// WithAttrs mocks base method.
func (m *MockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WithAttrs", attrs)
	ret0, _ := ret[0].(slog.Handler)
	return ret0
}

// WithAttrs indicates an expected call of WithAttrs.
func (mr *MockHandlerMockRecorder) WithAttrs(attrs interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithAttrs", reflect.TypeOf((*MockHandler)(nil).WithAttrs), attrs)
}

// WithGroup mocks base method.
func (m *MockHandler) WithGroup(name string) slog.Handler {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WithGroup", name)
	ret0, _ := ret[0].(slog.Handler)
	return ret0
}

// WithGroup indicates an expected call of WithGroup.
func (mr *MockHandlerMockRecorder) WithGroup(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithGroup", reflect.TypeOf((*MockHandler)(nil).WithGroup), name)
}
