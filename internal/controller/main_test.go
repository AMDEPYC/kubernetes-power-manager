package controller

import (
	"testing"

	"github.com/intel/power-optimization-library/pkg/power"
)

func TestMain(m *testing.M) {
	// TODO: This is a cheap hack to bypass calls to E-SMI in initUncore
	// in all tests in this package. Ideally, we would make the uncore
	// feature testable and set this as part of power.Host mock instance
	// setup, but we don't manage the instance lifecycle properly in most
	// tests.
	power.TestHookInitUncoreBypassESMICalls = true
	m.Run()
}
