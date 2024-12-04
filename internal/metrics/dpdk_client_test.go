package metrics

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type MockConn struct {
	mock.Mock
	net.Conn
}

func (mc *MockConn) SetWriteDeadline(t time.Time) error {
	return mc.Called(t).Error(0)
}

func (mc *MockConn) Write(c []byte) (int, error) {
	args := mc.Called(c)
	return args.Int(0), args.Error(1)
}

func (mc *MockConn) SetReadDeadline(t time.Time) error {
	return mc.Called(t).Error(0)
}

func (mc *MockConn) Read(buf []byte) (int, error) {
	args := mc.Called(buf)
	copy(buf, args.Get(2).([]byte))
	return args.Int(0), args.Error(1)
}

func (mc *MockConn) Close() error {
	return mc.Called().Error(0)
}

type metricsMap map[uint]telemetryResult

var (
	busynessOutputString = `{
		"/eal/lcore/busyness": {
			"1": 65,
			"2": 42,
			"3": -1
		}
	}`
)

func createNewDPDKTelemetryClient() dpdkTelemetryClientImpl {
	log.SetLogger(zap.New(
		zap.UseDevMode(true),
		func(opts *zap.Options) {
			opts.TimeEncoder = zapcore.ISO8601TimeEncoder
		},
	))

	return dpdkTelemetryClientImpl{
		log: ctrl.Log.WithName("test-log"),
	}
}

func TestDPDKTelemetryClient_CreateConnection(t *testing.T) {
	expectedUID := "foo"
	usedUID := ""
	expectedCPUList := []uint{0, 1, 2}
	usedCPUList := []uint{}

	t.Cleanup(func() {
		testHookNewSocketConnection = nil
	})
	testHookNewSocketConnection = func(conn *dpdkTelemetryConnection) {
		usedUID = conn.podUID
		usedCPUList = conn.watchedCPUs
	}
	newConnData := &DPDKTelemetryConnectionData{
		PodUID:      expectedUID,
		WatchedCPUs: expectedCPUList,
	}

	cl := createNewDPDKTelemetryClient()

	cl.CreateConnection(newConnData)
	assert.Equal(t, expectedUID, usedUID)
	assert.Equal(t, expectedCPUList, usedCPUList)
}

func TestDPDKTelemetryClient_ListConnections(t *testing.T) {
	expectedUID := "foo"
	expectedCPUList := []uint{0, 1, 2, 3}
	dummyConn := &dpdkTelemetryConnection{
		podUID: expectedUID,
	}
	dummyConn.watchedCPUs = expectedCPUList

	cl := createNewDPDKTelemetryClient()
	cl.connections.Store(expectedUID, dummyConn)

	connData := cl.ListConnections()

	assert.NotEmpty(t, connData)
	assert.Equal(t, expectedUID, connData[0].PodUID)
	assert.Equal(t, expectedCPUList, connData[0].WatchedCPUs)
}

func TestDPDKTelemetryClient_CloseConnection(t *testing.T) {
	uid := "foo"
	closeCallled := false
	t.Cleanup(func() {
		testHookCloseConnection = nil
	})
	testHookCloseConnection = func() {
		closeCallled = true
	}
	dummyConn := &dpdkTelemetryConnection{}

	cl := createNewDPDKTelemetryClient()

	cl.connections.Store(uid, dummyConn)

	cl.CloseConnection(uid)
	assert.True(t, closeCallled)
	cl.connections.Range(func(key, value any) bool {
		t.Error("Socket map was not cleaned.")
		return false
	})
}

