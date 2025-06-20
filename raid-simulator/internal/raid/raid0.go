package raid

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type RAID0Controller struct {
	disks    []*Disk
	stripeSz int // The size of each data stripe (chunk)
}

func NewRAID0Controller(diskCount int, stripeSize int) *RAID0Controller {
	disks := make([]*Disk, diskCount)
	for i := range disks {
		disks[i] = &Disk{
			ID:   i,
			Data: [][]byte{},
		}
	}
	return &RAID0Controller{
		disks:    disks,
		stripeSz: stripeSize,
	}
}

func (r *RAID0Controller) Write(data []byte, offset int) error {
	if len(data) == 0 {
		return nil // No data to write
	}
	if r.stripeSz <= 0 {
		return fmt.Errorf("stripe size must be greater than 0")
	}
	if len(r.disks) == 0 {
		return fmt.Errorf("no disks in RAID0 array")
	}
	if offset < 0 {
		return fmt.Errorf("write offset must be non-negative")
	}

	currentLogicalByteOffset := offset
	dataToWriteIndex := 0

	for dataToWriteIndex < len(data) {
		currentAbsoluteStripeIdx := currentLogicalByteOffset / r.stripeSz
		diskIndex := currentAbsoluteStripeIdx % len(r.disks)
		chunkIndexInDisk := currentAbsoluteStripeIdx / len(r.disks)

		// Ensure disk has enough pre-allocated chunks to write into, or extend it.
		// If writing into a new stripe, or extending existing ones.
		for chunkIndexInDisk >= len(r.disks[diskIndex].Data) {
			r.disks[diskIndex].Data = append(r.disks[diskIndex].Data, make([]byte, r.stripeSz))
		}

		// Calculate the start and end offsets within the current stripe chunk
		offsetInStripeChunk := currentLogicalByteOffset % r.stripeSz
		bytesToCopy := r.stripeSz - offsetInStripeChunk

		if bytesToCopy > (len(data) - dataToWriteIndex) {
			bytesToCopy = len(data) - dataToWriteIndex
		}

		// Perform Read-Modify-Write if it's a partial update of an existing chunk
		// or if data doesn't perfectly align to stripe boundaries.
		targetChunk := r.disks[diskIndex].Data[chunkIndexInDisk]

		// Ensure targetChunk is not nil and has correct size (it should be, due to append loop above)
		if targetChunk == nil || len(targetChunk) != r.stripeSz {
			return fmt.Errorf("RAID0 internal error: chunk for disk %d, stripe %d is nil or malformed", diskIndex, chunkIndexInDisk)
		}

		copy(targetChunk[offsetInStripeChunk:offsetInStripeChunk+bytesToCopy], data[dataToWriteIndex:dataToWriteIndex+bytesToCopy])

		currentLogicalByteOffset += bytesToCopy
		dataToWriteIndex += bytesToCopy
	}
	return nil
}

