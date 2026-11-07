package mp3

import (
	"bytes"
	"fmt"
	"math"
	"math/bits"
	"math/rand"
	"testing"
)

// testConfig is one way to set up the encoder.
type testConfig struct{ rate, channels int }

func (c testConfig) String() string { return fmt.Sprintf("%dHz-%dch", c.rate, c.channels) }

// testConfigs are every sample rate the encoder supports at its
// 128 kbit/s, in mono and in stereo, but MPEG-1 mono, whose side
// information length is wrong (NewEncoder gives it 9 bytes rather than 17;
// a separate fix).
var testConfigs = []testConfig{
	{16000, 1}, {22050, 1}, {24000, 1},
	{16000, 2}, {22050, 2}, {24000, 2},
	{32000, 2}, {44100, 2}, {48000, 2},
}

// testSignal is interleaved PCM shaped like speech: tones of random phase
// every 40 Hz from 100 Hz to 6 kHz and nothing above, swelling and fading
// away nearly four times a second. A busy band under an empty one is what
// lets padding inside part2_3_length be read as count1 quadruples over
// many lines, until the last of them runs past the end of the granule.
func testSignal(c testConfig, seconds float64) []int16 {
	r := rand.New(rand.NewSource(1))
	var freqs, phases []float64
	for f := 100.0; f < 6000; f += 40 {
		freqs = append(freqs, f)
		phases = append(phases, 2*math.Pi*r.Float64())
	}
	n := int(float64(c.rate) * seconds)
	pcm := make([]int16, 0, n*c.channels)
	for i := 0; i < n; i++ {
		at := float64(i) / float64(c.rate)
		var v float64
		for k, f := range freqs {
			v += math.Sin(2*math.Pi*f*at + phases[k])
		}
		swell := math.Sin(2 * math.Pi * 3.7 * at)
		v *= 0.4 * math.MaxInt16 * swell * swell / math.Sqrt(float64(len(freqs))/2)
		v = math.Max(math.MinInt16, math.Min(math.MaxInt16, v))
		for ch := 0; ch < c.channels; ch++ {
			pcm = append(pcm, int16(v))
		}
	}
	return pcm
}

// encode encodes seconds of testSignal with Write.
func encode(t *testing.T, c testConfig, seconds float64) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := NewEncoder(c.rate, c.channels).Write(&out, testSignal(c, seconds)); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// frameHeader is what the tests need of an MPEG audio frame header.
type frameHeader struct {
	mpeg1, mono bool
	rateIndex   int // into sampleRates and scaleFactorBandIndex
	length      int // the frame's bytes, header included
}

// readHeader reads the Layer III frame header at the start of b.
func readHeader(b []byte) (frameHeader, bool) {
	if len(b) < 4 || b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return frameHeader{}, false
	}
	version := int(b[1]>>3) & 3
	layer := int(b[1]>>1) & 3
	bitrateIndex := int(b[2] >> 4)
	rate := int(b[2]>>2) & 3
	if (version != int(MPEG_I) && version != int(MPEG_II)) || layer != int(LAYER_III) ||
		bitrateIndex == 0 || bitrateIndex == 15 || rate == 3 {
		return frameHeader{}, false
	}
	h := frameHeader{mpeg1: version == int(MPEG_I), mono: b[3]>>6 == 3, rateIndex: rate}
	slots := 144
	if !h.mpeg1 {
		slots = 72
		h.rateIndex += 3
	}
	h.length = slots*1000*int(bitRates[bitrateIndex][version])/int(sampleRates[h.rateIndex]) + int(b[2]>>1)&1
	return h, true
}

// splitFrames cuts a stream into its frames, failing the test where a
// frame does not begin with a header. tail is how many bytes of an
// unfinished frame the stream ends with.
func splitFrames(t *testing.T, stream []byte) (frames [][]byte, headers []frameHeader, tail int) {
	t.Helper()
	for pos := 0; pos < len(stream); {
		h, ok := readHeader(stream[pos:])
		if !ok {
			t.Fatalf("no frame header at byte %d of %d", pos, len(stream))
		}
		if pos+h.length > len(stream) {
			return frames, headers, len(stream) - pos
		}
		frames = append(frames, stream[pos:pos+h.length])
		headers = append(headers, h)
		pos += h.length
	}
	return frames, headers, 0
}

// bitReader reads a frame most significant bit first, as a decoder does.
type bitReader struct {
	b   []byte
	pos int // in bits
}