func TestDPDKTelemetryClient_GetXPercent(t *testing.T) {
	tcases := []struct {
		testCase       string
		cpuID          uint
		busynessResult *telemetryResult
		usageResult    *telemetryResult
		evalFn         func(v int, e error) bool
	}{
		{
			testCase: "Test Case 1 - Metrics Missing",
			cpuID:    0,
			evalFn: func(v int, e error) bool {
				return assert.ErrorIs(t, e, ErrDPDKMetricMissing)
			},
		},
		{
			testCase:       "Test Case 2 - Results with an error",
			cpuID:          3,
			busynessResult: &telemetryResult{0, ErrDPDKMetricNotProvided},
			usageResult:    &telemetryResult{0, ErrDPDKMetricNotProvided},
			evalFn: func(v int, e error) bool {
				return assert.ErrorIs(t, e, ErrDPDKMetricNotProvided)
			},
		},
		{
			testCase:       "Test Case 3 - Busyness priority",
			cpuID:          2,
			busynessResult: &telemetryResult{42, nil},
			usageResult:    &telemetryResult{24, nil},
			evalFn: func(v int, e error) bool {
				return assert.NoError(t, e) && assert.Equal(t, 42, v)
			},
		},
		{
			testCase:       "Test Case 4 - Usage fallback",
			cpuID:          2,
			busynessResult: &telemetryResult{0, ErrDPDKMetricNotProvided},
			usageResult:    &telemetryResult{42, nil},
			evalFn: func(v int, e error) bool {
				return assert.NoError(t, e) && assert.Equal(t, 42, v)
			},
		},
	}

	for _, tc := range tcases {
		t.Log(tc.testCase)

		cl := createNewDPDKTelemetryClient()
		if tc.busynessResult != nil {
			cl.busyness.Store(tc.cpuID, *tc.busynessResult)
		}
		if tc.usageResult != nil {
			cl.usage.Store(tc.cpuID, *tc.usageResult)
		}

		busyness, err := cl.GetBusynessPercent(tc.cpuID)

		tc.evalFn(busyness, err)
	}
}

func TestDPDKTelemetryClient_Close(t *testing.T) {
	connectionCount := 3
	cancelCounter := 0

	t.Cleanup(func() {
		testHookCloseConnection = nil
	})
	testHookCloseConnection = func() {
		cancelCounter++
	}

	cl := createNewDPDKTelemetryClient()

	for i := 0; i < connectionCount; i++ {
		dummyConn := &dpdkTelemetryConnection{}
		cl.connections.Store(fmt.Sprint(i), dummyConn)
	}

	cl.Close()

	assert.Equal(t, connectionCount, cancelCounter)
	cl.connections.Range(func(key, value any) bool {
		t.Error("Connection map was not cleaned.")
		return false
	})
}

func createNewDPDKConnection() dpdkTelemetryConnection {
	log.SetLogger(zap.New(
		zap.UseDevMode(true),
		func(opts *zap.Options) {
			opts.TimeEncoder = zapcore.ISO8601TimeEncoder
		},
	))

	return dpdkTelemetryConnection{
		log:      ctrl.Log.WithName("test-log"),
		busyness: &sync.Map{},
		usage:    &sync.Map{},
	}
}

func TestDPDKConnection_close(t *testing.T) {
	cancelFuncCalled := false
	cpuList := []uint{1, 2, 3}

	dpdkConn := createNewDPDKConnection()
	dpdkConn.watchedCPUs = cpuList
	dpdkConn.cancelFunc = func() { cancelFuncCalled = true }
	for _, cpuID := range cpuList {
		dpdkConn.busyness.Store(cpuID, nil)
		dpdkConn.usage.Store(cpuID, nil)
	}

	dpdkConn.close()

	assert.True(t, cancelFuncCalled)
	dpdkConn.busyness.Range(func(key, value any) bool {
		t.Error("Busyness metrics map was not cleaned.")
		return false
	})
	dpdkConn.usage.Range(func(key, value any) bool {
		t.Error("Usage metrics map was not cleaned.")
		return false
	})
}

