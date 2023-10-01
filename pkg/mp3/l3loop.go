package mp3

import (
	"math"
)

const (
	scaleFactorBand_LMax = 22
	enTotKrit            = 10
	enDifKrit            = 100
	enScfsiBandKrit      = 10
	xmScfsiBandKrit      = 10
)

// innerLoop selects the best quantizerStepSize for a particular set of scaleFactors.
func (enc *Encoder) innerLoop(ix *[GRANULE_SIZE]int64, maxBits int64, codeInfo *GranuleInfo, gr int64, ch int64) int64 {
	bits := int64(0)

	if maxBits < 0 {
		codeInfo.QuantizerStepSize--
	}
	for {
		codeInfo.QuantizerStepSize++
		// within table range?
		for enc.quantize(ix, codeInfo.QuantizerStepSize) > 8192 {
			codeInfo.QuantizerStepSize++
		}
		calcRunLength(ix, codeInfo)
		bits = count1BitCount(ix, codeInfo)
		enc.subDivide(codeInfo)
		bigValuesTableSelect(ix, codeInfo)
		bits += bigValuesBitCount(ix, codeInfo)
		if bits <= maxBits {
			break
		}
	}
	return bits
}

// outerLoop controls the masking conditions of all scaleFactorBands. It computes the best scaleFactor and
// global gain. This module calls the inner iteration loop.
// l3XMin: the allowed distortion of the scaleFactor.
// ix: vector of quantized values ix(0..575)
func (enc *Encoder) outerLoop(maxBits int64, l3XMin *PsyXMin, ix *[GRANULE_SIZE]int64, gr int64, ch int64) int64 {

	sideInfo := &enc.sideInfo
	codeInfo := &sideInfo.Granules[gr].Channels[ch].Tt

	codeInfo.QuantizerStepSize = enc.binSearchStepSize(maxBits, ix, codeInfo)
	codeInfo.Part2Length = uint64(enc.calcPart2Length(gr, ch))
	huffBits := int64(uint64(maxBits) - codeInfo.Part2Length)
	bits := enc.innerLoop(ix, huffBits, codeInfo, gr, ch)
	codeInfo.Part2_3Length = codeInfo.Part2Length + uint64(bits)
	return int64(codeInfo.Part2_3Length)
}

func (enc *Encoder) iterationLoop() {
	var l3XMin PsyXMin

	for ch := enc.Wave.Channels - 1; ch >= 0; ch-- {
		for gr := int64(0); gr < enc.Mpeg.GranulesPerFrame; gr++ {
			ix := &enc.l3Encoding[ch][gr]
			enc.l3loop.Xr = enc.mdctFrequency[ch][gr][:]
			enc.l3loop.Xrmax = 0
			for i := GRANULE_SIZE - 1; i >= 0; i-- {
				enc.l3loop.Xrsq[i] = mulSR(enc.l3loop.Xr[i], enc.l3loop.Xr[i])
				enc.l3loop.Xrabs[i] = int32(math.Abs(float64(enc.l3loop.Xr[i])))
				if int64(enc.l3loop.Xrabs[i]) > int64(enc.l3loop.Xrmax) {
					enc.l3loop.Xrmax = enc.l3loop.Xrabs[i]
				}
			}
			codeInfo := &(enc.sideInfo.Granules[gr].Channels[ch]).Tt
			codeInfo.ScaleFactorBandMaxLen = scaleFactorBand_LMax - 1
			calcXMin(&enc.ratio, codeInfo, &l3XMin, gr, ch)
			if enc.Mpeg.Version == MPEG_I {
				enc.calcSCFSI(&l3XMin, ch, gr)
			}
			max_bits := enc.maxReservoirBits(&enc.PerceptualEnergy[ch][gr])
			for i := 3; i >= 0; i-- {
				codeInfo.ScaleFactorLen[i] = 0
			}
			codeInfo.Part2_3Length = 0
			codeInfo.BigValues = 0
			codeInfo.Count1 = 0
			codeInfo.ScaleFactorCompress = 0
			codeInfo.TableSelect[0] = 0
			codeInfo.TableSelect[1] = 0
			codeInfo.TableSelect[2] = 0
			codeInfo.Region0Count = 0
			codeInfo.Region1Count = 0
			codeInfo.Part2Length = 0
			codeInfo.PreFlag = 0
			codeInfo.ScaleFactorScale = 0
			codeInfo.Count1TableSelect = 0
			if int64(enc.l3loop.Xrmax) != 0 {
				codeInfo.Part2_3Length = uint64(enc.outerLoop(max_bits, &l3XMin, ix, gr, ch))
			}
			enc.reservoirAdjust(codeInfo)
			codeInfo.GlobalGain = uint64(codeInfo.QuantizerStepSize + 210)
		}
	}
	enc.reservoirFrameEnd()
}

