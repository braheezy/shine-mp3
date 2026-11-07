// bitstream.go manages writes to the bitstream
package mp3

const (
	// Minimum size of the buffer in bytes
	MINIMUM = 4
	// Maximum length of word written or read from bit stream
	MAX_LENGTH  = 32
	BUFFER_SIZE = 4096
)

type bitstream struct {
	data         []uint8
	dataSize     int
	dataPosition int
	cache        uint32
	cacheBits    int
}

func (bs *bitstream) open(size int) {
	bs.data = make([]uint8, size)
	bs.dataSize = size
	bs.dataPosition = 0
	bs.cache = 0
	bs.cacheBits = 32
}

// putBits writes N bits of val into the bit stream.
func (bs *bitstream) putBits(val uint32, N uint) {
	if bs.cacheBits > int(N) {
		bs.cacheBits -= int(N)
		bs.cache |= val << uint32(bs.cacheBits)
	} else {
		if bs.dataPosition+4 >= bs.dataSize {
			newCapacity := bs.dataSize + (bs.dataSize >> 1)
			newSlice := make([]byte, newCapacity)
			copy(newSlice, bs.data)
			bs.data = newSlice
			bs.dataSize = newCapacity
		}
		N -= uint(bs.cacheBits)
		bs.cache |= val >> N
		bs.data[bs.dataPosition] = uint8(bs.cache >> 24)
		bs.data[bs.dataPosition+1] = uint8(bs.cache >> 16)
		bs.data[bs.dataPosition+2] = uint8(bs.cache >> 8)
		bs.data[bs.dataPosition+3] = uint8(bs.cache)

		bs.dataPosition += 4
		bs.cacheBits = int(32 - N)
		if N != 0 {
			bs.cache = val << uint(bs.cacheBits)
		} else {
			bs.cache = 0
		}
	}
}
func (bs *bitstream) getBitsCount() int {
	return bs.dataPosition*8 + 32 - bs.cacheBits
}

// flush writes out what is left in the cache and empties it. A frame is a
// whole number of bytes, but putBits writes out only whole 32-bit words,
// so at the end of a frame its last bytes, up to three, are still here.
func (bs *bitstream) flush() {
	if bs.dataPosition+4 >= bs.dataSize {
		newCapacity := bs.dataSize + (bs.dataSize >> 1)
		newSlice := make([]byte, newCapacity)
		copy(newSlice, bs.data)
		bs.data = newSlice
		bs.dataSize = newCapacity
	}
	for ; bs.cacheBits < 32; bs.cacheBits += 8 {
		bs.data[bs.dataPosition] = uint8(bs.cache >> 24)
		bs.dataPosition++
		bs.cache <<= 8
	}
	bs.cache = 0
	bs.cacheBits = 32
}