func TestDPDKConnection_connect(t *testing.T) {
	connectLoopCounter := 0
	t.Cleanup(func() {
		testHookStopConnectLoop = nil
	})
	testHookStopConnectLoop = func() bool {
		connectLoopCounter++
		return connectLoopCounter > 2
	}

	mkconn := &MockConn{}
	mkconn.On("Close").Return(nil)

	origRetryDuration := retryDuration
	origMetricsCooldown := samplePeriod
	origConnectFunc := connectWithTimeoutFunc
	t.Cleanup(func() {
		retryDuration = origRetryDuration
		samplePeriod = origMetricsCooldown
		connectWithTimeoutFunc = origConnectFunc
		testHookReadInitMsgReturn = nil
	})
	retryDuration = time.Millisecond
	samplePeriod = time.Millisecond
	connectWithTimeoutFunc = func(addr string, to time.Duration) (net.Conn, error) {
		return mkconn, nil
	}
	testHookReadInitMsgReturn = func() error {
		return fmt.Errorf("foo")
	}

	dpdkConn := createNewDPDKConnection()
	dpdkConn.podUID = "foo"
	dpdkConn.waitGroup.Add(1)

	ctx, cancel := context.WithCancel(context.TODO())
	t.Cleanup(cancel)

	dpdkConn.connect(ctx)

	mkconn.AssertCalled(t, "Close")
	assert.Panics(t, dpdkConn.waitGroup.Done)
}

func TestDPDKConnection_ioLoop(t *testing.T) {
	getMetricsCallCounter := 0
	t.Cleanup(func() {
		testHookReadInitMsgReturn = nil
		testHookHandleMetricsLoop = nil
	})
	testHookReadInitMsgReturn = func() error {
		return nil
	}
	testHookHandleMetricsLoop = func() error {
		getMetricsCallCounter++
		if getMetricsCallCounter > 2 {
			return fmt.Errorf("foo")
		}
		return nil
	}

	mkConn := &MockConn{}
	mkConn.On("Close").Return(nil)

	ctx, cancel := context.WithCancel(context.TODO())
	t.Cleanup(cancel)
	dpdkConn := createNewDPDKConnection()

	err := dpdkConn.ioLoop(ctx, mkConn)

	assert.NotNil(t, err)
	assert.ErrorContains(t, err, "foo")
	mkConn.AssertCalled(t, "Close")
}

func TestDPDKConnection_connectLoop(t *testing.T) {
	mkConn := &MockConn{}
	connectCallCounter := 0

	origConnectFunc := connectWithTimeoutFunc
	origRetryDuration := retryDuration
	t.Cleanup(func() {
		connectWithTimeoutFunc = origConnectFunc
		retryDuration = origRetryDuration
	})
	connectWithTimeoutFunc = func(addr string, to time.Duration) (net.Conn, error) {
		connectCallCounter++
		if connectCallCounter > 1 {
			return mkConn, nil
		}
		return nil, fmt.Errorf("foo")
	}
	retryDuration = time.Millisecond

	dpdkConn := createNewDPDKConnection()
	ctx, cancel := context.WithCancel(context.TODO())
	t.Cleanup(cancel)
	var conn net.Conn = nil

	conn = dpdkConn.connectLoop(ctx)

	assert.NotNil(t, conn)
}

