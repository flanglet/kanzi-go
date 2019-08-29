/*
Copyright 2011-2017 Frederic Langlet
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

	kanzi "github.com/flanglet/kanzi-go"
)

// Implementation of an Asymmetric Numeral System codec.
// See "Asymmetric Numeral System" by Jarek Duda at http://arxiv.org/abs/0902.0271
// Some code has been ported from https://github.com/rygorous/ryg_rans
// For an alternate C implementation example, see https://github.com/Cyan4973/FiniteStateEntropy

const (
	_ANS_TOP                 = 1 << 15       // max possible for ANS_TOP=1<23
	_DEFAULT_ANS0_CHUNK_SIZE = uint(1 << 15) // 32 KB by default
	_ANS_MAX_CHUNK_SIZE      = 1 << 27       // 8*MAX_CHUNK_SIZE must not overflow
	_DEFAULT_ANS_LOG_RANGE   = uint(12)
)

// ANSRangeEncoder Asymmetric Numeral System Encoder
type ANSRangeEncoder struct {
	bitstream kanzi.OutputBitStream
	alphabet  []int
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
	}

	if len(args) > 1 {
		chkSize = args[1]
	} else if order == 1 {
		chkSize <<= 8
	}

	if len(args) > 2 {
		logRange = args[2]
	}

	if order != 0 && order != 1 {
		return nil, errors.New("ANS codec: The order must be 0 or 1")
	}

	if chkSize != 0 && chkSize < 1024 {
		return nil, errors.New("ANS codec: The chunk size must be at least 1024")
	}

	if chkSize > _ANS_MAX_CHUNK_SIZE {
		return nil, fmt.Errorf("ANS codec: The chunk size must be at most %d", _ANS_MAX_CHUNK_SIZE)
	}

	if logRange < 8 || logRange > 16 {
		return nil, fmt.Errorf("ANS codec: Invalid range: %v (must be in [8..16])", logRange)
	}

	this := new(ANSRangeEncoder)
	this.bitstream = bs
	this.order = order
	dim := int(255*order + 1)
	this.alphabet = make([]int, dim*256)
	this.freqs = make([]int, dim*257) // freqs[x][256] = total(freqs[x][0..255])
	this.symbols = make([]encSymbol, dim*256)
	this.buffer = make([]byte, 0)
	this.logRange = logRange
	this.chunkSize = int(chkSize)
	return this, nil
}

// Compute cumulated frequencies and encode header
func (this *ANSRangeEncoder) updateFrequencies(frequencies []int, lr uint) (int, error) {
	res := 0
	endk := int(255*this.order + 1)
	this.bitstream.WriteBits(uint64(lr-8), 3) // logRange
	var err error

	for k := 0; k < endk; k++ {
		f := frequencies[257*k : 257*(k+1)]
		symb := this.symbols[k<<8 : (k+1)<<8]
		curAlphabet := this.alphabet[k<<8 : (k+1)<<8]
		var alphabetSize int

		if alphabetSize, err = NormalizeFrequencies(f, curAlphabet, f[256], 1<<lr); err != nil {
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

		if err = this.encodeHeader(alphabetSize, curAlphabet, f, lr); err != nil {
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

	chkSize := 12

	if alphabetSize < 64 {
		chkSize = 6
	}

	llr := uint(3)

	for 1<<llr <= lr {
		llr++
	}

	/// Encode all frequencies (but the first one) by chunks
	for i := 1; i < alphabetSize; i += chkSize {
		max := 0
		logMax := uint(1)
		endj := i + chkSize

		if endj > alphabetSize {
			endj = alphabetSize
		}

		// Search for max frequency log size in next chunk
		for j := i; j < endj; j++ {
			if frequencies[alphabet[j]] > max {
				max = frequencies[alphabet[j]]
			}
		}

		for 1<<logMax <= max {
			logMax++
		}

		this.bitstream.WriteBits(uint64(logMax-1), llr)

		// Write frequencies
		for j := i; j < endj; j++ {
			this.bitstream.WriteBits(uint64(frequencies[alphabet[j]]), logMax)
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

	if sizeChunk == 0 {
		sizeChunk = len(block)
	}

	if sizeChunk >= _ANS_MAX_CHUNK_SIZE {
		sizeChunk = _ANS_MAX_CHUNK_SIZE
	}

	for i := range this.symbols {
		this.symbols[i] = encSymbol{}
	}

	// Add some padding
	if len(this.buffer) < sizeChunk+(sizeChunk>>3) {
		this.buffer = make([]byte, sizeChunk+(sizeChunk>>3))
	}

	end := len(block)
	startChunk := 0

	for startChunk < end {
		endChunk := startChunk + sizeChunk

		if endChunk >= end {
			endChunk = end
			sizeChunk = endChunk - startChunk
		}

		lr := this.logRange

		// Lower log range if the size of the data block is small
		for lr > 8 && 1<<lr > endChunk-startChunk {
			lr--
		}

		if _, err := this.rebuildStatistics(block[startChunk:endChunk], lr); err != nil {
			return end, err
		}

		this.encodeChunk(block[startChunk:endChunk])
		startChunk = endChunk
	}

	return end, nil
}

func (this *ANSRangeEncoder) encodeChunk(block []byte) {
	st := _ANS_TOP
	n := len(this.buffer) - 1

	if this.order == 0 {
		symb := this.symbols[0:256]

		for i := len(block) - 1; i >= 0; i-- {
			sym := symb[block[i]]

			for st >= sym.xMax {
				this.buffer[n] = byte(st)
				st >>= 8
				this.buffer[n-1] = byte(st)
				st >>= 8
				n -= 2
			}

			// Compute next ANS state
			// C(s,x) = M floor(x/q_s) + mod(x,q_s) + b_s where b_s = q_0 + ... + q_{s-1}
			// st = ((st / freq) << lr) + (st % freq) + cumFreq[prv];
			st = st + sym.bias + int((uint64(st)*sym.invFreq)>>sym.invShift)*sym.cmplFreq
		}
	} else { // order 1
		symb := this.symbols
		prv := int(block[len(block)-1])

		for i := len(block) - 2; i >= 0; i-- {
			cur := int(block[i])
			sym := symb[(cur<<8)|prv]

			for st >= sym.xMax {
				this.buffer[n] = byte(st)
				st >>= 8
				this.buffer[n-1] = byte(st)
				st >>= 8
				n -= 2
			}

			// Compute next ANS state
			// C(s,x) = M floor(x/q_s) + mod(x,q_s) + b_s where b_s = q_0 + ... + q_{s-1}
			// st = ((st / freq) << lr) + (st % freq) + cumFreq[prv];
			st = st + sym.bias + int((uint64(st)*sym.invFreq)>>sym.invShift)*sym.cmplFreq
			prv = cur
		}

		// Last symbol
		sym := symb[prv]

		for st >= sym.xMax {
			this.buffer[n] = byte(st)
			st >>= 8
			this.buffer[n-1] = byte(st)
			st >>= 8
			n -= 2
		}

		// Compute next ANS state
		// C(s,x) = M floor(x/q_s) + mod(x,q_s) + b_s where b_s = q_0 + ... + q_{s-1}
		// st = ((st / freq) << lr) + (st % freq) + cumFreq[prv];
		st = st + sym.bias + int((uint64(st)*sym.invFreq)>>sym.invShift)*sym.cmplFreq
	}

	n++

	// Write chunk size
	WriteVarInt(this.bitstream, uint32(len(this.buffer)-n))

	// Write final ANS state
	this.bitstream.WriteBits(uint64(st), 32)

	if len(this.buffer) != n {
		// Write encoded data to bitstream
		this.bitstream.WriteArray(this.buffer[n:], 8*uint(len(this.buffer)-n))
	}
}

// Compute chunk frequencies, cumulated frequencies and encode chunk header
func (this *ANSRangeEncoder) rebuildStatistics(block []byte, lr uint) (int, error) {
	kanzi.ComputeHistogram(block, this.freqs, this.order == 0, true)
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
	bitstream kanzi.InputBitStream
	freqs     []int
	symbols   []decSymbol
	f2s       []byte // mapping frequency -> symbol
	alphabet  []int
	buffer    []byte
	chunkSize int
	logRange  uint
	order     uint
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

	if len(args) > 2 {
		return nil, errors.New("ANS codec: At most order and chunk size can be provided")
	}

	chkSize := _DEFAULT_ANS0_CHUNK_SIZE
	order := uint(0)

	if len(args) > 0 {
		order = args[0]
	}

	if len(args) > 1 {
		chkSize = args[1]
	} else if order == 1 {
		chkSize <<= 8
	}

	if order != 0 && order != 1 {
		return nil, errors.New("ANS codec: The order must be 0 or 1")
	}

	if chkSize != 0 && chkSize < 1024 {
		return nil, errors.New("ANS codec: The chunk size must be at least 1024")
	}

	if chkSize > _ANS_MAX_CHUNK_SIZE {
		return nil, fmt.Errorf("ANS codec: The chunk size must be at most %d", _ANS_MAX_CHUNK_SIZE)
	}

	this := new(ANSRangeDecoder)
	this.bitstream = bs
	this.chunkSize = int(chkSize)
	this.order = order
	dim := int(255*order + 1)
	this.alphabet = make([]int, dim*256)
	this.freqs = make([]int, dim*256)
	this.buffer = make([]byte, 0)
	this.f2s = make([]byte, 0)
	this.symbols = make([]decSymbol, dim*256)
	return this, nil
}

// Decodes alphabet and frequencies from the bitstream
func (this *ANSRangeDecoder) decodeHeader(frequencies []int) (int, error) {
	this.logRange = uint(8 + this.bitstream.ReadBits(3))

	if this.logRange < 8 || this.logRange > 16 {
		return 0, fmt.Errorf("ANS codec: Invalid range: %v (must be in [8..16])", this.logRange)
	}

	res := 0
	dim := int(255*this.order + 1)
	scale := 1 << this.logRange

	if len(this.f2s) < dim*scale {
		this.f2s = make([]byte, dim*scale)
	}

	for k := 0; k < dim; k++ {
		f := frequencies[k<<8 : (k+1)<<8]
		alphabet := this.alphabet[k<<8 : (k+1)<<8]
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

		chkSize := 12

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
			logMax := uint(1 + this.bitstream.ReadBits(llr))

			if 1<<logMax > scale {
				err := fmt.Errorf("Invalid bitstream: incorrect frequency size %v in ANS range decoder", logMax)
				return alphabetSize, err
			}

			endj := i + chkSize

			if endj > alphabetSize {
				endj = alphabetSize
			}

			// Read frequencies
			for j := i; j < endj; j++ {
				freq := int(this.bitstream.ReadBits(logMax))

				if freq <= 0 || freq >= scale {
					err := fmt.Errorf("Invalid bitstream: incorrect frequency %v for symbol '%v' in ANS range decoder", freq, alphabet[j])
					return alphabetSize, err
				}

				f[alphabet[j]] = freq
				sum += freq
			}
		}

		// Infer first frequency
		if scale <= sum {
			err := fmt.Errorf("Invalid bitstream: incorrect frequency %v for symbol '%v' in ANS range decoder", frequencies[alphabet[0]], this.alphabet[0])
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

	if sizeChunk == 0 {
		sizeChunk = len(block)
	}

	if sizeChunk >= _ANS_MAX_CHUNK_SIZE {
		sizeChunk = _ANS_MAX_CHUNK_SIZE
	}

	end := len(block)
	startChunk := 0

	for i := range this.symbols {
		this.symbols[i] = decSymbol{}
	}

	// Add some padding
	if len(this.buffer) < sizeChunk+(sizeChunk>>3) {
		this.buffer = make([]byte, sizeChunk+(sizeChunk>>3))
	}

	for startChunk < end {
		alphabetSize, err := this.decodeHeader(this.freqs)

		if err != nil || alphabetSize == 0 {
			return startChunk, err
		}

		endChunk := startChunk + sizeChunk

		if endChunk >= end {
			endChunk = end
			sizeChunk = end - startChunk
		}

		this.decodeChunk(block[startChunk:endChunk])
		startChunk = endChunk
	}

	return len(block), nil
}

func (this *ANSRangeDecoder) decodeChunk(block []byte) {
	// Read chunk size
	sz := ReadVarInt(this.bitstream) & (_ANS_MAX_CHUNK_SIZE - 1)

	// Read initial ANS state
	st := int(this.bitstream.ReadBits(32))

	// Read encoded data
	if sz != 0 {
		this.bitstream.ReadArray(this.buffer[0:sz], uint(8*sz))
	}

	n := 0
	lr := this.logRange
	mask := (1 << lr) - 1

	if this.order == 0 {
		freq2sym := this.f2s[0 : mask+1]
		symb := this.symbols[0:256]

		for i := range block {
			cur := freq2sym[st&mask]
			block[i] = cur
			sym := symb[cur]

			// Compute next ANS state
			// D(x) = (s, q_s (x/M) + mod(x,M) - b_s) where s is such b_s <= x mod M < b_{s+1}
			st = sym.freq*(st>>lr) + (st & mask) - sym.cumFreq

			// Normalize
			for st < _ANS_TOP {
				st = (st << 8) | int(this.buffer[n])
				st = (st << 8) | int(this.buffer[n+1])
				n += 2
			}
		}
	} else {
		symb := this.symbols
		prv := int(0)

		for i := range block {
			cur := this.f2s[(prv<<lr)|(st&mask)]
			block[i] = cur
			sym := symb[(prv<<8)|int(cur)]

			// Compute next ANS state
			// D(x) = (s, q_s (x/M) + mod(x,M) - b_s) where s is such b_s <= x mod M < b_{s+1}
			st = sym.freq*(st>>lr) + (st & mask) - sym.cumFreq

			// Normalize
			for st < _ANS_TOP {
				st = (st << 8) | int(this.buffer[n])
				st = (st << 8) | int(this.buffer[n+1])
				n += 2
			}

			prv = int(cur)
		}
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
