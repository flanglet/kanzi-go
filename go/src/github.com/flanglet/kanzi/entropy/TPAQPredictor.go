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
	kanzi "github.com/flanglet/kanzi"
)

// TPAQ predictor
// Derived from a heavily modified version of Tangelo 2.4 (by Jan Ondrus).
// PAQ8 is written by Matt Mahoney.
// See http://encode.ru/threads/1738-TANGELO-new-compressor-(derived-from-PAQ8-FP8)

const (
	TPAQ_MAX_LENGTH       = 88
	TPAQ_BUFFER_SIZE      = 64 * 1024 * 1024
	TPAQ_HASH_SIZE        = 64 * 1024 * 1024
	TPAQ_MASK_BUFFER      = TPAQ_BUFFER_SIZE - 1
	TPAQ_MASK_HASH        = TPAQ_HASH_SIZE - 1
	TPAQ_MASK_80808080    = int32(-2139062144) // 0x80808080
	TPAQ_MASK_F0F0F0F0    = int32(-252645136)  // 0xF0F0F0F0
	TPAQ_HASH             = int32(200002979)
	TPAQ_BEGIN_LEARN_RATE = 60 << 7
	TPAQ_END_LEARN_RATE   = 14 << 7
)

///////////////////////// state table ////////////////////////
// States represent a bit history within some context.
// State 0 is the starting state (no bits seen).
// States 1-30 represent all possible sequences of 1-4 bits.
// States 31-252 represent a pair of counts, (n0,n1), the number
//   of 0 and 1 bits respectively.  If n0+n1 < 16 then there are
//   two states for each pair, depending on if a 0 or 1 was the last
//   bit seen.
// If n0 and n1 are too large, then there is no state to represent this
// pair, so another state with about the same ratio of n0/n1 is substituted.
// Also, when a bit is observed and the count of the opposite bit is large,
// then part of this count is discarded to favor newer data over old.
var TPAQ_STATE_TRANSITIONS = [][]uint8{
	// Bit 0
	{
		1, 3, 143, 4, 5, 6, 7, 8, 9, 10,
		11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		21, 22, 23, 24, 25, 26, 27, 28, 29, 30,
		31, 32, 33, 34, 35, 36, 37, 38, 39, 40,
		41, 42, 43, 44, 45, 46, 47, 48, 49, 50,
		51, 52, 47, 54, 55, 56, 57, 58, 59, 60,
		61, 62, 63, 64, 65, 66, 67, 68, 69, 6,
		71, 71, 71, 61, 75, 56, 77, 78, 77, 80,
		81, 82, 83, 84, 85, 86, 87, 88, 77, 90,
		91, 92, 80, 94, 95, 96, 97, 98, 99, 90,
		101, 94, 103, 101, 102, 104, 107, 104, 105, 108,
		111, 112, 113, 114, 115, 116, 92, 118, 94, 103,
		119, 122, 123, 94, 113, 126, 113, 128, 129, 114,
		131, 132, 112, 134, 111, 134, 110, 134, 134, 128,
		128, 142, 143, 115, 113, 142, 128, 148, 149, 79,
		148, 142, 148, 150, 155, 149, 157, 149, 159, 149,
		131, 101, 98, 115, 114, 91, 79, 58, 1, 170,
		129, 128, 110, 174, 128, 176, 129, 174, 179, 174,
		176, 141, 157, 179, 185, 157, 187, 188, 168, 151,
		191, 192, 188, 187, 172, 175, 170, 152, 185, 170,
		176, 170, 203, 148, 185, 203, 185, 192, 209, 188,
		211, 192, 213, 214, 188, 216, 168, 84, 54, 54,
		221, 54, 55, 85, 69, 63, 56, 86, 58, 230,
		231, 57, 229, 56, 224, 54, 54, 66, 58, 54,
		61, 57, 222, 78, 85, 82, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0,
	},
	// Bit 1
	{
		2, 163, 169, 163, 165, 89, 245, 217, 245, 245,
		233, 244, 227, 74, 221, 221, 218, 226, 243, 218,
		238, 242, 74, 238, 241, 240, 239, 224, 225, 221,
		232, 72, 224, 228, 223, 225, 238, 73, 167, 76,
		237, 234, 231, 72, 31, 63, 225, 237, 236, 235,
		53, 234, 53, 234, 229, 219, 229, 233, 232, 228,
		226, 72, 74, 222, 75, 220, 167, 57, 218, 70,
		168, 72, 73, 74, 217, 76, 167, 79, 79, 166,
		162, 162, 162, 162, 165, 89, 89, 165, 89, 162,
		93, 93, 93, 161, 100, 93, 93, 93, 93, 93,
		161, 102, 120, 104, 105, 106, 108, 106, 109, 110,
		160, 134, 108, 108, 126, 117, 117, 121, 119, 120,
		107, 124, 117, 117, 125, 127, 124, 139, 130, 124,
		133, 109, 110, 135, 110, 136, 137, 138, 127, 140,
		141, 145, 144, 124, 125, 146, 147, 151, 125, 150,
		127, 152, 153, 154, 156, 139, 158, 139, 156, 139,
		130, 117, 163, 164, 141, 163, 147, 2, 2, 199,
		171, 172, 173, 177, 175, 171, 171, 178, 180, 172,
		181, 182, 183, 184, 186, 178, 189, 181, 181, 190,
		193, 182, 182, 194, 195, 196, 197, 198, 169, 200,
		201, 202, 204, 180, 205, 206, 207, 208, 210, 194,
		212, 184, 215, 193, 184, 208, 193, 163, 219, 168,
		94, 217, 223, 224, 225, 76, 227, 217, 229, 219,
		79, 86, 165, 217, 214, 225, 216, 216, 234, 75,
		214, 237, 74, 74, 163, 217, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0,
	},
}

