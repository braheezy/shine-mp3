package mp3

// formatBitstream is called after a frame of audio has been quantized and coded.
// It will write the encoded audio to the bitstream. Note that
// from a layer3 encoder's perspective the bit stream is primarily
// a series of main_data() blocks, with header and side information
// inserted at the proper locations to maintain framing. (See Figure A.7 in the IS).
func (enc *Encoder) formatBitstream() {
	for ch := int64(0); ch < enc.Wave.Channels; ch++ {
		for gr := int64(0); gr < enc.Mpeg.GranulesPerFrame; gr++ {
			pi := &enc.l3Encoding[ch][gr]
			pr := &enc.mdctFrequency[ch][gr]
			for i := 0; i < GRANULE_SIZE; i++ {

				if pr[i] < 0 && pi[i] > 0 {
					pi[i] *= -1
				}
			}
		}
	}
	enc.encodeSideInfo()
	enc.encodeMainData()
}
func (enc *Encoder) encodeMainData() {
	sideInfo := enc.sideInfo

	for gr := int64(0); gr < enc.Mpeg.GranulesPerFrame; gr++ {
		for ch := int64(0); ch < enc.Wave.Channels; ch++ {
			granInfo := &sideInfo.Granules[gr].Channels[ch].Tt
			sLen1 := uint64(sLen1Table[granInfo.ScaleFactorCompress])
			sLen2 := uint64(sLen2Table[granInfo.ScaleFactorCompress])
			ix := &enc.l3Encoding[ch][gr]

			if gr == 0 || sideInfo.ScaleFactorSelectInfo[ch][0] == 0 {
				for scaleFactorBand := 0; scaleFactorBand < 6; scaleFactorBand++ {
					enc.bitstream.putBits(uint32(enc.scaleFactor.L[gr][ch][scaleFactorBand]), uint(sLen1))
				}
			}
			if gr == 0 || sideInfo.ScaleFactorSelectInfo[ch][1] == 0 {
				for scaleFactorBand := 6; scaleFactorBand < 11; scaleFactorBand++ {
					enc.bitstream.putBits(uint32(enc.scaleFactor.L[gr][ch][scaleFactorBand]), uint(sLen1))
				}
			}
			if gr == 0 || sideInfo.ScaleFactorSelectInfo[ch][2] == 0 {
				for scaleFactorBand := 11; scaleFactorBand < 16; scaleFactorBand++ {
					enc.bitstream.putBits(uint32(enc.scaleFactor.L[gr][ch][scaleFactorBand]), uint(sLen2))
				}
			}
			if gr == 0 || sideInfo.ScaleFactorSelectInfo[ch][3] == 0 {
				for scaleFactorBand := 16; scaleFactorBand < 21; scaleFactorBand++ {
					enc.bitstream.putBits(uint32(enc.scaleFactor.L[gr][ch][scaleFactorBand]), uint(sLen2))
				}
			}
			enc.huffmanCodeBits(ix, granInfo)
		}
	}
}
func (enc *Encoder) encodeSideInfo() {

	sideInfo := enc.sideInfo

	enc.bitstream.putBits(2047, 11)
	enc.bitstream.putBits(uint32(enc.Mpeg.Version), 2)
	enc.bitstream.putBits(uint32(enc.Mpeg.Layer), 2)
	if enc.Mpeg.Crc == 0 {
		enc.bitstream.putBits(uint32(1), 1)
	} else {
		enc.bitstream.putBits(uint32(0), 1)
	}
	enc.bitstream.putBits(uint32(enc.Mpeg.BitrateIndex), 4)
	enc.bitstream.putBits(uint32(enc.Mpeg.SampleRateIndex%3), 2)
	enc.bitstream.putBits(uint32(enc.Mpeg.Padding), 1)
	enc.bitstream.putBits(uint32(enc.Mpeg.Ext), 1)
	enc.bitstream.putBits(uint32(enc.Mpeg.Mode), 2)
	enc.bitstream.putBits(uint32(enc.Mpeg.ModeExt), 2)
	enc.bitstream.putBits(uint32(enc.Mpeg.Copyright), 1)
	enc.bitstream.putBits(uint32(enc.Mpeg.Original), 1)
	enc.bitstream.putBits(uint32(enc.Mpeg.Emphasis), 2)
	if enc.Mpeg.Version == MPEG_I {
		enc.bitstream.putBits(0, 9)
		if enc.Wave.Channels == 2 {
			enc.bitstream.putBits(uint32(sideInfo.PrivateBits), 3)
		} else {
			enc.bitstream.putBits(uint32(sideInfo.PrivateBits), 5)
		}
	} else {
		enc.bitstream.putBits(0, 8)
		if enc.Wave.Channels == 2 {
			enc.bitstream.putBits(uint32(sideInfo.PrivateBits), 2)
		} else {
			enc.bitstream.putBits(uint32(sideInfo.PrivateBits), 1)
		}
	}
	if enc.Mpeg.Version == MPEG_I {
		for ch := int64(0); ch < enc.Wave.Channels; ch++ {
			for scaleFactorSelectionInfoBand := 0; scaleFactorSelectionInfoBand < 4; scaleFactorSelectionInfoBand++ {
				enc.bitstream.putBits(uint32(sideInfo.ScaleFactorSelectInfo[ch][scaleFactorSelectionInfoBand]), 1)
			}
		}
	}
	for gr := int64(0); gr < enc.Mpeg.GranulesPerFrame; gr++ {
		for ch := int64(0); ch < enc.Wave.Channels; ch++ {
			granInfo := &sideInfo.Granules[gr].Channels[ch].Tt
			enc.bitstream.putBits(uint32(granInfo.Part2_3Length), 12)
			enc.bitstream.putBits(uint32(granInfo.BigValues), 9)
			enc.bitstream.putBits(uint32(granInfo.GlobalGain), 8)
			if enc.Mpeg.Version == MPEG_I {
				enc.bitstream.putBits(uint32(granInfo.ScaleFactorCompress), 4)
			} else {
				enc.bitstream.putBits(uint32(granInfo.ScaleFactorCompress), 9)
			}
			enc.bitstream.putBits(0, 1)
			for region := 0; region < 3; region++ {
				enc.bitstream.putBits(uint32(granInfo.TableSelect[region]), 5)
			}
			enc.bitstream.putBits(uint32(granInfo.Region0Count), 4)
			enc.bitstream.putBits(uint32(granInfo.Region1Count), 3)
			if enc.Mpeg.Version == MPEG_I {
				enc.bitstream.putBits(uint32(granInfo.PreFlag), 1)
			}
			enc.bitstream.putBits(uint32(granInfo.ScaleFactorScale), 1)
			enc.bitstream.putBits(uint32(granInfo.Count1TableSelect), 1)
		}
	}
}
func (enc *Encoder) huffmanCodeBits(ix *[GRANULE_SIZE]int64, gi *GranuleInfo) {
	scaleFactor := scaleFactorBandIndex[enc.Mpeg.SampleRateIndex]

	bits := int64(enc.bitstream.getBitsCount())
	bigValues := int64(gi.BigValues << 1)
	scaleFactorIndex := gi.Region0Count + 1
	region1Start := scaleFactor[scaleFactorIndex]
	scaleFactorIndex += gi.Region1Count + 1
	region2Start := scaleFactor[scaleFactorIndex]
	for i := int64(0); i < bigValues; i += 2 {
		idx := 0
		if i >= region1Start {
			idx++
		}
		if i >= region2Start {
			idx++
		}
		var tableIndex uint64 = gi.TableSelect[idx]
		if tableIndex != 0 {
			x := (*ix)[i]
			y := (*ix)[i+1]
			huffmanCode(&enc.bitstream, int64(tableIndex), x, y)
		}
	}
	h := &huffmanCodeTable[gi.Count1TableSelect+32]
	count1End := int64(uint64(bigValues) + (gi.Count1 << 2))
	for i := bigValues; i < count1End; i += 4 {
		v := (*ix)[i]
		w := (*ix)[i+1]
		x := (*ix)[i+2]
		y := (*ix)[i+3]
		huffmanCoderCount1(&enc.bitstream, h, v, w, x, y)
	}
	bits = int64(enc.bitstream.getBitsCount()) - bits
	bits = int64(gi.Part2_3Length - gi.Part2Length - uint64(bits))
	if bits != 0 {
		var (
			stuffingWords int64 = bits / 32
			remainingBits int64 = bits % 32
		)
		// Due to the nature of the Huffman code tables, we will pad with ones
		for stuffingWords != 0 {
			enc.bitstream.putBits(^uint32(0), 32)
			stuffingWords--
		}
		if remainingBits != 0 {
			enc.bitstream.putBits(uint32((1<<remainingBits)-1), uint(remainingBits))
		}
	}
}
func absAndSign(x *int64) int64 {
	if *x > 0 {
		return 0
	}
	*x *= -1
	return 1
}
func huffmanCoderCount1(bs *bitstream, h *huffCodeTableInfo, v int64, w int64, x int64, y int64) {

	code := uint64(0)
	cBits := uint(0)

	signV := uint64(absAndSign(&v))
	signW := uint64(absAndSign(&w))
	signX := uint64(absAndSign(&x))
	signY := uint64(absAndSign(&y))
	p := v + (w << 1) + (x << 2) + (y << 3)
	bs.putBits(uint32(h.table[p]), uint(h.hLen[p]))
	if v != 0 {
		code = signV
		cBits = 1
	}
	if w != 0 {
		code = (code << 1) | signW
		cBits++
	}
	if x != 0 {
		code = (code << 1) | signX
		cBits++
	}
	if y != 0 {
		code = (code << 1) | signY
		cBits++
	}
	bs.putBits(uint32(code), cBits)
}
func huffmanCode(bs *bitstream, tableSelect int64, x int64, y int64) {
	xBits := int64(0)
	ext := uint64(0)

	signX := uint64(absAndSign(&x))
	signY := uint64(absAndSign(&y))
	h := &(huffmanCodeTable[tableSelect])
	yLen := uint64(h.yLen)
	if tableSelect > 15 {
		var (
			linBitsX uint64 = 0
			linBitsY uint64 = 0
			linBits  uint64 = uint64(h.linBits)
		)
		if x > 14 {
			linBitsX = uint64(x - 15)
			x = 15
		}
		if y > 14 {
			linBitsY = uint64(y - 15)
			y = 15
		}
		idx := (uint64(x) * yLen) + uint64(y)
		code := uint64(h.table[idx])
		cBits := int64(h.hLen[idx])
		if x > 14 {
			ext |= linBitsX
			xBits += int64(linBits)
		}
		if x != 0 {
			ext <<= 1
			ext |= signX
			xBits += 1
		}
		if y > 14 {
			ext <<= linBits
			ext |= linBitsY
			xBits += int64(linBits)
		}
		if y != 0 {
			ext <<= 1
			ext |= signY
			xBits += 1
		}
		bs.putBits(uint32(code), uint(cBits))
		bs.putBits(uint32(ext), uint(xBits))
	} else {
		idx := (uint64(x) * yLen) + uint64(y)
		code := uint64(h.table[idx])
		cBits := int64(h.hLen[idx])
		if x != 0 {
			code <<= 1
			code |= signX
			cBits += 1
		}
		if y != 0 {
			code <<= 1
			code |= signY
			cBits += 1
		}
		bs.putBits(uint32(code), uint(cBits))
	}
}
