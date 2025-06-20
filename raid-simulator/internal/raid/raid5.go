package raid

import (
	"fmt"

	"github.com/Anthya1104/raid-simulator/internal/rsutil"
	"github.com/klauspost/reedsolomon"
	"github.com/sirupsen/logrus"
)

// RAID5Controller implements the RAIDController interface for RAID 5.
type RAID5Controller struct {
	disks    []*Disk
	stripeSz int

	encoder          reedsolomon.Encoder    // Reed-Solomon encoder for Encode/Reconstruct
	encoderExtension reedsolomon.Extensions // Reed-Solomon extension for DataShards/ParityShards
}

// NewRAID5Controller creates and initializes a new RAID5Controller.
// It requires at least 3 disks (2 data + 1 parity) for RAID5 to be fault-tolerant.
// stripeSz must be greater than 0.
func NewRAID5Controller(diskCount, stripeSz int) (*RAID5Controller, error) {
	if diskCount < 3 {
		return nil, fmt.Errorf("RAID5 requires at least 3 disks (2 data + 1 parity). Provided: %d", diskCount)
	}
	if stripeSz <= 0 {
		return nil, fmt.Errorf("stripe size (chunk unit size) must be greater than 0. Provided: %d", stripeSz)
	}

	disks := make([]*Disk, diskCount)
	for i := range disks {
		disks[i] = &Disk{ID: i} // Assign an ID to each disk
	}

	numDataShards := diskCount - 1 // RAID5 with 1 parity shard
	numParityShards := 1           // RAID5 with 1 parity disk

	// init reedsolomon encoder
	enc, err := reedsolomon.New(numDataShards, numParityShards)
	if err != nil {
		return nil, fmt.Errorf("failed to create reedsolomon encoder for RAID5: %w", err)
	}

	// init reedsolomon extension
	encEx, ok := enc.(reedsolomon.Extensions)
	if !ok {
		return nil, fmt.Errorf("reedsolomon encoder does not implement Extensions interface")
	}

	return &RAID5Controller{
		disks:            disks,
		stripeSz:         stripeSz,
		encoder:          enc,
		encoderExtension: encEx,
	}, nil
}

// Write writes data to the RAID5 array.
// The `offset` parameter specifies the logical byte offset at which to start writing.
func (r *RAID5Controller) Write(data []byte, offset int) error {
	if len(r.disks) < 3 {
		return fmt.Errorf("RAID5 requires at least 3 disks, got %d", len(r.disks))
	}
	if r.stripeSz <= 0 {
		return fmt.Errorf("stripe size (chunk unit size) must be greater than 0")
	}

	numDisks := len(r.disks)

	numDataShards := r.encoderExtension.DataShards()
	numParityShards := r.encoderExtension.ParityShards()

	bytesPerFullStripe := r.stripeSz * numDataShards

	fullStripesCount := len(data) / bytesPerFullStripe
	remainingBytes := len(data) % bytesPerFullStripe

	currentDataOffsetInInput := 0

	// Iterate through each full RAID5 stripe that can be formed from the input data
	for i := 0; i < fullStripesCount; i++ {
		currentAbsoluteStripeIdx := (offset / bytesPerFullStripe) + i

		stripeData := data[currentDataOffsetInInput : currentDataOffsetInInput+bytesPerFullStripe]

		encodedShards, err := rsutil.EncodeStripeShards(stripeData, r.stripeSz, r.encoder, numDataShards, numParityShards)
		if err != nil {
			return fmt.Errorf("RAID5: failed to encode shards for stripe %d: %w", currentAbsoluteStripeIdx, err)
		}

		// RAID5 parity rotation
		parityDiskIdx := currentAbsoluteStripeIdx % numDisks

		logicalDataShardCounter := 0
		for d := 0; d < numDisks; d++ {
			for currentAbsoluteStripeIdx >= len(r.disks[d].Data) {
				r.disks[d].Data = append(r.disks[d].Data, make([]byte, r.stripeSz))
			}

			if d == parityDiskIdx {
				r.disks[d].Data[currentAbsoluteStripeIdx] = encodedShards[numDataShards]
			} else {
				r.disks[d].Data[currentAbsoluteStripeIdx] = encodedShards[logicalDataShardCounter]
				logicalDataShardCounter++
			}
		}

		logrus.Debugf("[RAID5] stripe %d (absolute) - data bytes %d-%d (input data) - parityDisk: %d, parity: %v",
			currentAbsoluteStripeIdx, currentDataOffsetInInput, currentDataOffsetInInput+bytesPerFullStripe-1, parityDiskIdx, encodedShards[numDataShards])

		currentDataOffsetInInput += bytesPerFullStripe // Advance the offset to the beginning of the next full stripe within the input data
	}

	if remainingBytes > 0 {
		// Calculate the absolute stripe index for the partial write.
		// This is the stripe immediately following the last full stripe written by this `Write` call,
		// or the stripe indicated by `offset` if there were no full stripes.
		absolutePartialStripeIndex := (offset + (fullStripesCount * bytesPerFullStripe)) / bytesPerFullStripe

		return r.handlePartialWrite(data, currentDataOffsetInInput, remainingBytes, absolutePartialStripeIndex, offset)
	}

	return nil
}

