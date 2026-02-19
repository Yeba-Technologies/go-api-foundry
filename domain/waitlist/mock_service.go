package waitlist

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

type MockWaitlistService struct {
	ctrl     *gomock.Controller
	recorder *MockWaitlistServiceMockRecorder
}

type MockWaitlistServiceMockRecorder struct {
	mock *MockWaitlistService
}

func NewMockWaitlistService(ctrl *gomock.Controller) *MockWaitlistService {
	mock := &MockWaitlistService{ctrl: ctrl}
	mock.recorder = &MockWaitlistServiceMockRecorder{mock}
	return mock
}

func (m *MockWaitlistService) EXPECT() *MockWaitlistServiceMockRecorder {
	return m.recorder
}

func (m *MockWaitlistService) CreateEntry(ctx context.Context, req *CreateWaitlistEntryRequest) (*WaitlistEntryResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateEntry", ctx, req)
	ret0, _ := ret[0].(*WaitlistEntryResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockWaitlistServiceMockRecorder) CreateEntry(ctx, req any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateEntry", reflect.TypeOf((*MockWaitlistService)(nil).CreateEntry), ctx, req)
}

func (m *MockWaitlistService) FindEntryByID(ctx context.Context, id uint) (*WaitlistEntryResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindEntryByID", ctx, id)
	ret0, _ := ret[0].(*WaitlistEntryResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockWaitlistServiceMockRecorder) FindEntryByID(ctx, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindEntryByID", reflect.TypeOf((*MockWaitlistService)(nil).FindEntryByID), ctx, id)
}

func (m *MockWaitlistService) UpdateEntry(ctx context.Context, id uint, req *UpdateWaitlistEntryRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateEntry", ctx, id, req)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockWaitlistServiceMockRecorder) UpdateEntry(ctx, id, req any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateEntry", reflect.TypeOf((*MockWaitlistService)(nil).UpdateEntry), ctx, id, req)
}

func (m *MockWaitlistService) GetAllEntries(ctx context.Context) ([]WaitlistEntryResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllEntries", ctx)
	ret0, _ := ret[0].([]WaitlistEntryResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockWaitlistServiceMockRecorder) GetAllEntries(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllEntries", reflect.TypeOf((*MockWaitlistService)(nil).GetAllEntries), ctx)
}

func (m *MockWaitlistService) DeleteEntry(ctx context.Context, id uint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteEntry", ctx, id)
	ret0, _ := ret[0].(error)
	return ret0
}

func (mr *MockWaitlistServiceMockRecorder) DeleteEntry(ctx, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteEntry", reflect.TypeOf((*MockWaitlistService)(nil).DeleteEntry), ctx, id)
}
