package metrics

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFileDescriptor struct {
	readData         []byte
	currentReadIndex int
	readErr          error
}

func (m *mockFileDescriptor) Read(fd int, b []byte) (n int, err error) {
	if m.readErr != nil {
		return 0, m.readErr
	}
	copy(b, m.readData[m.currentReadIndex:])
	n = len(b)
	m.currentReadIndex += n
	return n, nil
}

func setData(data []byte, value uint64, enabled uint64, running uint64) {
	binary.LittleEndian.PutUint64(data[0:], value)
	binary.LittleEndian.PutUint64(data[8:], enabled)
	binary.LittleEndian.PutUint64(data[16:], running)
}

func TestDefaultPerfEventReaderRead_Success(t *testing.T) {
	data := make([]byte, perfEventReaderBufSize)
	setData(data, 100, 2000, 2000)

	mockFd := &mockFileDescriptor{readData: data}
	reader := &defaultPerfEventReader{}
	reader.values.Store(&perfEventValues{})

	originalSyscallRead := syscallRead
	defer func() { syscallRead = originalSyscallRead }()
	syscallRead = mockFd.Read

	scaledValue, err := reader.read()
	require.NoError(t, err)
	assert.Equal(t, uint64(100), scaledValue)
}

func TestDefaultPerfEventReaderRead_Multiplexed(t *testing.T) {
	data1 := make([]byte, perfEventReaderBufSize)
	setData(data1, 100, 2000, 2000)

	data2 := make([]byte, perfEventReaderBufSize)
	setData(data2, 200, 4000, 3000)

	mockFd := &mockFileDescriptor{readData: append(data1, data2...)}
	reader := &defaultPerfEventReader{}
	reader.values.Store(&perfEventValues{})

	originalSyscallRead := syscallRead
	defer func() { syscallRead = originalSyscallRead }()
	syscallRead = mockFd.Read

	scaledValue, err := reader.read()
	require.NoError(t, err)
	assert.Equal(t, uint64(100), scaledValue)

	scaledValue, err = reader.read()
	require.NoError(t, err)
	expectedScaledValue := 100.0 + 100.0/0.5
	assert.Equal(t, uint64(expectedScaledValue), scaledValue)
}

func TestDefaultPerfEventReaderRead_NoTimeRunning(t *testing.T) {
	data := make([]byte, perfEventReaderBufSize)
	setData(data, 400, 0, 0)

	mockFd := &mockFileDescriptor{readData: data}
	reader := &defaultPerfEventReader{}
	reader.values.Store(&perfEventValues{lastScaledValue: 300})

	originalSyscallRead := syscallRead
	defer func() { syscallRead = originalSyscallRead }()
	syscallRead = mockFd.Read

	scaledValue, err := reader.read()
	require.NoError(t, err)
	assert.Equal(t, uint64(300), scaledValue)
}

func TestDefaultPerfEventReaderRead_InvalidValues(t *testing.T) {
	oldData := make([]byte, perfEventReaderBufSize)
	setData(oldData, 200, 2000, 2000)

	newData := make([]byte, perfEventReaderBufSize)
	setData(newData, 100, 1700, 1600)

	mockFd := &mockFileDescriptor{readData: append(oldData, newData...)}
	reader := &defaultPerfEventReader{}
	reader.values.Store(&perfEventValues{})

	originalSyscallRead := syscallRead
	defer func() { syscallRead = originalSyscallRead }()
	syscallRead = mockFd.Read

	scaledValue, err := reader.read()
	require.NoError(t, err)
	assert.Equal(t, uint64(200), scaledValue)

	_, err = reader.read()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inconsistent values")
}
