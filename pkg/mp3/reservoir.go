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
// bits. Note that stuffing bits are added by increasing a granule's
// part2_3_length. The bitstream formatter will detect this and write the
// appropriate stuffing bits to the bitstream.
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

	if stuffingBits != 0 {
		// plan a: put all into the first granule
		// This was preferred by someone designing a
		// real-time decoder...
		granInfo := &(sideInfo.Granules[0].Channels[0]).Tt
		if granInfo.Part2_3Length+uint64(stuffingBits) < 4095 {
			granInfo.Part2_3Length += uint64(stuffingBits)
		} else {
			// plan b: distribute throughout the granules
			for gr := int64(0); gr < enc.Mpeg.GranulesPerFrame; gr++ {
				for ch := int64(0); ch < enc.Wave.Channels; ch++ {
					granInfo := &(sideInfo.Granules[gr].Channels[ch]).Tt
					if stuffingBits == 0 {
						break
					}
					extraBits := int64(4095 - granInfo.Part2_3Length)
					bitsThisGranule := int64(0)
					if extraBits < stuffingBits {
						bitsThisGranule = extraBits
					} else {
						bitsThisGranule = stuffingBits
					}
					granInfo.Part2_3Length += uint64(bitsThisGranule)
					stuffingBits -= bitsThisGranule
				}
			}
			// If any stuffing bits remain, we elect to spill them
			// into ancillary data. The bitstream formatter will do this if
			// ReservoirDrain is set
			sideInfo.ReservoirDrain = stuffingBits
		}
	}
}
