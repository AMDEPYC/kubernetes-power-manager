package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
)

const (
	baseSocketPath  = "/var/lib/power-node-agent/pods/%s/dpdk/rte/dpdk_telemetry.v2"
	busynessCommand = "/eal/lcore/busyness"
	usageCommand    = "/eal/lcore/usage"
	ioTimeout       = 3 * time.Second
)

var (
	ErrDPDKMetricMissing     = errors.New("no entry found for this cpu")
	ErrDPDKMetricNotProvided = errors.New("dpdk telemetry did not provide a reading for this cpu")

	retryDuration = 1 * time.Second
	samplePeriod  = 10 * time.Millisecond

	connectWithTimeoutFunc = connectWithTimeout
	getCurrentTimestamp    = time.Now

	testHookReadInitMsgReturn    func() error
	testHookHandleMetricsLoop    func() error
	testHookProcessCommandReturn func(cmd string) (any, error)
	testHookNewSocketConnection  func(conn *dpdkTelemetryConnection)
	testHookStopConnectLoop      func() bool
	testHookCloseConnection      func()
)

type initialMessage struct {
	Version         string `json:"version"`
	PID             int    `json:"pid"`
	MaxOutputLength int    `json:"max_output_len"`
}
type busynessResponse struct {
	Busyness map[uint]int `json:"/eal/lcore/busyness"`
}
type usageResponse struct {
	Usage usageData `json:"/eal/lcore/usage"`
}
type usageData struct {
	LcoreIDs    []uint   `json:"lcore_ids"`
	TotalCycles []uint64 `json:"total_cycles"`
	BusyCycles  []uint64 `json:"busy_cycles"`
}

func connectWithTimeout(addr string, to time.Duration) (net.Conn, error) {
	return net.DialTimeout("unixpacket", addr, to)
}

type DPDKTelemetryConnectionData struct {
	PodUID      string
	WatchedCPUs []uint
}

type DPDKTelemetryClient interface {
	CreateConnection(data *DPDKTelemetryConnectionData)
	ListConnections() []DPDKTelemetryConnectionData
	CloseConnection(podUID string)
	GetBusynessPercent(cpuID uint) (int, error)
	Close()
}

type dpdkTelemetryClientImpl struct {
	log         logr.Logger
	connections sync.Map
	busyness    sync.Map
	usage       sync.Map
}

func NewDPDKTelemetryClient(logger logr.Logger) DPDKTelemetryClient {
	c := &dpdkTelemetryClientImpl{
		log: logger,
	}

	return c
}

func (cl *dpdkTelemetryClientImpl) CreateConnection(data *DPDKTelemetryConnectionData) {
	ctx, cancel := context.WithCancel(context.Background())
	podUID := data.PodUID
	newConn := &dpdkTelemetryConnection{
		podUID:      podUID,
		watchedCPUs: data.WatchedCPUs,
		busyness:    &cl.busyness,
		usage:       &cl.usage,
		log:         cl.log.WithValues("podUID", podUID),
		cancelFunc:  cancel,
	}

	if _, present := cl.connections.LoadOrStore(podUID, newConn); !present {
		if testHookNewSocketConnection != nil {
			testHookNewSocketConnection(newConn)
		} else {
			newConn.waitGroup.Add(1)
			go newConn.connect(ctx)
		}
	}
}

func (cl *dpdkTelemetryClientImpl) ListConnections() []DPDKTelemetryConnectionData {
	dataList := make([]DPDKTelemetryConnectionData, 0)

	cl.connections.Range(func(key, value any) bool {
		connection := value.(*dpdkTelemetryConnection)
		dataList = append(dataList, DPDKTelemetryConnectionData{
			PodUID:      connection.podUID,
			WatchedCPUs: connection.watchedCPUs,
		})
		return true
	})

	return dataList
}

func (cl *dpdkTelemetryClientImpl) CloseConnection(podUID string) {
	if connection, found := cl.connections.LoadAndDelete(podUID); found {
		connection := connection.(*dpdkTelemetryConnection)
		connection.close()
		cl.log.V(4).Info("stopped the connection.", "podUID", podUID)
	} else {
		cl.log.V(4).Info("connection does not exist.", "podUID", podUID)
	}
}

func (cl *dpdkTelemetryClientImpl) GetBusynessPercent(cpuID uint) (int, error) {
	if value, found := cl.busyness.Load(cpuID); found {
		r := value.(telemetryResult)
		if r.err == nil {
			return r.value, r.err
		}
	}
	if value, found := cl.usage.Load(cpuID); found {
		r := value.(telemetryResult)
		return r.value, r.err
	}

	return 0, ErrDPDKMetricMissing
}

func (cl *dpdkTelemetryClientImpl) Close() {
	cl.log.V(4).Info("stopping all connection loops.")

	cl.connections.Range(func(key, value any) bool {
		connection := value.(*dpdkTelemetryConnection)
		connection.close()
		cl.connections.Delete(key.(string))
		return true
	})

	cl.log.V(4).Info("all connection loops stopped.")
}

type dpdkTelemetryConnection struct {
	podUID      string
	watchedCPUs []uint
	busyness    *sync.Map
	usage       *sync.Map
	buffer      []byte
	log         logr.Logger
	waitGroup   sync.WaitGroup
	cancelFunc  func()
}