func TestDPDKConnection_processCommand(t *testing.T) {
	fooErr := fmt.Errorf("foo")
	command := "foo"
	commandBytes := []byte(command)
	timestamp := time.Now()
	timeout := timestamp.Add(ioTimeout)
	origDeadlineFunc := getCurrentTimestamp
	t.Cleanup(func() {
		getCurrentTimestamp = origDeadlineFunc
	})
	getCurrentTimestamp = func() time.Time {
		return timestamp
	}

	tcases := []struct {
		testCase     string
		connSetup    func() *MockConn
		expectedData busynessResponse
		evalFn       func(e error, d busynessResponse, c *MockConn)
	}{
		{
			testCase: "Test Case 1 - Write deadline error",
			connSetup: func() *MockConn {
				mc := MockConn{}
				mc.On("SetWriteDeadline", timeout).Return(fooErr)
				return &mc
			},
			evalFn: func(e error, d busynessResponse, c *MockConn) {
				assert.NotNil(t, e)
				assert.Empty(t, d)
				assert.ErrorContains(t, e, "error setting write deadline")
				c.AssertCalled(t, "SetWriteDeadline", timeout)
				c.AssertNotCalled(t, "Write")
			},
		},
		{
			testCase: "Test Case 2 - Write error",
			connSetup: func() *MockConn {
				mc := MockConn{}
				mc.On("SetWriteDeadline", timeout).Return(nil)
				mc.On("Write", commandBytes).Return(0, fooErr)
				return &mc
			},
			evalFn: func(e error, d busynessResponse, c *MockConn) {
				assert.NotNil(t, e)
				assert.Empty(t, d)
				assert.ErrorContains(t, e, "write error")
				c.AssertCalled(t, "SetWriteDeadline", timeout)
				c.AssertCalled(t, "Write", commandBytes)
				c.AssertNotCalled(t, "Read")
			},
		},
		{
			testCase: "Test Case 3 - Read deadline error",
			connSetup: func() *MockConn {
				mc := MockConn{}
				mc.On("SetWriteDeadline", timeout).Return(nil)
				mc.On("Write", commandBytes).Return(0, nil)
				mc.On("SetReadDeadline", timeout).Return(fooErr)
				return &mc
			},
			evalFn: func(e error, d busynessResponse, c *MockConn) {
				assert.NotNil(t, e)
				assert.Empty(t, d)
				assert.ErrorContains(t, e, "error setting read deadline")
				c.AssertCalled(t, "SetWriteDeadline", timeout)
				c.AssertCalled(t, "Write", commandBytes)
				c.AssertCalled(t, "SetReadDeadline", timeout)
				c.AssertNotCalled(t, "Read")
			},
		},
		{
			testCase: "Test Case 4 - Read error",
			connSetup: func() *MockConn {
				mc := MockConn{}
				mc.On("SetWriteDeadline", timeout).Return(nil)
				mc.On("Write", commandBytes).Return(0, nil)
				mc.On("SetReadDeadline", timeout).Return(nil)
				mc.On("Read", mock.Anything).Return(0, fooErr, []byte(""))
				return &mc
			},
			evalFn: func(e error, d busynessResponse, c *MockConn) {
				assert.NotNil(t, e)
				assert.Empty(t, d)
				assert.ErrorContains(t, e, "read error")
				c.AssertCalled(t, "SetWriteDeadline", timeout)
				c.AssertCalled(t, "Write", commandBytes)
				c.AssertCalled(t, "SetReadDeadline", timeout)
				// Anything is used here instead of AnythingOfType beacuse assert
				// still makes a distinction between filled and empty slice.
				c.AssertCalled(t, "Read", mock.Anything)
			},
		},
		{
			testCase: "Test Case 5 - Malformed JSON response",
			connSetup: func() *MockConn {
				mc := MockConn{}
				mc.On("SetWriteDeadline", timeout).Return(nil)
				mc.On("Write", commandBytes).Return(0, nil)
				mc.On("SetReadDeadline", timeout).Return(nil)
				mc.On("Read", mock.Anything).Return(3, nil, commandBytes)
				return &mc
			},
			evalFn: func(e error, d busynessResponse, c *MockConn) {
				assert.ErrorContains(t, e, "invalid character")
				assert.Empty(t, d)
				c.AssertCalled(t, "SetWriteDeadline", timeout)
				c.AssertCalled(t, "Write", commandBytes)
				c.AssertCalled(t, "SetReadDeadline", timeout)
				// Anything is used here instead of AnythingOfType beacuse assert
				// still makes a distinction between filled and empty slice.
				c.AssertCalled(t, "Read", mock.Anything)
			},
		},
		{
			testCase: "Test Case 6 - Success",
			connSetup: func() *MockConn {
				mc := MockConn{}
				mc.On("SetWriteDeadline", timeout).Return(nil)
				mc.On("Write", commandBytes).Return(0, nil)
				mc.On("SetReadDeadline", timeout).Return(nil)
				mc.On("Read", mock.Anything).Return(70, nil, []byte(busynessOutputString))
				return &mc
			},
			expectedData: busynessResponse{
				Busyness: map[uint]int{
					1: 65,
					2: 42,
					3: -1,
				},
			},
			evalFn: func(e error, d busynessResponse, c *MockConn) {
				assert.NoError(t, e)
				assert.NotEmpty(t, d)
				c.AssertCalled(t, "SetWriteDeadline", timeout)
				c.AssertCalled(t, "Write", commandBytes)
				c.AssertCalled(t, "SetReadDeadline", timeout)
				// Anything is used here instead of AnythingOfType beacuse assert
				// still makes a distinction between filled and empty slice.
				c.AssertCalled(t, "Read", mock.Anything)
			},
		},
	}

	for _, tc := range tcases {
		t.Log(tc.testCase)

		dpdkConn := createNewDPDKConnection()
		dpdkConn.buffer = make([]byte, 256)
		conn := tc.connSetup()

		data := busynessResponse{}
		err := dpdkConn.processCommand(conn, command, &data)

		tc.evalFn(err, data, conn)
		if len(tc.expectedData.Busyness) != 0 {
			assert.Equal(t, tc.expectedData, data)
		}
	}
}

