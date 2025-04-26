package microwakeword

// #cgo CXXFLAGS: -std=c++11 -I${SRCDIR}/tensorflow
// #cgo CFLAGS: -I${SRCDIR}/tensorflow
// #cgo LDFLAGS: -L${SRCDIR}/tensorflow -ldl -lc++
//
// #include <stdlib.h>
// #include <tensorflow/lite/c/c_api.h>
import "C"
import (
	"encoding/json"
	"fmt"
	"github.com/pmdroid/microwakeword/pkg/microfrontend"
	"io/ioutil"
	"math"
	"path/filepath"
	"unsafe"
)

const (
	SamplesPerSecond  = 16000
	SamplesPerChunk   = 160 // 10ms
	BytesPerSample    = 2   // 16-bit
	BytesPerChunk     = SamplesPerChunk * BytesPerSample
	SecondsPerChunk   = float64(SamplesPerChunk) / float64(SamplesPerSecond)
	Stride            = 3
	DefaultRefractory = 2.0 // seconds
)

type ClipResult struct {
	Detected        bool
	DetectedSeconds *float64
	Probabilities   []float64
}

type MicroConfig struct {
	ProbabilityCutoff float64 `json:"probability_cutoff"`
	SlidingWindowSize int     `json:"sliding_window_size"`
}

type ModelConfig struct {
	WakeWord  string      `json:"wake_word"`
	ModelFile string      `json:"model"`
	Micro     MicroConfig `json:"micro"`
}

type MicroWakeWord struct {
	modelPath         string
	probabilityCutoff float64
	slidingWindowSize int
	refractorySeconds float64

	interpreter  *C.TfLiteInterpreter
	inputTensor  *C.TfLiteTensor
	outputTensor *C.TfLiteTensor

	dataType       C.TfLiteType
	inputScale     float32
	inputZeroPoint int32

	outputScale     float32
	outputZeroPoint int32

	frontend      *microfrontend.MicroFrontend
	features      [][]float32
	probabilities []float64
	audioBuffer   []byte
	ignoreSeconds float64
}

func NewMicroWakeWord(
	tfliteModel string,
	micro MicroConfig,
	refractorySeconds float64,
) (*MicroWakeWord, error) {
	mww := &MicroWakeWord{
		modelPath:         tfliteModel,
		probabilityCutoff: micro.ProbabilityCutoff,
		slidingWindowSize: micro.SlidingWindowSize,
		refractorySeconds: refractorySeconds,
		probabilities:     make([]float64, 0, micro.SlidingWindowSize),
		audioBuffer:       make([]byte, 0),
		features:          make([][]float32, 0),
	}

	if err := mww.loadModel(); err != nil {
		return nil, err
	}

	frontend, err := microfrontend.NewMicroFrontend()
	if err != nil {
		return nil, err
	}

	mww.frontend = frontend
	return mww, nil
}

func (mww *MicroWakeWord) loadModel() error {
	cModelPath := C.CString(mww.modelPath)
	defer C.free(unsafe.Pointer(cModelPath))

	model := C.TfLiteModelCreateFromFile(cModelPath)
	if model == nil {
		return fmt.Errorf("failed to load model: %s", mww.modelPath)
	}
	defer C.TfLiteModelDelete(model)

	options := C.TfLiteInterpreterOptionsCreate()
	defer C.TfLiteInterpreterOptionsDelete(options)

	mww.interpreter = C.TfLiteInterpreterCreate(model, options)
	if mww.interpreter == nil {
		return fmt.Errorf("failed to create interpreter")
	}

	if C.TfLiteInterpreterAllocateTensors(mww.interpreter) != C.kTfLiteOk {
		return fmt.Errorf("failed to allocate tensors")
	}

	mww.inputTensor = C.TfLiteInterpreterGetInputTensor(mww.interpreter, 0)

	mww.dataType = C.TfLiteTensorType(mww.inputTensor)
	mww.inputScale = float32(C.TfLiteTensorQuantizationParams(mww.inputTensor).scale)
	mww.inputZeroPoint = int32(C.TfLiteTensorQuantizationParams(mww.inputTensor).zero_point)

	mww.outputTensor = C.TfLiteInterpreterGetOutputTensor(mww.interpreter, 0)
	mww.outputScale = float32(C.TfLiteTensorQuantizationParams(mww.outputTensor).scale)
	mww.outputZeroPoint = int32(C.TfLiteTensorQuantizationParams(mww.outputTensor).zero_point)

	return nil
}

func (mww *MicroWakeWord) Reset() error {
	mww.audioBuffer = make([]byte, 0)
	mww.features = make([][]float32, 0)
	mww.probabilities = make([]float64, 0, mww.slidingWindowSize)
	mww.ignoreSeconds = 0

	mww.frontend.Close()
	frontend, err := microfrontend.NewMicroFrontend()
	if err != nil {
		return err
	}

	mww.frontend = frontend

	C.TfLiteInterpreterDelete(mww.interpreter)
	return mww.loadModel()
}

func FromBuiltin(
	modelName string,
	modelsDir string,
	refractorySeconds float64,
) (*MicroWakeWord, error) {
	configPath := filepath.Join(modelsDir, modelName+".json")
	return FromConfig(configPath, refractorySeconds)
}

func FromConfig(
	configPath string,
	refractorySeconds float64,
) (*MicroWakeWord, error) {
	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config ModelConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	configDir := filepath.Dir(configPath)
	modelPath := filepath.Join(configDir, config.ModelFile)
	return NewMicroWakeWord(
		modelPath,
		config.Micro,
		refractorySeconds,
	)
}