// read returns the next n bits (n <= 32). Past the end of the frame they
// read as zeros, and the position runs on, so that whatever reads there is
// seen to have run past.
func (r *bitReader) read(n int) uint32 {
	var v uint32
	for i := r.pos; i < r.pos+n; i++ {
		v <<= 1
		if i < len(r.b)*8 {
			v |= uint32(r.b[i/8]>>(7-uint(i%8))) & 1
		}
	}
	r.pos += n
	return v
}

// granuleSide is what the tests need of one granule's side information.
type granuleSide struct {
	part2_3, bigValues, scaleFactorCompress, count1Table uint32
	windowSwitching                                      bool
	table                                                [3]uint32
	region0, region1                                     uint32
}

// readSideInfo reads a frame's side information (the encoder writes no
// CRC): main_data_begin, and the granules in the order their main data
// follows, granule by granule and channel by channel.
func readSideInfo(r *bitReader, h frameHeader) (uint32, []granuleSide) {
	get := r.read
	r.pos = 32
	channels, granules := 2, 1
	if h.mono {
		channels = 1
	}
	var mainDataBegin uint32
	if h.mpeg1 {
		granules = 2
		mainDataBegin = get(9)
		if h.mono {
			get(5) // private bits
		} else {
			get(3)
		}
		get(4 * channels) // scfsi
	} else {
		mainDataBegin = get(8)
		get(channels) // private bits
	}
	var side []granuleSide
	for gr := 0; gr < granules; gr++ {
		for ch := 0; ch < channels; ch++ {
			var g granuleSide
			g.part2_3 = get(12)
			g.bigValues = get(9)
			get(8) // global_gain
			if h.mpeg1 {
				g.scaleFactorCompress = get(4)
			} else {
				g.scaleFactorCompress = get(9)
			}
			if g.windowSwitching = get(1) == 1; g.windowSwitching {
				get(2 + 1 + 2*5 + 3*3)
			} else {
				for i := range g.table {
					g.table[i] = get(5)
				}
				g.region0, g.region1 = get(4), get(3)
			}
			if h.mpeg1 {
				get(1) // preflag
			}
			get(1) // scalefac_scale
			g.count1Table = get(1)
			side = append(side, g)
		}
	}
	return mainDataBegin, side
}

// codewords maps each Huffman table's codewords, by length and value, to
// the index of what they code: x*yLen+y for a pair, a count1 quadruple's
// four bits.
var codewords [HTN]map[[2]uint32]int

// readCodeword reads one codeword of the given Huffman table.
func readCodeword(r *bitReader, table int) (int, bool) {
	if codewords[table] == nil {
		h := &huffmanCodeTable[table]
		codewords[table] = make(map[[2]uint32]int, len(h.table))
		for i := range h.table {
			codewords[table][[2]uint32{uint32(h.hLen[i]), uint32(h.table[i])}] = i
		}
	}
	var code uint32
	for n := uint32(1); n <= 32; n++ {
		code = code<<1 | r.read(1)
		if i, ok := codewords[table][[2]uint32{n, code}]; ok {
			return i, true
		}
	}
	return 0, false
}

// readGranule reads one granule's main data as a decoder does: its scale
// factors (the encoder codes none), its big values, then count1 quadruples
// while bits of part2_3_length remain, up to the granule's last line. It
// returns how many bits the last quadruple ran past part2_3_length, or how
// many bits of it were left over once every line was read — both of them
// padding inside part2_3_length — or what else is wrong.
func readGranule(r *bitReader, g granuleSide, rateIndex int) (over, left int, wrong string) {
	if g.windowSwitching || g.scaleFactorCompress != 0 {
		return 0, 0, "the encoder codes long blocks without scale factors, but this granule is otherwise"
	}
	end := r.pos + int(g.part2_3)
	if end > len(r.b)*8 {
		return 0, 0, fmt.Sprintf("part2_3_length runs %d bits past the end of the frame", end-len(r.b)*8)
	}
	bands := scaleFactorBandIndex[rateIndex]
	region1 := int(bands[min(g.region0+1, 22)])
	region2 := int(bands[min(g.region0+g.region1+2, 22)])
	i := 0
	for ; i < int(g.bigValues)*2; i += 2 {
		table := g.table[0]
		if i >= region1 {
			table = g.table[1]
		}
		if i >= region2 {
			table = g.table[2]
		}
		if table == 0 {
			continue
		}
		h := &huffmanCodeTable[table]
		xy, ok := readCodeword(r, int(table))
		if !ok {
			return 0, 0, fmt.Sprintf("no table %d codeword at line %d", table, i)
		}
		for _, v := range []int{xy / int(h.yLen), xy % int(h.yLen)} {
			if h.linBits > 0 && v == 15 {
				r.read(int(h.linBits))
			}
			if v != 0 {
				r.read(1) // sign
			}
		}
		if r.pos > end {
			return 0, 0, fmt.Sprintf("the big-values pair at line %d runs %d bits past part2_3_length", i, r.pos-end)
		}
	}
	for ; i <= GRANULE_SIZE-4 && r.pos < end; i += 4 {
		vwxy, ok := readCodeword(r, 32+int(g.count1Table))
		if !ok {
			return 0, 0, fmt.Sprintf("no count1 codeword at line %d", i)
		}
		r.read(bits.OnesCount(uint(vwxy))) // signs
		if r.pos > end {
			return r.pos - end, 0, ""
		}
	}
	return 0, end - r.pos, ""
}

