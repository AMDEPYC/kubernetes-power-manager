package scaling

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	powersaveGovernor = "powersave"
	cpuFreqBasePath   = "/sys/devices/system/cpu/cpu%d/cpufreq"
)

func GetFrequencyFromPercent(minFreq, maxFreq int, percent int) int {
	return minFreq + (maxFreq-minFreq)*percent/100
}

func getCPUFreqPath(cpu uint, resource string) string {
	cpuFreqPath := fmt.Sprintf(cpuFreqBasePath, cpu)
	return filepath.Join(cpuFreqPath, resource)
}

var getCPUFreqPathFunction = getCPUFreqPath

// get current governor
func getCurrentGovernor(cpu uint) (string, error) {
	governorPath := getCPUFreqPathFunction(cpu, "scaling_governor")

	currentGovernor, err := os.ReadFile(governorPath)
	if err != nil {
		return "", fmt.Errorf("failed to read current governor for cpu %d: %w", cpu, err)
	}
	return strings.TrimSpace(string(currentGovernor)), nil
}

func isPowersaveGovernor(cpu uint) (bool, error) {
	governor, err := getCurrentGovernor(cpu)
	if err != nil {
		return false, fmt.Errorf("failed to read current governor for cpu %d: %w", cpu, err)
	}
	return governor == powersaveGovernor, nil
}

// setCPUFrequency sets the CPU max frequency in kHz using scaling_max_freq.
func setCPUFrequency(cpu uint, frequency uint) error {
	// Check that the powersave governor is enabled
	isPowersave, err := isPowersaveGovernor(cpu)
	if err != nil {
		return fmt.Errorf("failed to get governor for CPU %d: %w", cpu, err)
	}

	if !isPowersave {
		return fmt.Errorf("powersave governor not set for CPU %d", cpu)
	}

	scalingMaxFreqPath := getCPUFreqPathFunction(cpu, "scaling_max_freq")
	if err := os.WriteFile(scalingMaxFreqPath, []byte(fmt.Sprintf("%d", frequency)), 0644); err != nil {
		return fmt.Errorf("failed to set max frequency for CPU %d: %w", cpu, err)
	}

	return nil
}

// getCPUFrequency returns the CPU frequency in kHz for the specified CPU.
func getCPUFrequency(cpu uint) (uint, error) {
	scalingGetFreqPath := getCPUFreqPathFunction(cpu, "scaling_cur_freq")

	freqData, err := os.ReadFile(scalingGetFreqPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read current frequency for CPU %d: %w", cpu, err)
	}

	freqStr := strings.TrimSpace(string(freqData))
	freq, err := strconv.ParseUint(freqStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to convert frequency for CPU %d to uint: %w", cpu, err)
	}

	return uint(freq), nil
}