var TPAQ_STATE_MAP = []int32{
	-10, -436, 401, -521, -623, -689, -736, -812, -812, -900,
	-865, -891, -1006, -965, -981, -916, -946, -976, -1072, -1014,
	-1058, -1090, -1044, -1030, -1044, -1104, -1009, -1418, -1131, -1131,
	-1269, -1332, -1191, -1169, -1108, -1378, -1367, -1126, -1297, -1085,
	-1355, -1344, -1169, -1269, -1440, -1262, -1332, -2047, -2047, -1984,
	-2047, -2047, -2047, -225, -402, -556, -502, -746, -609, -647,
	-625, -718, -700, -805, -748, -935, -838, -1053, -787, -806,
	-269, -1006, -278, -212, -41, -399, 137, -984, -998, -219,
	-455, -524, -556, -564, -577, -592, -610, -690, -650, -140,
	-396, -471, -450, -168, -215, -301, -325, -364, -315, -401,
	-96, -174, -102, -146, -61, -9, 54, 81, 116, 140,
	192, 115, -41, -93, -183, -277, -365, 104, -134, 37,
	-80, 181, -111, -184, 194, 317, 63, 394, 105, -92,
	299, 166, -17, 333, 131, 386, 403, 450, 499, 480,
	493, 504, 89, -119, 333, 558, 568, 501, -7, -151,
	203, 557, 595, 603, 650, 104, 960, 204, 933, 239,
	247, -12, -105, 94, 222, -139, 40, 168, -203, 566,
	-53, 243, 344, 542, 42, 208, 14, 474, 529, 82,
	513, 504, 570, 616, 644, 92, 669, 91, -179, 677,
	720, 157, -10, 687, 672, 750, 686, 830, 787, 683,
	723, 780, 783, 9, 842, 816, 885, 901, 1368, 188,
	1356, 178, 1419, 173, -22, 1256, 240, 167, 1, -31,
	-165, 70, -493, -45, -354, -25, -142, 98, -17, -158,
	-355, -448, -142, -67, -76, -310, -324, -225, -96, 0,
	46, -72, 0, -439, 14, -55, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1,
}

func hashTPAQ(x, y int32) int32 {
	h := x*TPAQ_HASH ^ y*TPAQ_HASH
	return h>>1 ^ h>>9 ^ x>>2 ^ y>>3 ^ TPAQ_HASH
}

