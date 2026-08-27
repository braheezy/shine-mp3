package mp3

import "math"

const (
	psychoEnergyFloor = 1e-9
	tonalMaskConstant = 0.158  // ~= -8 dB
	noiseMaskConstant = 0.0631 // ~= -12 dB
)

// updatePsychoModel computes a basic psychoacoustic analysis: it derives
// per-band energy, applies a Bark-domain spreading function, estimates a
// coarse tonality metric, and turns the resulting signal-to-mask ratio into
// scalefactors and perceptual entropy values for the reservoir.
func (enc *Encoder) updatePsychoModel() {
	bandLayout := scaleFactorBandIndex[enc.Mpeg.SampleRateIndex]
	bandLimit := len(enc.ratio.L[0][0])
	if limit := len(bandLayout) - 1; bandLimit > limit {
		bandLimit = limit
	}
	sampleRate := float64(enc.Wave.SampleRate)

	var (
		bandBark   [scaleFactorBand_LMax]float64
		bandCenter [scaleFactorBand_LMax]float64
	)
	for band := 0; band < bandLimit; band++ {
		start := int(bandLayout[band])
		end := int(bandLayout[band+1])
		bandCenter[band] = bandCenterFrequency(sampleRate, start, end)
		bandBark[band] = hzToBark(bandCenter[band])
	}

	for ch := int64(0); ch < enc.Wave.Channels; ch++ {
		chIdx := int(ch)
		for gr := int64(0); gr < enc.Mpeg.GranulesPerFrame; gr++ {
			grIdx := int(gr)

			var (
				bandEnergy [scaleFactorBand_LMax]float64
				tinality   [scaleFactorBand_LMax]float64
				masking    [scaleFactorBand_LMax]float64
			)
			spectrum := enc.mdctFrequency[chIdx][grIdx][:]
			for band := 0; band < bandLimit; band++ {
				start := int(bandLayout[band])
				end := int(bandLayout[band+1])
				if end > len(spectrum) {
					end = len(spectrum)
				}
				energy, tonality := analyzeBand(spectrum[start:end])
				bandEnergy[band] = energy
				tinality[band] = tonality
			}

			for band := 0; band < bandLimit; band++ {
				sum := 0.0
				for other := 0; other < bandLimit; other++ {
					delta := bandBark[other] - bandBark[band]
					weight := spreadingFunction(delta)
					sum += bandEnergy[other] * weight
				}
				tonalMask := sum * tonalMaskConstant
				noiseMask := sum * noiseMaskConstant
				mask := tinality[band]*tonalMask + (1.0-tinality[band])*noiseMask
				ath := athEnergy(bandCenter[band])
				if mask < ath {
					mask = ath
				}
				masking[band] = mask
			}

			pe := 0.0
			for band := 0; band < bandLimit; band++ {
				mask := masking[band]
				energy := bandEnergy[band]
				if mask < psychoEnergyFloor {
					mask = psychoEnergyFloor
				}
				enc.ratio.L[grIdx][chIdx][band] = mask
				smr := energy / mask
				if smr > 1 {
					pe += math.Log2(smr)
				}
				enc.scaleFactor.L[grIdx][chIdx][band] = encodeScaleFactorFromSMR(smr)
			}

			for band := bandLimit; band < len(enc.scaleFactor.L[grIdx][chIdx]); band++ {
				enc.resetBandPsyData(grIdx, chIdx, band)
			}
			enc.PerceptualEnergy[chIdx][grIdx] = pe
		}
	}
}

func analyzeBand(samples []int32) (float64, float64) {
	if len(samples) == 0 {
		return psychoEnergyFloor, 0.0
	}
	var energy float64
	var logSum float64
	var absSum float64
	for _, sample := range samples {
		v := float64(sample)
		abs := math.Abs(v) + psychoEnergyFloor
		energy += v * v
		absSum += abs
		logSum += math.Log(abs)
	}
	meanAbs := absSum / float64(len(samples))
	geomMean := math.Exp(logSum / float64(len(samples)))
	sfm := geomMean / (meanAbs + psychoEnergyFloor)
	if sfm < 0 {
		sfm = 0
	} else if sfm > 1 {
		sfm = 1
	}
	tonality := 1 - sfm
	return energy + psychoEnergyFloor, tonality
}

func bandCenterFrequency(sampleRate float64, start, end int) float64 {
	if end <= start {
		end = start + 1
	}
	bin := (float64(start) + float64(end)) / 2
	freqStep := sampleRate / (2.0 * float64(GRANULE_SIZE))
	return bin * freqStep
}

func hzToBark(freq float64) float64 {
	f := freq / 1000.0
	return 13.0*math.Atan(0.00076*freq) + 3.5*math.Atan(math.Pow(f/7.5, 2))
}

func spreadingFunction(deltaBark float64) float64 {
	if deltaBark >= 0 {
		return math.Pow(10.0, -1.5*deltaBark)
	}
	return math.Pow(10.0, 0.5*deltaBark)
}

func athEnergy(freq float64) float64 {
	if freq <= 0 {
		return psychoEnergyFloor
	}
	f := freq / 1000.0
	ath := 3.64*math.Pow(f, -0.8) - 6.5*math.Exp(-0.6*math.Pow(f-3.3, 2)) + 0.001*math.Pow(f, 4)
	linear := psychoEnergyFloor * math.Pow(10, ath/10.0)
	if linear < psychoEnergyFloor {
		return psychoEnergyFloor
	}
	return linear
}

func (enc *Encoder) resetBandPsyData(grIdx, chIdx, band int) {
	if band < len(enc.ratio.L[grIdx][chIdx]) {
		enc.ratio.L[grIdx][chIdx][band] = psychoEnergyFloor
	}
	if band < len(enc.scaleFactor.L[grIdx][chIdx]) {
		enc.scaleFactor.L[grIdx][chIdx][band] = 0
	}
}

func encodeScaleFactorFromSMR(smr float64) int32 {
	if smr <= 1 {
		return 0
	}
	smrDb := 10 * math.Log10(smr)
	scaled := int32(smrDb / 1.5)
	switch {
	case scaled < 0:
		return 0
	case scaled > 63:
		return 63
	default:
		return scaled
	}
}
