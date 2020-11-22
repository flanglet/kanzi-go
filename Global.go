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

package kanzi

import (
	"errors"
)

// LOG2 is an array with 256 elements: int(Math.log2(x-1))
var LOG2 = [...]uint32{
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
var LOG2_4096 = [...]uint32{
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

// array with 33 elements: 65536/(1 + exp(-alpha*x))
var _INV_EXP = [33]int{
	// alpha = 0.54
	0, 8, 22, 47, 88, 160, 283, 492,
	848, 1451, 2459, 4117, 6766, 10819, 16608, 24127,
	32768, 41409, 48928, 54717, 58770, 61419, 63077, 64085,
	64688, 65044, 65253, 65376, 65448, 65489, 65514, 65528,
	65536,
}

// SQUASH contains p = 1/(1 + exp(-d)), d scaled by 8 bits, p scaled by 12 bits
var SQUASH [4096]int

// STRETCH is the inverse of squash. d = ln(p/(1-p)), d scaled by 8 bits, p by 12 bits.
// d has range -2047 to 2047 representing -8 to 8. p in [0..4095].
var STRETCH [4096]int

func init() {
	// Init squash
	for x := -2047; x <= 2047; x++ {
		w := x & 127
		y := (x >> 7) + 16
		SQUASH[x+2047] = (_INV_EXP[y]*(128-w) + _INV_EXP[y+1]*w) >> 11
	}

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

// Log2_1024 returns 1024 * log2(x). Max error is around 0.1%
func Log2_1024(x uint32) (uint32, error) {
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

// Max returns the maximum of 2 values without a branch
func Max(x, y int32) int32 {
	return x - (((x - y) >> 31) & (x - y))
}

// Min returns the minimum of 2 values without a branch
func Min(x, y int32) int32 {
	return y + (((x - y) >> 31) & (x - y))
}

// Abs returns the absolute value of the input without a branch
func Abs(x int32) int32 {
	// Patented (!) :  return (x ^ (x >> 31)) - (x >> 31)
	return (x + (x >> 31)) ^ (x >> 31)
}

// PositiveOrNull returns the input value if positive else 0
func PositiveOrNull(x int32) int32 {
	// return (x & ((-x) >> 31))
	return x & ^(x >> 31)
}

// IsPowerOf2 returns true if the input value is a power of two
func IsPowerOf2(x int32) bool {
	return (x & (x - 1)) == 0
}

// ResetLsb sets the Least significan bit to 0
func ResetLsb(x int32) int32 {
	return x & (x - 1)
}

// Lsb returns the least significant bit
func Lsb(x int32) int32 {
	return x & -x
}

// Msb returns the most significant bit
func Msb(x int32) int32 {
	x |= (x >> 1)
	x |= (x >> 2)
	x |= (x >> 4)
	x |= (x >> 8)
	x |= (x >> 16)
	return (x & ^(x >> 1))
}

// RoundUpPowerOfTwo returns the smnallest power of two greater than
// or equal to the input value
func RoundUpPowerOfTwo(x int32) int32 {
	x--
	x |= (x >> 1)
	x |= (x >> 2)
	x |= (x >> 4)
	x |= (x >> 8)
	x |= (x >> 16)
	return x + 1
}

// ComputeFirstOrderEntropy1024 computes the order 0 entropy of the block
// and scales the result by 1024 (result in the [0..1024] range)
// Fills in the histogram with order 0 frequencies. Incoming array size must be at least 256
func ComputeFirstOrderEntropy1024(blockLen int, histo []int) int {
	if blockLen == 0 {
		return 0
	}

	sum := uint64(0)
	logLength1024, _ := Log2_1024(uint32(blockLen))

	for i := 0; i < 256; i++ {
		if histo[i] == 0 {
			continue
		}

		log1024, _ := Log2_1024(uint32(histo[i]))
		sum += ((uint64(histo[i]) * uint64(logLength1024-log1024)) >> 3)
	}

	return int(sum / uint64(blockLen))
}

// ComputeHistogram computes the order 0 or order 1 histogram for the input block
// and returns it in the 'freqs' slice.
// If withTotal is true, the last spot in each frequencies order 0 array is for the total
// (each order 0 frequency slice must be of length 257 in this case).
func ComputeHistogram(block []byte, freqs []int, isOrder0, withTotal bool) {
	for i := range freqs {
		freqs[i] = 0
	}

	if isOrder0 == true {
		if withTotal == true {
			freqs[256] = len(block)
		}

		f0 := [256]int{}
		f1 := [256]int{}
		f2 := [256]int{}
		f3 := [256]int{}
		end4 := len(block) & -4

		for i := 0; i < end4; i += 4 {
			f0[block[i]]++
			f1[block[i+1]]++
			f2[block[i+2]]++
			f3[block[i+3]]++
		}

		for i := end4; i < len(block); i++ {
			freqs[block[i]]++
		}

		for i := 0; i < 256; i++ {
			freqs[i] += (f0[i] + f1[i] + f2[i] + f3[i])
		}
	} else { // Order 1
		prv := int(0)

		if withTotal == true {
			for _, cur := range block {
				freqs[prv+int(cur)]++
				freqs[prv+256]++
				prv = 257 * int(cur)
			}
		} else {
			for _, cur := range block {
				freqs[prv+int(cur)]++
				prv = int(cur) << 8
			}
		}
	}
}

// ComputeJobsPerTask computes the number of jobs associated with each task
// given a number of jobs available and a number of tasks to perform.
// The provided 'jobsPerTask' slice is returned as result.
func ComputeJobsPerTask(jobsPerTask []uint, jobs, tasks uint) []uint {
	if tasks == 0 {
		panic("Invalid number of tasks provided: 0")
	}

	if jobs == 0 {
		panic("Invalid number of jobs provided: 0")
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

	return jobsPerTask
}
