package raid

import (
	"fmt"

	"github.com/Anthya1104/raid-simulator/internal/rsutil" // Ensure correct import path
	"github.com/klauspost/reedsolomon"
	"github.com/sirupsen/logrus"
)

// RAID6Controller implements the RAIDController interface for RAID 6.
type RAID6Controller struct {
	disks    []*Disk
	stripeSz int

	encoder          reedsolomon.Encoder    // Reed-Solomon encoder instance (for Encode/Reconstruct)
	encoderExtension reedsolomon.Extensions // Reed-Solomon Extensions instance (for DataShards/ParityShards)
}

// NewRAID6Controller creates and initializes a new RAID6Controller.
// It requires at least 4 disks (2 data + 2 parity) for RAID6 to be fault-tolerant.
// stripeSz must be greater than 0.
func NewRAID6Controller(diskCount, stripeSz int) (*RAID6Controller, error) {
	if diskCount < 4 {
		return nil, fmt.Errorf("RAID6 requires at least 4 disks (2 data + 2 parity). Provided: %d", diskCount)
	}
	if stripeSz <= 0 {
		return nil, fmt.Errorf("stripe size (chunk unit size) must be greater than 0. Provided: %d", stripeSz)
	}

	disks := make([]*Disk, diskCount)
	for i := range disks {
		disks[i] = &Disk{ID: i}
	}

	numDataShards := diskCount - 2 // RAID6 has 2 parity shards
	numParityShards := 2           // RAID6 consistently has 2 parity shards

	enc, err := reedsolomon.New(numDataShards, numParityShards)
	if err != nil {
		return nil, fmt.Errorf("failed to create reedsolomon encoder for RAID6: %w", err)
	}
	encEx, ok := enc.(reedsolomon.Extensions)
	if !ok {
		return nil, fmt.Errorf("reedsolomon encoder does not implement Extensions interface")
	}

	return &RAID6Controller{
		disks:            disks,
		stripeSz:         stripeSz,
		encoder:          enc,
		encoderExtension: encEx,
	}, nil
}

// Write writes data to the RAID6 array.
// The `offset` parameter specifies the logical byte offset at which to start writing.
func (r *RAID6Controller) Write(data []byte, offset int) error {
	if len(r.disks) < 4 {
		return fmt.Errorf("RAID6 requires at least 4 disks, got %d", len(r.disks))
	}
	if r.stripeSz <= 0 {
		return fmt.Errorf("stripe size (chunk unit size) must be greater than 0")
	}

	numDisks := len(r.disks)
	numDataShards := r.encoderExtension.DataShards()
	numParityShards := r.encoderExtension.ParityShards() // Should be 2

	bytesPerFullStripe := r.stripeSz * numDataShards

	fullStripesCount := len(data) / bytesPerFullStripe
	remainingBytes := len(data) % bytesPerFullStripe

	currentDataOffsetInInput := 0

	// Iterate through each full RAID6 stripe that can be formed from the input data
	for i := 0; i < fullStripesCount; i++ {
		currentAbsoluteStripeIdx := (offset / bytesPerFullStripe) + i

		stripeData := data[currentDataOffsetInInput : currentDataOffsetInInput+bytesPerFullStripe]

		encodedShards, err := rsutil.EncodeStripeShards(stripeData, r.stripeSz, r.encoder, numDataShards, numParityShards)
		if err != nil {
			return fmt.Errorf("RAID6: failed to encode shards for stripe %d: %w", currentAbsoluteStripeIdx, err)
		}

		// Write the encoded shards (containing data and parity) to the disks
		// RAID6 uses a fixed parity disk strategy for this implementation.
		// TODO: Currently, parity rotation is not implemented here. For future expansion,
		// refer to "Diagonal Parity RAID6" or "RAID 6 P-Q matrix methods" for dynamic parity placement.
		logicalDataShardCounter := 0 // Track the logical data shard index in encodedShards
		for d := 0; d < numDisks; d++ {
			for currentAbsoluteStripeIdx >= len(r.disks[d].Data) {
				r.disks[d].Data = append(r.disks[d].Data, make([]byte, r.stripeSz))
			}

			if d == numDisks-2 { // The second to last disk stores the first parity (P)
				r.disks[d].Data[currentAbsoluteStripeIdx] = encodedShards[numDataShards] // Logical Parity0
			} else if d == numDisks-1 { // The last disk stores the second parity (Q)
				r.disks[d].Data[currentAbsoluteStripeIdx] = encodedShards[numDataShards+1] // Logical Parity1
			} else { // This is a data disk (0 to numDataShards-1)
				r.disks[d].Data[currentAbsoluteStripeIdx] = encodedShards[logicalDataShardCounter]
				logicalDataShardCounter++
			}
		}

		logrus.Debugf("[RAID6] stripe %d (absolute) - data bytes %d-%d (input data) - Parity0: %v, Parity1: %v",
			currentAbsoluteStripeIdx, currentDataOffsetInInput, currentDataOffsetInInput+bytesPerFullStripe-1, encodedShards[numDataShards], encodedShards[numDataShards+1])

		currentDataOffsetInInput += bytesPerFullStripe
	}

	if remainingBytes > 0 {
		absolutePartialStripeIndex := (offset + (fullStripesCount * bytesPerFullStripe)) / bytesPerFullStripe

		return r.handlePartialWrite(data, currentDataOffsetInInput, remainingBytes, absolutePartialStripeIndex, offset)
	}

	return nil
}