// calcSCFSI calculates the scaleFactor select information ( scfsi )
func (enc *Encoder) calcSCFSI(l3XMin *PsyXMin, ch int64, gr int64) {
	sideInfo := &enc.sideInfo
	scfsiBandLong := [5]int64{0, 6, 11, 16, 21}

	condition := int64(0)
	// This is the scfsi_band table from 2.4.2.7 of the IS
	scaleFactorBandLong := &scaleFactorBandIndex[enc.Mpeg.SampleRateIndex]
	enc.l3loop.Xrmaxl[gr] = enc.l3loop.Xrmax

	temp := int64(0)
	// the total energy of the granule
	for i := GRANULE_SIZE - 1; i >= 0; i-- {
		// a bit of scaling to avoid overflow, (not very good)
		temp += int64(enc.l3loop.Xrsq[i]) >> 10
	}

	if temp != 0 {
		enc.l3loop.EnTot[gr] = int32(math.Log(float64(temp)*4.768371584e-07) / LN2)
	} else {
		enc.l3loop.EnTot[gr] = 0
	}

	// the energy of each scaleFactor band, en
	// the allowed distortion of each scaleFactor band, xm
	for sfb := 20; sfb >= 0; sfb-- {
		start := scaleFactorBandLong[sfb]
		end := scaleFactorBandLong[sfb+1]

		temp = 0
		for i := start; i < end; i++ {
			temp += int64(enc.l3loop.Xrsq[i]) >> 10
		}
		if temp != 0 {
			enc.l3loop.En[gr][sfb] = int32(math.Log(float64(temp)*4.768371584e-07) / LN2)
		} else {
			enc.l3loop.En[gr][sfb] = 0
		}
		if l3XMin.L[gr][ch][sfb] != 0 {
			enc.l3loop.Xm[gr][sfb] = int32(math.Log(l3XMin.L[gr][ch][sfb]) / LN2)
		} else {
			enc.l3loop.Xm[gr][sfb] = 0
		}
	}
	if gr == 1 {
		for gr2 := 1; gr2 >= 0; gr2-- {
			if int64(enc.l3loop.Xrmaxl[gr2]) != 0 {
				condition++
			}
			condition++
		}
		if math.Abs(float64(enc.l3loop.EnTot[0])-float64(enc.l3loop.EnTot[1])) < enTotKrit {
			condition++
		}
		tp := int64(0)
		for sfb := 20; sfb >= 0; sfb-- {
			tp += int64(math.Abs(float64(enc.l3loop.En[0][sfb]) - float64(enc.l3loop.En[1][sfb])))
		}
		if tp < enDifKrit {
			condition++
		}
		if condition == 6 {
			for scfsi_band := 0; scfsi_band < 4; scfsi_band++ {
				var (
					sum0 int64 = 0
					sum1 int64 = 0
				)
				sideInfo.ScaleFactorSelectInfo[ch][scfsi_band] = 0
				start := scfsiBandLong[scfsi_band]
				end := scfsiBandLong[scfsi_band+1]
				for sfb := start; sfb < end; sfb++ {
					sum0 += int64(math.Abs(float64(enc.l3loop.En[0][sfb]) - float64(enc.l3loop.En[1][sfb])))
					sum1 += int64(math.Abs(float64(enc.l3loop.Xm[0][sfb]) - float64(enc.l3loop.Xm[1][sfb])))
				}
				if sum0 < enScfsiBandKrit && sum1 < xmScfsiBandKrit {
					sideInfo.ScaleFactorSelectInfo[ch][scfsi_band] = 1
				} else {
					sideInfo.ScaleFactorSelectInfo[ch][scfsi_band] = 0
				}
			}
		} else {
			for scfsi_band := 0; scfsi_band < 4; scfsi_band++ {
				sideInfo.ScaleFactorSelectInfo[ch][scfsi_band] = 0
			}
		}
	}
}