// handlePartialWrite performs a Read-Modify-Write operation for partial data that does not form a full RAID5 stripe.
func (r *RAID5Controller) handlePartialWrite(data []byte, partialDataOffsetInInput int, remainingBytes int, targetStripeIndex int, originalWriteOffset int) error {
	logrus.Debugf("[RAID5] Handling partial write of %d bytes using Read-Modify-Write for absolute stripe index %d.", remainingBytes, targetStripeIndex)

	numDisks := len(r.disks)
	numDataShards := r.encoderExtension.DataShards()
	numParityShards := r.encoderExtension.ParityShards()
	bytesPerFullStripe := r.stripeSz * numDataShards

	for d := 0; d < numDisks; d++ {
		for targetStripeIndex >= len(r.disks[d].Data) {
			r.disks[d].Data = append(r.disks[d].Data, make([]byte, r.stripeSz))
		}
	}

	physicalShards := make([][]byte, numDisks)

	for d := 0; d < numDisks; d++ {
		if targetStripeIndex < len(r.disks[d].Data) && r.disks[d].Data[targetStripeIndex] != nil && len(r.disks[d].Data[targetStripeIndex]) > 0 {
			chunkCopy := make([]byte, r.stripeSz)
			copy(chunkCopy, r.disks[d].Data[targetStripeIndex])
			physicalShards[d] = chunkCopy
		} else {
			physicalShards[d] = nil // tag as lost (reed solomon defined as nil)
			logrus.Debugf("Disk %d considered failed for stripe %d during RMW read.", d, targetStripeIndex)
		}
	}

	// RAID5 parity rotation
	parityDiskIdxForThisStripe := targetStripeIndex % numDisks

	rsShards := make([][]byte, numDataShards+numParityShards)
	logicalDataShardCounter := 0
	for d := 0; d < numDisks; d++ {
		if d == parityDiskIdxForThisStripe {
			rsShards[numDataShards] = physicalShards[d]
		} else {
			rsShards[logicalDataShardCounter] = physicalShards[d]
			logicalDataShardCounter++
		}
	}

	err := rsutil.ReconstructStripeShards(rsShards, r.encoder, numParityShards)
	if err != nil {
		return fmt.Errorf("RAID5: failed to reconstruct shards in stripe %d for RMW: %w", targetStripeIndex, err)
	}

	fullLogicalStripeBuffer := make([]byte, bytesPerFullStripe)
	for i := 0; i < numDataShards; i++ {
		copy(fullLogicalStripeBuffer[i*r.stripeSz:(i+1)*r.stripeSz], rsShards[i])
	}

	startOffsetInTargetStripe := (originalWriteOffset + partialDataOffsetInInput) % bytesPerFullStripe

	copy(fullLogicalStripeBuffer[startOffsetInTargetStripe:startOffsetInTargetStripe+remainingBytes], data[partialDataOffsetInInput:partialDataOffsetInInput+remainingBytes])

	newShards, err := rsutil.EncodeStripeShards(fullLogicalStripeBuffer, r.stripeSz, r.encoder, numDataShards, numParityShards)
	if err != nil {
		return fmt.Errorf("RAID5: failed to re-encode shards for stripe %d during RMW: %w", targetStripeIndex, err)
	}

	logicalDataShardCounter = 0
	for d := 0; d < numDisks; d++ {
		if d == parityDiskIdxForThisStripe {
			r.disks[d].Data[targetStripeIndex] = newShards[numDataShards]
		} else {
			r.disks[d].Data[targetStripeIndex] = newShards[logicalDataShardCounter]
			logicalDataShardCounter++
		}
	}

	logrus.Debugf("[RAID5] Partial write handled for stripe %d. New parity: %v", targetStripeIndex, newShards[numDataShards])
	return nil
}