// handlePartialWrite performs a Read-Modify-Write operation for partial data that does not form a full RAID6 stripe.
func (r *RAID6Controller) handlePartialWrite(data []byte, partialDataOffsetInInput int, remainingBytes int, targetStripeIndex int, originalWriteOffset int) error {
	logrus.Debugf("[RAID6] Handling partial write of %d bytes using Read-Modify-Write for absolute stripe index %d.", remainingBytes, targetStripeIndex)

	numDisks := len(r.disks)
	numDataShards := r.encoderExtension.DataShards()
	numParityShards := r.encoderExtension.ParityShards() // Should be 2
	bytesPerFullStripe := r.stripeSz * numDataShards

	// Ensure that all disks have enough space in their Data slice to handle the new stripe write
	for d := 0; d < numDisks; d++ {
		for targetStripeIndex >= len(r.disks[d].Data) {
			r.disks[d].Data = append(r.disks[d].Data, make([]byte, r.stripeSz))
		}
	}

	// 1. Read all affected stripe shards (in physical disk order)
	physicalShards := make([][]byte, numDisks)

	for d := 0; d < numDisks; d++ {
		if targetStripeIndex < len(r.disks[d].Data) && r.disks[d].Data[targetStripeIndex] != nil && len(r.disks[d].Data[targetStripeIndex]) > 0 {
			chunkCopy := make([]byte, r.stripeSz)
			copy(chunkCopy, r.disks[d].Data[targetStripeIndex])
			physicalShards[d] = chunkCopy
		} else {
			physicalShards[d] = nil // Mark as missing (reedsolomon library requires nil)
			logrus.Debugf("Disk %d considered failed for stripe %d during RMW read.", d, targetStripeIndex)
		}
	}

	// 2. Prepare shards in the order required by the reedsolomon library (logical order)
	// The RS library expects the order: [Data0, ..., DataN-1, Parity0, Parity1]
	// TODO: Currently, parity rotation is not implemented here. For future expansion,
	// refer to "Diagonal Parity RAID6" or "RAID 6 P-Q matrix methods" for dynamic parity placement.
	rsShards := make([][]byte, numDataShards+numParityShards)
	for i := 0; i < numDataShards; i++ {
		rsShards[i] = physicalShards[i] // Data shards directly map to physical disks 0 to numDataShards-1
	}
	rsShards[numDataShards] = physicalShards[numDisks-2]   // Parity0 (P) comes from the second to last disk
	rsShards[numDataShards+1] = physicalShards[numDisks-1] // Parity1 (Q) comes from the last disk

	// 3. Attempt to reconstruct missing shards using rsutil.ReconstructStripeShards
	// RAID6 can tolerate 2 failures
	err := rsutil.ReconstructStripeShards(rsShards, r.encoder, numParityShards)
	if err != nil {
		return fmt.Errorf("RAID6: failed to reconstruct shards in stripe %d for RMW: %w", targetStripeIndex, err)
	}

	// 4. Assemble a complete logical stripe buffer (extracted from reconstructed data shards) and overlay new partial data
	fullLogicalStripeBuffer := make([]byte, bytesPerFullStripe)
	for i := 0; i < numDataShards; i++ {
		copy(fullLogicalStripeBuffer[i*r.stripeSz:(i+1)*r.stripeSz], rsShards[i])
	}

	startOffsetInTargetStripe := (originalWriteOffset + partialDataOffsetInInput) % bytesPerFullStripe

	copy(fullLogicalStripeBuffer[startOffsetInTargetStripe:startOffsetInTargetStripe+remainingBytes], data[partialDataOffsetInInput:partialDataOffsetInInput+remainingBytes])

	// 5. Recompute parity checksums based on the updated logical stripe buffer
	// newShards will contain new data blocks (newShards[0]...newShards[numDataShards-1])
	// and new parity blocks (newShards[numDataShards] and newShards[numDataShards+1])
	newShards, err := rsutil.EncodeStripeShards(fullLogicalStripeBuffer, r.stripeSz, r.encoder, numDataShards, numParityShards)
	if err != nil {
		return fmt.Errorf("RAID6: failed to re-encode shards for stripe %d during RMW: %w", targetStripeIndex, err)
	}

	// 6. Write the updated shards (data and parity) back to the corresponding physical disks
	logicalDataShardCounter := 0
	for d := 0; d < numDisks; d++ {
		if d == numDisks-2 { // P shard written to the second to last disk
			r.disks[d].Data[targetStripeIndex] = newShards[numDataShards]
		} else if d == numDisks-1 { // Q shard written to the last disk
			r.disks[d].Data[targetStripeIndex] = newShards[numDataShards+1]
		} else { // Data shards written to data disks
			r.disks[d].Data[targetStripeIndex] = newShards[logicalDataShardCounter]
			logicalDataShardCounter++
		}
	}

	logrus.Debugf("[RAID6] Partial write handled for stripe %d. New Parity0: %v, New Parity1: %v", targetStripeIndex, newShards[numDataShards], newShards[numDataShards+1])
	return nil
}