// calcPart2Length calculates the number of bits needed to encode the scaleFactor in the
// main data block.
func (enc *Encoder) calcPart2Length(gr int64, ch int64) int64 {

	gi := &enc.sideInfo.Granules[gr].Channels[ch].Tt

	bits := int64(0)
	sLen1 := sLen1Table[gi.ScaleFactorCompress]
	sLen2 := sLen2Table[gi.ScaleFactorCompress]
	if gr == 0 || (enc.sideInfo.ScaleFactorSelectInfo[ch][0]) == 0 {
		bits += sLen1 * 6
	}
	if gr == 0 || (enc.sideInfo.ScaleFactorSelectInfo[ch][1]) == 0 {
		bits += sLen1 * 5
	}
	if gr == 0 || (enc.sideInfo.ScaleFactorSelectInfo[ch][2]) == 0 {
		bits += sLen2 * 5
	}
	if gr == 0 || (enc.sideInfo.ScaleFactorSelectInfo[ch][3]) == 0 {
		bits += sLen2 * 5
	}
	return bits
}

// calcXMin calculates the allowed distortion for each scaleFactor band,
// as determined by the psychoacoustic model. XMin(sb) = ratio(sb) * en(sb) / bw(sb)
func calcXMin(ratio *PsyRatio, codeInfo *GranuleInfo, l3XMin *PsyXMin, gr int64, ch int64) {
	for scaleFactorBand := int64(codeInfo.ScaleFactorBandMaxLen) - 1; scaleFactorBand >= 0; scaleFactorBand-- {
		// XMin will always be zero with no psychoacoustic model...
		l3XMin.L[gr][ch][scaleFactorBand] = 0
	}
}

// loopInitialize calculates the look up tables used by the iteration loop.
func (enc *Encoder) loopInitialize() {
	// quantize: stepSize conversion, fourth root of 2 table.
	// The table is inverted (negative power) from the equation given
	// in the spec because it is quicker to do x*y than x/y.
	// The 0.5 is for rounding.
	for i := 127; i >= 0; i-- {
		enc.l3loop.StepTable[i] = math.Pow(2.0, float64(127-i)/4)
		if (enc.l3loop.StepTable[i] * 2) > math.MaxInt32 {
			enc.l3loop.StepTableI[i] = math.MaxInt32
		} else {
			// The table is multiplied by 2 to give an extra bit of accuracy.
			// In quantize, the long multiply does not shift it's result left one
			// bit to compensate.
			enc.l3loop.StepTableI[i] = int32((enc.l3loop.StepTable[i] * 2) + 0.5)
		}
	}
	// quantize: vector conversion, three quarter power table.
	// The 0.5 is for rounding, the .0946 comes from the spec.
	for i := 9999; i >= 0; i-- {
		enc.l3loop.Int2idx[i] = int64(math.Sqrt(math.Sqrt(float64(i))*float64(i)) - 0.0946 + 0.5)
	}
}

