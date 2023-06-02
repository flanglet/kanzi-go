/*
Copyright 2011-2022 Frederic Langlet
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
you may obtain a copy of the License at

                http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package entropy

import (
	"errors"
	"fmt"

	kanzi "github.com/flanglet/kanzi-go/v2"
)

// Implementation of an Asymmetric Numeral System codec.
// See "Asymmetric Numeral System" by Jarek Duda at http://arxiv.org/abs/0902.0271
// Some code has been ported from https://github.com/rygorous/ryg_rans
// For an alternate C implementation example, see https://github.com/Cyan4973/FiniteStateEntropy

const (
	_ANS_TOP                 = 1 << 15       // max possible for ANS_TOP=1<23
	_DEFAULT_ANS0_CHUNK_SIZE = uint(1 << 15) // 32 KB by default
	_ANS_MIN_CHUNK_SIZE      = 1024
	_ANS_MAX_CHUNK_SIZE      = 1 << 27 // 8*MAX_CHUNK_SIZE must not overflow
	_DEFAULT_ANS_LOG_RANGE   = uint(12)
)

// ANSRangeEncoder Asymmetric Numeral System Encoder
type ANSRangeEncoder struct {
	bitstream kanzi.OutputBitStream
	freqs     []int
	symbols   []encSymbol
	buffer    []byte
	chunkSize int
	order     uint
	logRange  uint
}

// NewANSRangeEncoder creates an instance of ANS encoder.
// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block
// Since the number of args is variable, this function can be called like this:
// NewANSRangeEncoder(bs) or NewANSRangeEncoder(bs, 0, 16384, 12)
// Arguments are order, chunk size and log range.
// chunkSize = 0 means 'use input buffer length' during decoding
func NewANSRangeEncoder(bs kanzi.OutputBitStream, args ...uint) (*ANSRangeEncoder, error) {
	if bs == nil {
		return nil, errors.New("ANS codec: Invalid null bitstream parameter")
	}

	if len(args) > 3 {
		return nil, errors.New("ANS codec: At most order, chunk size and log range can be provided")
	}

	chkSize := _DEFAULT_ANS0_CHUNK_SIZE
	logRange := _DEFAULT_ANS_LOG_RANGE
	order := uint(0)

	if len(args) > 0 {
		order = args[0]

		if len(args) > 1 {
			chkSize = args[1]

			if len(args) > 2 {
				logRange = args[2]
			}
		}

		if order != 0 && order != 1 {
			return nil, errors.New("ANS codec: The order must be 0 or 1")
		}

		if chkSize < _ANS_MIN_CHUNK_SIZE {
			return nil, fmt.Errorf("ANS codec: The chunk size must be at least %d", _ANS_MIN_CHUNK_SIZE)
		}

		if chkSize > _ANS_MAX_CHUNK_SIZE {
			return nil, fmt.Errorf("ANS codec: The chunk size must be at most %d", _ANS_MAX_CHUNK_SIZE)
		}

		if logRange < 8 || logRange > 16 {
			return nil, fmt.Errorf("ANS codec: Invalid range: %d (must be in [8..16])", logRange)
		}

		if order == 1 {
			chkSize <<= 8

			if chkSize > _ANS_MAX_CHUNK_SIZE {
				chkSize = _ANS_MAX_CHUNK_SIZE
			}
		}
	}

	this := new(ANSRangeEncoder)
	this.bitstream = bs
	this.order = order
	dim := int(255*order + 1)
	this.freqs = make([]int, dim*257) // freqs[x][256] = total(freqs[x][0..255])
	this.symbols = make([]encSymbol, dim*256)
	this.buffer = make([]byte, 0)
	this.logRange = logRange - order
	this.chunkSize = int(chkSize)
	return this, nil
}

// NewANSRangeEncoderWithCtx creates a new instance of ANSRangeEncoder providing a
// context map.
func NewANSRangeEncoderWithCtx(bs kanzi.OutputBitStream, order uint, ctx *map[string]interface{}) (*ANSRangeEncoder, error) {
	if bs == nil {
		return nil, errors.New("ANS codec: Invalid null bitstream parameter")
	}

	if order != 0 && order != 1 {
		return nil, errors.New("ANS codec: The order must be 0 or 1")
	}

	chkSize := _DEFAULT_ANS0_CHUNK_SIZE

	if order == 1 {
		chkSize <<= 8
	}

	this := new(ANSRangeEncoder)
	this.bitstream = bs
	this.order = order
	dim := int(255*order + 1)
	this.freqs = make([]int, dim*257) // freqs[x][256] = total(freqs[x][0..255])
	this.symbols = make([]encSymbol, dim*256)
	this.buffer = make([]byte, 0)
	this.logRange = _DEFAULT_ANS_LOG_RANGE
	this.chunkSize = int(chkSize)
	return this, nil
}

// Compute cumulated frequencies and encode header
func (this *ANSRangeEncoder) updateFrequencies(frequencies []int, lr uint) (int, error) {
	res := 0
	endk := int(255*this.order + 1)
	this.bitstream.WriteBits(uint64(lr-8), 3) // logRange
	var alphabet [256]int

	var err error

	for k := 0; k < endk; k++ {
		f := frequencies[257*k : 257*(k+1)]
		symb := this.symbols[k<<8 : (k+1)<<8]
		var alphabetSize int

		if alphabetSize, err = NormalizeFrequencies(f, alphabet[:], f[256], 1<<lr); err != nil {
			break
		}

		if alphabetSize > 0 {
			sum := 0

			for i := 0; i < 256; i++ {
				if f[i] == 0 {
					continue
				}

				symb[i].reset(sum, f[i], lr)
				sum += f[i]
			}
		}

		if err = this.encodeHeader(alphabetSize, alphabet[:], f, lr); err != nil {
			break
		}

		res += alphabetSize
	}

	return res, err
}

// Encodes alphabet and frequencies into the bitstream
func (this *ANSRangeEncoder) encodeHeader(alphabetSize int, alphabet []int, frequencies []int, lr uint) error {
	if _, err := EncodeAlphabet(this.bitstream, alphabet[0:alphabetSize:256]); err != nil {
		return err
	}

	if alphabetSize == 0 {
		return nil
	}

	chkSize := 8

	if alphabetSize < 64 {
		chkSize = 6
	}

	llr := uint(3)

	for 1<<llr <= lr {
		llr++
	}

	// Encode all frequencies (but the first one) by chunks
	for i := 1; i < alphabetSize; i += chkSize {
		max := frequencies[alphabet[i]] - 1
		logMax := uint(0)
		endj := i + chkSize

		if endj > alphabetSize {
			endj = alphabetSize
		}

		// Search for max frequency log size in next chunk
		for j := i + 1; j < endj; j++ {
			if frequencies[alphabet[j]]-1 > max {
				max = frequencies[alphabet[j]] - 1
			}
		}

		for 1<<logMax <= max {
			logMax++
		}

		this.bitstream.WriteBits(uint64(logMax), llr)

		if logMax == 0 {
			// all frequencies equal one in this chunk
			continue
		}

		// Write frequencies
		for j := i; j < endj; j++ {
			this.bitstream.WriteBits(uint64(frequencies[alphabet[j]]-1), logMax)
		}
	}

	return nil
}

// Write  Dynamically compute the frequencies for every chunk of data in the block
// and encode each chunk of the block sequentially
func (this *ANSRangeEncoder) Write(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Invalid null block parameter")
	}

	if len(block) == 0 {
		return 0, nil
	}

	sizeChunk := this.chunkSize

	for i := range this.symbols {
		this.symbols[i] = encSymbol{}
	}

	size := 2 * len(block)

	if size > sizeChunk+(sizeChunk>>3) { // min
		size = sizeChunk + (sizeChunk >> 3)
	}

	if size < 65536 { // max
		size = 65536
	}

	// Add some padding
	if len(this.buffer) < size {
		this.buffer = make([]byte, size)
	}

	end := len(block)
	startChunk := 0

	for startChunk < end {
		endChunk := startChunk + sizeChunk

		if endChunk >= end {
			endChunk = end
			sizeChunk = endChunk - startChunk
		}

		alphabetSize, err := this.rebuildStatistics(block[startChunk:endChunk], this.logRange)

		if err != nil {
			return end, err
		}

		if this.order == 1 || alphabetSize > 1 {
			this.encodeChunk(block[startChunk:endChunk])
		}

		startChunk = endChunk
	}

	return end, nil
}

func (this *ANSRangeEncoder) encodeSymbol(n int, st *int, sym encSymbol) int {
	if *st >= sym.xMax {
		this.buffer[n] = byte(*st)
		this.buffer[n-1] = byte(*st >> 8)
		*st >>= 16
		n -= 2
	}

	*st = *st + sym.bias + int((uint64(*st)*sym.invFreq)>>sym.invShift)*sym.cmplFreq
	return n
}

func (this *ANSRangeEncoder) encodeChunk(block []byte) {
	st0 := _ANS_TOP
	st1 := _ANS_TOP
	st2 := _ANS_TOP
	st3 := _ANS_TOP
	n := len(this.buffer) - 1
	end4 := len(block) & -4

	for i := len(block) - 1; i >= end4; i-- {
		this.buffer[n] = block[i]
		n--
	}

	if this.order == 0 {
		symb := this.symbols[0:256]

		for i := end4 - 1; i > 0; i -= 4 {
			n = this.encodeSymbol(n, &st0, symb[block[i]])
			n = this.encodeSymbol(n, &st1, symb[block[i-1]])
			n = this.encodeSymbol(n, &st2, symb[block[i-2]])
			n = this.encodeSymbol(n, &st3, symb[block[i-3]])
		}
	} else { // order 1
		quarter := end4 >> 2
		i0 := 1*quarter - 2
		i1 := 2*quarter - 2
		i2 := 3*quarter - 2
		i3 := end4 - 2
		prv0 := int(block[i0+1])
		prv1 := int(block[i1+1])
		prv2 := int(block[i2+1])
		prv3 := int(block[i3+1])

		for i0 >= 0 {
			cur0 := int(block[i0])
			n = this.encodeSymbol(n, &st0, this.symbols[(cur0<<8)|prv0])
			cur1 := int(block[i1])
			n = this.encodeSymbol(n, &st1, this.symbols[(cur1<<8)|prv1])
			cur2 := int(block[i2])
			n = this.encodeSymbol(n, &st2, this.symbols[(cur2<<8)|prv2])
			cur3 := int(block[i3])
			n = this.encodeSymbol(n, &st3, this.symbols[(cur3<<8)|prv3])
			prv0 = cur0
			prv1 = cur1
			prv2 = cur2
			prv3 = cur3
			i0--
			i1--
			i2--
			i3--
		}

		// Last symbols
		n = this.encodeSymbol(n, &st0, this.symbols[prv0])
		n = this.encodeSymbol(n, &st1, this.symbols[prv1])
		n = this.encodeSymbol(n, &st2, this.symbols[prv2])
		n = this.encodeSymbol(n, &st3, this.symbols[prv3])
	}

	n++

	// Write chunk size
	WriteVarInt(this.bitstream, uint32(len(this.buffer)-n))

	// Write final ANS state
	this.bitstream.WriteBits(uint64(st0), 32)
	this.bitstream.WriteBits(uint64(st1), 32)
	this.bitstream.WriteBits(uint64(st2), 32)
	this.bitstream.WriteBits(uint64(st3), 32)

	if len(this.buffer) != n {
		// Write encoded data to bitstream
		this.bitstream.WriteArray(this.buffer[n:], 8*uint(len(this.buffer)-n))
	}
}

// Compute chunk frequencies, cumulated frequencies and encode chunk header
func (this *ANSRangeEncoder) rebuildStatistics(block []byte, lr uint) (int, error) {
	for i := range this.freqs {
		this.freqs[i] = 0
	}

	if this.order == 0 {
		kanzi.ComputeHistogram(block, this.freqs, true, true)
	} else {
		quarter := len(block) >> 2
		kanzi.ComputeHistogram(block[0*quarter:1*quarter], this.freqs, false, true)
		kanzi.ComputeHistogram(block[1*quarter:2*quarter], this.freqs, false, true)
		kanzi.ComputeHistogram(block[2*quarter:3*quarter], this.freqs, false, true)
		kanzi.ComputeHistogram(block[3*quarter:4*quarter], this.freqs, false, true)
	}

	return this.updateFrequencies(this.freqs, lr)
}

// Dispose this implementation does nothing
func (this *ANSRangeEncoder) Dispose() {
}

// BitStream returns the underlying bitstream
func (this *ANSRangeEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

type encSymbol struct {
	xMax     int    // (Exclusive) upper bound of pre-normalization interval
	bias     int    // Bias
	cmplFreq int    // Complement of frequency: (1 << scale_bits) - freq
	invShift uint8  // Reciprocal shift
	invFreq  uint64 // Fixed-point reciprocal frequency
}

func (this *encSymbol) reset(cumFreq, freq int, logRange uint) {
	// Make sure xMax is a positive int32. Compatibility with Java implementation
	if freq >= 1<<logRange {
		freq = (1 << logRange) - 1
	}

	this.xMax = ((_ANS_TOP >> logRange) << 16) * freq
	this.cmplFreq = (1 << logRange) - freq

	if freq < 2 {
		this.invFreq = 0xFFFFFFFF
		this.invShift = 32
		this.bias = cumFreq + (1 << logRange) - 1
	} else {
		shift := uint(0)

		for freq > 1<<shift {
			shift++
		}

		// Alverson, "Integer Division using reciprocals"
		this.invFreq = (((1 << (shift + 31)) + uint64(freq-1)) / uint64(freq)) & 0xFFFFFFFF
		this.invShift = uint8(32 + shift - 1)
		this.bias = cumFreq
	}
}

// ANSRangeDecoder Asymmetric Numeral System Decoder
type ANSRangeDecoder struct {
	bitstream    kanzi.InputBitStream
	freqs        []int
	symbols      []decSymbol
	f2s          []byte // mapping frequency -> symbol
	buffer       []byte
	chunkSize    int
	logRange     uint
	order        uint
	isBsVersion1 bool
}

// NewANSRangeDecoder creates an instance of ANS decoder.
// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats.
// Since the number of args is variable, this function can be called like this:
// NewANSRangeDecoder(bs) or NewANSRangeDecoder(bs, 0, 16384, 12)
// Arguments are order and chunk size
// chunkSize = 0 means 'use input buffer length' during decoding
func NewANSRangeDecoder(bs kanzi.InputBitStream, args ...uint) (*ANSRangeDecoder, error) {
	if bs == nil {
		return nil, errors.New("ANS codec: Invalid null bitstream parameter")
	}

	if len(args) > 3 {
		return nil, errors.New("ANS codec: At most order, chunk size and bitstream version can be provided")
	}

	chkSize := _DEFAULT_ANS0_CHUNK_SIZE
	order := uint(0)

	if len(args) > 0 {
		order = args[0]

		if len(args) > 1 {
			chkSize = args[1]
		}

		if chkSize < _ANS_MIN_CHUNK_SIZE {
			return nil, fmt.Errorf("ANS codec: The chunk size must be at least %d", _ANS_MIN_CHUNK_SIZE)
		}

		if chkSize > _ANS_MAX_CHUNK_SIZE {
			return nil, fmt.Errorf("ANS codec: The chunk size must be at most %d", _ANS_MAX_CHUNK_SIZE)
		}

		if order != 0 && order != 1 {
			return nil, errors.New("ANS codec: The order must be 0 or 1")
		}

		if order == 1 {
			chkSize <<= 8

			if chkSize > _ANS_MAX_CHUNK_SIZE {
				chkSize = _ANS_MAX_CHUNK_SIZE
			}
		}
	}

	this := new(ANSRangeDecoder)
	this.bitstream = bs
	this.chunkSize = int(chkSize)
	this.order = order
	dim := int(255*order + 1)
	this.freqs = make([]int, dim*256)
	this.buffer = make([]byte, 0)
	this.f2s = make([]byte, 0)
	this.symbols = make([]decSymbol, dim*256)
	this.isBsVersion1 = false
	this.logRange = _DEFAULT_ANS_LOG_RANGE
	return this, nil
}

// NewANSRangeDecoderWithCtx creates a new instance of ANSRangeDecoder providing a
// context map.
func NewANSRangeDecoderWithCtx(bs kanzi.InputBitStream, order uint, ctx *map[string]interface{}) (*ANSRangeDecoder, error) {
	if bs == nil {
		return nil, errors.New("ANS codec: Invalid null bitstream parameter")
	}

	if order != 0 && order != 1 {
		return nil, errors.New("ANS codec: The order must be 0 or 1")
	}

	chkSize := _DEFAULT_ANS0_CHUNK_SIZE

	if order == 1 {
		chkSize <<= 8
	}

	bsVersion := uint(2)

	if ctx != nil {
		if val, containsKey := (*ctx)["bsVersion"]; containsKey {
			bsVersion = val.(uint)
		}
	}

	this := new(ANSRangeDecoder)
	this.bitstream = bs
	this.chunkSize = int(chkSize)
	this.order = order
	dim := int(255*order + 1)
	this.freqs = make([]int, dim*256)
	this.buffer = make([]byte, 0)
	this.f2s = make([]byte, 0)
	this.symbols = make([]decSymbol, dim*256)
	this.isBsVersion1 = bsVersion == 1
	return this, nil
}

// Decodes alphabet and frequencies from the bitstream
func (this *ANSRangeDecoder) decodeHeader(frequencies, alphabet []int) (int, error) {
	this.logRange = uint(8 + this.bitstream.ReadBits(3))

	if this.logRange < 8 || this.logRange > 16 {
		return 0, fmt.Errorf("Invalid bitstream: range = %d (must be in [8..16])", this.logRange)
	}

	res := 0
	dim := int(255*this.order + 1)
	scale := 1 << this.logRange

	if len(this.f2s) < dim*scale {
		this.f2s = make([]byte, dim*scale)
	}

	for k := 0; k < dim; k++ {
		f := frequencies[k<<8 : (k+1)<<8]
		alphabetSize, err := DecodeAlphabet(this.bitstream, alphabet)

		if err != nil {
			return alphabetSize, err
		}

		if alphabetSize == 0 {
			continue
		}

		if alphabetSize != 256 {
			for i := range f {
				f[i] = 0
			}
		}

		chkSize := 8

		if alphabetSize < 64 {
			chkSize = 6
		}

		llr := uint(3)

		for 1<<llr <= this.logRange {
			llr++
		}

		sum := 0

		// Decode all frequencies (but the first one) by chunks
		for i := 1; i < alphabetSize; i += chkSize {
			// Read frequencies size for current chunk
			logMax := uint(this.bitstream.ReadBits(llr))

			if 1<<logMax > scale {
				err := fmt.Errorf("Invalid bitstream: incorrect frequency size %d in ANS range decoder", logMax)
				return alphabetSize, err
			}

			endj := i + chkSize

			if endj > alphabetSize {
				endj = alphabetSize
			}

			// Read frequencies
			for j := i; j < endj; j++ {
				freq := 1

				if logMax > 0 {
					freq = int(1 + this.bitstream.ReadBits(logMax))

					if freq <= 0 || freq >= scale {
						err := fmt.Errorf("Invalid bitstream: incorrect frequency %d for symbol '%d' in ANS range decoder", freq, alphabet[j])
						return alphabetSize, err
					}
				}

				f[alphabet[j]] = freq
				sum += freq
			}
		}

		// Infer first frequency
		if scale <= sum {
			err := fmt.Errorf("Invalid bitstream: incorrect frequency %d for symbol '%d' in ANS range decoder", frequencies[alphabet[0]], alphabet[0])
			return alphabetSize, err
		}

		f[alphabet[0]] = scale - sum
		sum = 0
		symb := this.symbols[k<<8 : (k+1)<<8]
		freq2sym := this.f2s[k<<this.logRange : (k+1)<<this.logRange]

		// Create reverse mapping
		for i := range f {
			if f[i] == 0 {
				continue
			}

			for j := f[i] - 1; j >= 0; j-- {
				freq2sym[sum+j] = byte(i)
			}

			symb[i].reset(sum, f[i], this.logRange)
			sum += f[i]
		}

		res += alphabetSize
	}

	return res, nil
}

// Decode data from the bitstream and write them, chunk by chunk,
// into the block.
func (this *ANSRangeDecoder) Read(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Invalid null block parameter")
	}

	if len(block) == 0 {
		return 0, nil
	}

	sizeChunk := this.chunkSize
	end := len(block)
	startChunk := 0

	for i := range this.symbols {
		this.symbols[i] = decSymbol{}
	}

	var alphabet [256]int

	for startChunk < end {
		endChunk := startChunk + sizeChunk

		if endChunk >= end {
			endChunk = end
			sizeChunk = end - startChunk
		}

		alphabetSize, err := this.decodeHeader(this.freqs, alphabet[:])

		if err != nil || alphabetSize == 0 {
			return startChunk, err
		}

		if this.order == 0 && alphabetSize == 1 {
			// Shortcut for chunks with only one symbol
			for i := startChunk; i < endChunk; i++ {
				block[i] = byte(alphabet[0])
			}
		} else {
			if this.isBsVersion1 == true {
				this.decodeChunkV1(block[startChunk:endChunk])
			} else {
				this.decodeChunkV2(block[startChunk:endChunk])
			}
		}

		startChunk = endChunk
	}

	return len(block), nil
}

func (this *ANSRangeDecoder) decodeChunkV1(block []byte) {
	// Read chunk size
	sz := ReadVarInt(this.bitstream) & (_ANS_MAX_CHUNK_SIZE - 1)

	// Read initial ANS state
	st0 := int(this.bitstream.ReadBits(32))
	st1 := 0

	if this.order == 0 {
		st1 = int(this.bitstream.ReadBits(32))
	}

	// Read encoded data
	if sz != 0 {
		// Add some padding
		if len(this.buffer) < int(sz) {
			this.buffer = make([]byte, sz+(sz>>3))
		}

		this.bitstream.ReadArray(this.buffer[0:sz], uint(8*sz))
	}

	n := 0
	lr := this.logRange
	mask := (1 << lr) - 1

	if this.order == 0 {
		freq2sym := this.f2s[0 : mask+1]
		symb := this.symbols[0:256]
		end2 := (len(block) & -2) - 1

		for i := 0; i < end2; i += 2 {
			cur1 := freq2sym[st1&mask]
			block[i] = cur1
			sym1 := symb[cur1]
			cur0 := freq2sym[st0&mask]
			block[i+1] = cur0
			sym0 := symb[cur0]

			// Compute next ANS state
			// D(x) = (s, q_s (x/M) + mod(x,M) - b_s) where s is such b_s <= x mod M < b_{s+1}
			st1 = sym1.freq*(st1>>lr) + (st1 & mask) - sym1.cumFreq
			st0 = sym0.freq*(st0>>lr) + (st0 & mask) - sym0.cumFreq

			// Normalize
			for st1 < _ANS_TOP {
				st1 = (st1 << 8) | int(this.buffer[n])
				st1 = (st1 << 8) | int(this.buffer[n+1])
				n += 2
			}

			for st0 < _ANS_TOP {
				st0 = (st0 << 8) | int(this.buffer[n])
				st0 = (st0 << 8) | int(this.buffer[n+1])
				n += 2
			}
		}

		if len(block)&1 != 0 {
			block[len(block)-1] = this.buffer[sz-1]
		}
	} else { // order1
		prv := int(0)

		for i := range block {
			cur := this.f2s[(prv<<lr)|(st0&mask)]
			block[i] = cur
			sym := this.symbols[(prv<<8)|int(cur)]

			// Compute next ANS state
			// D(x) = (s, q_s (x/M) + mod(x,M) - b_s) where s is such b_s <= x mod M < b_{s+1}
			st0 = sym.freq*(st0>>lr) + (st0 & mask) - sym.cumFreq

			// Normalize
			for st0 < _ANS_TOP {
				st0 = (st0 << 8) | int(this.buffer[n])
				st0 = (st0 << 8) | int(this.buffer[n+1])
				n += 2
			}

			prv = int(cur)
		}
	}
}

func (this *ANSRangeDecoder) decodeSymbol(n int, st *int, sym decSymbol, mask int) int {
	// Compute next ANS state
	// D(x) = (s, q_s (x/M) + mod(x,M) - b_s) where s is such b_s <= x mod M < b_{s+1}
	*st = sym.freq*(*st>>this.logRange) + (*st & mask) - sym.cumFreq

	// Normalize
	if *st < _ANS_TOP {
		*st = (*st << 8) | int(this.buffer[n])
		*st = (*st << 8) | int(this.buffer[n+1])
		n += 2
	}

	return n
}

func (this *ANSRangeDecoder) decodeChunkV2(block []byte) {
	// Read chunk size
	sz := ReadVarInt(this.bitstream) & (_ANS_MAX_CHUNK_SIZE - 1)

	// Read initial ANS state
	st0 := int(this.bitstream.ReadBits(32))
	st1 := int(this.bitstream.ReadBits(32))
	st2 := int(this.bitstream.ReadBits(32))
	st3 := int(this.bitstream.ReadBits(32))

	// Read encoded data
	if sz != 0 {
		// Add some padding
		if len(this.buffer) < int(sz) {
			this.buffer = make([]byte, sz+(sz>>3))
		}

		this.bitstream.ReadArray(this.buffer[0:sz], uint(8*sz))
	}

	n := 0
	lr := this.logRange
	mask := (1 << lr) - 1
	end4 := len(block) & -4

	if this.order == 0 {
		freq2sym := this.f2s[0 : mask+1]
		symb := this.symbols[0:256]

		for i := 0; i < end4; i += 4 {
			cur3 := freq2sym[st3&mask]
			block[i] = byte(cur3)
			n = this.decodeSymbol(n, &st3, symb[cur3], mask)
			cur2 := freq2sym[st2&mask]
			block[i+1] = byte(cur2)
			n = this.decodeSymbol(n, &st2, symb[cur2], mask)
			cur1 := freq2sym[st1&mask]
			block[i+2] = byte(cur1)
			n = this.decodeSymbol(n, &st1, symb[cur1], mask)
			cur0 := freq2sym[st0&mask]
			block[i+3] = byte(cur0)
			n = this.decodeSymbol(n, &st0, symb[cur0], mask)
		}
	} else { // order 1
		quarter := end4 >> 2
		i0 := 0
		i1 := 1 * quarter
		i2 := 2 * quarter
		i3 := 3 * quarter
		prv0, prv1, prv2, prv3 := 0, 0, 0, 0

		for i0 < quarter {
			symbols3 := this.symbols[prv3<<8:]
			symbols2 := this.symbols[prv2<<8:]
			symbols1 := this.symbols[prv1<<8:]
			symbols0 := this.symbols[prv0<<8:]
			cur3 := int(this.f2s[(prv3<<this.logRange)+(st3&mask)])
			block[i3] = byte(cur3)
			n = this.decodeSymbol(n, &st3, symbols3[cur3], mask)
			cur2 := int(this.f2s[(prv2<<this.logRange)+(st2&mask)])
			block[i2] = byte(cur2)
			n = this.decodeSymbol(n, &st2, symbols2[cur2], mask)
			cur1 := int(this.f2s[(prv1<<this.logRange)+(st1&mask)])
			block[i1] = byte(cur1)
			n = this.decodeSymbol(n, &st1, symbols1[cur1], mask)
			cur0 := int(this.f2s[(prv0<<this.logRange)+(st0&mask)])
			block[i0] = byte(cur0)
			n = this.decodeSymbol(n, &st0, symbols0[cur0], mask)
			prv3 = cur3
			prv2 = cur2
			prv1 = cur1
			prv0 = cur0
			i0++
			i1++
			i2++
			i3++
		}
	}

	for i := end4; i < len(block); i++ {
		block[i] = this.buffer[n]
		n++
	}
}

// BitStream returns the underlying bitstream
func (this *ANSRangeDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

// Dispose this implementation does nothing
func (this *ANSRangeDecoder) Dispose() {
}

type decSymbol struct {
	cumFreq int
	freq    int
}

func (this *decSymbol) reset(cumFreq, freq int, logRange uint) {
	// Mirror encoder
	if freq >= 1<<logRange {
		freq = (1 << logRange) - 1
	}

	this.cumFreq = cumFreq
	this.freq = freq
}