type TPAQPredictor struct {
	pr              int   // next predicted value (0-4095)
	c0              int32 // bitwise context: last 0-7 bits with a leading 1 (1-255)
	c4              int32 // last 4 whole bytes, last is in low 8 bits
	c8              int32 // last 8 to 4 whole bytes, last is in low 8 bits
	bpos            uint  // number of bits in c0 (0-7)
	pos             int32
	binCount        int32
	matchLen        int32
	matchPos        int32
	hash            int32
	statesMask      int32
	mixersMask      int32
	sse             *LogisticAdaptiveProbMap
	mixers          []TPAQMixer
	mixer           *TPAQMixer // current mixer
	buffer          []int8
	hashes          []int32 // hash table(context, buffer position)
	bigStatesMap    []uint8 // hash table(context, prediction)
	smallStatesMap0 []uint8 // hash table(context, prediction)
	smallStatesMap1 []uint8 // hash table(context, prediction)
	cp0             *uint8  // context pointers
	cp1             *uint8
	cp2             *uint8
	cp3             *uint8
	cp4             *uint8
	cp5             *uint8
	cp6             *uint8
	ctx0            int32 // contexts
	ctx1            int32
	ctx2            int32
	ctx3            int32
	ctx4            int32
	ctx5            int32
	ctx6            int32
}

func NewTPAQPredictor(ctx *map[string]interface{}) (*TPAQPredictor, error) {
	this := new(TPAQPredictor)
	statesSize := 1 << 28
	mixersSize := 1 << 12

	if ctx != nil {
		// Block size requested by the user
		// The user can request a big block size to force more states
		rbsz := (*ctx)["blockSize"].(uint)

		if rbsz >= 64*1024*1024 {
			statesSize = 1 << 29
		} else if rbsz >= 16*1024*1024 {
			statesSize = 1 << 28
		} else if rbsz >= 1024*1024 {
			statesSize = 1 << 27
		} else {
			statesSize = 1 << 26
		}

		// Actual size of the current block
		// Too many mixers hurts compression for small blocks.
		// Too few mixers hurts compression for big blocks.
		absz := (*ctx)["size"].(uint)

		if absz >= 8*1024*1024 {
			mixersSize = 1 << 15
		} else if absz >= 4*1024*1024 {
			mixersSize = 1 << 12
		} else if absz >= 1024*1024 {
			mixersSize = 1 << 10
		} else {
			mixersSize = 1 << 9
		}
	}

	this.mixers = make([]TPAQMixer, mixersSize)

	for i := range this.mixers {
		this.mixers[i].init()
	}

	this.mixer = &this.mixers[0]
	this.pr = 2048
	this.c0 = 1
	this.bigStatesMap = make([]uint8, statesSize)
	this.smallStatesMap0 = make([]uint8, 1<<16)
	this.smallStatesMap1 = make([]uint8, 1<<24)
	this.hashes = make([]int32, TPAQ_HASH_SIZE)
	this.buffer = make([]int8, TPAQ_BUFFER_SIZE)
	this.statesMask = int32(statesSize - 1)
	this.mixersMask = int32(mixersSize - 1)
	this.cp0 = &this.smallStatesMap0[0]
	this.cp1 = &this.smallStatesMap1[0]
	this.cp2 = &this.bigStatesMap[0]
	this.cp3 = &this.bigStatesMap[0]
	this.cp4 = &this.bigStatesMap[0]
	this.cp5 = &this.bigStatesMap[0]
	this.cp6 = &this.bigStatesMap[0]

	var err error
	this.sse, err = newLogisticAdaptiveProbMap(65536, 7)
	return this, err
}

