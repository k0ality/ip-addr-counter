package main

import (
	"fmt"
	"math"
	"math/bits"
)

type HyperLogLog struct {
	precision uint8   // Number of bits used for bucketing (b)
	m         uint32  // Number of counters (or "registers")
	registers []uint8 // Array of registers
	alphaMM   float64 // Bias correction constant * harmonic mean transformation across m buckets
}

func NewHyperLogLog(precision uint8) *HyperLogLog {
	if precision < 4 || precision > 18 {
		panic("precision must be between 4 and 18.")
	}

	m := uint32(1 << precision)

	var alpha float64
	switch m {
	case 16:
		alpha = 0.673
	case 32:
		alpha = 0.697
	case 64:
		alpha = 0.709
	default:
		alpha = 0.7213 / (1 + 1.079 / float64(m))
	}

	return &HyperLogLog{
		precision: precision,
		m:         m,
		registers: make([]uint8, m),
		alphaMM:   alpha * float64(m) * float64(m),
	}
}

func (h *HyperLogLog) Add(item uint32) {
	hash := hash32(item)
	bucketIdx := hash >> (32 - h.precision)
	remainingBits := hash << h.precision
	leadingZeros := uint8(bits.LeadingZeros32(remainingBits)) + 1
	if leadingZeros > h.registers[bucketIdx] {
		h.registers[bucketIdx] = leadingZeros
	}
}

func (h *HyperLogLog) Merge(other *HyperLogLog) error {
	if h.precision != other.precision {
		return fmt.Errorf("cannot merge HLLs with different precision")
	}

	for i := range h.registers {
		if other.registers[i] > h.registers[i] {
			h.registers[i] = other.registers[i]
		}
	}

	return nil
}

func (h *HyperLogLog) Count() uint64 {
	sum := 0.0
	zeros := 0

	for _, val := range h.registers {
		sum += 1.0 / float64(uint64(1)<<val)
		if val == 0 {
			zeros++
		}
	}

	estimate := h.alphaMM / sum

	if estimate <= 2.5 * float64(h.m) {
		if zeros != 0 {
			estimate = float64(h.m) * math.Log(float64(h.m) / float64(zeros))
		}
	} else if estimate <= (1.0 / 30.0) * math.Pow(2, 32) {
	} else {
		estimate = -math.Pow(2, 32) * math.Log(1 - estimate / math.Pow(2, 32))
	}

	return uint64(estimate)
}