// quantize perform quantization of the vector xr ( -> ix).
// Returns maximum value of ix
func (enc *Encoder) quantize(ix *[GRANULE_SIZE]int64, stepSize int64) int64 {

	ixMax := int64(0)
	// 2**(-stepSize/4)
	scaleI := enc.l3loop.StepTableI[stepSize+math.MaxInt8]
	// a quick check to see if ixMax will be less than 8192 */
	// this speeds up the early calls to binSearchStepSize
	// 165140 == 8192**(4/3)
	if mulR(enc.l3loop.Xrmax, scaleI) > 165140 {
		// no point in continuing, stepSize not big enough
		ixMax = 16384
	} else {
		for i := 0; i < GRANULE_SIZE; i++ {
			// This calculation is very sensitive. The multiply must round it's
			// result or bad things happen to the quality.
			ln := int64(mulR(int32(math.Abs(float64(enc.l3loop.Xr[i]))), scaleI))
			// ln < 10000 catches most values
			if ln < 10000 {
				// quick look up method
				ix[i] = enc.l3loop.Int2idx[ln]
			} else {
				//outside table range so have to do it using floats
				scale := enc.l3loop.StepTable[stepSize+math.MaxInt8]
				dbl := (float64(enc.l3loop.Xrabs[i])) * scale * 4.656612875e-10
				// dbl**(3/4)
				ix[i] = int64(math.Sqrt(math.Sqrt(dbl) * dbl))
			}

			// calculate ixMax while we're here */
			// note. ix cannot be negative
			if ixMax < ix[i] {
				ixMax = ix[i]
			}
		}
	}
	return ixMax
}

// ixMax calculates the maximum of ix from 0 to 575
func ixMax(ix *[GRANULE_SIZE]int64, begin uint64, end uint64) int64 {
	max := int64(0)
	for i := uint64(begin); uint64(i) < end; i++ {
		if max < ix[i] {
			max = ix[i]
		}
	}
	return max
}

// calcRunLength calculates rZero, count1, big_values (Partitions ix into big values, quadruples and zeros)
func calcRunLength(ix *[GRANULE_SIZE]int64, codeInfo *GranuleInfo) {

	rZero := int64(0)
	i := GRANULE_SIZE

	for ; i > 1; i -= 2 {
		if ix[i-1] == 0 && ix[i-2] == 0 {
			rZero++
		} else {
			break
		}
	}
	codeInfo.Count1 = 0
	for ; i > 3; i -= 4 {
		if ix[i-1] <= 1 && ix[i-2] <= 1 && ix[i-3] <= 1 && ix[i-4] <= 1 {
			codeInfo.Count1++
		} else {
			break
		}
	}
	codeInfo.BigValues = uint64(i >> 1)
}

// count1BitCount determines the number of bits to encode the quadruples.
func count1BitCount(ix *[GRANULE_SIZE]int64, cod_info *GranuleInfo) int64 {
	sum0 := int64(0)
	sum1 := int64(0)

	i := int64(cod_info.BigValues << 1)
	for k := uint64(0); k < cod_info.Count1; k++ {
		v := ix[i]
		w := ix[i+1]
		x := ix[i+2]
		y := ix[i+3]
		p := v + (w << 1) + (x << 2) + (y << 3)
		signBits := int64(0)
		if v != 0 {
			signBits++
		}
		if w != 0 {
			signBits++
		}
		if x != 0 {
			signBits++
		}
		if y != 0 {
			signBits++
		}
		sum0 += signBits
		sum1 += signBits
		sum0 += int64(huffmanCodeTable[32].hLen[p])
		sum1 += int64(huffmanCodeTable[33].hLen[p])

		i += 4
	}
	if sum0 < sum1 {
		cod_info.Count1TableSelect = 0
		return sum0
	} else {
		cod_info.Count1TableSelect = 1
		return sum1
	}
}

