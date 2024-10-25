package metrics

import (
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	// vars not defined in x/sys/unix.
	perfSampleIdentifier = 1 << 16
	perfIOCFlagGroup     = 1 << 0

	// 3 uint64 values: the event value, running time, enabled time
	perfEventReaderBufSize = 3 * unsafe.Sizeof(uint64(0))

	maxWriteConflictRetries = 10
)

var syscallRead = syscall.Read

type perfEventValues struct {
	lastTimeEnabled uint64
	lastTimeRunning uint64
	lastRawValue    uint64
	lastScaledValue uint64
}

type perfEventReader interface {
	start() error
	close() error
	read() (uint64, error)
}

type defaultPerfEventReader struct {
	cpu    int
	kind   int
	config int

	values atomic.Pointer[perfEventValues]

	fd int
}

func newDefaultPerfEventReader(cpu, kind, config int) (perfEventReader, error) {
	eventAttr := &unix.PerfEventAttr{
		Type:        uint32(kind),
		Config:      uint64(config),
		Size:        uint32(unsafe.Sizeof(unix.PerfEventAttr{})),
		Bits:        unix.PerfBitDisabled,
		Read_format: unix.PERF_FORMAT_TOTAL_TIME_RUNNING | unix.PERF_FORMAT_TOTAL_TIME_ENABLED,
		Sample_type: perfSampleIdentifier,
	}
	fd, err := unix.PerfEventOpen(eventAttr, -1, cpu, -1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open perf event counter file for CPU %d, type %d, kind %d: %w",
			cpu, kind, config, err)
	}

	dp := defaultPerfEventReader{cpu: cpu, kind: kind, config: config, fd: fd}
	dp.values.Store(&perfEventValues{})

	return &dp, nil
}

func (p *defaultPerfEventReader) start() error {
	return unix.IoctlSetInt(p.fd, unix.PERF_EVENT_IOC_ENABLE, 0)
}

func (p *defaultPerfEventReader) close() error {
	return syscall.Close(p.fd)
}

func (p *defaultPerfEventReader) read() (uint64, error) {
	buf := make([]byte, perfEventReaderBufSize)
	for attempt := 0; attempt < maxWriteConflictRetries; attempt++ {
		oldValues := p.values.Load()

		if _, err := syscallRead(p.fd, buf); err != nil {
			return 0, fmt.Errorf("failed to read properly created perf event counter file for CPU %d, type %d, kind %d: %w",
				p.cpu, p.kind, p.config, err)
		}

		rawValue := binary.LittleEndian.Uint64(buf[0:8])
		timeEnabled := binary.LittleEndian.Uint64(buf[8:16])
		timeRunning := binary.LittleEndian.Uint64(buf[16:24])

		deltaTimeEnabled := timeEnabled - oldValues.lastTimeEnabled
		deltaTimeRunning := timeRunning - oldValues.lastTimeRunning
		deltaValue := rawValue - oldValues.lastRawValue

		if deltaTimeRunning == 0 {
			// no time counted since last read
			return oldValues.lastScaledValue, nil
		}
		if timeRunning < oldValues.lastTimeRunning || timeEnabled < oldValues.lastTimeEnabled || rawValue < oldValues.lastRawValue {
			return 0, fmt.Errorf("inconsistent values from perf event counter for CPU %d, type %d, kind %d",
				p.cpu, p.kind, p.config)
		}
		scaledValue := oldValues.lastScaledValue
		if deltaTimeRunning < deltaTimeEnabled {
			// multiplexing happened since last read, scaling needs to be done
			scaledValue += uint64(float64(deltaValue) * float64(deltaTimeEnabled) / float64(deltaTimeRunning))
		} else {
			scaledValue += deltaValue
		}
		newValues := &perfEventValues{
			lastTimeEnabled: timeEnabled,
			lastTimeRunning: timeRunning,
			lastRawValue:    rawValue,
			lastScaledValue: scaledValue,
		}
		if p.values.CompareAndSwap(oldValues, newValues) {
			return scaledValue, nil
		}
	}
	return 0, fmt.Errorf("attempts exceeded while trying to update perf event counter for CPU %d, type %d, kind %d",
		p.cpu, p.kind, p.config)
}
