package scaling

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/intel/kubernetes-power-manager/internal/metrics"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type updaterMock struct {
	mock.Mock
}

func (u *updaterMock) Update(opts *CPUScalingOpts) time.Duration {
	return u.Called(opts).Get(0).(time.Duration)
}

type MockDPDKTelemetryClient struct {
	mock.Mock
	metrics.DPDKTelemetryClient
}

func (md *MockDPDKTelemetryClient) GetBusynessPercent(cpuID uint) (int, error) {
	return md.Called().Get(0).(int), md.Called().Error(1)
}

func TestCPUScalingUpdater_Update(t *testing.T) {
	originalGetCPUFreqPath := getCPUFreqPathFunction
	defer func() { getCPUFreqPathFunction = originalGetCPUFreqPath }()

	tCases := []struct {
		testCase        string
		scalingOpts     *CPUScalingOpts
		currentBusyness int
		busynessErr     error
		expectedFreq    string
		nextSetIn       time.Duration
	}{
		{
			testCase: "Test Case 1 - New frequency is set - 1",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      0,
				TargetBusyness:             80,
				AllowedBusynessDifference:  5,
				SamplePeriod:               10 * time.Millisecond,
				CurrentFrequency:           300000,
				FallbackFreq:               200000,
				HWMaxFrequency:             370000,
				HWMinFrequency:             40000,
				AllowedFrequencyDifference: 10,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 46,
			busynessErr:     nil,
			expectedFreq:    "229750",
			nextSetIn:       20 * time.Millisecond,
		},
		{
			testCase: "Test Case 1 - New frequency is set - 2",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      1,
				TargetBusyness:             41,
				AllowedBusynessDifference:  5,
				SamplePeriod:               10 * time.Millisecond,
				FallbackFreq:               200000,
				CurrentFrequency:           250000,
				HWMaxFrequency:             320000,
				HWMinFrequency:             100000,
				AllowedFrequencyDifference: 10,
				CooldownPeriod:             35 * time.Millisecond,
			},
			currentBusyness: 5,
			busynessErr:     nil,
			expectedFreq:    "126829",
			nextSetIn:       35 * time.Millisecond,
		},
		{
			testCase: "Test Case 1 - New frequency is set - 3, capped to HWMax",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      1,
				TargetBusyness:             60,
				AllowedBusynessDifference:  5,
				SamplePeriod:               10 * time.Millisecond,
				FallbackFreq:               200000,
				CurrentFrequency:           250000,
				HWMaxFrequency:             320000,
				HWMinFrequency:             100000,
				AllowedFrequencyDifference: 10,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 66,
			busynessErr:     nil,
			expectedFreq:    "320000",
			nextSetIn:       20 * time.Millisecond,
		},
		{
			testCase: "Test Case 2 - Busyness difference is within accepted range",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      2,
				TargetBusyness:             80,
				AllowedBusynessDifference:  5,
				SamplePeriod:               10 * time.Millisecond,
				FallbackFreq:               200000,
				CurrentFrequency:           250000,
				HWMaxFrequency:             370000,
				HWMinFrequency:             40000,
				AllowedFrequencyDifference: 10,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 82,
			busynessErr:     nil,
			expectedFreq:    "",
			nextSetIn:       10 * time.Millisecond,
		},
		{
			testCase: "Test Case 3 - Frequency difference is within accepted range",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      2,
				TargetBusyness:             80,
				AllowedBusynessDifference:  1,
				SamplePeriod:               15 * time.Millisecond,
				FallbackFreq:               200000,
				CurrentFrequency:           351750,
				HWMaxFrequency:             370000,
				HWMinFrequency:             40000,
				AllowedFrequencyDifference: 10000,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 78,
			busynessErr:     nil,
			expectedFreq:    "",
			nextSetIn:       15 * time.Millisecond,
		},
		{
			testCase: "Test Case 4 - Error getting busyness",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      2,
				TargetBusyness:             80,
				AllowedBusynessDifference:  1,
				SamplePeriod:               10 * time.Millisecond,
				FallbackFreq:               200000,
				CurrentFrequency:           351250,
				HWMaxFrequency:             370000,
				HWMinFrequency:             40000,
				AllowedFrequencyDifference: 10000,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 0,
			busynessErr:     metrics.ErrDPDKMetricMissing,
			expectedFreq:    "200000",
			nextSetIn:       10 * time.Millisecond,
		},
	}

	for _, tc := range tCases {
		t.Log(tc.testCase)

		tempDir := t.TempDir()
		setFreqTestFilepath := createTempFile(t, tempDir, "")
		governorFilepath := createTempFile(t, tempDir, "userspace\n")

		getCPUFreqPathFunction = func(_ uint, resource string) string {
			switch resource {
			case "scaling_governor":
				return governorFilepath
			case "scaling_setspeed":
				return setFreqTestFilepath
			default:
				require.Fail(t, "Unexpected resource type")
			}

			return ""
		}

		dpdkmock := &MockDPDKTelemetryClient{}
		dpdkmock.On("GetBusynessPercent").Return(tc.currentBusyness, tc.busynessErr)
		updater := &cpuScalingUpdaterImpl{
			dpdkClient: dpdkmock,
		}

		nextSetIn := updater.Update(tc.scalingOpts)

		frequencyRaw, err := os.ReadFile(setFreqTestFilepath)
		assert.NoError(t, err)
		frequency := strings.Trim(string(frequencyRaw), " \n")
		assert.Equal(t, tc.expectedFreq, frequency)
		assert.Equal(t, tc.nextSetIn, nextSetIn)

		dpdkmock.AssertExpectations(t)
	}
}
