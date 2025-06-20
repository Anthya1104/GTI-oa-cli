package raid

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type RAID1Controller struct {
	disks    []*Disk
	stripeSz int // Added stripe size for block-level operations
}

func NewRAID1Controller(diskCount int, stripeSz int) (*RAID1Controller, error) {
	if diskCount < 2 {
		return nil, fmt.Errorf("RAID1 requires at least 2 disks. Provided: %d", diskCount)
	}
	if stripeSz <= 0 {
		return nil, fmt.Errorf("stripe size must be greater than 0. Provided: %d", stripeSz)
	}
	disks := make([]*Disk, diskCount)
	for i := range disks {
		disks[i] = &Disk{
			ID:   i,
			Data: [][]byte{},
		}
	}
	return &RAID1Controller{disks: disks, stripeSz: stripeSz}, nil
}

func (r *RAID1Controller) Write(data []byte, offset int) error {
	if len(r.disks) < 2 {
		return fmt.Errorf("RAID1 requires at least 2 disks, got %d", len(r.disks))
	}
	if len(data) == 0 {
		return nil // No data to write
	}
	if r.stripeSz <= 0 {
		return fmt.Errorf("stripe size must be greater than 0")
	}
	if offset < 0 {
		return fmt.Errorf("write offset must be non-negative")
	}

	currentLogicalByteOffset := offset
	dataToWriteIndex := 0

	for dataToWriteIndex < len(data) {
		currentAbsoluteChunkIdx := currentLogicalByteOffset / r.stripeSz
		offsetInChunk := currentLogicalByteOffset % r.stripeSz

		bytesToCopy := 0
		// For each disk (mirror)
		for _, disk := range r.disks {
			for currentAbsoluteChunkIdx >= len(disk.Data) {
				disk.Data = append(disk.Data, make([]byte, r.stripeSz))
			}

			bytesToCopy = r.stripeSz - offsetInChunk
			if bytesToCopy > (len(data) - dataToWriteIndex) {
				bytesToCopy = len(data) - dataToWriteIndex
			}

			targetChunk := disk.Data[currentAbsoluteChunkIdx]
			if targetChunk == nil || len(targetChunk) != r.stripeSz {
				return fmt.Errorf("RAID1 internal error: chunk for disk %d, index %d is nil or malformed", disk.ID, currentAbsoluteChunkIdx)
			}
			copy(targetChunk[offsetInChunk:offsetInChunk+bytesToCopy], data[dataToWriteIndex:dataToWriteIndex+bytesToCopy])
		}
		currentLogicalByteOffset += bytesToCopy
		dataToWriteIndex += bytesToCopy
	}
	return nil
}

func (r *RAID1Controller) Read(start, length int) ([]byte, error) {
	if start < 0 || length < 0 {
		return nil, fmt.Errorf("read start and length must be non-negative")
	}
	if len(r.disks) == 0 {
		return nil, fmt.Errorf("no disks in RAID1 array to read from")
	}
	if r.stripeSz <= 0 {
		return nil, fmt.Errorf("stripe size must be greater than 0")
	}

	result := make([]byte, 0, length)
	endLogicalOffset := start + length

	maxWrittenLogicalOffset := -1
	for _, disk := range r.disks {
		if len(disk.Data) > 0 && disk.Data[0] != nil {
			// Total data on this disk is (number of chunks) * stripeSz
			currentDiskMaxOffset := len(disk.Data) * r.stripeSz
			maxWrittenLogicalOffset = max(maxWrittenLogicalOffset, currentDiskMaxOffset)
		}
	}

	if maxWrittenLogicalOffset == -1 || start >= maxWrittenLogicalOffset {
		if start > maxWrittenLogicalOffset {
			return nil, fmt.Errorf("read start offset %d is beyond total data stored %d", start, maxWrittenLogicalOffset)
		}
		return []byte{}, nil
	}

	if endLogicalOffset > maxWrittenLogicalOffset {
		logrus.Warnf("[RAID1] Read request for %d bytes starting at %d exceeds total data stored %d. Truncating read length to %d.",
			length, start, maxWrittenLogicalOffset, maxWrittenLogicalOffset-start)
		endLogicalOffset = maxWrittenLogicalOffset
		length = endLogicalOffset - start
	}
	if length <= 0 {
		return []byte{}, nil
	}

	currentLogicalReadOffset := start

	for currentLogicalReadOffset < endLogicalOffset {
		currentAbsoluteChunkIdx := currentLogicalReadOffset / r.stripeSz
		offsetInChunk := currentLogicalReadOffset % r.stripeSz

		var sourceChunk []byte
		foundHealthyDisk := false
		// Try to read from any healthy mirrored disk
		for _, disk := range r.disks {
			if currentAbsoluteChunkIdx < len(disk.Data) && disk.Data[currentAbsoluteChunkIdx] != nil && len(disk.Data[currentAbsoluteChunkIdx]) > 0 {
				sourceChunk = disk.Data[currentAbsoluteChunkIdx]
				foundHealthyDisk = true
				break
			}
		}

		if !foundHealthyDisk {
			return nil, fmt.Errorf("no healthy disk found for chunk %d (logical offset %d). RAID1 cannot recover from all mirrors failing for this chunk", currentAbsoluteChunkIdx, currentLogicalReadOffset)
		}

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

// ClearDisk simulates a disk failure by clearing the data on the specified disk.
func (r *RAID1Controller) ClearDisk(index int) error {
	if index < 0 || index >= len(r.disks) {
		return fmt.Errorf("invalid disk index: %d, out of bounds for %d disks", index, len(r.disks))
	}
	r.disks[index].Data = [][]byte{} // Clear the data to simulate failure
	logrus.Infof("[RAID1] Disk %d has been cleared (simulating failure).", index)
	return nil
}

// Raid1SimulationFlow is a helper function to simulate a write, clear, and read cycle for RAID1.
func Raid1SimulationFlow(input string, diskCount int, stripeSz int, clearTarget int) {
	raid, err := NewRAID1Controller(diskCount, stripeSz) // Pass stripeSz
	if err != nil {
		logrus.Errorf("[RAID1] Init Raid1 controller failed: %v", err)
		return
	}
	err = raid.Write([]byte(input), initialOffset) // Ensure write uses offset for API consistency
	if err != nil {
		logrus.Errorf("[RAID1] Write failed: %v", err)
		return
	}
	logrus.Infof("[RAID1] Write done: %s", input)

	output, err := raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID1] Read failed: %v", err)
	} else {
		logrus.Infof("[RAID1] Recovered string before clear: %s", string(output))
	}

	err = raid.ClearDisk(clearTarget)
	if err != nil {
		logrus.Errorf("[RAID1] ClearDisk failed: %v", err)
		return
	}
	logrus.Infof("[RAID1] Disk %d cleared", clearTarget)

	output, err = raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID1] Read failed after clear: %v", err)
	} else {
		logrus.Infof("[RAID1] Recovered string after clear: %s", string(output))
	}
}