// Read reads data from the RAID5 array.
// It uses parity to reconstruct data from a single failed disk.
func (r *RAID5Controller) Read(start, length int) ([]byte, error) {
	if start < 0 || length < 0 {
		return nil, fmt.Errorf("read start and length must be non-negative")
	}

	if len(r.disks) < 3 {
		return nil, fmt.Errorf("RAID5 requires at least 3 disks, got %d", len(r.disks))
	}
	if r.stripeSz <= 0 {
		return nil, fmt.Errorf("stripe size (chunk unit unit size) must be greater than 0")
	}

	numDisks := len(r.disks)
	numDataShards := r.encoderExtension.DataShards()
	numParityShards := r.encoderExtension.ParityShards()
	bytesPerFullStripe := r.stripeSz * numDataShards

	if bytesPerFullStripe == 0 {
		return nil, fmt.Errorf("invalid RAID5 configuration: bytes per full stripe is zero (check stripeSz or diskCount)")
	}

	// Determine the maximum logical stripe index that has ever been written across the array.
	maxWrittenLogicalStripeIdx := -1
	for _, disk := range r.disks {
		if len(disk.Data)-1 > maxWrittenLogicalStripeIdx {
			maxWrittenLogicalStripeIdx = len(disk.Data) - 1
		}
	}

	if maxWrittenLogicalStripeIdx == -1 {
		return []byte{}, fmt.Errorf("no data has been written to the RAID array yet to read from")
	}

	totalDataStored := (maxWrittenLogicalStripeIdx + 1) * bytesPerFullStripe

	// Adjust read range to not exceed available data
	if start >= totalDataStored {
		return nil, fmt.Errorf("read start offset %d is beyond total data stored %d", start, totalDataStored)
	}
	if start+length > totalDataStored {
		logrus.Warnf("Read request for %d bytes starting at %d exceeds total data stored %d. Truncating read length to %d.",
			length, start, totalDataStored, totalDataStored-start)
		length = totalDataStored - start
	}
	if length <= 0 { // After truncation, length might become 0 or negative
		return []byte{}, nil
	}

	// Determine the first and last logical stripe indices involved in the read
	startStripeIdx := start / bytesPerFullStripe
	endStripeIdx := (start + length - 1) / bytesPerFullStripe

	// Determine the offset within the starting stripe and the length within the ending stripe
	startOffsetInFirstStripe := start % bytesPerFullStripe
	endOffsetInLastStripe := (start + length - 1) % bytesPerFullStripe

	result := make([]byte, 0, length) // Pre-allocate capacity for the result

	// Iterate through each required stripe
	for currentStripeIdx := startStripeIdx; currentStripeIdx <= endStripeIdx; currentStripeIdx++ {

		physicalShards := make([][]byte, numDisks)

		for d := 0; d < numDisks; d++ {
			if currentStripeIdx >= len(r.disks[d].Data) || r.disks[d].Data[currentStripeIdx] == nil || len(r.disks[d].Data[currentStripeIdx]) == 0 {
				physicalShards[d] = nil // mark as lost
				logrus.Debugf("Disk %d considered failed for stripe %d during read.", d, currentStripeIdx)
			} else {
				chunkCopy := make([]byte, r.stripeSz)
				copy(chunkCopy, r.disks[d].Data[currentStripeIdx])
				physicalShards[d] = chunkCopy
			}
		}

		// RAID5 parity rotation
		parityDiskIdxForThisStripe := currentStripeIdx % numDisks

		rsShards := make([][]byte, numDataShards+numParityShards)
		logicalDataShardCounter := 0
		for d := 0; d < numDisks; d++ {
			if d == parityDiskIdxForThisStripe {
				rsShards[numDataShards] = physicalShards[d]
			} else {
				rsShards[logicalDataShardCounter] = physicalShards[d]
				logicalDataShardCounter++
			}
		}

		err := rsutil.ReconstructStripeShards(rsShards, r.encoder, numParityShards)
		if err != nil {
			return nil, fmt.Errorf("RAID5: failed to reconstruct data for stripe %d: %w", currentStripeIdx, err)
		}

		currentStripeLogicalData := make([]byte, 0, bytesPerFullStripe)
		for i := 0; i < numDataShards; i++ {
			if rsShards[i] == nil || len(rsShards[i]) != r.stripeSz {
				return nil, fmt.Errorf("RAID5 internal error: logical data shard %d for stripe %d is nil or malformed after reconstruction", i, currentStripeIdx)
			}
			currentStripeLogicalData = append(currentStripeLogicalData, rsShards[i]...)
		}

		startCopyOffset := 0
		endCopyOffset := len(currentStripeLogicalData) // Default to full stripe length

		if currentStripeIdx == startStripeIdx {
			startCopyOffset = startOffsetInFirstStripe
		}
		if currentStripeIdx == endStripeIdx {
			endCopyOffset = endOffsetInLastStripe + 1 // +1 because slice end index is exclusive
		}

		if startCopyOffset < 0 {
			startCopyOffset = 0
		}
		if endCopyOffset > len(currentStripeLogicalData) {
			endCopyOffset = len(currentStripeLogicalData)
		}

		if startCopyOffset < endCopyOffset {
			dataToAppend := currentStripeLogicalData[startCopyOffset:endCopyOffset]
			result = append(result, dataToAppend...)
		}
	}

	if len(result) > length {
		result = result[:length]
	}

	return result, nil
}

