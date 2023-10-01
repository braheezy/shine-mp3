package mp3

import (
	"math"
)

// This is table B.9: coefficients for aliasing reduction
func MDCT_CA(coefficient float64) int32 {
	return int32(coefficient / math.Sqrt(1.0+(coefficient*coefficient)) * float64(math.MaxInt32))
}
func MDCT_CS(coefficient float64) int32 {
	return int32(1.0 / math.Sqrt(1.0+(coefficient*coefficient)) * float64(math.MaxInt32))
}

var (
	MDCT_CA0 = MDCT_CA(-0.6)
	MDCT_CA1 = MDCT_CA(-0.535)
	MDCT_CA2 = MDCT_CA(-0.33)
	MDCT_CA3 = MDCT_CA(-0.185)
	MDCT_CA4 = MDCT_CA(-0.095)
	MDCT_CA5 = MDCT_CA(-0.041)
	MDCT_CA6 = MDCT_CA(-0.0142)
	MDCT_CA7 = MDCT_CA(-0.0037)

	MDCT_CS0 = MDCT_CS(-0.6)
	MDCT_CS1 = MDCT_CS(-0.535)
	MDCT_CS2 = MDCT_CS(-0.33)
	MDCT_CS3 = MDCT_CS(-0.185)
	MDCT_CS4 = MDCT_CS(-0.095)
	MDCT_CS5 = MDCT_CS(-0.041)
	MDCT_CS6 = MDCT_CS(-0.0142)
	MDCT_CS7 = MDCT_CS(-0.0037)
)

func (enc *Encoder) mdctInitialize() {
	// prepare the mdct coefficients
	for m := 17; m >= 0; m-- {
		for k := 35; k >= 0; k-- {
			// combine window and mdct coefficients into a single table
			// scale and convert to fixed point before storing
			enc.mdct.CosL[m][k] = int32(math.Sin(PI36*(float64(k)+0.5)) * math.Cos((PI/72)*float64(k*2+19)*float64(m*2+1)) * math.MaxInt32)
		}
	}
}
func (enc *Encoder) mdctSub(stride int64) {
	mdctIn := make([]int32, 36)
	for ch := enc.Wave.Channels - 1; ch >= 0; ch-- {
		for gr := int64(0); gr < enc.Mpeg.GranulesPerFrame; gr++ {
			mdctEnc := &enc.mdctFrequency[ch][gr]

			// polyphase filtering
			for k := 0; k < 18; k += 2 {
				enc.windowFilterSubband(&enc.buffer[ch], &enc.l3SubbandSamples[ch][gr+1][k], ch, stride)
				enc.windowFilterSubband(&enc.buffer[ch], &enc.l3SubbandSamples[ch][gr+1][k+1], ch, stride)

				// Compensate for inversion in the analysis filter
				// (every odd index of band AND k)
				for band := 1; band < 32; band += 2 {
					enc.l3SubbandSamples[ch][gr+1][k+1][band] *= -1
				}
			}

			// Perform imdct of 18 previous subband samples + 18 current subband
			// samples
			for band := 0; band < 32; band++ {
				for k := 17; k >= 0; k-- {
					mdctIn[k] = enc.l3SubbandSamples[ch][gr][k][band]
					mdctIn[k+18] = enc.l3SubbandSamples[ch][gr+1][k][band]
				}

				// Calculation of the MDCT
				// In the case of long blocks ( block_type 0,1,3 ) there are
				// 36 coefficients in the time domain and 18 in the frequency
				// domain.
				for k := 17; k >= 0; k-- {
					vm := mul(mdctIn[35], enc.mdct.CosL[k][35])
					for j := 35; j != 0; j -= 7 {
						vm += mul(mdctIn[j-1], enc.mdct.CosL[k][j-1])
						vm += mul(mdctIn[j-2], enc.mdct.CosL[k][j-2])
						vm += mul(mdctIn[j-3], enc.mdct.CosL[k][j-3])
						vm += mul(mdctIn[j-4], enc.mdct.CosL[k][j-4])
						vm += mul(mdctIn[j-5], enc.mdct.CosL[k][j-5])
						vm += mul(mdctIn[j-6], enc.mdct.CosL[k][j-6])
						vm += mul(mdctIn[j-7], enc.mdct.CosL[k][j-7])
					}
					mdctEnc[band*18+k] = vm
				}
				// Perform aliasing reduction butterfly
				if band != 0 {
					mdctEnc[band*18+0], mdctEnc[(band-1)*18+17-0] = cmuls(
						&mdctEnc[band*18+0], &mdctEnc[(band-1)*18+17-0],
						&MDCT_CS0, &MDCT_CA0,
					)

					mdctEnc[band*18+1], mdctEnc[(band-1)*18+17-1] = cmuls(
						&mdctEnc[band*18+1], &mdctEnc[(band-1)*18+17-1],
						&MDCT_CS1, &MDCT_CA1,
					)

					mdctEnc[band*18+2], mdctEnc[(band-1)*18+17-2] = cmuls(
						&mdctEnc[band*18+2], &mdctEnc[(band-1)*18+17-2],
						&MDCT_CS2, &MDCT_CA2,
					)

					mdctEnc[band*18+3], mdctEnc[(band-1)*18+17-3] = cmuls(
						&mdctEnc[band*18+3], &mdctEnc[(band-1)*18+17-3],
						&MDCT_CS3, &MDCT_CA3,
					)

					mdctEnc[band*18+4], mdctEnc[(band-1)*18+17-4] = cmuls(
						&mdctEnc[band*18+4], &mdctEnc[(band-1)*18+17-4],
						&MDCT_CS4, &MDCT_CA4,
					)

					mdctEnc[band*18+5], mdctEnc[(band-1)*18+17-5] = cmuls(
						&mdctEnc[band*18+5], &mdctEnc[(band-1)*18+17-5],
						&MDCT_CS5, &MDCT_CA5,
					)

					mdctEnc[band*18+6], mdctEnc[(band-1)*18+17-6] = cmuls(
						&mdctEnc[band*18+6], &mdctEnc[(band-1)*18+17-6],
						&MDCT_CS6, &MDCT_CA6,
					)

					mdctEnc[band*18+7], mdctEnc[(band-1)*18+17-7] = cmuls(
						&mdctEnc[band*18+7], &mdctEnc[(band-1)*18+17-7],
						&MDCT_CS7, &MDCT_CA7,
					)
				}
			}
		}
		// Save latest granule's subband samples to be used in the next mdct call
		copy(enc.l3SubbandSamples[ch][0][:], enc.l3SubbandSamples[ch][enc.Mpeg.GranulesPerFrame][:])
	}
}
