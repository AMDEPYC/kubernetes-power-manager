package power

/*
#cgo CFLAGS: -I${SRCDIR}/../../../e-sms/e_smi/include -I${SRCDIR}/../../../e-sms/amd_hsmp
#cgo LDFLAGS: -L${SRCDIR}/../../../e-sms/e_smi/lib -le_smi64
#include <stdint.h>
#include "e_smi/e_smi.h"
*/
import "C"

import (
	"fmt"
)

type (
	uncoreFreq struct {
		min uint
		max uint
	}
	Uncore interface {
		write(pkgID, dieID uint) error
	}
)

func NewUncore(minFreq uint, maxFreq uint) (Uncore, error) {
	if !featureList.isFeatureIdSupported(UncoreFeature) {
		return nil, featureList.getFeatureIdError(UncoreFeature)
	}
	if minFreq < defaultUncore.min {
		return nil, fmt.Errorf("specified Min frequency is lower than %d kHZ allowed by the hardware", defaultUncore.min)
	}
	if maxFreq > defaultUncore.max {
		return nil, fmt.Errorf("specified Max frequency is higher than %d kHz allowed by the hardware", defaultUncore.max)
	}
	if maxFreq < minFreq {
		return nil, fmt.Errorf("max freq cannot be lower than min")
	}

	return &uncoreFreq{min: minFreq, max: maxFreq}, nil
}

func (u *uncoreFreq) write(pkgId, dieId uint) error {
	if u.min == u.max {
		esmi_status := C.esmi_apb_disable(C.uint32_t(pkgId), C.uint8_t(u.min))
		if esmi_status != 0 {
			return fmt.Errorf("DF Pstate set failed: [%d]", esmi_status)
		}
	} else {
		esmi_status := C.esmi_df_pstate_range_set(C.uint8_t(pkgId), C.uint8_t(u.min), C.uint8_t(u.max))
		if esmi_status != 0 {
			return fmt.Errorf("DF Pstate range set failed: [%d]", esmi_status)
		}
	}
	return nil
}

var (
	defaultUncore         = &uncoreFreq{}
	kernelModulesFilePath = "/proc/modules"
)

var (
	// TestHookInitUncoreBypassESMICalls is a flag used in tests to bypass calls to E-SMI in initUncore.
	TestHookInitUncoreBypassESMICalls bool
)

func initUncore() featureStatus {
	feature := featureStatus{
		name:     "Uncore frequency",
		driver:   "N/A",
		initFunc: initUncore,
	}

	if !initEsmi() {
		feature.err = fmt.Errorf("uncore feature error: %w", fmt.Errorf("ESMI Initialization failed"))
		return feature
	}

	/* TBD: add platform probe support instead of hardcoded values */
	defaultUncore.min = 0
	defaultUncore.max = 2

	var esmi_status C.esmi_status_t
	if !TestHookInitUncoreBypassESMICalls {
		esmi_status = C.esmi_df_pstate_range_set(C.uint8_t(0), C.uint8_t(defaultUncore.min), C.uint8_t(defaultUncore.max))
	}
	if esmi_status != 0 {
		feature.err = fmt.Errorf("uncore feature error: %w", fmt.Errorf("DF Pstate range set failed"))
		return feature
	}

	return feature
}

func initEsmi() bool {
	var esmi_status C.esmi_status_t
	if !TestHookInitUncoreBypassESMICalls {
		esmi_status = C.esmi_init()
	}
	if esmi_status != 0 {
		return false
	}
	return true
}

type hasUncore interface {
	SetUncore(uncore Uncore) error
	applyUncore() error
	getEffectiveUncore() Uncore
}

func (s *cpuTopology) SetUncore(uncore Uncore) error {
	s.uncore = uncore
	return s.applyUncore()
}

func (s *cpuTopology) getEffectiveUncore() Uncore {
	if s.uncore == nil {
		return defaultUncore
	}
	return s.uncore
}
func (s *cpuTopology) applyUncore() error {
	for _, pkg := range s.packages {
		if err := pkg.applyUncore(); err != nil {
			return err
		}
	}
	return nil
}
func (c *cpuPackage) SetUncore(uncore Uncore) error {
	c.uncore = uncore
	return c.applyUncore()
}

func (c *cpuPackage) applyUncore() error {
	for _, die := range c.dies {
		if err := die.applyUncore(); err != nil {
			return err
		}
	}
	return nil
}

func (c *cpuPackage) getEffectiveUncore() Uncore {
	if c.uncore != nil {
		return c.uncore
	}
	return c.topology.getEffectiveUncore()
}

func (d *cpuDie) SetUncore(uncore Uncore) error {
	d.uncore = uncore
	return d.applyUncore()
}

func (d *cpuDie) applyUncore() error {
	return d.getEffectiveUncore().write(d.parentSocket.GetID(), d.id)
}

func (d *cpuDie) getEffectiveUncore() Uncore {
	if d.uncore != nil {
		return d.uncore
	}
	return d.parentSocket.getEffectiveUncore()
}
