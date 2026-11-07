// Layer3 bit reservoir: Described in C.1.5.4.2.2 of the IS
package mp3

// maxReservoirBits is called at the beginning of each granule to get the max bit
// allowance for the current granule based on reservoir size and perceptual entropy.
func (enc *Encoder) maxReservoirBits(perceptualEntropy *float64) int64 {
	meanBits := enc.meanBits

	meanBits /= enc.Wave.Channels
	maxBits := meanBits
	if maxBits > 4095 {
		maxBits = 4095
	}
	if enc.reservoirMaxSize == 0 {
		return maxBits
	}
	moreBits := int64(*perceptualEntropy*3.1 - float64(meanBits))
	addBits := int64(0)
	if moreBits > 100 {
		var frac int64 = (enc.reservoirSize * 6) / 10
		if frac < moreBits {
			addBits = frac
		} else {
			addBits = moreBits
		}
	}
	overBits := enc.reservoirSize - (enc.reservoirMaxSize<<3)/10 - addBits
	if overBits > 0 {
		addBits += overBits
	}
	maxBits += addBits
	if maxBits > 4095 {
		maxBits = 4095
	}
	return maxBits
}

// reservoirAdjust is called after a granule's bit allocation. It readjusts the size of
// the reservoir to reflect the granule's usage.
func (enc *Encoder) reservoirAdjust(gi *GranuleInfo) {
	enc.reservoirSize += int64(uint64(enc.meanBits/enc.Wave.Channels) - gi.Part2_3Length)
}

// Called after all granules in a frame have been allocated. Makes sure
// that the reservoir size is within limits, possibly by adding stuffing
// bits. The stuffing bits follow the frame's last granule as ancillary
// data (ReservoirDrain, written by encodeMainData) and are counted in no
// granule's part2_3_length: a decoder reads part2_3_length as scale
// factors and Huffman codes, so stuffing inside it is read as count1
// quadruples, the last of which can run past the end of the frame — a
// frame some decoders (Symphonia, Chromium's) reject. Nor need the
// stuffing then fit in a granule's 12-bit part2_3_length.
func (enc *Encoder) reservoirFrameEnd() {
	sideInfo := &enc.sideInfo

	ancillaryPad := int64(0)
	// just in case mean_bits is odd, this is necessary...
	if enc.Wave.Channels == 2 && (enc.meanBits&1) != 0 {
		enc.reservoirSize += 1
	}
	overBits := enc.reservoirSize - enc.reservoirMaxSize
	if overBits < 0 {
		overBits = 0
	}
	enc.reservoirSize -= overBits
	stuffingBits := overBits + ancillaryPad

	// we must be byte aligned
	overBits = enc.reservoirSize % 8
	if overBits != 0 {
		stuffingBits += overBits
		enc.reservoirSize -= overBits
	}

	sideInfo.ReservoirDrain = stuffingBits
}
