// Automatically generated by MockGen. DO NOT EDIT!
// Source: github.com/tus/tusd/pkg/gcsstore (interfaces: GCSReader,GCSAPI)

package gcsstore_test

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	gcsstore "github.com/tus/tusd/pkg/gcsstore"
	io "io"
)

// Mock of GCSReader interface
type MockGCSReader struct {
	ctrl     *gomock.Controller
	recorder *_MockGCSReaderRecorder
}

// Recorder for MockGCSReader (not exported)
type _MockGCSReaderRecorder struct {
	mock *MockGCSReader
}

func NewMockGCSReader(ctrl *gomock.Controller) *MockGCSReader {
	mock := &MockGCSReader{ctrl: ctrl}
	mock.recorder = &_MockGCSReaderRecorder{mock}
	return mock
}

func (_m *MockGCSReader) EXPECT() *_MockGCSReaderRecorder {
	return _m.recorder
}

func (_m *MockGCSReader) Close() error {
	ret := _m.ctrl.Call(_m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockGCSReaderRecorder) Close() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Close")
}

func (_m *MockGCSReader) ContentType() string {
	ret := _m.ctrl.Call(_m, "ContentType")
	ret0, _ := ret[0].(string)
	return ret0
}

func (_mr *_MockGCSReaderRecorder) ContentType() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "ContentType")
}

func (_m *MockGCSReader) Read(_param0 []byte) (int, error) {
	ret := _m.ctrl.Call(_m, "Read", _param0)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockGCSReaderRecorder) Read(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Read", arg0)
}

func (_m *MockGCSReader) Remain() int64 {
	ret := _m.ctrl.Call(_m, "Remain")
	ret0, _ := ret[0].(int64)
	return ret0
}

func (_mr *_MockGCSReaderRecorder) Remain() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Remain")
}

func (_m *MockGCSReader) Size() int64 {
	ret := _m.ctrl.Call(_m, "Size")
	ret0, _ := ret[0].(int64)
	return ret0
}

func (_mr *_MockGCSReaderRecorder) Size() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Size")
}

// Mock of GCSAPI interface
type MockGCSAPI struct {
	ctrl     *gomock.Controller
	recorder *_MockGCSAPIRecorder
}

// Recorder for MockGCSAPI (not exported)
type _MockGCSAPIRecorder struct {
	mock *MockGCSAPI
}

func NewMockGCSAPI(ctrl *gomock.Controller) *MockGCSAPI {
	mock := &MockGCSAPI{ctrl: ctrl}
	mock.recorder = &_MockGCSAPIRecorder{mock}
	return mock
}

func (_m *MockGCSAPI) EXPECT() *_MockGCSAPIRecorder {
	return _m.recorder
}

func (_m *MockGCSAPI) ComposeObjects(_param0 context.Context, _param1 gcsstore.GCSComposeParams) error {
	ret := _m.ctrl.Call(_m, "ComposeObjects", _param0, _param1)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockGCSAPIRecorder) ComposeObjects(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "ComposeObjects", arg0, arg1)
}

func (_m *MockGCSAPI) DeleteObject(_param0 context.Context, _param1 gcsstore.GCSObjectParams) error {
	ret := _m.ctrl.Call(_m, "DeleteObject", _param0, _param1)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockGCSAPIRecorder) DeleteObject(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "DeleteObject", arg0, arg1)
}

func (_m *MockGCSAPI) DeleteObjectsWithFilter(_param0 context.Context, _param1 gcsstore.GCSFilterParams) error {
	ret := _m.ctrl.Call(_m, "DeleteObjectsWithFilter", _param0, _param1)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockGCSAPIRecorder) DeleteObjectsWithFilter(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "DeleteObjectsWithFilter", arg0, arg1)
}

func (_m *MockGCSAPI) FilterObjects(_param0 context.Context, _param1 gcsstore.GCSFilterParams) ([]string, error) {
	ret := _m.ctrl.Call(_m, "FilterObjects", _param0, _param1)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockGCSAPIRecorder) FilterObjects(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "FilterObjects", arg0, arg1)
}

func (_m *MockGCSAPI) GetObjectSize(_param0 context.Context, _param1 gcsstore.GCSObjectParams) (int64, error) {
	ret := _m.ctrl.Call(_m, "GetObjectSize", _param0, _param1)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockGCSAPIRecorder) GetObjectSize(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "GetObjectSize", arg0, arg1)
}

func (_m *MockGCSAPI) ReadObject(_param0 context.Context, _param1 gcsstore.GCSObjectParams) (gcsstore.GCSReader, error) {
	ret := _m.ctrl.Call(_m, "ReadObject", _param0, _param1)
	ret0, _ := ret[0].(gcsstore.GCSReader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockGCSAPIRecorder) ReadObject(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "ReadObject", arg0, arg1)
}

func (_m *MockGCSAPI) SetObjectMetadata(_param0 context.Context, _param1 gcsstore.GCSObjectParams, _param2 map[string]string) error {
	ret := _m.ctrl.Call(_m, "SetObjectMetadata", _param0, _param1, _param2)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockGCSAPIRecorder) SetObjectMetadata(arg0, arg1, arg2 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "SetObjectMetadata", arg0, arg1, arg2)
}

func (_m *MockGCSAPI) WriteObject(_param0 context.Context, _param1 gcsstore.GCSObjectParams, _param2 io.Reader) (int64, error) {
	ret := _m.ctrl.Call(_m, "WriteObject", _param0, _param1, _param2)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockGCSAPIRecorder) WriteObject(arg0, arg1, arg2 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "WriteObject", arg0, arg1, arg2)
}