func (mww *MicroWakeWord) ProcessClip(audioBytes []byte) (*ClipResult, error) {
	audioIdx := 0
	numAudioBytes := len(audioBytes)
	features := make([][]float32, 0)

	for audioIdx+BytesPerChunk <= numAudioBytes {
		chunkBytes := audioBytes[audioIdx : audioIdx+BytesPerChunk]
		frontendResult := mww.frontend.ProcessSamples(chunkBytes)
		audioIdx += frontendResult.SamplesRead * BytesPerSample

		if len(frontendResult.Features) > 0 {
			features = append(features, frontendResult.Features)
		}
	}

	if len(features) < Stride {
		return &ClipResult{Detected: false}, nil
	}

	probabilities := make([]float64, 0)
	var detectedIdx *int
	for featuresIdx := 0; featuresIdx < len(features)-(len(features)%Stride); featuresIdx += Stride {
		if featuresIdx+Stride > len(features) {
			break
		}

		result, err := mww.runInference(features)
		if err != nil {
			return nil, err
		}

		probabilities = append(probabilities, result)
		if detectedIdx == nil && len(probabilities) >= mww.slidingWindowSize {
			mean := 0.0
			for i := len(probabilities) - mww.slidingWindowSize; i < len(probabilities); i++ {
				mean += probabilities[i]
			}
			mean /= float64(mww.slidingWindowSize)

			if mean > mww.probabilityCutoff {
				idx := featuresIdx
				detectedIdx = &idx
			}
		}
	}

	var detectedSeconds *float64
	if detectedIdx != nil {
		audioSeconds := float64(numAudioBytes/BytesPerSample) / float64(SamplesPerSecond)
		seconds := audioSeconds * (float64(*detectedIdx) / float64(len(features)))
		detectedSeconds = &seconds
	}

	err := mww.Reset()
	if err != nil {
		return nil, err
	}

	return &ClipResult{
		Detected:        detectedIdx != nil,
		DetectedSeconds: detectedSeconds,
		Probabilities:   probabilities,
	}, nil
}

func clampToInt8(val float32) int8 {
	if val > 127 {
		return 127
	} else if val < -128 {
		return -128
	}
	return int8(val)
}

func (mww *MicroWakeWord) runInference(features [][]float32) (float64, error) {
	var input []float32
	for _, feat := range features {
		input = append(input, feat...)
	}

	tensorData := make([]byte, len(input))
	for i, val := range input {
		scaledValue := val / mww.inputScale
		scaledWithZP := scaledValue + float32(mww.inputZeroPoint)
		tensorData[i] = byte(scaledWithZP)
	}

	cTensorData := unsafe.Pointer(&tensorData[0])
	C.TfLiteTensorCopyFromBuffer(mww.inputTensor, cTensorData, C.size_t(len(tensorData)))

	if C.TfLiteInterpreterInvoke(mww.interpreter) != C.kTfLiteOk {
		return 0, fmt.Errorf("failed to run inference")
	}

	outputSize := C.TfLiteTensorByteSize(mww.outputTensor)
	outputData := make([]byte, outputSize)
	cOutputData := unsafe.Pointer(&outputData[0])
	C.TfLiteTensorCopyToBuffer(mww.outputTensor, cOutputData, C.size_t(outputSize))

	floatVal := float64(outputData[0])
	zeroPointAdjusted := floatVal - float64(mww.outputZeroPoint)
	result := float64(mww.outputScale) * zeroPointAdjusted
	return result, nil
}

func (mww *MicroWakeWord) ProcessStreaming(audioBytes []byte) (bool, error) {
	mww.audioBuffer = append(mww.audioBuffer, audioBytes...)
	if len(mww.audioBuffer) < BytesPerChunk {
		return false, nil
	}

	audioBufferIdx := 0
	for audioBufferIdx+BytesPerChunk <= len(mww.audioBuffer) {
		chunkBytes := mww.audioBuffer[audioBufferIdx : audioBufferIdx+BytesPerChunk]
		frontendResult := mww.frontend.ProcessSamples(chunkBytes)
		audioBufferIdx += frontendResult.SamplesRead * BytesPerSample
		mww.ignoreSeconds = math.Max(0, mww.ignoreSeconds-SecondsPerChunk)

		if len(frontendResult.Features) == 0 {
			continue
		}

		mww.features = append(mww.features, frontendResult.Features)
		if len(mww.features) < Stride {
			continue
		}

		result, err := mww.runInference(mww.features)
		if err != nil {
			return false, err
		}

		mww.features = mww.features[0:0]
		mww.probabilities = append(mww.probabilities, result)
		if len(mww.probabilities) > mww.slidingWindowSize {
			mww.probabilities = mww.probabilities[1:]
		}

		if len(mww.probabilities) < mww.slidingWindowSize {
			continue
		}

		mean := 0.0
		for _, p := range mww.probabilities {
			mean += p
		}
		mean /= float64(len(mww.probabilities))

		if mean > mww.probabilityCutoff {
			if mww.ignoreSeconds <= 0 {
				mww.ignoreSeconds = mww.refractorySeconds
				mww.audioBuffer = make([]byte, 0)
				return true, nil
			}
		}
	}

	mww.audioBuffer = make([]byte, 0)
	return false, nil
}
