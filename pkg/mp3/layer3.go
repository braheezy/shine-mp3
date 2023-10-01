package mp3

import (
	"encoding/binary"
	"fmt"
	"io"
)

const SHINE_MAX_SAMPLES = 1152

type channel int

const (
	PCM_MONO   channel = 1
	PCM_STEREO channel = 2
)

type mpegVersion int

const (
	MPEG_25 mpegVersion = 0
	MPEG_II mpegVersion = 2
	MPEG_I  mpegVersion = 3
)

type mpegLayer int

// Only Layer III currently implemented
const LAYER_III mpegLayer = 1

var mpegGranulesPerFrame = [4]int{
	// MPEG 2.5
	1,
	// Reserved
	-1,
	// MPEG II
	1,
	// MPEG I
	2,
}

func getMpegVersion(sampleRateIndex int) mpegVersion {
	if sampleRateIndex < 3 {
		return MPEG_I
	} else if sampleRateIndex < 6 {
		return MPEG_II
	} else {
		return MPEG_25
	}
}

// findSampleRateIndex checks if a given sampleRate is supported by the encoder
func findSampleRateIndex(freq int) (int, error) {
	var i int
	for i = 0; i < 9; i++ {
		if freq == int(sampleRates[i]) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("unsupported frequency: %v", freq)
}

// findBitrateIndex checks if a given bitrate is supported by the encoder
func findBitrateIndex(bitrate int, mpegVer mpegVersion) (int, error) {
	var i int
	for i = 0; i < 16; i++ {
		if bitrate == int(bitRates[i][mpegVer]) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("unsupported bitrate: %v", bitrate)
}

// CheckConfig checks if a given bitrate and sampleRate is supported by the encoder
func CheckConfig(freq int, bitrate int) (mpegVersion, error) {
	sampleRateIndex, err := findSampleRateIndex(freq)
	if err != nil {
		return -1, err
	}
	mpegVer := getMpegVersion(sampleRateIndex)
	_, err = findBitrateIndex(bitrate, mpegVer)
	if err != nil {
		return -1, err
	}
	return mpegVer, nil
}

// samplesPerPass returns the audio samples expected in each frame.
func (enc *Encoder) samplesPerPass() int64 {
	return enc.Mpeg.GranulesPerFrame * GRANULE_SIZE
}

// NewEncoder creates a new encoder with sensible encoding defaults
func NewEncoder(sampleRate, channels int) *Encoder {
	enc := new(Encoder)

	if channels > 1 {
		enc.Mpeg.Mode = STEREO
	} else {
		enc.Mpeg.Mode = MONO
	}

	enc.subbandInitialize()
	enc.mdctInitialize()
	enc.loopInitialize()
	enc.Wave.Channels = int64(channels)
	enc.Wave.SampleRate = int64(sampleRate)
	enc.Mpeg.Bitrate = 128
	enc.Mpeg.Emphasis = NONE
	enc.Mpeg.Copyright = 0
	enc.Mpeg.Original = 1
	enc.reservoirMaxSize = 0
	enc.reservoirSize = 0
	enc.Mpeg.Layer = int64(LAYER_III)
	enc.Mpeg.Crc = 0
	enc.Mpeg.Ext = 0
	enc.Mpeg.ModeExt = 0
	enc.Mpeg.BitsPerSlot = 8

	sampleRateIndex, _ := findSampleRateIndex(int(enc.Wave.SampleRate))
	enc.Mpeg.SampleRateIndex = int64(sampleRateIndex)

	enc.Mpeg.Version = getMpegVersion(int(enc.Mpeg.SampleRateIndex))

	bitrateIndex, _ := findBitrateIndex(int(enc.Mpeg.Bitrate), enc.Mpeg.Version)
	enc.Mpeg.BitrateIndex = int64(bitrateIndex)

	enc.Mpeg.GranulesPerFrame = int64(mpegGranulesPerFrame[enc.Mpeg.Version])
	avg_slots_per_frame := (float64(enc.Mpeg.GranulesPerFrame) * GRANULE_SIZE / (float64(enc.Wave.SampleRate))) * (float64(enc.Mpeg.Bitrate) * 1000 / float64(enc.Mpeg.BitsPerSlot))
	enc.Mpeg.WholeSlotsPerFrame = int64(avg_slots_per_frame)
	enc.Mpeg.FracSlotsPerFrame = avg_slots_per_frame - float64(enc.Mpeg.WholeSlotsPerFrame)
	enc.Mpeg.SlotLag = -enc.Mpeg.FracSlotsPerFrame
	if enc.Mpeg.FracSlotsPerFrame == 0 {
		enc.Mpeg.Padding = 0
	}
	enc.bitstream.open(BUFFER_SIZE)

	// determine the mean bitrate for main data
	if enc.Mpeg.GranulesPerFrame == 2 {
		// MPEG 1
		delta := 4 + 32
		if enc.Wave.Channels == 1 {
			delta = 4 + 9
		}
		enc.sideInfoLen = int64(8 * delta)
	} else {
		// MPEG 2
		delta := 4 + 17
		if enc.Wave.Channels == 1 {
			delta = 4 + 9
		}
		enc.sideInfoLen = int64(8 * delta)
	}
	return enc
}
func (enc *Encoder) encodeBufferInternal(stride int) ([]uint8, int) {
	if enc.Mpeg.FracSlotsPerFrame != 0 {
		if enc.Mpeg.SlotLag <= (enc.Mpeg.FracSlotsPerFrame - 1.0) {
			enc.Mpeg.Padding = 1
		} else {
			enc.Mpeg.Padding = 0
		}
		enc.Mpeg.SlotLag += float64(enc.Mpeg.Padding) - enc.Mpeg.FracSlotsPerFrame
	}
	enc.Mpeg.BitsPerFrame = (enc.Mpeg.WholeSlotsPerFrame + enc.Mpeg.Padding) * 8
	enc.meanBits = (enc.Mpeg.BitsPerFrame - enc.sideInfoLen) / enc.Mpeg.GranulesPerFrame

	// apply mdct to the polyphase output
	enc.mdctSub(int64(stride))

	// bit and noise allocation
	enc.iterationLoop()

	// write the frame to the bitstream
	enc.formatBitstream()

	// Return data
	written := enc.bitstream.dataPosition
	enc.bitstream.dataPosition = 0
	return enc.bitstream.data, written
}

func (enc *Encoder) EncodeBufferInterleaved(data []int16) ([]uint8, int) {
	enc.buffer[0] = &data[0]
	if enc.Wave.Channels == 2 {
		enc.buffer[1] = &data[1]
	}
	return enc.encodeBufferInternal(int(enc.Wave.Channels))
}

func (enc *Encoder) Write(out io.Writer, data []int16) error {
	samples_per_pass := int(enc.samplesPerPass())

	samplesRead := len(data)
	for i := 0; i < samplesRead; i += samples_per_pass * 2 {
		end := i + samples_per_pass
		if end > samplesRead {
			end = samplesRead
		}

		chunk := data[i:end]

		// Encode and write the chunk to the output file.
		data, written := enc.EncodeBufferInterleaved(chunk)
		err := binary.Write(out, binary.LittleEndian, data[:written])
		if err != nil {
			return err
		}
	}
	return nil
}