// A granule's part2_3_length is the bits of its scale factors and Huffman
// codes, and nothing more: a frame's padding follows its last granule, as
// ancillary data. Inside part2_3_length a decoder reads padding as count1
// quadruples, and the last of them can run past part2_3_length; where that
// granule is the last of its frame (mono at 22.05 or 24 kHz, one granule a
// frame) it runs past the end of the frame, which FFmpeg forgives
// ("overread") and Symphonia, Chromium's MP3 decoder, does not ("huffman
// decode overrun"). Padding more than a granule's part2_3_length can hold
// (mono at 16 kHz) must be written all the same, or the frame is short.
func TestPart2_3LengthHoldsOnlyCodedBits(t *testing.T) {
	for _, c := range testConfigs {
		t.Run(c.String(), func(t *testing.T) {
			frames, headers, _ := splitFrames(t, encode(t, c, 2))
			granules, padded, overruns := 0, 0, 0
			for f, frame := range frames {
				r := &bitReader{b: frame}
				mainDataBegin, side := readSideInfo(r, headers[f])
				if mainDataBegin != 0 {
					t.Fatalf("frame %d: main_data_begin is %d, but the encoder keeps no bit reservoir", f, mainDataBegin)
				}
				for g, granule := range side {
					start := r.pos
					over, left, wrong := readGranule(r, granule, headers[f].rateIndex)
					if wrong != "" {
						t.Fatalf("frame %d, granule %d: %s", f, g, wrong)
					}
					granules++
					if over > 0 || left > 0 {
						padded++
					}
					if over > 0 {
						overruns++
					}
					r.pos = start + int(granule.part2_3)
				}
			}
			if padded > 0 {
				t.Errorf("%d of %d granules hold padding in part2_3_length; in %d of them a count1 quadruple runs past it",
					padded, granules, overruns)
			}
		})
	}
}

// A frame is a whole number of bytes, but the bit writer writes out only
// whole 32-bit words: each frame's last bytes must go out with it, or the
// stream ends short of its last frame, which Symphonia rejects ("mpa:
// invalid packet length"). Where frames are not a multiple of four bytes
// it depends on how many there are, so streams of one to eight are tried.
func TestWriteEndsOnAWholeFrame(t *testing.T) {
	for _, c := range testConfigs {
		t.Run(c.String(), func(t *testing.T) {
			perFrame := NewEncoder(c.rate, c.channels).samplesPerPass()
			for n := 1; n <= 8; n++ {
				frames, _, tail := splitFrames(t, encode(t, c, float64(n)*float64(perFrame)/float64(c.rate)))
				if tail != 0 {
					t.Errorf("%d frames: the stream ends %d bytes into the frame after its %d whole ones", n, tail, len(frames))
				}
			}
		})
	}
}

// EncodeBufferInterleaved returns each frame whole, and nothing more.
func TestEncodeBufferInterleavedReturnsAWholeFrame(t *testing.T) {
	for _, c := range testConfigs {
		t.Run(c.String(), func(t *testing.T) {
			enc := NewEncoder(c.rate, c.channels)
			pcm := testSignal(c, 0.5)
			step := int(enc.samplesPerPass()) * c.channels
			for i := 0; i+step <= len(pcm); i += step {
				data, n := enc.EncodeBufferInterleaved(pcm[i : i+step])
				if frames, _, tail := splitFrames(t, data[:n]); len(frames) != 1 || tail != 0 {
					t.Fatalf("frame %d: returned %d whole frames and %d bytes more", i/step, len(frames), tail)
				}
			}
		})
	}
}
