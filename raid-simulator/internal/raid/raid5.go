package raid

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type RAID5Controller struct {
	disks    []*Disk
	stripeSz int
}

func NewRAID5Controller(diskCount, stripeSz int) (*RAID5Controller, error) {
	if diskCount < 2 {
		return nil, fmt.Errorf("RAID5 requires at least 2 disks (1 data + 1 parity). Provided: %d", diskCount)
	}
	if stripeSz <= 0 {
		return nil, fmt.Errorf("stripe size (chunk unit size) must be greater than 0. Provided: %d", stripeSz)
	}

	disks := make([]*Disk, diskCount)
	for i := range disks {
		disks[i] = &Disk{ID: i} // Assign an ID to each disk
	}
	return &RAID5Controller{
		disks:    disks,
		stripeSz: stripeSz,
	}, nil
}

func (r *RAID5Controller) Write(data []byte, offset int) error {
	// Basic validation for RAID5 configuration
	if len(r.disks) < 2 {
		return fmt.Errorf("RAID5 requires at least 2 disks, got %d", len(r.disks))
	}
	if r.stripeSz <= 0 {
		return fmt.Errorf("stripe size (chunk unit size) must be greater than 0")
	}

	numDisks := len(r.disks)     // Total number of disks in the array
	numDataDisks := numDisks - 1 // Number of disks that hold actual data chunks in a stripe

	// RAID5 stripe across all data disks (excluding the parity block).
	bytesPerFullStripe := r.stripeSz * numDataDisks

	// Calculate how many complete RAID5 stripes can be written from the input `data`.
	fullStripesCount := len(data) / bytesPerFullStripe
	remainingBytes := len(data) % bytesPerFullStripe

	currentDataOffset := 0 // Tracks the current read position in the input `data` byte slice

	// Iterate through each full RAID5 stripe that can be formed from the input data
	for i := 0; i < fullStripesCount; i++ {
		stripeData := data[currentDataOffset : currentDataOffset+bytesPerFullStripe]
		parityDiskIdx := i % numDisks

		chunksForThisStripe := make([][]byte, numDisks)
		dataChunkIndexInStripe := 0 // Counter for which data chunk (0 to numDataDisks-1) we are processing within this stripe

		// Distribute data chunks from `stripeData` to the appropriate data disks
		for d := 0; d < numDisks; d++ {
			if d == parityDiskIdx {
				continue // Skip the disk designated for parity in this stripe
			}

			chunkStart := dataChunkIndexInStripe * r.stripeSz
			chunkEnd := chunkStart + r.stripeSz
			if chunkEnd > len(stripeData) {
				chunkEnd = len(stripeData)
			}

			if chunkStart >= len(stripeData) {
				break
			}

			chunk := make([]byte, r.stripeSz)
			copy(chunk, stripeData[chunkStart:chunkEnd])

			chunksForThisStripe[d] = chunk
			r.disks[d].Data = append(r.disks[d].Data, chunk)
			dataChunkIndexInStripe++
		}

		// Calculate the parity for the current stripe.
		// The parity chunk will have the same size as a data chunk (`r.stripeSz`).
		parityChunk := make([]byte, r.stripeSz)
		for byteIdx := 0; byteIdx < r.stripeSz; byteIdx++ {
			for d := 0; d < numDisks; d++ {
				if d == parityDiskIdx {
					continue // Do not include the parity disk's slot in the XOR calculation
				}
				// Perform XOR operation only if the chunk exists and has enough bytes for `byteIdx`
				if chunksForThisStripe[d] != nil && len(chunksForThisStripe[d]) > byteIdx {
					parityChunk[byteIdx] ^= chunksForThisStripe[d][byteIdx]
				}
			}
		}

		// Write the calculated parity chunk to the disk designated for parity in this stripe.
		r.disks[parityDiskIdx].Data = append(r.disks[parityDiskIdx].Data, parityChunk)

		// Log informational details about the current stripe operation.
		logrus.Infof("[RAID5] stripe %d (data bytes %d-%d) - parityDisk: %d, chunks: %v, parity: %v",
			i, currentDataOffset, currentDataOffset+bytesPerFullStripe-1, parityDiskIdx, chunksForThisStripe, parityChunk)

		currentDataOffset += bytesPerFullStripe // Advance the offset to the beginning of the next full stripe
	}

	// Warning for partial writes:
	// A full RAID5 implementation would handle partial remaining bytes using a read-modify-write cycle.
	// For this simulator's scope, we simply inform the user that these bytes are not processed.
	if remainingBytes > 0 {
		logrus.Warnf("RAID5: Partial write of %d bytes not fully implemented. For optimal utilization, "+
			"input data length should be a multiple of %d (stripe size * (num_disks - 1)).",
			remainingBytes, bytesPerFullStripe)
	}

	return nil
}