func (r *RAID0Controller) Read(start, length int) ([]byte, error) {
	if start < 0 || length < 0 {
		return nil, fmt.Errorf("read start and length must be non-negative")
	}
	if len(r.disks) == 0 {
		return nil, fmt.Errorf("no disks in RAID0 array to read from")
	}
	if r.stripeSz <= 0 {
		return nil, fmt.Errorf("stripe size must be greater than 0")
	}

	result := make([]byte, 0, length)
	endLogicalOffset := start + length

	// Determine the maximum logical byte offset ever written to the array.
	// This helps in correctly handling reads that go beyond written data.
	// Let's refine `maxWrittenLogicalOffset` to reflect the maximum data that *could* be read
	// assuming no failures, then apply failure logic during the read loop.
	maxDiskStripeCount := 0
	for _, disk := range r.disks {
		if len(disk.Data) > maxDiskStripeCount {
			maxDiskStripeCount = len(disk.Data)
		}
	}
	// This `maxWrittenLogicalOffset` represents the maximum possible logical size if all disks were full
	// up to `maxDiskStripeCount`.
	maxWrittenLogicalOffset := maxDiskStripeCount * len(r.disks) * r.stripeSz

	if maxWrittenLogicalOffset == -1 || start >= maxWrittenLogicalOffset {
		if start > maxWrittenLogicalOffset {
			return nil, fmt.Errorf("read start offset %d is beyond total data stored %d", start, maxWrittenLogicalOffset)
		}
		return []byte{}, nil
	}

	if endLogicalOffset > maxWrittenLogicalOffset {
		logrus.Warnf("[RAID0] Read request for %d bytes starting at %d exceeds total data stored %d. Truncating read length to %d.",
			length, start, maxWrittenLogicalOffset, maxWrittenLogicalOffset-start)
		endLogicalOffset = maxWrittenLogicalOffset
		length = endLogicalOffset - start
	}
	if length <= 0 {
		return []byte{}, nil
	}

	currentLogicalReadOffset := start

	for currentLogicalReadOffset < endLogicalOffset {
		currentAbsoluteStripeIdx := currentLogicalReadOffset / r.stripeSz
		diskIndex := currentAbsoluteStripeIdx % len(r.disks)
		chunkIndexInDisk := currentAbsoluteStripeIdx / len(r.disks)

		// In RAID0, if any part of a logical stripe (chunk) is on a failed disk, the entire stripe is considered unrecoverable.
		// This `Read` method demonstrates this fundamental RAID0 characteristic by returning an error immediately
		// if a required data chunk is missing due to disk failure.
		// While the underlying logic can read partial data from a *healthy* chunk (as shown in the selected code snippet),
		// in the context of RAID0's lack of fault tolerance, any missing component means the logical data cannot be reliably presented.
		if diskIndex >= len(r.disks) || r.disks[diskIndex] == nil || chunkIndexInDisk >= len(r.disks[diskIndex].Data) || r.disks[diskIndex].Data[chunkIndexInDisk] == nil || len(r.disks[diskIndex].Data[chunkIndexInDisk]) == 0 {
			return nil, fmt.Errorf("RAID0: Data unrecoverable due to missing chunk at disk %d, chunk %d (logical stripe %d, offset %d). All disks must be healthy", diskIndex, chunkIndexInDisk, currentAbsoluteStripeIdx, currentLogicalReadOffset)
		}

		chunk := r.disks[diskIndex].Data[chunkIndexInDisk]
		offsetInChunk := currentLogicalReadOffset % r.stripeSz

		bytesToRead := r.stripeSz - offsetInChunk
		if bytesToRead > (endLogicalOffset - currentLogicalReadOffset) {
			bytesToRead = endLogicalOffset - currentLogicalReadOffset
		}

		if offsetInChunk+bytesToRead > len(chunk) {
			bytesToRead = len(chunk) - offsetInChunk
			if bytesToRead < 0 {
				bytesToRead = 0
			}
		}

		if bytesToRead > 0 {
			result = append(result, chunk[offsetInChunk:offsetInChunk+bytesToRead]...)
		}
		currentLogicalReadOffset += bytesToRead
	}
	return result, nil
}

func (r *RAID0Controller) ClearDisk(index int) error {
	if index < 0 || index >= len(r.disks) {
		return fmt.Errorf("invalid disk index: %d, out of bounds for %d disks", index, len(r.disks))
	}
	r.disks[index].Data = [][]byte{}
	logrus.Infof("[RAID0] Disk %d has been cleared (simulating failure).", index)
	return nil
}

// Raid0SimulationFlow is a helper function to simulate a write, clear, and read cycle for RAID0.
func Raid0SimulationFlow(input string, diskCount int, stripeSz int, clearTarget int) {
	raid := NewRAID0Controller(diskCount, stripeSz)
	err := raid.Write([]byte(input), initialOffset) // Ensure write uses offset
	if err != nil {
		logrus.Errorf("[RAID0] Write failed: %v", err)
		return
	}
	logrus.Infof("[RAID0] Write done: %s", input)

	// First read
	output, err := raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID0] Read failed: %v", err)
	} else {
		logrus.Infof("[RAID0] Recovered string before clear: %s", string(output))
	}

	// Clear disk
	err = raid.ClearDisk(clearTarget) // Use clearTarget
	if err != nil {
		logrus.Errorf("[RAID0] ClearDisk failed: %v", err)
		return
	}
	logrus.Infof("[RAID0] Disk %d cleared", clearTarget)

	// Read again
	output, err = raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID0] Read failed after clear: %v", err)
	} else {
		logrus.Infof("[RAID0] Recovered string after clear: %s", string(output))
	}
}