// subDivide subdivides the bigValue region which will use separate Huffman tables.
func (enc *Encoder) subDivide(codeInfo *GranuleInfo) {
	subdivideTable := [23]struct {
		Region0_count uint64
		Region1_count uint64
	}{
		{0, 0}, /* 0 bands */
		{0, 0}, /* 1 bands */
		{0, 0}, /* 2 bands */
		{0, 0}, /* 3 bands */
		{0, 0}, /* 4 bands */
		{0, 1}, /* 5 bands */
		{1, 1}, /* 6 bands */
		{1, 1}, /* 7 bands */
		{1, 2}, /* 8 bands */
		{2, 2}, /* 9 bands */
		{2, 3}, /* 10 bands */
		{2, 3}, /* 11 bands */
		{3, 4}, /* 12 bands */
		{3, 4}, /* 13 bands */
		{3, 4}, /* 14 bands */
		{4, 5}, /* 15 bands */
		{4, 5}, /* 16 bands */
		{4, 6}, /* 17 bands */
		{5, 6}, /* 18 bands */
		{5, 6}, /* 19 bands */
		{5, 7}, /* 20 bands */
		{6, 7}, /* 21 bands */
		{6, 7}, /* 22 bands */
	}

	if codeInfo.BigValues == 0 {
		codeInfo.Region0Count = 0
		codeInfo.Region1Count = 0
	} else {
		var (
			scalefac_band_long []int64 = scaleFactorBandIndex[enc.Mpeg.SampleRateIndex][:]
			bigvalues_region   int64
			scfb_anz           int64
			thiscount          int64
		)
		bigvalues_region = int64(codeInfo.BigValues * 2)
		scfb_anz = 0
		for scalefac_band_long[scfb_anz] < bigvalues_region {
			scfb_anz++
		}
		for thiscount = int64(subdivideTable[scfb_anz].Region0_count); thiscount != 0; thiscount-- {
			if scalefac_band_long[thiscount+1] <= bigvalues_region {
				break
			}
		}
		codeInfo.Region0Count = uint64(thiscount)
		codeInfo.Address1 = uint64(scalefac_band_long[thiscount+1])
		scalefac_band_long = scalefac_band_long[codeInfo.Region0Count+1:]
		for thiscount = int64(subdivideTable[scfb_anz].Region1_count); thiscount != 0; thiscount-- {
			if scalefac_band_long[thiscount+1] <= bigvalues_region {
				break
			}
		}
		codeInfo.Region1Count = uint64(thiscount)
		codeInfo.Address2 = uint64(scalefac_band_long[thiscount+1])
		codeInfo.Address3 = uint64(bigvalues_region)
	}
}

// bigValuesTableSelect selects huffman code tables for the bigValues region
func bigValuesTableSelect(ix *[GRANULE_SIZE]int64, codeInfo *GranuleInfo) {
	codeInfo.TableSelect[0] = 0
	codeInfo.TableSelect[1] = 0
	codeInfo.TableSelect[2] = 0
	{
		if codeInfo.Address1 > 0 {
			codeInfo.TableSelect[0] = uint64(newChooseTable(ix, 0, codeInfo.Address1))
		}
		if codeInfo.Address2 > codeInfo.Address1 {
			codeInfo.TableSelect[1] = uint64(newChooseTable(ix, codeInfo.Address1, codeInfo.Address2))
		}
		if codeInfo.BigValues<<1 > codeInfo.Address2 {
			codeInfo.TableSelect[2] = uint64(newChooseTable(ix, codeInfo.Address2, codeInfo.BigValues<<1))
		}
	}
}

// newChooseTable chooses the Huffman table that will encode ix[begin..end] with the fewest bits.
// Note: This code contains knowledge about the sizes and characteristics of the Huffman tables as
// defined in the IS (Table B.7), and will not work with any arbitrary tables.
func newChooseTable(ix *[GRANULE_SIZE]int64, begin uint64, end uint64) int64 {
	choice := [2]int64{}
	sum := [2]int64{}
	max := ixMax(ix, begin, end)
	if max == 0 {
		return 0
	}
	choice[0] = 0
	choice[1] = 0
	if max < 15 {
		// try tables with no linBits
		for i := int64(13); i >= 0; i-- {
			if huffmanCodeTable[i].xLen > uint(max) {
				choice[0] = i
				break
			}
		}
		sum[0] = countBit(ix, begin, end, uint64(choice[0]))
		switch choice[0] {
		case 2:
			sum[1] = countBit(ix, begin, end, 3)
			if sum[1] <= sum[0] {
				choice[0] = 3
			}
		case 5:
			sum[1] = countBit(ix, begin, end, 6)
			if sum[1] <= sum[0] {
				choice[0] = 6
			}
		case 7:
			sum[1] = countBit(ix, begin, end, 8)
			if sum[1] <= sum[0] {
				choice[0] = 8
				sum[0] = sum[1]
			}
			sum[1] = countBit(ix, begin, end, 9)
			if sum[1] <= sum[0] {
				choice[0] = 9
			}
		case 10:
			sum[1] = countBit(ix, begin, end, 11)
			if sum[1] <= sum[0] {
				choice[0] = 11
				sum[0] = sum[1]
			}
			sum[1] = countBit(ix, begin, end, 12)
			if sum[1] <= sum[0] {
				choice[0] = 12
			}
		case 13:
			sum[1] = countBit(ix, begin, end, 15)
			if sum[1] <= sum[0] {
				choice[0] = 15
			}
		}
	} else {
		// try tables with linBits
		max -= 15
		for i := 15; i < 24; i++ {
			if huffmanCodeTable[i].linMax >= uint(max) {
				choice[0] = int64(i)
				break
			}
		}
		for i := 24; i < 32; i++ {
			if huffmanCodeTable[i].linMax >= uint(max) {
				choice[1] = int64(i)
				break
			}
		}
		sum[0] = countBit(ix, begin, end, uint64(choice[0]))
		sum[1] = countBit(ix, begin, end, uint64(choice[1]))
		if sum[1] < sum[0] {
			choice[0] = choice[1]
		}
	}
	return choice[0]
}

