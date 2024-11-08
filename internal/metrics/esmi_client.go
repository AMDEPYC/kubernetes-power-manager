package metrics

/*
#cgo CFLAGS: -I${SRCDIR}/../../e-sms/e_smi/include -I${SRCDIR}/../../e-sms/amd_hsmp
#cgo LDFLAGS: -L${SRCDIR}/../../e-sms/e_smi/lib -le_smi64
#include <stdint.h>
#include <stdlib.h>
#include "e_smi/e_smi.h"

// CGO does not support bitfields, wrappers defined below adress that
struct dimm_power_no_bitfields {
    uint16_t power;
    uint16_t update_rate;
    uint8_t dimm_addr;
};

esmi_status_t esmi_dimm_power_consumption_get_wrapper(uint8_t pkg_id, uint8_t dimm_addr,
                          struct dimm_power_no_bitfields* cgo_dimm_power) {
    struct dimm_power dimm_power = {0};
    esmi_status_t status = esmi_dimm_power_consumption_get(pkg_id, dimm_addr, &dimm_power);

    if (status == 0) {
        cgo_dimm_power->power = dimm_power.power;
        cgo_dimm_power->update_rate = dimm_power.update_rate;
        cgo_dimm_power->dimm_addr = dimm_power.dimm_addr;
    }

    return status;
}

struct dimm_thermal_no_bitfields {
    uint16_t sensor;
    uint16_t update_rate;
    uint8_t dimm_addr;
    float temp;
};

esmi_status_t esmi_dimm_thermal_sensor_get_wrapper(uint8_t pkg_id, uint8_t dimm_addr,
                          struct dimm_thermal_no_bitfields* cgo_dimm_thermal) {
    struct dimm_thermal dimm_thermal = {0};
    esmi_status_t status = esmi_dimm_thermal_sensor_get(pkg_id, dimm_addr, &dimm_thermal);

    if (status == 0) {
        cgo_dimm_thermal->sensor = dimm_thermal.sensor;
        cgo_dimm_thermal->update_rate = dimm_thermal.update_rate;
        cgo_dimm_thermal->dimm_addr = dimm_thermal.dimm_addr;
        cgo_dimm_thermal->temp = dimm_thermal.temp;
    }

    return status;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/intel/power-optimization-library/pkg/power"

	"github.com/go-logr/logr"
)

// Enum of bandwidth types with names preserved from e_smi lib
const (
	AGG_BW int = 1 << iota
	RD_BW
	WR_BW
)

const (
	microMultiplier float64 = 1e-6
	milliMultiplier float64 = 1e-3
)

var (
	// ErrESMIMetricUnavailable is returned when status code of CGO function call indicates that the
	// metric is not available to read currently, but the issue might be temporary.
	ErrESMIMetricReadFailure = errors.New("esmi metric read failed")
)

// ESMIClient implements ESMI library CGO bindings. Instance should be
// created using constructor and passed as a pointer.
type ESMIClient struct{}

// ESMIClient initializes ESMI C library and returns pointer to ESMIClient.
func NewESMIClient(log logr.Logger) (*ESMIClient, error) {
	if esmiStatus := C.esmi_init(); esmiStatus != 0 {
		err := fmt.Errorf("esmi initialization failed: return code %d, ESMIClient cannot be created", esmiStatus)
		log.Error(err, "")
		return nil, err
	}

	return &ESMIClient{}, nil
}

func (e *ESMIClient) GetCoreEnergy(core power.Core) (float64, error) {
	var coreEnergy uint64
	if esmiStatus := C.esmi_core_energy_get(
		C.uint32_t(core.GetID()),
		(*C.uint64_t)(unsafe.Pointer(&coreEnergy)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	// Convert result from microJoules to Joules
	return float64(coreEnergy) * microMultiplier, nil
}

func (e *ESMIClient) GetPackageEnergy(pkg power.Package) (float64, error) {
	var pkgEnergy uint64
	if esmiStatus := C.esmi_socket_energy_get(
		C.uint32_t(pkg.GetID()),
		(*C.uint64_t)(unsafe.Pointer(&pkgEnergy)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	// Convert result from microJoules to Joules
	return float64(pkgEnergy) * microMultiplier, nil
}

func (e *ESMIClient) GetDataFabricClock(pkg power.Package) (uint32, error) {
	var fabricClock, dataClock uint32
	if esmiStatus := C.esmi_fclk_mclk_get(
		C.uint32_t(pkg.GetID()),
		(*C.uint32_t)(unsafe.Pointer(&fabricClock)),
		(*C.uint32_t)(unsafe.Pointer(&dataClock)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return fabricClock, nil
}

func (e *ESMIClient) GetMemoryClock(pkg power.Package) (uint32, error) {
	var fabricClock, dataClock uint32
	if esmiStatus := C.esmi_fclk_mclk_get(
		C.uint32_t(pkg.GetID()),
		(*C.uint32_t)(unsafe.Pointer(&fabricClock)),
		(*C.uint32_t)(unsafe.Pointer(&dataClock)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return dataClock, nil
}

func (e *ESMIClient) GetCoreClockThrottleLimit(pkg power.Package) (uint32, error) {
	var cclk uint32
	if esmiStatus := C.esmi_cclk_limit_get(
		C.uint32_t(pkg.GetID()),
		(*C.uint32_t)(unsafe.Pointer(&cclk)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return cclk, nil
}

func (e *ESMIClient) GetPackageFreqLimit(pkg power.Package) (uint16, error) {
	var freq uint16
	// size 8 is taken from esmi_library repository
	cArr := C.malloc(C.size_t(8) * C.size_t(unsafe.Sizeof(uintptr(0))))
	defer C.free(unsafe.Pointer(cArr))

	if esmiStatus := C.esmi_socket_current_active_freq_limit_get(
		C.uint32_t(pkg.GetID()),
		(*C.uint16_t)(unsafe.Pointer(&freq)),
		((**C.char)(cArr)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return freq, nil
}

func (e *ESMIClient) GetPackageMinFreq(pkg power.Package) (uint16, error) {
	var fMin, fMax uint16

	if esmiStatus := C.esmi_socket_freq_range_get(
		C.uint8_t(pkg.GetID()),
		(*C.uint16_t)(unsafe.Pointer(&fMax)),
		(*C.uint16_t)(unsafe.Pointer(&fMin)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return fMin, nil
}

func (e *ESMIClient) GetPackageMaxFreq(pkg power.Package) (uint16, error) {
	var fMin, fMax uint16

	if esmiStatus := C.esmi_socket_freq_range_get(
		C.uint8_t(pkg.GetID()),
		(*C.uint16_t)(unsafe.Pointer(&fMax)),
		(*C.uint16_t)(unsafe.Pointer(&fMin)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return fMax, nil
}

func (e *ESMIClient) GetCoreFreqLimit(core power.Core) (uint32, error) {
	var freq uint32
	if esmiStatus := C.esmi_current_freq_limit_core_get(
		C.uint32_t(core.GetID()),
		(*C.uint32_t)(unsafe.Pointer(&freq)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return freq, nil
}

func (e *ESMIClient) GetRailFreqLimitPolicy(pkg power.Package) (uint8, error) {
	var railPolicy bool

	if esmiStatus := C.esmi_cpurail_isofreq_policy_get(
		C.uint8_t(pkg.GetID()),
		(*C.bool)(unsafe.Pointer(&railPolicy)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	if railPolicy {
		return 1, nil
	}
	return 0, nil
}

func (e *ESMIClient) GetDFCStateEnablingControl(pkg power.Package) (uint8, error) {
	var dfCStateControl bool

	if esmiStatus := C.esmi_dfc_ctrl_setting_get(
		C.uint8_t(pkg.GetID()),
		(*C.bool)(unsafe.Pointer(&dfCStateControl)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	if dfCStateControl {
		return 1, nil
	}
	return 0, nil
}

func (e *ESMIClient) GetPackagePower(pkg power.Package) (float64, error) {
	var pkgPower uint32
	if esmiStatus := C.esmi_socket_power_get(
		C.uint32_t(pkg.GetID()),
		(*C.uint32_t)(unsafe.Pointer(&pkgPower)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	// Convert result from milliwatts to watts.
	return float64(pkgPower) * milliMultiplier, nil
}

func (e *ESMIClient) GetPackagePowerCap(pkg power.Package) (float64, error) {
	var pkgPowerCap uint32
	if esmiStatus := C.esmi_socket_power_cap_get(
		C.uint32_t(pkg.GetID()),
		(*C.uint32_t)(unsafe.Pointer(&pkgPowerCap)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	// Convert result from milliwatts to watts.
	return float64(pkgPowerCap) * milliMultiplier, nil
}

func (e *ESMIClient) GetPackagePowerMaxCap(pkg power.Package) (float64, error) {
	var pkgPowerMaxCap uint32
	if esmiStatus := C.esmi_socket_power_cap_max_get(
		C.uint32_t(pkg.GetID()),
		(*C.uint32_t)(unsafe.Pointer(&pkgPowerMaxCap)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	// Convert result from milliwatts to watts.
	return float64(pkgPowerMaxCap) * milliMultiplier, nil
}

func (e *ESMIClient) GetPackagePowerEfficiencyMode(pkg power.Package) (uint8, error) {
	var pkgPowerEffMode uint8
	if esmiStatus := C.esmi_pwr_efficiency_mode_get(
		C.uint8_t(pkg.GetID()),
		(*C.uint8_t)(unsafe.Pointer(&pkgPowerEffMode)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return pkgPowerEffMode, nil
}

func (e *ESMIClient) GetCoreBoostLimit(core power.Core) (uint32, error) {
	var pBoostLimit uint32
	if esmiStatus := C.esmi_core_boostlimit_get(
		C.uint32_t(core.GetID()),
		(*C.uint32_t)(unsafe.Pointer(&pBoostLimit)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return pBoostLimit, nil
}

func (e *ESMIClient) GetPackageC0Residency(pkg power.Package) (uint32, error) {
	var c0Residency uint32
	if esmiStatus := C.esmi_socket_c0_residency_get(
		C.uint32_t(pkg.GetID()),
		(*C.uint32_t)(unsafe.Pointer(&c0Residency)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return c0Residency, nil
}

func (e *ESMIClient) GetPackageTemp(pkg power.Package) (float64, error) {
	var pkgTemp uint32
	if esmiStatus := C.esmi_socket_temperature_get(
		C.uint32_t(pkg.GetID()),
		(*C.uint32_t)(unsafe.Pointer(&pkgTemp)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	// Convert result from millicelsius to celsius.
	return float64(pkgTemp) * milliMultiplier, nil
}

func (e *ESMIClient) GetDDRBandwidthUtil() (uint32, error) {
	ddrBW := C.struct_ddr_bw_metrics{
		max_bw:       C.uint32_t(0),
		utilized_bw:  C.uint32_t(0),
		utilized_pct: C.uint32_t(0),
	}

	if esmiStatus := C.esmi_ddr_bw_get(
		&ddrBW,
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return uint32(ddrBW.utilized_bw), nil
}

func (e *ESMIClient) GetDDRBandwidthUtilPercent() (uint32, error) {
	ddrBW := C.struct_ddr_bw_metrics{
		max_bw:       C.uint32_t(0),
		utilized_bw:  C.uint32_t(0),
		utilized_pct: C.uint32_t(0),
	}

	if esmiStatus := C.esmi_ddr_bw_get(
		&ddrBW,
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return uint32(ddrBW.utilized_pct), nil
}

func (e *ESMIClient) GetDIMMPower(pkg power.Package, dimmAddr uint8) (float64, error) {
	dimmPower := C.struct_dimm_power_no_bitfields{
		power:       C.uint16_t(0),
		update_rate: C.uint16_t(0),
		dimm_addr:   C.uint8_t(0),
	}

	if esmiStatus := C.esmi_dimm_power_consumption_get_wrapper(
		C.uint8_t(pkg.GetID()),
		C.uint8_t(dimmAddr),
		&dimmPower,
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	// update_rate == 0 is indication of metric unavailability on the system
	if dimmPower.update_rate == 0 {
		return 0, ErrMetricMissing
	}

	return float64(dimmPower.power) * milliMultiplier, nil
}

func (e *ESMIClient) GetDIMMTemp(pkg power.Package, dimmAddr uint8) (float32, error) {
	dimmThermal := C.struct_dimm_thermal_no_bitfields{
		sensor:      C.uint16_t(0),
		update_rate: C.uint16_t(0),
		dimm_addr:   C.uint8_t(0),
		temp:        C.float(0),
	}

	if esmiStatus := C.esmi_dimm_thermal_sensor_get_wrapper(
		C.uint8_t(pkg.GetID()),
		C.uint8_t(dimmAddr),
		&dimmThermal,
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	// update_rate == 0 is indication of metric unavailability on the system
	if dimmThermal.update_rate == 0 {
		return 0, ErrMetricMissing
	}

	return float32(dimmThermal.temp), nil
}

func (e *ESMIClient) GetLCLKDPMMaxLevel(pkg power.Package, nbioID uint8) (uint8, error) {
	dpmLevel := C.struct_dpm_level{
		max_dpm_level: C.uint8_t(0),
		min_dpm_level: C.uint8_t(0),
	}

	if esmiStatus := C.esmi_socket_lclk_dpm_level_get(
		C.uint8_t(pkg.GetID()),
		C.uint8_t(nbioID),
		&dpmLevel,
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return uint8(dpmLevel.max_dpm_level), nil
}

func (e *ESMIClient) GetLCLKDPMMinLevel(pkg power.Package, nbioID uint8) (uint8, error) {
	dpmLevel := C.struct_dpm_level{
		max_dpm_level: C.uint8_t(0),
		min_dpm_level: C.uint8_t(0),
	}

	if esmiStatus := C.esmi_socket_lclk_dpm_level_get(
		C.uint8_t(pkg.GetID()),
		C.uint8_t(nbioID),
		&dpmLevel,
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return uint8(dpmLevel.min_dpm_level), nil
}

func (e *ESMIClient) GetIOLinkBandwidthUtil(pkg power.Package, linkName string) (uint32, error) {
	var ioBW uint32
	linkIDBandwidthType := C.struct_link_id_bw_type{
		// Only aggregate bandwidth type is supported for this metric
		bw_type:   C.io_bw_encoding(AGG_BW),
		link_name: C.CString(linkName),
	}

	if esmiStatus := C.esmi_current_io_bandwidth_get(
		C.uint8_t(pkg.GetID()),
		linkIDBandwidthType,
		(*C.uint32_t)(unsafe.Pointer(&ioBW)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return ioBW, nil
}

func (e *ESMIClient) GetxGMIAggregateBandwidthUtil(linkName string) (uint32, error) {
	var xGMIBW uint32
	linkIDBandwidthType := C.struct_link_id_bw_type{
		bw_type:   C.io_bw_encoding(AGG_BW),
		link_name: C.CString(linkName),
	}

	if esmiStatus := C.esmi_current_xgmi_bw_get(
		linkIDBandwidthType,
		(*C.uint32_t)(unsafe.Pointer(&xGMIBW)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return xGMIBW, nil
}

func (e *ESMIClient) GetxGMIReadBandwidthUtil(linkName string) (uint32, error) {
	var xGMIBW uint32
	linkIDBandwidthType := C.struct_link_id_bw_type{
		bw_type:   C.io_bw_encoding(RD_BW),
		link_name: C.CString(linkName),
	}

	if esmiStatus := C.esmi_current_xgmi_bw_get(
		linkIDBandwidthType,
		(*C.uint32_t)(unsafe.Pointer(&xGMIBW)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return xGMIBW, nil
}

func (e *ESMIClient) GetxGMIWriteBandwidthUtil(linkName string) (uint32, error) {
	var xGMIBW uint32
	linkIDBandwidthType := C.struct_link_id_bw_type{
		bw_type:   C.io_bw_encoding(WR_BW),
		link_name: C.CString(linkName),
	}

	if esmiStatus := C.esmi_current_xgmi_bw_get(
		linkIDBandwidthType,
		(*C.uint32_t)(unsafe.Pointer(&xGMIBW)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return xGMIBW, nil
}

func (e *ESMIClient) GetCPUFamily() (uint32, error) {
	var cpuFamily uint32
	if esmiStatus := C.esmi_cpu_family_get(
		(*C.uint32_t)(unsafe.Pointer(&cpuFamily)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return cpuFamily, nil
}

func (e *ESMIClient) GetCPUModel() (uint32, error) {
	var cpuModel uint32
	if esmiStatus := C.esmi_cpu_model_get(
		(*C.uint32_t)(unsafe.Pointer(&cpuModel)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return cpuModel, nil
}

func (e *ESMIClient) GetNumberOfCPUsPerCore() (uint32, error) {
	var threadsPerCore uint32
	if esmiStatus := C.esmi_threads_per_core_get(
		(*C.uint32_t)(unsafe.Pointer(&threadsPerCore)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return threadsPerCore, nil
}

func (e *ESMIClient) GetNumberOfCPUs() (uint32, error) {
	var cpusTotal uint32
	if esmiStatus := C.esmi_number_of_cpus_get(
		(*C.uint32_t)(unsafe.Pointer(&cpusTotal)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return cpusTotal, nil
}

func (e *ESMIClient) GetNumberOfPackages() (uint32, error) {
	var pkgsTotal uint32
	if esmiStatus := C.esmi_number_of_sockets_get(
		(*C.uint32_t)(unsafe.Pointer(&pkgsTotal)),
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return pkgsTotal, nil
}

func (e *ESMIClient) GetSMUFWMajorVersion() (uint8, error) {
	fwVersion := C.struct_smu_fw_version{
		debug:  C.uint8_t(0),
		minor:  C.uint8_t(0),
		major:  C.uint8_t(0),
		unused: C.uint8_t(0),
	}

	if esmiStatus := C.esmi_smu_fw_version_get(
		&fwVersion,
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return uint8(fwVersion.major), nil
}

func (e *ESMIClient) GetSMUFWMinorVersion() (uint8, error) {
	fwVersion := C.struct_smu_fw_version{
		debug:  C.uint8_t(0),
		minor:  C.uint8_t(0),
		major:  C.uint8_t(0),
		unused: C.uint8_t(0),
	}

	if esmiStatus := C.esmi_smu_fw_version_get(
		&fwVersion,
	); esmiStatus != 0 {
		return 0, mapESMIError(esmiStatus)
	}

	return uint8(fwVersion.minor), nil
}

// mapESMIError maps esmi_status_t (CGO functions return code), which is integer, to golang errors.
func mapESMIError(esmiErr C.esmi_status_t) error {
	switch esmiErr {
	// Errors numbers which indicate metric won't be readable during process lifitme.
	case C.ESMI_NO_ENERGY_DRV, C.ESMI_NO_MSR_DRV, C.ESMI_NO_HSMP_DRV, C.ESMI_NO_HSMP_SUP, C.ESMI_NO_DRV,
		C.ESMI_NOT_SUPPORTED, C.ESMI_NOT_INITIALIZED, C.ESMI_FILE_ERROR, C.ESMI_INVALID_INPUT, C.ESMI_PERMISSION,
		C.ESMI_FILE_NOT_FOUND, C.ESMI_NO_HSMP_MSG_SUP:
		return fmt.Errorf("%w: esmi function returned %d status code", ErrMetricMissing, esmiErr)
	default:
		return fmt.Errorf("%w: esmi function returned %d status code", ErrESMIMetricReadFailure, esmiErr)
	}
}
