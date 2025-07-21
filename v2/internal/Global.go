/*
Copyright 2011-2025 Frederic Langlet
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

package internal

import (
	"errors"
)

// DataType captures the type of input data
type DataType int

const (
	DT_UNDEFINED      DataType = 0
	DT_TEXT           DataType = 1
	DT_MULTIMEDIA     DataType = 2
	DT_EXE            DataType = 3
	DT_NUMERIC        DataType = 4
	DT_BASE64         DataType = 5
	DT_DNA            DataType = 6
	DT_BIN            DataType = 7
	DT_UTF8           DataType = 8
	DT_SMALL_ALPHABET DataType = 9
)

var (
	// LOG2 is an array with 256 elements: int(Math.log2(x-1))
	LOG2 = [...]uint32{
		0, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, 3, 4,
		4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 5,
		5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5,
		5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 6,
		6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
		6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
		6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
		6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 7,
		7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
		7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
		7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
		7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
		7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
		7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
		7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
		7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 8,
	}

	// LOG2_4096 is an array with 256 elements: 4096*Math.log2(x)
	LOG2_4096 = [...]uint32{
		0, 0, 4096, 6492, 8192, 9511, 10588, 11499, 12288, 12984,
		13607, 14170, 14684, 15157, 15595, 16003, 16384, 16742, 17080, 17400,
		17703, 17991, 18266, 18529, 18780, 19021, 19253, 19476, 19691, 19898,
		20099, 20292, 20480, 20662, 20838, 21010, 21176, 21338, 21496, 21649,
		21799, 21945, 22087, 22226, 22362, 22495, 22625, 22752, 22876, 22998,
		23117, 23234, 23349, 23462, 23572, 23680, 23787, 23892, 23994, 24095,
		24195, 24292, 24388, 24483, 24576, 24668, 24758, 24847, 24934, 25021,
		25106, 25189, 25272, 25354, 25434, 25513, 25592, 25669, 25745, 25820,
		25895, 25968, 26041, 26112, 26183, 26253, 26322, 26390, 26458, 26525,
		26591, 26656, 26721, 26784, 26848, 26910, 26972, 27033, 27094, 27154,
		27213, 27272, 27330, 27388, 27445, 27502, 27558, 27613, 27668, 27722,
		27776, 27830, 27883, 27935, 27988, 28039, 28090, 28141, 28191, 28241,
		28291, 28340, 28388, 28437, 28484, 28532, 28579, 28626, 28672, 28718,
		28764, 28809, 28854, 28898, 28943, 28987, 29030, 29074, 29117, 29159,
		29202, 29244, 29285, 29327, 29368, 29409, 29450, 29490, 29530, 29570,
		29609, 29649, 29688, 29726, 29765, 29803, 29841, 29879, 29916, 29954,
		29991, 30027, 30064, 30100, 30137, 30172, 30208, 30244, 30279, 30314,
		30349, 30384, 30418, 30452, 30486, 30520, 30554, 30587, 30621, 30654,
		30687, 30719, 30752, 30784, 30817, 30849, 30880, 30912, 30944, 30975,
		31006, 31037, 31068, 31099, 31129, 31160, 31190, 31220, 31250, 31280,
		31309, 31339, 31368, 31397, 31426, 31455, 31484, 31513, 31541, 31569,
		31598, 31626, 31654, 31681, 31709, 31737, 31764, 31791, 31818, 31846,
		31872, 31899, 31926, 31952, 31979, 32005, 32031, 32058, 32084, 32109,
		32135, 32161, 32186, 32212, 32237, 32262, 32287, 32312, 32337, 32362,
		32387, 32411, 32436, 32460, 32484, 32508, 32533, 32557, 32580, 32604,
		32628, 32651, 32675, 32698, 32722, 32745, 32768,
	}

	// 65536 /(1 + exp(-alpha*x)) with alpha ~= 0.54
	_INV_EXP = [33]int{
		0, 8, 22, 47, 88, 160, 283, 492,
		848, 1451, 2459, 4117, 6766, 10819, 16608, 24127,
		32768, 41409, 48928, 54717, 58770, 61419, 63077, 64085,
		64688, 65044, 65253, 65376, 65448, 65489, 65514, 65528,
		65536,
	}

	_BASE64_SYMBOLS  = []byte(`ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/`)
	_NUMERIC_SYMBOLS = []byte(`0123456789+-*/=,.:; `)
	_DNA_SYMBOLS     = []byte(`acgntuACGNTU"`) // either T or U and N for unknown

	// SQUASH contains p = 1/(1 + exp(-d)), d scaled by 8 bits, p scaled by 12 bits
	SQUASH [4096]int

	// STRETCH is the inverse of squash. d = ln(p/(1-p)), d scaled by 8 bits, p by 12 bits.
	// d has range -2047 to 2047 representing -8 to 8. p in [0..4095].
	STRETCH [4096]int
)

