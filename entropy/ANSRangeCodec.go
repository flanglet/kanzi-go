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
	ANS_TOP                 = 1 << 23
	DEFAULT_ANS0_CHUNK_SIZE = uint(1 << 15) // 32 KB by default
	DEFAULT_ANS_LOG_RANGE   = uint(13)      // max possible for ANS_TOP=1<23
)

type ANSRangeEncoder struct {
	bitstream kanzi.OutputBitStream
	alphabet  []int
	freqs     []int
	symbols   []EncSymbol
	buffer    []byte
	eu        *EntropyUtils
	chunkSize int
	order     uint
	logRange  uint
}

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats. 0 means that frequencies calculated at the
// beginning of the block apply to the whole block
// Since the number of args is variable, this function can be called like this:
// NewANSRangeEncoder(bs) or NewANSRangeEncoder(bs, 0, 16384, 12)
// Arguments are order, chunk size and log range.
// The default chunk size is 65536 bytes.
// chunkSize = 0 means 'use input buffer length' during decoding
func NewANSRangeEncoder(bs kanzi.OutputBitStream, args ...uint) (*ANSRangeEncoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if len(args) > 3 {
		return nil, errors.New("At most order, chunk size and log range can be provided")
	}

	chkSize := DEFAULT_ANS0_CHUNK_SIZE
	logRange := DEFAULT_ANS_LOG_RANGE
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
		return nil, errors.New("The order must be 0 or 1")
	}

	if chkSize != 0 && chkSize < 1024 {
		return nil, errors.New("The chunk size must be at least 1024")
	}

	if chkSize > 1<<30 {
		return nil, errors.New("The chunk size must be at most 2^30")
	}

	if logRange < 8 || logRange > 16 {
		return nil, fmt.Errorf("Invalid range: %v (must be in [8..16])", logRange)
	}

	this := new(ANSRangeEncoder)
	this.bitstream = bs
	this.order = order
	dim := int(255*order + 1)
	this.alphabet = make([]int, dim*256)
	this.freqs = make([]int, dim*257) // freqs[x][256] = total(freqs[x][0..255])
	this.symbols = make([]EncSymbol, dim*256)
	this.buffer = make([]byte, 0)
	this.logRange = logRange
	this.chunkSize = int(chkSize)
	var err error
	this.eu, err = NewEntropyUtils()
	return this, err
}

