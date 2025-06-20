package raid

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type RAID10Controller struct {
	mirrors  [][]*Disk // Array of RAID1 mirror pairs
	stripeSz int       // The size of each data stripe (chunk)
}

// NewRAID10Controller creates and initializes a new RAID10Controller.
// Requires an even number of totalDisks (min 4).
// stripeSz must be greater than 0.
func NewRAID10Controller(totalDisks int, stripeSz int) (*RAID10Controller, error) {
	if totalDisks < 4 || totalDisks%2 != 0 {
		return nil, fmt.Errorf("RAID10 requires an even number of disks, minimum 4. Provided: %d", totalDisks)
	}
	if stripeSz <= 0 {
		return nil, fmt.Errorf("stripe size must be greater than 0. Provided: %d", stripeSz)
	}

	var mirrors [][]*Disk
	for i := 0; i < totalDisks; i += 2 {
		mirrors = append(mirrors, []*Disk{
			{ID: i, Data: [][]byte{}},
			{ID: i + 1, Data: [][]byte{}},
		})
	}

	return &RAID10Controller{
		mirrors:  mirrors,
		stripeSz: stripeSz,
	}, nil
}

// Write writes data to the RAID10 array, striping data across mirror pairs.
// Supports block-level writes and Read-Modify-Write for partial updates.
func (r *RAID10Controller) Write(data []byte, offset int) error {
	if len(data) == 0 {
		return nil // No data to write
	}
	if r.stripeSz <= 0 {
		return fmt.Errorf("stripe size must be greater than 0")
	}
	if len(r.mirrors) == 0 {
		return fmt.Errorf("no mirror pairs in RAID10 array")
	}
	if offset < 0 {
		return fmt.Errorf("write offset must be non-negative")
	}

	currentLogicalByteOffset := offset
	dataToWriteIndex := 0

	for dataToWriteIndex < len(data) {
		currentAbsoluteStripeIdx := currentLogicalByteOffset / r.stripeSz
		mirrorIndex := currentAbsoluteStripeIdx % len(r.mirrors)
		chunkIndexInMirrorPair := currentAbsoluteStripeIdx / len(r.mirrors)

		primaryDisk := r.mirrors[mirrorIndex][0]
		backupDisk := r.mirrors[mirrorIndex][1]

		// Ensure disks have enough pre-allocated chunks
		for chunkIndexInMirrorPair >= len(primaryDisk.Data) {
			primaryDisk.Data = append(primaryDisk.Data, make([]byte, r.stripeSz))
			backupDisk.Data = append(backupDisk.Data, make([]byte, r.stripeSz)) // Mirror also needs a new chunk
		}

		// Determine how much data to write into the current chunk
		offsetInStripeChunk := currentLogicalByteOffset % r.stripeSz
		bytesToCopy := r.stripeSz - offsetInStripeChunk
		if bytesToCopy > (len(data) - dataToWriteIndex) {
			bytesToCopy = len(data) - dataToWriteIndex
		}

		// Perform Read-Modify-Write (if needed) and copy data to both mirrored disks
		targetChunkPrimary := primaryDisk.Data[chunkIndexInMirrorPair]
		targetChunkBackup := backupDisk.Data[chunkIndexInMirrorPair]

		if targetChunkPrimary == nil || len(targetChunkPrimary) != r.stripeSz ||
			targetChunkBackup == nil || len(targetChunkBackup) != r.stripeSz {
			return fmt.Errorf("RAID10 internal error: mirrored chunks for mirror pair %d, stripe %d are nil or malformed", mirrorIndex, chunkIndexInMirrorPair)
		}

		copy(targetChunkPrimary[offsetInStripeChunk:offsetInStripeChunk+bytesToCopy], data[dataToWriteIndex:dataToWriteIndex+bytesToCopy])
		copy(targetChunkBackup[offsetInStripeChunk:offsetInStripeChunk+bytesToCopy], data[dataToWriteIndex:dataToWriteIndex+bytesToCopy])

		currentLogicalByteOffset += bytesToCopy
		dataToWriteIndex += bytesToCopy
	}
	return nil
}