func init() {
	// Init squash
	for x := -2047; x <= 2047; x++ {
		w := x & 127
		y := (x >> 7) + 16
		SQUASH[x+2047] = (_INV_EXP[y]*(128-w) + _INV_EXP[y+1]*w) >> 11
	}

	SQUASH[4095] = 4095
	pi := 0

	for x := -2047; x <= 2047; x++ {
		i := Squash(x)

		for pi <= i {
			STRETCH[pi] = x
			pi++
		}
	}

	STRETCH[4095] = 2047
}

// Squash returns p = 1/(1 + exp(-d)), d scaled by 8 bits, p scaled by 12 bits
func Squash(d int) int {
	if d >= 2048 {
		return 4095
	}

	if d <= -2048 {
		return 0
	}

	return SQUASH[d+2047]
}

// Log2 returns a fast, integer rounded value for log2(x)
func Log2(x uint32) (uint32, error) {
	if x == 0 {
		return 0, errors.New("Cannot calculate log of a negative or null value")
	}

	return Log2NoCheck(x), nil
}

// Log2NoCheck does the same as Log2() minus a null check on input value
func Log2NoCheck(x uint32) uint32 {
	var res uint32

	if x >= 1<<16 {
		x >>= 16
		res = 16
	} else {
		res = 0
	}

	if x >= 1<<8 {
		x >>= 8
		res += 8
	}

	return res + LOG2[x-1]
}

// Log2ScaledBy1024 returns 1024 * log2(x). Max error is around 0.1%
func Log2ScaledBy1024(x uint32) (uint32, error) {
	if x == 0 {
		return 0, errors.New("Cannot calculate log of a negative or null value")
	}

	if x < 256 {
		return (LOG2_4096[x] + 2) >> 2, nil
	}

	log := Log2NoCheck(x)

	if x&(x-1) == 0 {
		return log << 10, nil
	}

	return ((log - 7) * 1024) + ((LOG2_4096[x>>(log-7)] + 2) >> 2), nil
}

// ComputeFirstOrderEntropy1024 computes the order 0 entropy of the block
// and scales the result by 1024 (result in the [0..1024] range)
// Fills in the histogram with order 0 frequencies. Incoming array size must be at least 256
func ComputeFirstOrderEntropy1024(blockLen int, histo []int) int {
	if blockLen == 0 {
		return 0
	}

	sum := uint64(0)
	logLength1024, _ := Log2ScaledBy1024(uint32(blockLen))

	for i := 0; i < 256; i++ {
		if histo[i] == 0 {
			continue
		}

		log1024, _ := Log2ScaledBy1024(uint32(histo[i]))
		sum += ((uint64(histo[i]) * uint64(logLength1024-log1024)) >> 3)
	}

	return int(sum / uint64(blockLen))
}