func (r *RAID6Controller) Read(start, length int) ([]byte, error) {
	if start < 0 || length < 0 {
		return nil, fmt.Errorf("read start and length must be non-negative")
	}

	if len(r.disks) < 4 { // RAID6 requires a minimum of 4 disks
		return nil, fmt.Errorf("RAID6 requires at least 4 disks, got %d", len(r.disks))
	}
	if r.stripeSz <= 0 {
		return nil, fmt.Errorf("stripe size (chunk unit unit size) must be greater than 0")
	}

	numDisks := len(r.disks)
	numDataShards := r.encoderExtension.DataShards()
	numParityShards := r.encoderExtension.ParityShards() // Should be 2
	bytesPerFullStripe := r.stripeSz * numDataShards

	if bytesPerFullStripe == 0 {
		return nil, fmt.Errorf("invalid RAID6 configuration: bytes per full stripe is zero (check stripeSz or diskCount)")
	}

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

	startStripeIdx := start / bytesPerFullStripe
	endStripeIdx := (start + length - 1) / bytesPerFullStripe

	startOffsetInFirstStripe := start % bytesPerFullStripe
	endOffsetInLastStripe := (start + length - 1) % bytesPerFullStripe

	result := make([]byte, 0, length)
	for currentStripeIdx := startStripeIdx; currentStripeIdx <= endStripeIdx; currentStripeIdx++ {
		// 1. Collect shards from disks (in physical disk order)
		physicalShards := make([][]byte, numDisks) // Shards arranged by physical disk index

		for d := 0; d < numDisks; d++ {
			if currentStripeIdx >= len(r.disks[d].Data) || r.disks[d].Data[currentStripeIdx] == nil || len(r.disks[d].Data[currentStripeIdx]) == 0 {
				physicalShards[d] = nil // Mark as missing
				logrus.Debugf("Disk %d considered failed for stripe %d during read.", d, currentStripeIdx)
			} else {
				chunkCopy := make([]byte, r.stripeSz)
				copy(chunkCopy, r.disks[d].Data[currentStripeIdx])
				physicalShards[d] = chunkCopy
			}
		}

		// 2. Prepare shards in the order required by the reedsolomon library (logical order)
		// The RS library expects the order: [Data0, ..., DataN-1, Parity0, Parity1]
		// TODO: Currently, parity rotation is not implemented here. For future expansion,
		// refer to "Diagonal Parity RAID6" or "RAID 6 P-Q matrix methods" for dynamic parity placement.
		rsShards := make([][]byte, numDataShards+numParityShards)
		for i := 0; i < numDataShards; i++ {
			rsShards[i] = physicalShards[i]
		}
		rsShards[numDataShards] = physicalShards[numDisks-2]   // Parity0 (P) comes from the second to last disk
		rsShards[numDataShards+1] = physicalShards[numDisks-1] // Parity1 (Q) comes from the last disk

		// 3. Use rsutil.ReconstructStripeShards to handle failures. RAID6 can tolerate 2 failures.
		err := rsutil.ReconstructStripeShards(rsShards, r.encoder, numParityShards)
		if err != nil {
			return nil, fmt.Errorf("RAID6: failed to reconstruct data for stripe %d: %w", currentStripeIdx, err)
		}

		// 4. Assemble logical data (extract data chunks from reconstructed rsShards)
		currentStripeLogicalData := make([]byte, 0, bytesPerFullStripe)
		for i := 0; i < numDataShards; i++ {
			if rsShards[i] == nil || len(rsShards[i]) != r.stripeSz {
				return nil, fmt.Errorf("RAID6 internal error: logical data shard %d for stripe %d is nil or malformed after reconstruction", i, currentStripeIdx)
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
func (r *RAID6Controller) ClearDisk(index int) error {
	if index < 0 || index >= len(r.disks) {
		return fmt.Errorf("disk index %d out of bounds for %d disks", index, len(r.disks))
	}

	r.disks[index].Data = [][]byte{} // Clear the data to simulate failure
	logrus.Infof("Disk %d has been cleared (simulating failure).", index)
	return nil
}

// Raid6SimulationFlow is a helper function to simulate a write, clear, and read cycle for RAID6.
// This function is typically placed in a _test.go file or a separate simulation package.
// For demonstration, it's included here.
func Raid6SimulationFlow(input string, diskCount int, stripeSz int, clearTargets []int) {
	initialOffset := 0

	raid, err := NewRAID6Controller(diskCount, stripeSz)
	if err != nil {
		logrus.Errorf("[RAID6] Init Raid6 controller failed: %v", err)
		return // Exit if controller initialization fails
	}
	err = raid.Write([]byte(input), initialOffset)
	if err != nil {
		logrus.Errorf("[RAID6] Write failed: %v", err)
		return // Exit if write fails
	}
	logrus.Infof("[RAID6] Write done: %s", input)

	// First read
	output, err := raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID6] Read failed: %v", err)
	} else {
		logrus.Infof("[RAID6] Recovered string before clear: %s", string(output))
	}

	// Clear disks (can clear multiple for RAID6)
	for _, target := range clearTargets {
		err = raid.ClearDisk(target)
		if err != nil {
			logrus.Errorf("[RAID6] ClearDisk failed for disk %d: %v", target, err)
			return
		}
		logrus.Infof("[RAID6] Disk %d cleared", target)
	}

	// Read again after clearing disks
	output, err = raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID6] Read failed after clear: %v", err)
	} else {
		logrus.Infof("[RAID6] Recovered string after clear: %s", string(output))
	}
}
