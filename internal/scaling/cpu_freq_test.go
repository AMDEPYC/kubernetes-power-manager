package scaling

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTempFile(t *testing.T, dir, content string) string {
	tempFile, err := os.CreateTemp(dir, "tempfile-")
	require.NoError(t, err, "failed to create temp file")

	if content != "" {
		_, err := tempFile.WriteString(content)
		require.NoError(t, err, "failed to write to temp file")
	}
	require.NoError(t, tempFile.Close(), "failed to close temp file")

	return tempFile.Name()
}

func overrideGetCPUFreqPath(t *testing.T, cpu uint, resource string) string {
	tempDir := t.TempDir()
	switch resource {
	case "scaling_governor":
		if cpu == 0 {
			return createTempFile(t, tempDir, "userspace\n")
		} else {
			return createTempFile(t, tempDir, "powersave\n")
		}
	case "scaling_setspeed":
		return createTempFile(t, tempDir, "")
	case "scaling_cur_freq":
		return createTempFile(t, tempDir, "2000000\n")
	default:
		require.Fail(t, "Unexpected resource type")
	}
	return ""
}

func TestGetFrequencyFromPercent(t *testing.T) {
	for _, tc := range []struct {
		min     int
		max     int
		percent int
		result  int
	}{
		{
			min:     40000,
			max:     370000,
			percent: 60,
			result:  238000,
		},
		{
			min:     40000,
			max:     370000,
			percent: 33,
			result:  148900,
		},
		{
			min:     70000,
			max:     420000,
			percent: 100,
			result:  420000,
		},
		{
			min:     120000,
			max:     250000,
			percent: 0,
			result:  120000,
		},
	} {
		freq := GetFrequencyFromPercent(tc.min, tc.max, tc.percent)
		assert.Equal(t, tc.result, freq)
	}
}

func TestGetCurrentGovernorUserspace(t *testing.T) {
	originalGetCPUFreqPath := getCPUFreqPathFunction
	getCPUFreqPathFunction = func(cpu uint, resource string) string {
		return overrideGetCPUFreqPath(t, cpu, resource)
	}
	defer func() { getCPUFreqPathFunction = originalGetCPUFreqPath }()

	governor, err := getCurrentGovernor(0)
	require.NoError(t, err)
	assert.Equal(t, "userspace", governor)
}

func TestGetCurrentGovernorPowersave(t *testing.T) {
	originalGetCPUFreqPath := getCPUFreqPathFunction
	getCPUFreqPathFunction = func(cpu uint, resource string) string {
		return overrideGetCPUFreqPath(t, cpu, resource)
	}
	defer func() { getCPUFreqPathFunction = originalGetCPUFreqPath }()

	governor, err := getCurrentGovernor(1)
	require.NoError(t, err)
	assert.Equal(t, "powersave", governor)
}

func TestIsUserspaceGovernor(t *testing.T) {
	originalGetCPUFreqPath := getCPUFreqPathFunction
	getCPUFreqPathFunction = func(cpu uint, resource string) string {
		return overrideGetCPUFreqPath(t, cpu, resource)
	}
	defer func() { getCPUFreqPathFunction = originalGetCPUFreqPath }()

	isUserspace, err := isUserspaceGovernor(0)
	require.NoError(t, err)
	assert.True(t, isUserspace)
}

func TestIsUserspaceGovernorNegative(t *testing.T) {
	originalGetCPUFreqPath := getCPUFreqPathFunction
	getCPUFreqPathFunction = func(cpu uint, resource string) string {
		return overrideGetCPUFreqPath(t, cpu, resource)
	}
	defer func() { getCPUFreqPathFunction = originalGetCPUFreqPath }()

	isUserspace, err := isUserspaceGovernor(1)
	require.NoError(t, err)
	assert.False(t, isUserspace)
}

func TestSetCPUFrequency(t *testing.T) {
	originalGetCPUFreqPath := getCPUFreqPathFunction
	getCPUFreqPathFunction = func(cpu uint, resource string) string {
		return overrideGetCPUFreqPath(t, cpu, resource)
	}
	defer func() { getCPUFreqPathFunction = originalGetCPUFreqPath }()

	err := setCPUFrequency(0, 3000000)
	require.NoError(t, err)
}

func TestGetCPUFrequency(t *testing.T) {
	originalGetCPUFreqPath := getCPUFreqPathFunction
	getCPUFreqPathFunction = func(cpu uint, resource string) string {
		return overrideGetCPUFreqPath(t, cpu, resource)
	}
	defer func() { getCPUFreqPathFunction = originalGetCPUFreqPath }()

	freq, err := getCPUFrequency(0)
	require.NoError(t, err)
	assert.Equal(t, uint(2000000), freq)
}