// ComputeHistogram computes the order 0 or order 1 histogram for the input block
// and returns it in the 'freqs' slice.
// If withTotal is true, the last spot in each frequencies order 0 array is for the total
// (each order 0 frequency slice must be of length 257 in this case).
func ComputeHistogram(block []byte, freqs []int, isOrder0, withTotal bool) {
	if isOrder0 == true {
		if withTotal == true {
			freqs[256] = len(block)
		}

		end16 := len(block) & -16

		for i := 0; i < end16; {
			d := block[i : i+16]
			freqs[d[0]]++
			freqs[d[1]]++
			freqs[d[2]]++
			freqs[d[3]]++
			freqs[d[4]]++
			freqs[d[5]]++
			freqs[d[6]]++
			freqs[d[7]]++
			freqs[d[8]]++
			freqs[d[9]]++
			freqs[d[10]]++
			freqs[d[11]]++
			freqs[d[12]]++
			freqs[d[13]]++
			freqs[d[14]]++
			freqs[d[15]]++
			i += 16
		}

		for i := end16; i < len(block); i++ {
			freqs[block[i]]++
		}
	} else { // Order 1
		length := len(block)
		quarter := length >> 2
		n0 := 0 * quarter
		n1 := 1 * quarter
		n2 := 2 * quarter
		n3 := 3 * quarter

		if withTotal == true {
			if length < 32 {
				prv := uint(0)

				for i := 0; i < length; i++ {
					freqs[prv+uint(block[i])]++
					freqs[prv+256]++
					prv = 257 * uint(block[i])
				}
			} else {
				prv0 := uint(0)
				prv1 := 257 * uint(block[n1-1])
				prv2 := 257 * uint(block[n2-1])
				prv3 := 257 * uint(block[n3-1])

				for n0 < quarter {
					cur0 := uint(block[n0])
					cur1 := uint(block[n1])
					cur2 := uint(block[n2])
					cur3 := uint(block[n3])
					freqs[prv0+cur0]++
					freqs[prv0+256]++
					freqs[prv1+cur1]++
					freqs[prv1+256]++
					freqs[prv2+cur2]++
					freqs[prv2+256]++
					freqs[prv3+cur3]++
					freqs[prv3+256]++
					prv0 = 257 * cur0
					prv1 = 257 * cur1
					prv2 = 257 * cur2
					prv3 = 257 * cur3
					n0++
					n1++
					n2++
					n3++
				}

				for ; n3 < length; n3++ {
					freqs[prv3+uint(block[n3])]++
					freqs[prv3+256]++
					prv3 = 257 * uint(block[n3])
				}
			}
		} else { // order 1, no total
			if length < 32 {
				prv := uint(0)

				for i := 0; i < length; i++ {
					freqs[prv+uint(block[i])]++
					prv = 256 * uint(block[i])
				}
			} else {
				prv0 := uint(0)
				prv1 := 256 * uint(block[n1-1])
				prv2 := 256 * uint(block[n2-1])
				prv3 := 256 * uint(block[n3-1])

				for n0 < quarter {
					cur0 := uint(block[n0])
					cur1 := uint(block[n1])
					cur2 := uint(block[n2])
					cur3 := uint(block[n3])
					freqs[prv0+cur0]++
					freqs[prv1+cur1]++
					freqs[prv2+cur2]++
					freqs[prv3+cur3]++
					prv0 = cur0 << 8
					prv1 = cur1 << 8
					prv2 = cur2 << 8
					prv3 = cur3 << 8
					n0++
					n1++
					n2++
					n3++
				}

				for ; n3 < length; n3++ {
					freqs[prv3+uint(block[n3])]++
					prv3 = uint(block[n3]) << 8
				}
			}
		}
	}
}

func DetectSimpleType(count int, freqs0 []int) DataType {
	if count == 0 {
		return DT_UNDEFINED
	}

	sum := 0

	for i := 0; i < 12; i++ {
		sum += freqs0[_DNA_SYMBOLS[i]]
	}

	if sum > count-count/12 {
		return DT_DNA
	}

	sum = 0

	for i := 0; i < 20; i++ {
		sum += freqs0[_NUMERIC_SYMBOLS[i]]
	}

	if sum == count {
		return DT_NUMERIC
	}

	sum = 0

	for i := 0; i < 64; i++ {
		sum += freqs0[_BASE64_SYMBOLS[i]]
	}

	if sum+freqs0[0x3D] == count {
		return DT_BASE64
	}

	sum = 0

	for i := 0; i < 256; i += 8 {
		if freqs0[i] > 0 {
			sum++
		}
		if freqs0[i+1] > 0 {
			sum++
		}
		if freqs0[i+2] > 0 {
			sum++
		}
		if freqs0[i+3] > 0 {
			sum++
		}
		if freqs0[i+4] > 0 {
			sum++
		}
		if freqs0[i+5] > 0 {
			sum++
		}
		if freqs0[i+6] > 0 {
			sum++
		}
		if freqs0[i+7] > 0 {
			sum++
		}
	}

	if sum == 256 {
		return DT_BIN
	}

	if sum <= 4 {
		return DT_SMALL_ALPHABET
	}

	return DT_UNDEFINED
}

// ComputeJobsPerTask computes the number of jobs associated with each task
// given a number of jobs available and a number of tasks to perform.
// The provided 'jobsPerTask' slice is returned as result.
func ComputeJobsPerTask(jobsPerTask []uint, jobs, tasks uint) ([]uint, error) {
	if tasks == 0 {
		return jobsPerTask, errors.New("Invalid number of tasks provided: 0")
	}

	if jobs == 0 {
		return jobsPerTask, errors.New("Invalid number of jobs provided: 0")
	}

	var q, r uint

	if jobs <= tasks {
		q = 1
		r = 0
	} else {
		q = jobs / tasks
		r = jobs - q*tasks
	}

	for i := range jobsPerTask {
		jobsPerTask[i] = q
	}

	n := uint(0)

	for r != 0 {
		jobsPerTask[n]++
		r--
		n++

		if n == tasks {
			n = 0
		}
	}

	return jobsPerTask, nil
}