func TestDPDKConnection_readInitialMessage(t *testing.T) {
	t.Cleanup(func() {
		testHookProcessCommandReturn = nil
	})

	tcases := []struct {
		TestCase string
		data     initialMessage
		err      error
		evalFn   func(e error, buflen int)
	}{
		{
			TestCase: "Test Case 1 - Read Deadline error",
			err:      fmt.Errorf("foo"),
			evalFn: func(e error, buflen int) {
				assert.NotNil(t, e)
				assert.ErrorContains(t, e, "initial message error:")
			},
		},
		{
			TestCase: "Test Case 2 - Success",
			data: initialMessage{
				Version:         "1",
				PID:             1,
				MaxOutputLength: 10,
			},
			evalFn: func(e error, buflen int) {
				assert.NoError(t, e)
				assert.Equal(t, 10, buflen)
			},
		},
	}
	for _, tc := range tcases {
		t.Log(tc.TestCase)

		usedCommand := ""
		testHookProcessCommandReturn = func(cmd string) (any, error) {
			usedCommand = cmd
			return tc.data, tc.err
		}

		mkConn := &MockConn{}
		dpdkConn := createNewDPDKConnection()
		err := dpdkConn.handleInitialMessage(mkConn)

		assert.Equal(t, "", usedCommand)
		tc.evalFn(err, len(dpdkConn.buffer))
	}
}

func TestDPDKConnection_handleBusyness(t *testing.T) {
	t.Cleanup(func() {
		testHookProcessCommandReturn = nil
	})

	tcases := []struct {
		testCase         string
		initialBusyness  metricsMap
		expectedBusyness metricsMap
		watchlist        []uint
		data             busynessResponse
		err              error
		evalFn           func(e error, expected metricsMap, b *sync.Map)
	}{
		{
			testCase: "Test Case 1 - Command processing error",
			err:      fmt.Errorf("foo"),
			evalFn: func(e error, expected metricsMap, b *sync.Map) {
				assert.ErrorContains(t, e, "busyness error:")
			},
		},
		{
			testCase: "Test Case 2 - Busyness not available",
			expectedBusyness: metricsMap{
				1: telemetryResult{0, ErrDPDKMetricNotProvided},
			},
			watchlist: []uint{1},
			evalFn: func(e error, expected metricsMap, b *sync.Map) {
				assert.NoError(t, e)
				for id, expVal := range expected {
					val, found := b.Load(id)
					assert.True(t, found)
					assert.Equal(t, expVal, val.(telemetryResult))
				}
			},
		},
		{
			testCase: "Test Case 3 - Update busyness readings",
			initialBusyness: metricsMap{
				1: telemetryResult{65, nil},
				2: telemetryResult{0, nil},
				3: telemetryResult{56, nil},
			},
			expectedBusyness: metricsMap{
				1: telemetryResult{65, nil},
				2: telemetryResult{42, nil},
				3: telemetryResult{0, ErrDPDKMetricNotProvided},
				4: telemetryResult{0, ErrDPDKMetricNotProvided},
			},
			watchlist: []uint{1, 2, 3, 4},
			data: busynessResponse{
				Busyness: map[uint]int{
					1: 65,
					2: 42,
					3: -1,
				},
			},
			evalFn: func(e error, expected metricsMap, b *sync.Map) {
				assert.NoError(t, e)
				for id, expVal := range expected {
					val, found := b.Load(id)
					assert.True(t, found)
					assert.Equal(t, expVal, val.(telemetryResult))
				}
			},
		},
	}

	for _, tc := range tcases {
		t.Log(tc.testCase)

		usedCommand := ""
		testHookProcessCommandReturn = func(cmd string) (any, error) {
			usedCommand = cmd
			return tc.data, tc.err
		}

		mkConn := &MockConn{}
		dpdkConn := createNewDPDKConnection()
		dpdkConn.watchedCPUs = tc.watchlist

		for id, val := range tc.initialBusyness {
			dpdkConn.busyness.Store(id, val)
		}

		err := dpdkConn.handleBusyness(mkConn)

		assert.Equal(t, busynessCommand, usedCommand)
		tc.evalFn(err, tc.expectedBusyness, dpdkConn.busyness)
	}
}