// bigValuesBitCount Count the number of bits necessary to code the bigValues region.
func bigValuesBitCount(ix *[GRANULE_SIZE]int64, granInfo *GranuleInfo) int64 {

	bits := int64(0)

	// region0
	table := granInfo.TableSelect[0]
	if table != 0 {
		bits += countBit(ix, 0, granInfo.Address1, table)
	}
	// region1
	table = granInfo.TableSelect[1]
	if table != 0 {
		bits += countBit(ix, granInfo.Address1, granInfo.Address2, table)
	}
	// region2
	table = granInfo.TableSelect[2]
	if table != 0 {
		bits += countBit(ix, granInfo.Address2, granInfo.Address3, table)
	}
	return bits
}

// countBit counts the number of bits necessary to code the subregion.
func countBit(ix *[GRANULE_SIZE]int64, start uint64, end uint64, table uint64) int64 {
	if table == 0 {
		return 0
	}
	h := &(huffmanCodeTable[table])
	sum := int64(0)
	yLen := uint64(h.yLen)
	linBits := uint64(h.linBits)
	if table > 15 {
		// ESC-table is used
		for i := int64(start); uint64(i) < end; i += 2 {
			x := ix[i]
			y := ix[i+1]
			if x > 14 {
				x = 15
				sum += int64(linBits)
			}
			if y > 14 {
				y = 15
				sum += int64(linBits)
			}
			sum += int64(h.hLen[x*int64(yLen)+y])
			if x != 0 {
				sum++
			}
			if y != 0 {
				sum++
			}
		}
	} else {
		// No ESC-words
		for i := int64(start); uint64(i) < end; i += 2 {
			x := ix[i]
			y := ix[i+1]
			sum += int64(h.hLen[x*int64(yLen)+y])
			if x != 0 {
				sum++
			}
			if y != 0 {
				sum++
			}
		}
	}
	return sum
}

// binSearchStepSize successively approximates an approach to obtaining a initial quantizer
// step size. The following optional code written by Seymour Shlien will speed up the outerLoop code which is
// called by iterationLoop. When BIN_SEARCH(?) is defined, the No ESC-words function precedes the call to the
// function innerLoop with a call to bin_search gain defined below, which returns a good starting quantizerStepSize.
func (enc *Encoder) binSearchStepSize(desiredRate int64, ix *[GRANULE_SIZE]int64, codeInfo *GranuleInfo) int64 {
	next := int64(-120)
	count := int64(120)
	bit := int64(0)
	for {
		{
			half := count / 2
			if enc.quantize(ix, next+half) > 8192 {
				// fail
				bit = 100000
			} else {
				calcRunLength(ix, codeInfo)
				bit = count1BitCount(ix, codeInfo)
				enc.subDivide(codeInfo)
				bigValuesTableSelect(ix, codeInfo)
				bit += bigValuesBitCount(ix, codeInfo)
			}
			if bit < desiredRate {
				count = half
			} else {
				next += half
				count -= half
			}
		}
		if count <= 1 {
			break
		}
	}
	return next
}