// ClearDisk simulates a disk failure by clearing the data on the specified disk.
func (r *RAID5Controller) ClearDisk(index int) error {
	if index < 0 || index >= len(r.disks) {
		return fmt.Errorf("disk index %d out of bounds for %d disks", index, len(r.disks))
	}

	r.disks[index].Data = [][]byte{} // Clear the data to simulate failure
	logrus.Infof("Disk %d has been cleared (simulating failure).", index)
	return nil
}

// Raid5SimulationFlow is a helper function to simulate a write, clear, and read cycle for RAID5.
// This function is typically placed in a _test.go file or a separate simulation package.
// For demonstration, it's included here.
func Raid5SimulationFlow(input string, diskCount int, stripeSz int, clearTarget int) {
	initialOffset := 0

	raid, err := NewRAID5Controller(diskCount, stripeSz)
	if err != nil {
		logrus.Errorf("[RAID5] Init Raid5 controller failed: %v", err)
		return // Exit if controller initialization fails
	}
	err = raid.Write([]byte(input), initialOffset)
	if err != nil {
		logrus.Errorf("[RAID5] Write failed: %v", err)
		return // Exit if write fails
	}
	logrus.Infof("[RAID5] Write done: %s", input)

	// First read
	output, err := raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID5] Read failed: %v", err)
	} else {
		logrus.Infof("[RAID5] Recovered string before clear: %s", string(output))
	}

	// Clear disk
	err = raid.ClearDisk(clearTarget) // Use clearTarget parameter
	if err != nil {
		logrus.Errorf("[RAID5] ClearDisk failed for disk %d: %v", clearTarget, err)
		return
	}
	logrus.Infof("[RAID5] Disk %d cleared", clearTarget)

	// Read again
	output, err = raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID5] Read failed after clear: %v", err)
	} else {
		logrus.Infof("[RAID5] Recovered string after clear: %s", string(output))
	}
}