func TestDPDKConnection_handleUsage(t *testing.T) {
	t.Cleanup(func() {
		testHookProcessCommandReturn = nil
	})

	tcases := []struct {
		testCase      string
		initialUsage  metricsMap
		expectedUsage metricsMap
		watchlist     []uint
		data          usageResponse
		err           error
		evalFn        func(e error, expected metricsMap, b *sync.Map)
	}{
		{
			testCase: "Test Case 1 - Command processing error",
			err:      fmt.Errorf("foo"),
			evalFn: func(e error, expected metricsMap, b *sync.Map) {
				assert.NotNil(t, e)
				assert.ErrorContains(t, e, "usage error:")
			},
		},
		{
			testCase: "Test Case 2 - Usage not available",
			expectedUsage: metricsMap{
				1: telemetryResult{0, ErrDPDKMetricNotProvided},
			},
			watchlist: []uint{1},
			evalFn: func(e error, expected metricsMap, b *sync.Map) {
				assert.NoError(t, e)
				for id, expVal := range expected {
					val, found := b.Load(id)
					assert.True(t, found)
					assert.Equal(t, expVal, val.(telemetryResult))
				}
			},
		},
		{
			testCase: "Test Case 3 - Update usage readings",
			initialUsage: metricsMap{
				1: telemetryResult{88, nil},
				2: telemetryResult{0, nil},
				3: telemetryResult{56, nil},
			},
			expectedUsage: metricsMap{
				1: telemetryResult{88, nil},
				2: telemetryResult{89, nil},
				3: telemetryResult{0, ErrDPDKMetricNotProvided},
				4: telemetryResult{0, ErrDPDKMetricNotProvided},
			},
			watchlist: []uint{1, 2, 3, 4},
			data: usageResponse{
				Usage: usageData{
					LcoreIDs:    []uint{1, 2},
					TotalCycles: []uint64{23846845590, 23900558914},
					BusyCycles:  []uint64{21043446682, 21448837316},
				},
			},
			evalFn: func(e error, expected metricsMap, b *sync.Map) {
				assert.NoError(t, e)
				for id, expVal := range expected {
					val, found := b.Load(id)
					assert.True(t, found)
					assert.Equal(t, expVal, val.(telemetryResult))
				}
			},
		},
	}

	for _, tc := range tcases {
		t.Log(tc.testCase)

		usedCommand := ""
		testHookProcessCommandReturn = func(cmd string) (any, error) {
			usedCommand = cmd
			return tc.data, tc.err
		}

		mkConn := &MockConn{}
		dpdkConn := createNewDPDKConnection()
		dpdkConn.watchedCPUs = tc.watchlist

		for id, val := range tc.initialUsage {
			dpdkConn.usage.Store(id, val)
		}

		err := dpdkConn.handleUsage(mkConn)

		assert.Equal(t, usageCommand, usedCommand)
		tc.evalFn(err, tc.expectedUsage, dpdkConn.usage)
	}
}
