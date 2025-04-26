package microfrontend

// #cgo LDFLAGS: -lmicrofrontend -lm -ltensorflowlite_c
/*
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include "tensorflow/lite/experimental/microfrontend/lib/frontend.h"
#include "tensorflow/lite/experimental/microfrontend/lib/frontend_util.h"
*/
import "C"
import (
	"fmt"
	"unsafe"
)

const (
	FeaturesStepSize        = 10
	PreprocessorFeatureSize = 40
	FeatureDurationMs       = 30
	AudioSampleFrequency    = 16000
	SamplesPerChunk         = FeaturesStepSize * (AudioSampleFrequency / 1000)
	Float32Scale            = 0.0390625
)

type ProcessOutput struct {
	Features    []float32
	SamplesRead int
}

type MicroFrontend struct {
	frontendConfig C.struct_FrontendConfig
	frontendState  C.struct_FrontendState
}

func NewMicroFrontend() (*MicroFrontend, error) {
	mf := &MicroFrontend{}

	mf.frontendConfig.window.size_ms = FeatureDurationMs
	mf.frontendConfig.window.step_size_ms = FeaturesStepSize
	mf.frontendConfig.filterbank.num_channels = PreprocessorFeatureSize
	mf.frontendConfig.filterbank.lower_band_limit = 125.0
	mf.frontendConfig.filterbank.upper_band_limit = 7500.0
	mf.frontendConfig.noise_reduction.smoothing_bits = 10
	mf.frontendConfig.noise_reduction.even_smoothing = 0.025
	mf.frontendConfig.noise_reduction.odd_smoothing = 0.06
	mf.frontendConfig.noise_reduction.min_signal_remaining = 0.05
	mf.frontendConfig.pcan_gain_control.enable_pcan = 1
	mf.frontendConfig.pcan_gain_control.strength = 0.95
	mf.frontendConfig.pcan_gain_control.offset = 80.0
	mf.frontendConfig.pcan_gain_control.gain_bits = 21
	mf.frontendConfig.log_scale.enable_log = 1
	mf.frontendConfig.log_scale.scale_shift = 6

	returnCode := C.FrontendPopulateState(&mf.frontendConfig, &mf.frontendState, AudioSampleFrequency)
	if returnCode != 1 {
		return nil, fmt.Errorf("failed to populate frontend state: %d", returnCode)
	}

	return mf, nil
}

func (mf *MicroFrontend) ProcessSamples(audioBytes []byte) *ProcessOutput {
	ptrInput := (*C.int16_t)(unsafe.Pointer(&audioBytes[0]))

	var samplesRead C.size_t
	frontendOutput := C.FrontendProcessSamples(&mf.frontendState, ptrInput,
		SamplesPerChunk, &samplesRead)

	output := &ProcessOutput{
		Features:    make([]float32, frontendOutput.size),
		SamplesRead: int(samplesRead),
	}

	values := unsafe.Slice(frontendOutput.values, frontendOutput.size)
	output.Features = make([]float32, frontendOutput.size)
	for i := 0; i < int(frontendOutput.size); i++ {
		output.Features[i] = float32(values[i]) * Float32Scale
	}

	return output
}

func (mf *MicroFrontend) Close() {
	C.FrontendFreeStateContents(&mf.frontendState)
}
