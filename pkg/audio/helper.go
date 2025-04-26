package audio

import "encoding/binary"

const (
	SampleRate     = 16000
	BytesPerSample = 2                    // 16-bit PCM
	ChunkSize      = 160 * BytesPerSample // 10ms at 16kHz
)

func ToLittleEndian(audioBuffer [][2]float64, numSamples int) []byte {
	audioBytes := make([]byte, ChunkSize)
	for i := 0; i < numSamples && i*2+1 < len(audioBytes); i++ {
		sampleFloat := (audioBuffer[i][0] + audioBuffer[i][1]) / 2.0
		sampleInt := int16(sampleFloat * 32767.0)
		binary.LittleEndian.PutUint16(audioBytes[i*2:i*2+2], uint16(sampleInt))
	}

	return audioBytes
}