// Compute cumulated frequencies and encode header
func (this *ANSRangeEncoder) updateFrequencies(frequencies []int, lr uint) (int, error) {
	res := 0
	endk := int(255*this.order + 1)
	this.bitstream.WriteBits(uint64(lr-8), 3) // logRange

	for k := 0; k < endk; k++ {
		f := frequencies[257*k : 257*(k+1)]
		symb := this.symbols[k<<8 : (k+1)<<8]
		curAlphabet := this.alphabet[k<<8 : (k+1)<<8]
		alphabetSize, err := this.eu.NormalizeFrequencies(f, curAlphabet, f[256], 1<<lr)

		if err != nil {
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

		this.encodeHeader(alphabetSize, curAlphabet, f, lr)
		res += alphabetSize
	}

	return res, nil
}

// Encode alphabet and frequencies
func (this *ANSRangeEncoder) encodeHeader(alphabetSize int, alphabet []int, frequencies []int, lr uint) bool {
	EncodeAlphabet(this.bitstream, alphabet[0:alphabetSize:256])

	if alphabetSize == 0 {
		return true
	}

	chkSize := 16

	if alphabetSize <= 64 {
		chkSize = 8
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

	return true
}

// Dynamically compute the frequencies for every chunk of data in the block
func (this *ANSRangeEncoder) Encode(block []byte) (int, error) {
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

	end := len(block)
	startChunk := 0

	for i := range this.symbols {
		this.symbols[i] = EncSymbol{}
	}

	if len(this.buffer) < 2*sizeChunk {
		this.buffer = make([]byte, 2*sizeChunk)
	}

	for startChunk < end {
		endChunk := startChunk + sizeChunk
		lr := this.logRange

		if endChunk > end {
			endChunk = end
		}

		// Lower log range if the size of the data block is small
		for lr > 8 && 1<<lr > endChunk-startChunk {
			lr--
		}

		this.rebuildStatistics(block[startChunk:endChunk], lr)
		this.encodeChunk(block[startChunk:endChunk])
		startChunk = endChunk
	}

	return end, nil
}

func (this *ANSRangeEncoder) encodeChunk(block []byte) {
	st := ANS_TOP
	n := 0

	if this.order == 0 {
		symb := this.symbols[0:256]

		for i := len(block) - 1; i >= 0; i-- {
			sym := symb[block[i]]
			max := sym.xMax

			for st >= max {
				this.buffer[n] = byte(st)
				n++
				st >>= 8
			}

			// Compute next ANS state
			// C(s,x) = M floor(x/q_s) + mod(x,q_s) + b_s where b_s = q_0 + ... + q_{s-1}
			// st = ((st / freq) << lr) + (st % freq) + cumFreq[prv];
			q := int((uint64(st) * sym.invFreq) >> sym.invShift)
			st = st + sym.bias + q*sym.cmplFreq
		}
	} else { // order 1
		symb := this.symbols
		prv := int(block[len(block)-1])

		for i := len(block) - 2; i >= 0; i-- {
			cur := int(block[i])
			sym := symb[(cur<<8)+prv]
			max := sym.xMax

			for st >= max {
				this.buffer[n] = byte(st)
				n++
				st >>= 8
			}

			// Compute next ANS state
			// C(s,x) = M floor(x/q_s) + mod(x,q_s) + b_s where b_s = q_0 + ... + q_{s-1}
			// st = ((st / freq) << lr) + (st % freq) + cumFreq[prv];
			q := int((uint64(st) * sym.invFreq) >> sym.invShift)
			st = st + sym.bias + q*sym.cmplFreq
			prv = cur
		}

		// Last symbol
		sym := symb[prv]
		max := sym.xMax

		for st >= max {
			this.buffer[n] = byte(st)
			n++
			st >>= 8
		}

		// Compute next ANS state
		// C(s,x) = M floor(x/q_s) + mod(x,q_s) + b_s where b_s = q_0 + ... + q_{s-1}
		// st = ((st / freq) << lr) + (st % freq) + cumFreq[prv];
		q := int((uint64(st) * sym.invFreq) >> sym.invShift)
		st = st + sym.bias + q*sym.cmplFreq
	}

	// Write final ANS state
	this.bitstream.WriteBits(uint64(st), 32)

	// Write encoded data to bitstream
	for n--; n >= 0; n-- {
		this.bitstream.WriteBits(uint64(this.buffer[n]), 8)
	}
}

// Compute chunk frequencies, cumulated frequencies and encode chunk header
func (this *ANSRangeEncoder) rebuildStatistics(block []byte, lr uint) (int, error) {
	for i := range this.freqs {
		this.freqs[i] = 0
	}

	if this.order == 0 {
		f := this.freqs[0:257]
		f[256] = len(block)

		for _, cur := range block {
			f[cur]++
		}
	} else {
		prv := int(0)

		for _, cur := range block {
			this.freqs[prv+int(cur)]++
			this.freqs[prv+256]++
			prv = 257 * int(cur)
		}
	}

	return this.updateFrequencies(this.freqs, lr)
}

func (this *ANSRangeEncoder) Dispose() {
}

func (this *ANSRangeEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

type EncSymbol struct {
	xMax     int    // (Exclusive) upper bound of pre-normalization interval
	bias     int    // Bias
	cmplFreq int    // Complement of frequency: (1 << scale_bits) - freq
	invShift uint8  // Reciprocal shift
	invFreq  uint64 // Fixed-point reciprocal frequency
}

func (this *EncSymbol) reset(cumFreq, freq int, logRange uint) {
	// Make sure xMax is a positive int32. Compatibility with Java implementation
	if freq >= 1<<logRange {
		freq = (1 << logRange) - 1
	}

	this.xMax = ((ANS_TOP >> logRange) << 8) * freq
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

type ANSRangeDecoder struct {
	bitstream kanzi.InputBitStream
	freqs     []int
	symbols   []DecSymbol
	f2s       []byte // mapping frequency -> symbol
	alphabet  []int
	chunkSize int
	logRange  uint
	order     uint
}

// The chunk size indicates how many bytes are encoded (per block) before
// resetting the frequency stats.
// Since the number of args is variable, this function can be called like this:
// NewANSRangeDecoder(bs) or NewANSRangeDecoder(bs, 0, 16384, 12)
// Arguments are order and chunk size
// The default chunk size is 65536 bytes.
// chunkSize = 0 means 'use input buffer length' during decoding
func NewANSRangeDecoder(bs kanzi.InputBitStream, args ...uint) (*ANSRangeDecoder, error) {
	if bs == nil {
		return nil, errors.New("Invalid null bitstream parameter")
	}

	if len(args) > 2 {
		return nil, errors.New("At most order and chunk size can be provided")
	}

	chkSize := DEFAULT_ANS0_CHUNK_SIZE
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
		return nil, errors.New("The order must be 0 or 1")
	}

	if chkSize != 0 && chkSize < 1024 {
		return nil, errors.New("The chunk size must be at least 1024")
	}

	if chkSize > 1<<30 {
		return nil, errors.New("The chunk size must be at most 2^30")
	}

	this := new(ANSRangeDecoder)
	this.bitstream = bs
	this.chunkSize = int(chkSize)
	this.order = order
	dim := int(255*order + 1)
	this.alphabet = make([]int, dim*256)
	this.freqs = make([]int, dim*256) // freqs[x][256] = total(freqs[x][0..255])
	this.f2s = make([]byte, 0)
	this.symbols = make([]DecSymbol, dim*256)
	return this, nil
}

// Decode alphabet and frequencies
func (this *ANSRangeDecoder) decodeHeader(frequencies []int) (int, error) {
	res := 0
	dim := int(255*this.order + 1)
	this.logRange = uint(8 + this.bitstream.ReadBits(3))
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

		chkSize := 16
		sum := 0
		llr := uint(3)

		if alphabetSize <= 64 {
			chkSize = 8
		}

		for 1<<llr <= this.logRange {
			llr++
		}

		// Decode all frequencies (but the first one) by chunks
		for i := 1; i < alphabetSize; i += chkSize {
			// Read frequencies size for current chunk
			logMax := uint(1 + this.bitstream.ReadBits(llr))

			if 1<<logMax > scale {
				error := fmt.Errorf("Invalid bitstream: incorrect frequency size %v in ANS range decoder", logMax)
				return alphabetSize, error
			}

			endj := i + chkSize

			if endj > alphabetSize {
				endj = alphabetSize
			}

			// Read frequencies
			for j := i; j < endj; j++ {
				freq := int(this.bitstream.ReadBits(logMax))

				if freq <= 0 || freq >= scale {
					error := fmt.Errorf("Invalid bitstream: incorrect frequency %v for symbol '%v' in ANS range decoder", freq, alphabet[j])
					return alphabetSize, error
				}

				f[alphabet[j]] = freq
				sum += freq
			}
		}

		// Infer first frequency
		if scale <= sum {
			error := fmt.Errorf("Invalid bitstream: incorrect frequency %v for symbol '%v' in ANS range decoder", frequencies[alphabet[0]], this.alphabet[0])
			return alphabetSize, error
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

func (this *ANSRangeDecoder) Decode(block []byte) (int, error) {
	if block == nil {
		return 0, errors.New("Invalid null block parameter")
	}

	if len(block) == 0 {
		return 0, nil
	}

	end := len(block)
	startChunk := 0
	sizeChunk := this.chunkSize

	if sizeChunk == 0 {
		sizeChunk = len(block)
	}

	for i := range this.symbols {
		this.symbols[i] = DecSymbol{}
	}

	for startChunk < end {
		alphabetSize, err := this.decodeHeader(this.freqs)

		if err != nil || alphabetSize == 0 {
			return startChunk, err
		}

		endChunk := startChunk + sizeChunk

		if endChunk > end {
			endChunk = end
		}

		this.decodeChunk(block[startChunk:endChunk])
		startChunk = endChunk
	}

	return len(block), nil
}

func (this *ANSRangeDecoder) decodeChunk(block []byte) {
	// Read initial ANS state
	st := int(this.bitstream.ReadBits(32))
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
			for st < ANS_TOP {
				st = (st << 8) | int(this.bitstream.ReadBits(8))
			}
		}
	} else {
		symb := this.symbols
		prv := int(0)

		for i := range block {
			cur := this.f2s[(prv<<lr)+(st&mask)]
			block[i] = cur
			sym := symb[(prv<<8)+int(cur)]

			// Compute next ANS state
			// D(x) = (s, q_s (x/M) + mod(x,M) - b_s) where s is such b_s <= x mod M < b_{s+1}
			st = sym.freq*(st>>lr) + (st & mask) - sym.cumFreq

			// Normalize
			for st < ANS_TOP {
				st = (st << 8) | int(this.bitstream.ReadBits(8))
			}

			prv = int(cur)
		}
	}
}

func (this *ANSRangeDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

func (this *ANSRangeDecoder) Dispose() {
}

type DecSymbol struct {
	cumFreq int
	freq    int
}

func (this *DecSymbol) reset(cumFreq, freq int, logRange uint) {
	// Mirror encoder
	if freq >= 1<<logRange {
		freq = (1 << logRange) - 1
	}

	this.cumFreq = cumFreq
	this.freq = freq
}