// Update the probability model
func (this *TPAQPredictor) Update(bit byte) {
	y := int(bit)
	this.mixer.update(y)
	this.bpos++
	this.c0 = (this.c0 << 1) | int32(bit)

	if this.c0 > 255 {
		this.buffer[this.pos&TPAQ_MASK_BUFFER] = int8(this.c0)
		this.pos++
		this.c8 = (this.c8 << 8) | ((this.c4 >> 24) & 0xFF)
		this.c4 = (this.c4 << 8) | (this.c0 & 0xFF)
		this.hash = (((this.hash * 43707) << 4) + this.c4) & TPAQ_MASK_HASH
		this.c0 = 1
		this.bpos = 0
		this.binCount += ((this.c4 >> 7) & 1)

		// Select Neural Net
		this.mixer = &this.mixers[this.c4&this.mixersMask]

		// Add contexts to NN
		this.ctx0 = (this.c4 & 0xFF) << 8
		this.ctx1 = (this.c4 & 0xFFFF) << 8
		this.ctx2 = createContext(2, this.c4&0x00FFFFFF)
		this.ctx3 = createContext(3, this.c4)

		if this.binCount < this.pos>>2 {
			// Mostly text or mixed
			var h1, h2 int32

			if this.c4&TPAQ_MASK_80808080 == 0 {
				h1 = this.c4
			} else {
				h1 = this.c4 >> 16
			}

			if this.c8&TPAQ_MASK_80808080 == 0 {
				h2 = this.c8
			} else {
				h2 = this.c8 >> 16
			}

			this.ctx4 = createContext(4, this.c4^(this.c8&0xFFFF))
			this.ctx5 = createContext(5, hashTPAQ(h1, h2))
			this.ctx6 = hashTPAQ(TPAQ_HASH, this.c4&TPAQ_MASK_F0F0F0F0)
		} else {
			// Mostly binary
			this.ctx4 = createContext(4, this.c4^(this.c4&0xFFFF))
			this.ctx5 = createContext(5, hashTPAQ(this.c4>>16, this.c8>>16))
			this.ctx6 = ((this.c4 & 0xFF) << 8) | ((this.c8 & 0xFFFF) << 16)
		}

		// Find match
		this.findMatch()

		// Keep track of current position
		this.hashes[this.hash] = this.pos
	}

	// Get initial predictions
	c := this.c0
	table := TPAQ_STATE_TRANSITIONS[bit]
	*this.cp0 = table[*this.cp0]
	this.cp0 = &this.smallStatesMap0[this.ctx0+c]
	p0 := TPAQ_STATE_MAP[*this.cp0]
	*this.cp1 = table[*this.cp1]
	this.cp1 = &this.smallStatesMap1[this.ctx1+c]
	p1 := TPAQ_STATE_MAP[*this.cp1]
	*this.cp2 = table[*this.cp2]
	this.cp2 = &this.bigStatesMap[(this.ctx2+c)&this.statesMask]
	p2 := TPAQ_STATE_MAP[*this.cp2]
	*this.cp3 = table[*this.cp3]
	this.cp3 = &this.bigStatesMap[(this.ctx3+c)&this.statesMask]
	p3 := TPAQ_STATE_MAP[*this.cp3]
	*this.cp4 = table[*this.cp4]
	this.cp4 = &this.bigStatesMap[(this.ctx4+c)&this.statesMask]
	p4 := TPAQ_STATE_MAP[*this.cp4]
	*this.cp5 = table[*this.cp5]
	this.cp5 = &this.bigStatesMap[(this.ctx5+c)&this.statesMask]
	p5 := TPAQ_STATE_MAP[*this.cp5]
	*this.cp6 = table[*this.cp6]
	this.cp6 = &this.bigStatesMap[(this.ctx6+c)&this.statesMask]
	p6 := TPAQ_STATE_MAP[*this.cp6]

	p7 := this.getMatchContextPred()

	// Mix predictions using NN
	p := this.mixer.get(p0, p1, p2, p3, p4, p5, p6, p7)

	// SSE (Secondary Symbol Estimation)
	p = this.sse.get(y, p, int(c|(this.c4&0xFF00)))

	p32 := uint32(p)
	this.pr = p + int((p32-2048)>>31)
}

// Return the split value representing the probability of 1 in the [0..4095] range.
func (this *TPAQPredictor) Get() int {
	return this.pr
}

