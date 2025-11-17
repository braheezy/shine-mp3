package mp3

import "math"

const (
	psychoEnergyFloor  = 1e-9
	psychoMaskFraction = 0.02
)

// updatePsychoModel computes a very small psychoacoustic model that estimates
// band energy, derives conservative masking thresholds, and seeds the encoder's
// scale factors.
func (enc *Encoder) updatePsychoModel() {
	bandLayout := scaleFactorBandIndex[enc.Mpeg.SampleRateIndex]
	bandLimit := len(bandLayout) - 1
	if limit := scaleFactorBand_LMax - 1; bandLimit > limit {
		bandLimit = limit
	}
	if max := len(enc.scaleFactor.L[0][0]); bandLimit > max {
		bandLimit = max
	}
	if max := len(enc.ratio.L[0][0]); bandLimit > max {
		bandLimit = max
	}
	for ch := int64(0); ch < enc.Wave.Channels; ch++ {
		chIdx := int(ch)
		for gr := int64(0); gr < enc.Mpeg.GranulesPerFrame; gr++ {
			grIdx := int(gr)
			var totalEnergy float64
			spectrum := enc.mdctFrequency[chIdx][grIdx][:]
			for band := 0; band < bandLimit; band++ {
				start := int(bandLayout[band])
				end := int(bandLayout[band+1])
				if end > len(spectrum) {
					end = len(spectrum)
				}
				if start >= end {
					enc.resetBandPsyData(grIdx, chIdx, band)
					continue
				}
				energy := bandEnergy(spectrum[start:end])
				totalEnergy += energy
				bandWidth := float64(end - start)
				avgEnergy := energy / bandWidth
				mask := avgEnergy * psychoMaskFraction
				if mask < psychoEnergyFloor {
					mask = psychoEnergyFloor
				}
				enc.ratio.L[grIdx][chIdx][band] = mask
				enc.scaleFactor.L[grIdx][chIdx][band] = encodeScaleFactor(avgEnergy)
			}
			// ensure remaining bands are reset so stale data does not leak
			for band := bandLimit; band < len(enc.scaleFactor.L[grIdx][chIdx]); band++ {
				enc.resetBandPsyData(grIdx, chIdx, band)
			}
			enc.PerceptualEnergy[chIdx][grIdx] = 10 * math.Log10(totalEnergy+psychoEnergyFloor)
		}
	}
}

func (enc *Encoder) resetBandPsyData(grIdx, chIdx, band int) {
	if band < len(enc.ratio.L[grIdx][chIdx]) {
		enc.ratio.L[grIdx][chIdx][band] = psychoEnergyFloor
	}
	if band < len(enc.scaleFactor.L[grIdx][chIdx]) {
		enc.scaleFactor.L[grIdx][chIdx][band] = 0
	}
}

func bandEnergy(data []int32) float64 {
	energy := 0.0
	for _, sample := range data {
		v := float64(sample)
		energy += v * v
	}
	return energy
}

func encodeScaleFactor(power float64) int32 {
	if power <= psychoEnergyFloor {
		return 0
	}
	// Convert to a compact 6-bit representation using a simple logarithmic scale.
	db := 10 * math.Log10(power)
	scaled := int32((db + 60) / 1.5)
	switch {
	case scaled < 0:
		return 0
	case scaled > 63:
		return 63
	default:
		return scaled
	}
}
