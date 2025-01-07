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
		currentFreq     string
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
				CurrentTargetFrequency:     1340980,
				FallbackFreq:               2000000,
				HWMaxFrequency:             3700000,
				HWMinFrequency:             400000,
				ScaleFactor:                1.0,
				AllowedFrequencyDifference: 10000,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 46,
			busynessErr:     nil,
			currentFreq:     "1340980",
			expectedFreq:    "771063",
			nextSetIn:       20 * time.Millisecond,
		},
		{
			testCase: "Test Case 1 - New frequency is set - 2",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      1,
				TargetBusyness:             41,
				AllowedBusynessDifference:  3,
				SamplePeriod:               10 * time.Millisecond,
				FallbackFreq:               2000000,
				CurrentTargetFrequency:     2134003,
				HWMaxFrequency:             3200000,
				HWMinFrequency:             1000000,
				ScaleFactor:                0.5,
				AllowedFrequencyDifference: 10000,
				CooldownPeriod:             35 * time.Millisecond,
			},
			currentBusyness: 74,
			busynessErr:     nil,
			currentFreq:     "2134003",
			expectedFreq:    "2992809",
			nextSetIn:       35 * time.Millisecond,
		},
		{
			testCase: "Test Case 1 - New frequency is set - 3",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      1,
				TargetBusyness:             58,
				AllowedBusynessDifference:  3,
				SamplePeriod:               10 * time.Millisecond,
				FallbackFreq:               2000000,
				CurrentTargetFrequency:     3024616,
				HWMaxFrequency:             3700000,
				HWMinFrequency:             400000,
				ScaleFactor:                1.9,
				AllowedFrequencyDifference: 10000,
				CooldownPeriod:             35 * time.Millisecond,
			},
			currentBusyness: 52,
			busynessErr:     nil,
			currentFreq:     "3024616",
			expectedFreq:    "2430122",
			nextSetIn:       35 * time.Millisecond,
		},
		{
			testCase: "Test Case 1 - New frequency is set - 4, capped to HWMax",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      1,
				TargetBusyness:             60,
				AllowedBusynessDifference:  5,
				SamplePeriod:               10 * time.Millisecond,
				FallbackFreq:               2000000,
				CurrentTargetFrequency:     2844827,
				HWMaxFrequency:             3200000,
				HWMinFrequency:             1000000,
				ScaleFactor:                1.6,
				AllowedFrequencyDifference: 10000,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 66,
			busynessErr:     nil,
			currentFreq:     "2844827",
			expectedFreq:    "3200000",
			nextSetIn:       20 * time.Millisecond,
		},
		{
			testCase: "Test Case 1 - New frequency is set - 5, capped to HWMin",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      1,
				TargetBusyness:             70,
				AllowedBusynessDifference:  5,
				SamplePeriod:               10 * time.Millisecond,
				FallbackFreq:               2000000,
				CurrentTargetFrequency:     1260000,
				HWMaxFrequency:             3200000,
				HWMinFrequency:             1000000,
				ScaleFactor:                0.75,
				AllowedFrequencyDifference: 10000,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 50,
			busynessErr:     nil,
			currentFreq:     "1260000",
			expectedFreq:    "1000000",
			nextSetIn:       20 * time.Millisecond,
		},
		{
			testCase: "Test Case 2 - Busyness difference is within accepted range",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      2,
				TargetBusyness:             80,
				AllowedBusynessDifference:  5,
				SamplePeriod:               10 * time.Millisecond,
				FallbackFreq:               2000000,
				CurrentTargetFrequency:     2500000,
				HWMaxFrequency:             3700000,
				HWMinFrequency:             400000,
				ScaleFactor:                1.0,
				AllowedFrequencyDifference: 10000,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 82,
			busynessErr:     nil,
			currentFreq:     "2500000",
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
				FallbackFreq:               2000000,
				CurrentTargetFrequency:     3418600,
				HWMaxFrequency:             3700000,
				HWMinFrequency:             400000,
				ScaleFactor:                1.0,
				AllowedFrequencyDifference: 100000,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 78,
			busynessErr:     nil,
			currentFreq:     "3607692",
			expectedFreq:    "",
			nextSetIn:       15 * time.Millisecond,
		},
		{
			testCase: "Test Case 4 - Current frequency is not yet set",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      2,
				TargetBusyness:             80,
				AllowedBusynessDifference:  1,
				SamplePeriod:               15 * time.Millisecond,
				FallbackFreq:               2000000,
				CurrentTargetFrequency:     FrequencyNotYetSet,
				HWMaxFrequency:             3700000,
				HWMinFrequency:             400000,
				ScaleFactor:                1.0,
				AllowedFrequencyDifference: 100000,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 78,
			busynessErr:     nil,
			currentFreq:     "3607692",
			expectedFreq:    "3517499",
			nextSetIn:       20 * time.Millisecond,
		},
		{
			testCase: "Test Case 5 - Error getting busyness",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      2,
				TargetBusyness:             80,
				AllowedBusynessDifference:  1,
				SamplePeriod:               10 * time.Millisecond,
				FallbackFreq:               2000000,
				CurrentTargetFrequency:     3512500,
				HWMaxFrequency:             3700000,
				HWMinFrequency:             400000,
				ScaleFactor: 1.0,
				AllowedFrequencyDifference: 100000,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 0,
			busynessErr:     metrics.ErrDPDKMetricMissing,
			currentFreq:     "3512500",
			expectedFreq:    "2000000",
			nextSetIn:       10 * time.Millisecond,
		},
		{
			testCase: "Test Case 6 - Error getting busyness, fallback frequency is already set",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      2,
				TargetBusyness:             80,
				AllowedBusynessDifference:  1,
				SamplePeriod:               10 * time.Millisecond,
				FallbackFreq:               2000000,
				CurrentTargetFrequency:     2000000,
				HWMaxFrequency:             3700000,
				HWMinFrequency:             400000,
				AllowedFrequencyDifference: 100000,
				ScaleFactor: 1.0,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 0,
			busynessErr:     metrics.ErrDPDKMetricMissing,
			currentFreq:     "2000000",
			expectedFreq:    "",
			nextSetIn:       10 * time.Millisecond,
		},
		{
			testCase: "Test Case 7 - Error getting current frequency",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      2,
				TargetBusyness:             80,
				AllowedBusynessDifference:  1,
				SamplePeriod:               10 * time.Millisecond,
				FallbackFreq:               2000000,
				CurrentTargetFrequency:     3512500,
				HWMaxFrequency:             3700000,
				HWMinFrequency:             400000,
				AllowedFrequencyDifference: 100000,
				ScaleFactor: 1.0,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 70,
			busynessErr:     nil,
			currentFreq:     "not-a-frequency",
			expectedFreq:    "2000000",
			nextSetIn:       10 * time.Millisecond,
		},
		{
			testCase: "Test Case 8 - Error getting current frequency, fallback frequency is already set",
			scalingOpts: &CPUScalingOpts{
				CPUID:                      2,
				TargetBusyness:             80,
				AllowedBusynessDifference:  1,
				SamplePeriod:               10 * time.Millisecond,
				FallbackFreq:               2000000,
				CurrentTargetFrequency:     2000000,
				HWMaxFrequency:             3700000,
				HWMinFrequency:             400000,
				AllowedFrequencyDifference: 100000,
				ScaleFactor: 1.0,
				CooldownPeriod:             20 * time.Millisecond,
			},
			currentBusyness: 70,
			busynessErr:     nil,
			currentFreq:     "not-a-frequency",
			expectedFreq:    "",
			nextSetIn:       10 * time.Millisecond,
		},
	}

	for _, tc := range tCases {
		t.Log(tc.testCase)

		tempDir := t.TempDir()
		getFreqTestFilepath := createTempFile(t, tempDir, tc.currentFreq+"\n")
		setFreqTestFilepath := createTempFile(t, tempDir, "")
		governorFilepath := createTempFile(t, tempDir, "userspace\n")

		getCPUFreqPathFunction = func(_ uint, resource string) string {
			switch resource {
			case "scaling_governor":
				return governorFilepath
			case "scaling_setspeed":
				return setFreqTestFilepath
			case "scaling_cur_freq":
				return getFreqTestFilepath
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