func (this *TPAQPredictor) findMatch() {
	// Update ongoing sequence match or detect match in the buffer (LZ like)
	if this.matchLen > 0 {
		if this.matchLen < TPAQ_MAX_LENGTH {
			this.matchLen++
		}

		this.matchPos++
	} else {
		// Retrieve match position
		this.matchPos = this.hashes[this.hash]

		// Detect match
		if this.matchPos != 0 && this.pos-this.matchPos <= TPAQ_MASK_BUFFER {
			r := this.matchLen + 1

			for r <= TPAQ_MAX_LENGTH && this.buffer[(this.pos-r)&TPAQ_MASK_BUFFER] == this.buffer[(this.matchPos-r)&TPAQ_MASK_BUFFER] {
				r++
			}

			this.matchLen = r - 1
		}
	}
}

// Get a prediction from the match model in [-2047..2048]
func (this *TPAQPredictor) getMatchContextPred() int32 {
	p := int32(0)

	if this.matchLen > 0 {
		if this.c0 == ((int32(this.buffer[this.matchPos&TPAQ_MASK_BUFFER])&0xFF)|256)>>(8-this.bpos) {
			// Add match length to NN inputs. Compute input based on run length

			if this.matchLen <= 24 {
				p = this.matchLen
			} else {
				p = (24 + ((this.matchLen - 24) >> 3))
			}

			if ((this.buffer[this.matchPos&TPAQ_MASK_BUFFER] >> (7 - this.bpos)) & 1) == 0 {
				p = -p
			}

			p <<= 6
		} else {
			this.matchLen = 0
		}
	}

	return p
}

func createContext(ctxId, cx int32) int32 {
	cx = cx*987654323 + ctxId
	cx = (cx << 16) | int32(uint32(cx)>>16)
	return cx*123456791 + ctxId
}

// Mixer combines models using neural networks with 8 inputs.
type TPAQMixer struct {
	pr                             int // squashed prediction
	skew                           int32
	w0, w1, w2, w3, w4, w5, w6, w7 int32
	p0, p1, p2, p3, p4, p5, p6, p7 int32
	learnRate                      int32
}

func (this *TPAQMixer) init() {
	this.pr = 2048
	this.skew = 0
	this.w0 = 2048
	this.w1 = 2048
	this.w2 = 2048
	this.w3 = 2048
	this.w4 = 2048
	this.w5 = 2048
	this.w6 = 2048
	this.w7 = 2048
	this.learnRate = TPAQ_BEGIN_LEARN_RATE
}

// Adjust weights to minimize coding cost of last prediction
func (this *TPAQMixer) update(bit int) {
	err := int32((bit << 12) - this.pr)

	if err == 0 {
		return
	}

	// Quickly decaying learn rate
	err = (err * this.learnRate) >> 7
	this.learnRate += ((TPAQ_END_LEARN_RATE - this.learnRate) >> 31)
	this.skew += err

	// Train Neural Network: update weights
	this.w0 += ((this.p0*err + 0) >> 15)
	this.w1 += ((this.p1*err + 0) >> 15)
	this.w2 += ((this.p2*err + 0) >> 15)
	this.w3 += ((this.p3*err + 0) >> 15)
	this.w4 += ((this.p4*err + 0) >> 15)
	this.w5 += ((this.p5*err + 0) >> 15)
	this.w6 += ((this.p6*err + 0) >> 15)
	this.w7 += ((this.p7*err + 0) >> 15)
}

func (this *TPAQMixer) get(p0, p1, p2, p3, p4, p5, p6, p7 int32) int {
	this.p0 = p0
	this.p1 = p1
	this.p2 = p2
	this.p3 = p3
	this.p4 = p4
	this.p5 = p5
	this.p6 = p6
	this.p7 = p7

	// Neural Network dot product (sum weights*inputs)
	p := this.w0*p0 + this.w1*p1 + this.w2*p2 + this.w3*p3 +
		this.w4*p4 + this.w5*p5 + this.w6*p6 + this.w7*p7 +
		this.skew

	this.pr = kanzi.Squash(int((p + 65536) >> 17))
	return this.pr
}