// Read reads data from the RAID10 array, reading from healthy disks in each mirror pair.
func (r *RAID10Controller) Read(start, length int) ([]byte, error) {
	if start < 0 || length < 0 {
		return nil, fmt.Errorf("read start and length must be non-negative")
	}
	if len(r.mirrors) == 0 {
		return nil, fmt.Errorf("no mirror pairs in RAID10 array to read from")
	}
	if r.stripeSz <= 0 {
		return nil, fmt.Errorf("stripe size must be greater than 0")
	}

	result := make([]byte, 0, length)
	endLogicalOffset := start + length

	// Determine the maximum logical stripe index that has ever been written across the array.
	// This needs to check both disks in a mirror pair to find the true max written data.
	maxWrittenLogicalStripeIdx := -1
	if len(r.mirrors) > 0 {
		for mirrorIdx, mirror := range r.mirrors {
			// Find the maximum number of chunks written to *either* disk in this mirror pair.
			// This accounts for one disk in the pair failing, but the other still holding the data.
			chunksInThisPair := 0
			for _, disk := range mirror {
				if len(disk.Data) > chunksInThisPair {
					chunksInThisPair = len(disk.Data)
				}
			}

			if chunksInThisPair > 0 {
				// The absolute stripe index of the *last* stripe written to this mirror pair
				// is (chunksInThisPair - 1).
				// Its logical position in the overall array is then calculated based on its mirrorIdx.
				logicalStripeIndexOfLastChunkInPair := (chunksInThisPair-1)*len(r.mirrors) + mirrorIdx
				maxWrittenLogicalStripeIdx = max(maxWrittenLogicalStripeIdx, logicalStripeIndexOfLastChunkInPair)
			}
		}
	}

	maxWrittenLogicalOffset := -1
	if maxWrittenLogicalStripeIdx != -1 {
		maxWrittenLogicalOffset = (maxWrittenLogicalStripeIdx + 1) * r.stripeSz
	}

	if maxWrittenLogicalOffset == -1 || start >= maxWrittenLogicalOffset {
		if start > maxWrittenLogicalOffset {
			return nil, fmt.Errorf("read start offset %d is beyond total data stored %d", start, maxWrittenLogicalOffset)
		}
		return []byte{}, nil
	}

	if endLogicalOffset > maxWrittenLogicalOffset {
		logrus.Warnf("[RAID10] Read request for %d bytes starting at %d exceeds total data stored %d. Truncating read length to %d.",
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
		mirrorIndex := currentAbsoluteStripeIdx % len(r.mirrors)
		chunkIndexInMirrorPair := currentAbsoluteStripeIdx / len(r.mirrors)

		currentMirror := r.mirrors[mirrorIndex]
		var sourceChunk []byte // The chunk to read from
		foundHealthyDisk := false

		// Try to read from any healthy disk in the mirror pair
		for _, disk := range currentMirror {
			if chunkIndexInMirrorPair < len(disk.Data) && disk.Data[chunkIndexInMirrorPair] != nil && len(disk.Data[chunkIndexInMirrorPair]) > 0 {
				sourceChunk = disk.Data[chunkIndexInMirrorPair]
				foundHealthyDisk = true
				break
			}
		}

		if !foundHealthyDisk {
			return nil, fmt.Errorf("missing stripe data at mirror pair %d, chunk %d. Both disks in mirror pair might have failed for stripe %d (logical offset %d)", mirrorIndex, chunkIndexInMirrorPair, currentAbsoluteStripeIdx, currentLogicalReadOffset)
		}

		offsetInChunk := currentLogicalReadOffset % r.stripeSz

		bytesToRead := r.stripeSz - offsetInChunk
		if bytesToRead > (endLogicalOffset - currentLogicalReadOffset) {
			bytesToRead = endLogicalOffset - currentLogicalReadOffset
		}

		if offsetInChunk+bytesToRead > len(sourceChunk) {
			bytesToRead = len(sourceChunk) - offsetInChunk
			if bytesToRead < 0 {
				bytesToRead = 0
			}
		}

		if bytesToRead > 0 {
			result = append(result, sourceChunk[offsetInChunk:offsetInChunk+bytesToRead]...)
		}
		currentLogicalReadOffset += bytesToRead
	}
	return result, nil
}

// ClearDisk simulates a disk failure for a specific disk in the RAID10 array.
func (r *RAID10Controller) ClearDisk(index int) error {
	found := false
	for _, mirror := range r.mirrors {
		for _, disk := range mirror {
			if disk.ID == index {
				disk.Data = [][]byte{} // Clear the data to simulate failure
				found = true
				logrus.Infof("[RAID10] Disk %d has been cleared (simulating failure).", index)
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return fmt.Errorf("disk %d not found in RAID10 array", index)
	}
	return nil
}

// Raid10SimulationFlow is a helper function to simulate a write, clear, and read cycle for RAID10.
func Raid10SimulationFlow(input string, totalDisks int, stripeSz int, clearTarget int) {
	raid, err := NewRAID10Controller(totalDisks, stripeSz) // Corrected function name
	if err != nil {
		logrus.Errorf("[RAID10] Init Raid10 controller failed: %v", err)
		return
	}

	err = raid.Write([]byte(input), 0)
	if err != nil {
		logrus.Errorf("[RAID10] Write failed: %v", err)
		return
	}
	logrus.Infof("[RAID10] Write done: %s", input)

	output, err := raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID10] Read failed: %v", err)
	} else {
		logrus.Infof("[RAID10] Recovered string before clear: %s", string(output))
	}

	err = raid.ClearDisk(clearTarget)
	if err != nil {
		logrus.Errorf("[RAID10] ClearDisk failed: %v", err)
		return
	}
	logrus.Infof("[RAID10] Disk %d cleared", clearTarget)

	output, err = raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID10] Read failed after clear: %v", err)
	} else {
		logrus.Infof("[RAID10] Recovered string after clear: %s", string(output))
	}
}