type telemetryResult struct {
	value int
	err   error
}

func (c *dpdkTelemetryConnection) close() {
	if testHookCloseConnection != nil {
		testHookCloseConnection()
		return
	}

	c.cancelFunc()
	c.waitGroup.Wait()
	c.clearMetrics(c.watchedCPUs)
}

func (c *dpdkTelemetryConnection) connect(ctx context.Context) {
	defer c.waitGroup.Done()

	for {
		if testHookStopConnectLoop != nil {
			if testHookStopConnectLoop() {
				return
			}
		}
		conn := c.connectLoop(ctx)
		if conn == nil {
			c.log.V(4).Info("client loop finished")
			return
		}
		if err := c.ioLoop(ctx, conn); err != nil {
			c.clearMetrics(c.watchedCPUs)
			c.log.Error(err, "connection closed")
		}
	}
}

func (c *dpdkTelemetryConnection) connectLoop(ctx context.Context) net.Conn {
	address := fmt.Sprintf(baseSocketPath, c.podUID)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(retryDuration):
			conn, err := connectWithTimeoutFunc(address, ioTimeout)
			if err == nil {
				c.log.V(4).Info("connection opened")
				return conn
			}
			if strings.Contains(err.Error(), "no such file or directory") {
				c.log.Error(err, "dpdk telemetry socket not found")
			}
		}
	}
}

func (c *dpdkTelemetryConnection) ioLoop(ctx context.Context, conn net.Conn) error {
	defer conn.Close()

	if err := c.handleInitialMessage(conn); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(samplePeriod):
			if err := c.handleBusyness(conn); err != nil {
				return err
			}
			if err := c.handleUsage(conn); err != nil {
				return err
			}
		}
	}
}

func (c *dpdkTelemetryConnection) processCommand(conn net.Conn, cmd string, v any) error {
	var err error

	if testHookProcessCommandReturn != nil {
		value, e := testHookProcessCommandReturn(cmd)
		if e != nil {
			return e
		}
		rv := reflect.ValueOf(v)
		rv.Elem().Set(reflect.ValueOf(value))
		return nil
	}

	if cmd != "" {
		if err = conn.SetWriteDeadline(
			getCurrentTimestamp().Add(ioTimeout),
		); err != nil {
			return fmt.Errorf("error setting write deadline: %w", err)
		}
		_, err = conn.Write([]byte(cmd))
		if err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}

	if err := conn.SetReadDeadline(
		getCurrentTimestamp().Add(ioTimeout),
	); err != nil {
		return fmt.Errorf("error setting read deadline: %w", err)
	}
	bytesRead, err := conn.Read(c.buffer)
	if err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	if err := json.Unmarshal(c.buffer[:bytesRead], v); err != nil {
		return err
	}

	return nil
}

func (c *dpdkTelemetryConnection) handleInitialMessage(conn net.Conn) error {
	if testHookReadInitMsgReturn != nil {
		return testHookReadInitMsgReturn()
	}

	c.buffer = make([]byte, 1024)
	initMsg := initialMessage{}
	if err := c.processCommand(conn, "", &initMsg); err != nil {
		return fmt.Errorf("initial message error: %w", err)
	}

	c.log.V(4).Info("connection established",
		"version", initMsg.Version,
		"pid", initMsg.PID,
		"max_output_len", initMsg.MaxOutputLength,
	)
	c.buffer = make([]byte, initMsg.MaxOutputLength)

	return nil
}

func (c *dpdkTelemetryConnection) handleBusyness(conn net.Conn) error {
	if testHookHandleMetricsLoop != nil {
		return testHookHandleMetricsLoop()
	}

	var res busynessResponse
	if err := c.processCommand(conn, busynessCommand, &res); err != nil {
		return fmt.Errorf("busyness error: %w", err)
	}

	for _, cpuID := range c.watchedCPUs {
		result := telemetryResult{}
		reading, found := res.Busyness[cpuID]

		if !found || reading == -1 {
			result.err = ErrDPDKMetricNotProvided
		} else {
			result.value = reading
		}

		c.busyness.Store(cpuID, result)
	}

	return nil
}

func (c *dpdkTelemetryConnection) handleUsage(conn net.Conn) error {
	if testHookHandleMetricsLoop != nil {
		return testHookHandleMetricsLoop()
	}

	var res usageResponse
	if err := c.processCommand(conn, usageCommand, &res); err != nil {
		return fmt.Errorf("usage error: %w", err)
	}

	lcoreIDs := map[uint]int{}

	for index, cpuID := range res.Usage.LcoreIDs {
		lcoreIDs[cpuID] = index
	}

	for _, cpuID := range c.watchedCPUs {
		result := telemetryResult{}

		if index, found := lcoreIDs[cpuID]; found {
			total := res.Usage.TotalCycles[index]
			busy := res.Usage.BusyCycles[index]
			percent := (busy * 100) / total
			result.value = int(percent)
		} else {
			result.err = ErrDPDKMetricNotProvided
		}
		c.usage.Store(cpuID, result)
	}

	return nil
}

func (c *dpdkTelemetryConnection) clearMetrics(cpuList []uint) {
	for _, cpuID := range cpuList {
		c.busyness.Delete(cpuID)
		c.usage.Delete(cpuID)
	}
}
