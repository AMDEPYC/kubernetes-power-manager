package metrics

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const (
	msrDirPath       string = "/dev/cpu/"
	msrFilename      string = "msr"
	msrReaderBufSize int    = 8
)

type msrReader interface {
	read(offset uint64) (uint64, error)
	close() error
}

type msrReaderImpl struct {
	cpu  int
	file *os.File
}

func newMSRReader(cpu int) (msrReader, error) {
	msrFile, err := os.OpenFile(
		filepath.Join(msrDirPath, strconv.Itoa(cpu), msrFilename),
		os.O_RDONLY,
		0660,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open MSR file for CPU %d: %w", cpu, err)
	}

	return &msrReaderImpl{cpu: cpu, file: msrFile}, nil
}

func (m *msrReaderImpl) read(offset uint64) (uint64, error) {
	buf := make([]byte, perfEventReaderBufSize)
	if _, err := m.file.ReadAt(buf, int64(offset)); err != nil {
		return 0, fmt.Errorf("failed to read properly opened MSR file for CPU %d: %w", m.cpu, err)
	}

	return binary.LittleEndian.Uint64(buf), nil
}

func (m *msrReaderImpl) close() error {
	return m.file.Close()
}
