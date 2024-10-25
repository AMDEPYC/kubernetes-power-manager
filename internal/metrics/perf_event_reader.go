package metrics

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	// vars not defined in x/sys/unix.
	perfSampleIdentifier = 1 << 16
	perfIOCFlagGroup     = 1 << 0

	perfEventReaderBufSize = 8
)

type perfEventReader interface {
	start() error
	close() error
	read() (uint64, error)
}

type defaultPerfEventReader struct {
	cpu    int
	kind   int
	config int

	fd int
}

func newDefaultPerfEventReader(cpu, kind, config int) (perfEventReader, error) {
	dp := defaultPerfEventReader{cpu, kind, config, -1}

	eventAttr := &unix.PerfEventAttr{
		Type:        uint32(dp.kind),
		Config:      uint64(dp.config),
		Size:        uint32(unsafe.Sizeof(unix.PerfEventAttr{})),
		Bits:        unix.PerfBitDisabled | unix.PerfBitExcludeHv | unix.PerfBitInherit,
		Sample_type: perfSampleIdentifier,
	}
	fd, err := unix.PerfEventOpen(eventAttr, -1, cpu, -1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open perf event counter file for CPU %d, type %d, kind %d: %w",
			cpu, kind, config, err)
	}
	dp.fd = fd

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
	if _, err := syscall.Read(p.fd, buf); err != nil {
		return 0, fmt.Errorf("failed to read properly created perf event counter file for CPU %d, type %d, kind %d: %w",
			p.cpu, p.kind, p.config, err)
	}

	return binary.LittleEndian.Uint64(buf), nil
}
