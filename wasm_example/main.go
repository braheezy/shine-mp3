package main

import (
	"bytes"
	"syscall/js"

	"github.com/braheezy/shine-mp3/pkg/mp3"
	"github.com/go-audio/wav"
)

func encodeWAV(this js.Value, args []js.Value) interface{} {
	// Get WAV data from JavaScript
	array := args[0]
	wavData := make([]byte, array.Length())
	js.CopyBytesToGo(wavData, array)

	// Decode WAV
	wavReader := bytes.NewReader(wavData)
	wavDecoder := wav.NewDecoder(wavReader)
	wavBuffer, err := wavDecoder.FullPCMBuffer()
	if err != nil {
		return js.ValueOf(map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Convert audio data to int16
	decodedData := make([]int16, len(wavBuffer.Data))
	for i, val := range wavBuffer.Data {
		decodedData[i] = int16(val)
	}

	// Create encoder with audio settings
	mp3Encoder := mp3.NewEncoder(wavBuffer.Format.SampleRate, wavBuffer.Format.NumChannels)

	// Create buffer for MP3 output
	var outBuffer bytes.Buffer

	// Encode to MP3
	err = mp3Encoder.Write(&outBuffer, decodedData)
	if err != nil {
		return js.ValueOf(map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Convert to Uint8Array for JavaScript
	mp3Data := outBuffer.Bytes()
	uint8Array := js.Global().Get("Uint8Array").New(len(mp3Data))
	js.CopyBytesToJS(uint8Array, mp3Data)

	return uint8Array
}

func main() {
	c := make(chan struct{})
	js.Global().Set("encodeMP3", js.FuncOf(encodeWAV))
	<-c
}
