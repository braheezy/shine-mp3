package mp3

import (
	"math"
	"unsafe"
)

// subbandInitialize calculates the analysis filterbank coefficients and rounds to the  9th decimal
// place accuracy of the filterbank tables in the ISO document. The coefficients are stored in #filter#
func (enc *Encoder) subbandInitialize() {
	for i := SUBBAND_LIMIT - 1; i >= 0; i-- {
		for j := 63; j >= 0; j-- {
			filter := math.Cos(float64((i*2+1)*(16-j))*PI64) * 1e+09
			if filter >= 0 {
				filter, _ = math.Modf(filter + 0.5)
			} else {
				filter, _ = math.Modf(filter - 0.5)
			}
			// scale and convert to fixed point before storing
			enc.subband.Fl[i][j] = int32(filter * (math.MaxInt32 * 1e-09))
		}
	}
}

// Overlapping window on PCM samples 32 16-bit pcm samples are scaled to fractional 2's complement and
// concatenated to the end of the window buffer #x#. The updated window buffer #x# is then windowed by
// the analysis window #enWindow# to produce the windowed sample #z# Calculates the analysis filter bank
// coefficients The windowed samples #z# is filtered by the digital filter matrix #filter# to produce the subband
// samples #s#. This done by first selectively picking out values from the windowed samples, and then
// multiplying them by the filter matrix, producing 32 subband samples.
func (enc *Encoder) windowFilterSubband(buffer **int16, s *[SUBBAND_LIMIT]int32, ch int64, stride int64) {
	y := make([]int32, 64)
	ptr := *buffer

	// replace 32 oldest samples with 32 new samples
	for i := int64(31); i >= 0; i-- {
		enc.subband.X[ch][i+enc.subband.Off[ch]] = int32(*ptr) << 16
		ptr = (*int16)(unsafe.Add(unsafe.Pointer(ptr), unsafe.Sizeof(int16(0))*uintptr(stride)))
	}
	*buffer = ptr

	for i := int64(63); i >= 0; i-- {
		sValue := mul((enc.subband.X[ch][(enc.subband.Off[ch]+i+(0<<6))&(HAN_SIZE-1)]), enWindow[i+(0<<6)])
		sValue += mul((enc.subband.X[ch][(enc.subband.Off[ch]+i+(1<<6))&(HAN_SIZE-1)]), enWindow[i+(1<<6)])
		sValue += mul((enc.subband.X[ch][(enc.subband.Off[ch]+i+(2<<6))&(HAN_SIZE-1)]), enWindow[i+(2<<6)])
		sValue += mul((enc.subband.X[ch][(enc.subband.Off[ch]+i+(3<<6))&(HAN_SIZE-1)]), enWindow[i+(3<<6)])
		sValue += mul((enc.subband.X[ch][(enc.subband.Off[ch]+i+(4<<6))&(HAN_SIZE-1)]), enWindow[i+(4<<6)])
		sValue += mul((enc.subband.X[ch][(enc.subband.Off[ch]+i+(5<<6))&(HAN_SIZE-1)]), enWindow[i+(5<<6)])
		sValue += mul((enc.subband.X[ch][(enc.subband.Off[ch]+i+(6<<6))&(HAN_SIZE-1)]), enWindow[i+(6<<6)])
		sValue += mul((enc.subband.X[ch][(enc.subband.Off[ch]+i+(7<<6))&(HAN_SIZE-1)]), enWindow[i+(7<<6)])

		y[i] = sValue
	}
	enc.subband.Off[ch] = (enc.subband.Off[ch] + 480) & (HAN_SIZE - 1)
	for i := SUBBAND_LIMIT - 1; i >= 0; i-- {
		sValue := mul(enc.subband.Fl[i][63], y[63])
		for j := 63; j != 0; j -= 7 {
			sValue += mul(enc.subband.Fl[i][j-1], y[j-1])
			sValue += mul(enc.subband.Fl[i][j-2], y[j-2])
			sValue += mul(enc.subband.Fl[i][j-3], y[j-3])
			sValue += mul(enc.subband.Fl[i][j-4], y[j-4])
			sValue += mul(enc.subband.Fl[i][j-5], y[j-5])
			sValue += mul(enc.subband.Fl[i][j-6], y[j-6])
			sValue += mul(enc.subband.Fl[i][j-7], y[j-7])
		}
		s[i] = sValue
	}
}
