/*
Copyright 2011-2026 Frederic Langlet
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

	kanzi "github.com/flanglet/kanzi-go/v2"
)

var _EXPG_VALUES = [2][256]uint{
	// Unsigned
	{
		513, 1538, 1539, 2564, 2565, 2566, 2567, 3592, 3593, 3594, 3595, 3596, 3597, 3598, 3599, 4624,
		4625, 4626, 4627, 4628, 4629, 4630, 4631, 4632, 4633, 4634, 4635, 4636, 4637, 4638, 4639, 5664,
		5665, 5666, 5667, 5668, 5669, 5670, 5671, 5672, 5673, 5674, 5675, 5676, 5677, 5678, 5679, 5680,
		5681, 5682, 5683, 5684, 5685, 5686, 5687, 5688, 5689, 5690, 5691, 5692, 5693, 5694, 5695, 6720,
		6721, 6722, 6723, 6724, 6725, 6726, 6727, 6728, 6729, 6730, 6731, 6732, 6733, 6734, 6735, 6736,
		6737, 6738, 6739, 6740, 6741, 6742, 6743, 6744, 6745, 6746, 6747, 6748, 6749, 6750, 6751, 6752,
		6753, 6754, 6755, 6756, 6757, 6758, 6759, 6760, 6761, 6762, 6763, 6764, 6765, 6766, 6767, 6768,
		6769, 6770, 6771, 6772, 6773, 6774, 6775, 6776, 6777, 6778, 6779, 6780, 6781, 6782, 6783, 7808,
		7809, 7810, 7811, 7812, 7813, 7814, 7815, 7816, 7817, 7818, 7819, 7820, 7821, 7822, 7823, 7824,
		7825, 7826, 7827, 7828, 7829, 7830, 7831, 7832, 7833, 7834, 7835, 7836, 7837, 7838, 7839, 7840,
		7841, 7842, 7843, 7844, 7845, 7846, 7847, 7848, 7849, 7850, 7851, 7852, 7853, 7854, 7855, 7856,
		7857, 7858, 7859, 7860, 7861, 7862, 7863, 7864, 7865, 7866, 7867, 7868, 7869, 7870, 7871, 7872,
		7873, 7874, 7875, 7876, 7877, 7878, 7879, 7880, 7881, 7882, 7883, 7884, 7885, 7886, 7887, 7888,
		7889, 7890, 7891, 7892, 7893, 7894, 7895, 7896, 7897, 7898, 7899, 7900, 7901, 7902, 7903, 7904,
		7905, 7906, 7907, 7908, 7909, 7910, 7911, 7912, 7913, 7914, 7915, 7916, 7917, 7918, 7919, 7920,
		7921, 7922, 7923, 7924, 7925, 7926, 7927, 7928, 7929, 7930, 7931, 7932, 7933, 7934, 7935, 8960,
	},
	// Signed
	{
		513, 2052, 2054, 3080, 3082, 3084, 3086, 4112, 4114, 4116, 4118, 4120, 4122, 4124, 4126, 5152,
		5154, 5156, 5158, 5160, 5162, 5164, 5166, 5168, 5170, 5172, 5174, 5176, 5178, 5180, 5182, 6208,
		6210, 6212, 6214, 6216, 6218, 6220, 6222, 6224, 6226, 6228, 6230, 6232, 6234, 6236, 6238, 6240,
		6242, 6244, 6246, 6248, 6250, 6252, 6254, 6256, 6258, 6260, 6262, 6264, 6266, 6268, 6270, 7296,
		7298, 7300, 7302, 7304, 7306, 7308, 7310, 7312, 7314, 7316, 7318, 7320, 7322, 7324, 7326, 7328,
		7330, 7332, 7334, 7336, 7338, 7340, 7342, 7344, 7346, 7348, 7350, 7352, 7354, 7356, 7358, 7360,
		7362, 7364, 7366, 7368, 7370, 7372, 7374, 7376, 7378, 7380, 7382, 7384, 7386, 7388, 7390, 7392,
		7394, 7396, 7398, 7400, 7402, 7404, 7406, 7408, 7410, 7412, 7414, 7416, 7418, 7420, 7422, 8448,
		8451, 8449, 7423, 7421, 7419, 7417, 7415, 7413, 7411, 7409, 7407, 7405, 7403, 7401, 7399, 7397,
		7395, 7393, 7391, 7389, 7387, 7385, 7383, 7381, 7379, 7377, 7375, 7373, 7371, 7369, 7367, 7365,
		7363, 7361, 7359, 7357, 7355, 7353, 7351, 7349, 7347, 7345, 7343, 7341, 7339, 7337, 7335, 7333,
		7331, 7329, 7327, 7325, 7323, 7321, 7319, 7317, 7315, 7313, 7311, 7309, 7307, 7305, 7303, 7301,
		7299, 7297, 6271, 6269, 6267, 6265, 6263, 6261, 6259, 6257, 6255, 6253, 6251, 6249, 6247, 6245,
		6243, 6241, 6239, 6237, 6235, 6233, 6231, 6229, 6227, 6225, 6223, 6221, 6219, 6217, 6215, 6213,
		6211, 6209, 5183, 5181, 5179, 5177, 5175, 5173, 5171, 5169, 5167, 5165, 5163, 5161, 5159, 5157,
		5155, 5153, 4127, 4125, 4123, 4121, 4119, 4117, 4115, 4113, 3087, 3085, 3083, 3081, 2055, 2053,
	},
}

// ExpGolombEncoder Exponential Golomb Entropy Encoder
type ExpGolombEncoder struct {
	signed    bool
	cache     []uint
	bitstream kanzi.OutputBitStream
}

// NewExpGolombEncoder creates a new instance of ExpGolombEncoder
// If sgn is true, values will be encoded as signed (int8) in the bitstream.
// Using a sign improves compression ratio for distributions centered on 0 (E.G. Gaussian)
// Example: -1 is better compressed as -1 (1 followed by '-') than as 255
func NewExpGolombEncoder(bs kanzi.OutputBitStream, sgn bool) (*ExpGolombEncoder, error) {
	if bs == nil {
		return nil, errors.New("ExpGolomb codec: Invalid null bitstream parameter")
	}

	this := &ExpGolombEncoder{}
	this.bitstream = bs
	this.signed = sgn

	if sgn == true {
		this.cache = _EXPG_VALUES[1][:]
	} else {
		this.cache = _EXPG_VALUES[0][:]
	}

	return this, nil
}

// Signed returns true if this encoder is sign aware
func (this *ExpGolombEncoder) Signed() bool {
	return this.signed
}

// Dispose this implementation does nothing
func (this *ExpGolombEncoder) Dispose() {
}

// EncodeByte encodes the given value into the bitstream
func (this *ExpGolombEncoder) EncodeByte(val byte) {
	if val == 0 {
		this.bitstream.WriteBit(1)
		return
	}

	emit := this.cache[val]
	this.bitstream.WriteBits(uint64(emit&0x1FF), emit>>9)
}

// BitStream returns the underlying bitstream
func (this *ExpGolombEncoder) BitStream() kanzi.OutputBitStream {
	return this.bitstream
}

// Write encodes the data provided into the bitstream. Return the number of byte
// written to the bitstream
func (this *ExpGolombEncoder) Write(block []byte) (int, error) {
	for i := range block {
		this.EncodeByte(block[i])
	}

	return len(block), nil
}

// ExpGolombDecoder Exponential Golomb Entropy Decoder
type ExpGolombDecoder struct {
	signed    bool
	bitstream kanzi.InputBitStream
}

// NewExpGolombDecoder creates a new instance of ExpGolombDecoder
// If sgn is true, values from the bitstream will be decoded as signed (int8)
func NewExpGolombDecoder(bs kanzi.InputBitStream, sgn bool) (*ExpGolombDecoder, error) {
	if bs == nil {
		return nil, errors.New("ExpGolomb codec: Invalid null bitstream parameter")
	}

	this := &ExpGolombDecoder{}
	this.signed = sgn
	this.bitstream = bs
	return this, nil
}

// Signed returns true if this decoder is sign aware
func (this *ExpGolombDecoder) Signed() bool {
	return this.signed
}

// Dispose this implementation does nothing
func (this *ExpGolombDecoder) Dispose() {
}

// DecodeByte decodes one byte from the bitstream
// If the decoder is sign aware, the returned value is an int8 cast to a byte
func (this *ExpGolombDecoder) DecodeByte() byte {
	if this.bitstream.ReadBit() == 1 {
		return 0
	}

	log2 := uint(1)

	for {
		if this.bitstream.ReadBit() == 1 {
			break
		}

		log2++
	}

	// Clamp. Do not attempt to detect a corrupted bitstream.
	// Unsigned byte 255 requires an 8-bit suffix; signed values require at most 7.
	maxLog2 := uint(8)

	if this.signed {
		maxLog2--
	}

	if log2 > maxLog2 {
		log2 = maxLog2
	}

	if this.signed == true {
		// Decode signed: read value + sign
		val := this.bitstream.ReadBits(log2 + 1)
		res := val>>1 + 1<<log2 - 1

		if val&1 == 1 {
			res = ^res + 1
		}

		return byte(res)
	}

	// Decode unsigned
	val := this.bitstream.ReadBits(log2)
	return byte((1 << log2) - 1 + val)
}

// BitStream returns the underlying bitstream
func (this *ExpGolombDecoder) BitStream() kanzi.InputBitStream {
	return this.bitstream
}

// Read decodes data from the bitstream and return it in the provided buffer.
// Return the number of bytes read from the bitstream
func (this *ExpGolombDecoder) Read(block []byte) (int, error) {
	for i := range block {
		block[i] = this.DecodeByte()
	}

	return len(block), nil
}
