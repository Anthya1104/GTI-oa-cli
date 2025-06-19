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
	if diskCount < 3 {
		return nil, fmt.Errorf("RAID5 requires at least 3 disks (1 data + 1 parity). Provided: %d", diskCount)
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

	numDisks := len(r.disks)
	numDataDisks := numDisks - 1

	bytesPerFullStripe := r.stripeSz * numDataDisks

	fullStripesCount := len(data) / bytesPerFullStripe
	remainingBytes := len(data) % bytesPerFullStripe

	currentDataOffsetInInput := 0

	// Iterate through each full RAID5 stripe that can be formed from the input data
	for i := 0; i < fullStripesCount; i++ {
		currentAbsoluteStripeIdx := (offset / bytesPerFullStripe) + i

		stripeData := data[currentDataOffsetInInput : currentDataOffsetInInput+bytesPerFullStripe]

		parityDiskIdx := currentAbsoluteStripeIdx % numDisks

		chunksForThisStripe := make([][]byte, numDisks)
		dataChunkIndexInStripe := 0 // Counter for which data chunk (0 to numDataDisks-1) we are processing within this stripe

		for d := 0; d < numDisks; d++ {
			for currentAbsoluteStripeIdx >= len(r.disks[d].Data) {
				r.disks[d].Data = append(r.disks[d].Data, make([]byte, r.stripeSz))
			}
		}
		// MODIFICATION END

		// Distribute data chunks from `stripeData` to the appropriate data disks
		for d := 0; d < numDisks; d++ {
			if d == parityDiskIdx {
				continue // Skip the disk designated for parity in this stripe
			}

			// Calculate the start and end byte indices for the current data chunk within `stripeData`.
			chunkStart := dataChunkIndexInStripe * r.stripeSz
			chunkEnd := chunkStart + r.stripeSz

			// For full stripes, `chunkEnd` should always be within `stripeData`'s length.
			if chunkEnd > len(stripeData) {
				chunkEnd = len(stripeData)
			}
			// If we've already extracted all available data from `stripeData`, exit the loop.
			if chunkStart >= len(stripeData) {
				break
			}

			// Create a new byte slice for the chunk and copy the data.
			// This is crucial to prevent memory aliasing issues where multiple disk `Data` slices
			// might point to the same underlying array segment.
			chunk := make([]byte, r.stripeSz)
			copy(chunk, stripeData[chunkStart:chunkEnd])

			chunksForThisStripe[d] = chunk
			r.disks[d].Data[currentAbsoluteStripeIdx] = chunk
			dataChunkIndexInStripe++
		}

		// Calculate the parity for the current stripe.
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
		r.disks[parityDiskIdx].Data[currentAbsoluteStripeIdx] = parityChunk

		// Log informational details about the current stripe operation.
		logrus.Debugf("[RAID5] stripe %d (absolute) - data bytes %d-%d (input data) - parityDisk: %d, chunks: %v, parity: %v",
			currentAbsoluteStripeIdx, currentDataOffsetInInput, currentDataOffsetInInput+bytesPerFullStripe-1, parityDiskIdx, chunksForThisStripe, parityChunk)

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

func (r *RAID5Controller) handlePartialWrite(data []byte, partialDataOffsetInInput int, remainingBytes int, targetStripeIndex int, originalWriteOffset int) error {
	logrus.Debugf("[RAID5] Handling partial write of %d bytes using Read-Modify-Write for absolute stripe index %d.", remainingBytes, targetStripeIndex)

	numDisks := len(r.disks)
	numDataDisks := numDisks - 1
	bytesPerFullStripe := r.stripeSz * numDataDisks

	for d := 0; d < numDisks; d++ {
		for targetStripeIndex >= len(r.disks[d].Data) {
			r.disks[d].Data = append(r.disks[d].Data, make([]byte, r.stripeSz))
		}
	}

	// 1. Read the affected stripe (targetStripeIndex)
	currentStripeChunks := make([][]byte, numDisks)
	var failedDiskInStripe int = -1 // Track a single failed disk for reconstruction

	for d := 0; d < numDisks; d++ {
		if targetStripeIndex < len(r.disks[d].Data) && r.disks[d].Data[targetStripeIndex] != nil && len(r.disks[d].Data[targetStripeIndex]) > 0 {
			// Copy existing chunk data.
			chunkCopy := make([]byte, r.stripeSz)
			copy(chunkCopy, r.disks[d].Data[targetStripeIndex])
			currentStripeChunks[d] = chunkCopy
		} else {
			// If a disk is truly failed (e.g., cleared), its Data[targetStripeIndex] might be nil or empty.
			// Treat as a missing chunk for reconstruction.
			currentStripeChunks[d] = make([]byte, r.stripeSz)
			if failedDiskInStripe == -1 {
				failedDiskInStripe = d
			} else {
				return fmt.Errorf("RAID5: Multiple disk failures (%v) detected in stripe %d during partial write RMW. Data unrecoverable", []int{failedDiskInStripe, d}, targetStripeIndex)
			}
		}
	}

	// If a disk was "failed" (empty) in this stripe, reconstruct its content before modification.
	if failedDiskInStripe != -1 {
		reconstructedChunk := make([]byte, r.stripeSz)
		for d, chunk := range currentStripeChunks {
			if d == failedDiskInStripe {
				continue // Skip the one we are reconstructing
			}
			for byteIdx := 0; byteIdx < r.stripeSz; byteIdx++ {
				reconstructedChunk[byteIdx] ^= chunk[byteIdx]
			}
		}
		currentStripeChunks[failedDiskInStripe] = reconstructedChunk
		logrus.Warnf("RAID5: Reconstructed existing data for disk %d in stripe %d for RMW.", failedDiskInStripe, targetStripeIndex)
	}

	// 2. Form a complete logical stripe buffer from currentStripeChunks and overlay newPartialData
	// Create a full logical stripe from the read chunks (including reconstructed ones)
	fullLogicalStripeBuffer := make([]byte, bytesPerFullStripe)
	dataChunkCursor := 0
	parityDiskIdxForThisStripe := targetStripeIndex % numDisks

	for d := 0; d < numDisks; d++ {
		if d == parityDiskIdxForThisStripe {
			continue // Skip parity disk
		}
		// Copy the data chunk from currentStripeChunks into the logical buffer
		copy(fullLogicalStripeBuffer[dataChunkCursor*r.stripeSz:(dataChunkCursor+1)*r.stripeSz], currentStripeChunks[d])
		dataChunkCursor++
	}

	// Calculate the starting byte offset of the new partial data *within this specific target logical stripe*.
	// This ensures it's placed correctly regardless of where the overall `Write` started.
	startOffsetInTargetStripe := (originalWriteOffset + partialDataOffsetInInput) % bytesPerFullStripe

	// Source data: data[partialDataOffsetInInput : partialDataOffsetInInput+remainingBytes]
	// Destination: fullLogicalStripeBuffer
	// Destination offset: startOffsetInTargetStripe
	// Length: remainingBytes
	copy(fullLogicalStripeBuffer[startOffsetInTargetStripe:startOffsetInTargetStripe+remainingBytes], data[partialDataOffsetInInput:partialDataOffsetInInput+remainingBytes])

	// 3. Re-distribute modified data and Recalculate Parity
	newParityChunk := make([]byte, r.stripeSz)
	dataChunkCursor = 0

	for d := 0; d < numDisks; d++ {
		if d == parityDiskIdxForThisStripe {
			continue
		}

		newDataChunk := fullLogicalStripeBuffer[dataChunkCursor*r.stripeSz : (dataChunkCursor+1)*r.stripeSz]
		currentStripeChunks[d] = newDataChunk

		// Accumulate XOR for parity
		for byteIdx := 0; byteIdx < r.stripeSz; byteIdx++ {
			newParityChunk[byteIdx] ^= newDataChunk[byteIdx]
		}
		dataChunkCursor++
	}

	// Set the new parity chunk
	currentStripeChunks[parityDiskIdxForThisStripe] = newParityChunk

	// 4. Write new data chunks and new parity chunk back to disks
	for d := 0; d < numDisks; d++ {
		r.disks[d].Data[targetStripeIndex] = currentStripeChunks[d] // Overwrite existing chunk
	}

	logrus.Debugf("[RAID5] Partial write handled for stripe %d. New parity: %v", targetStripeIndex, newParityChunk)
	return nil
}

func (r *RAID5Controller) Read(start, length int) ([]byte, error) {
	if start < 0 || length < 0 {
		return nil, fmt.Errorf("read start and length must be non-negative")
	}

	if len(r.disks) < 3 { // Corrected: r.disks -> r.disks
		return nil, fmt.Errorf("RAID5 requires at least 3 disks, got %d", len(r.disks))
	}
	if r.stripeSz <= 0 { // Corrected: r.stripeSz -> r.stripeSz
		return nil, fmt.Errorf("stripe size (chunk unit size) must be greater than 0")
	}

	numDisks := len(r.disks)
	numDataDisks := numDisks - 1
	bytesPerFullStripe := r.stripeSz * numDataDisks // Corrected: r.stripeSz -> r.stripeSz

	if bytesPerFullStripe == 0 {
		return nil, fmt.Errorf("invalid RAID5 configuration: bytes per full stripe is zero (check StripeSz or diskCount)")
	}

	// Determine the maximum logical stripe index that has ever been written across the array.
	// This should not be limited by a single failed disk's zero length, as data can be reconstructed.
	maxWrittenLogicalStripeIdx := -1
	for _, disk := range r.disks {
		if len(disk.Data)-1 > maxWrittenLogicalStripeIdx {
			maxWrittenLogicalStripeIdx = len(disk.Data) - 1
		}
	}

	// If maxWrittenLogicalStripeIdx is still -1, it means no data has been written at all.
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
		parityDiskIdx := currentStripeIdx % numDisks

		failedDisksInStripe := []int{} // Stores IDs of disks that are considered failed for this specific stripe

		stripeChunks := make(map[int][]byte)

		// First pass: identify failed disks and collect available chunks
		for d := 0; d < numDisks; d++ {
			if currentStripeIdx >= len(r.disks[d].Data) || r.disks[d].Data[currentStripeIdx] == nil || len(r.disks[d].Data[currentStripeIdx]) == 0 { // Corrected: r.disks -> r.disks
				failedDisksInStripe = append(failedDisksInStripe, d)
				logrus.Debugf("Disk %d considered failed for stripe %d", d, currentStripeIdx)
			} else {
				chunkCopy := make([]byte, r.stripeSz)
				copy(chunkCopy, r.disks[d].Data[currentStripeIdx])
				stripeChunks[d] = chunkCopy
			}
		}

		if len(failedDisksInStripe) > 1 {
			return nil, fmt.Errorf("multiple disk failures (%v) detected in stripe %d. Data unrecoverable",
				failedDisksInStripe, currentStripeIdx)
		}

		// If exactly one disk failed, reconstruct its data/parity
		if len(failedDisksInStripe) == 1 {
			failedDiskIdx := failedDisksInStripe[0]
			reconstructedChunk := make([]byte, r.stripeSz)

			// XOR all available chunks (both data and parity if parity is available)
			// to reconstruct the missing one.
			for d, chunk := range stripeChunks {
				if len(chunk) != r.stripeSz {
					return nil, fmt.Errorf("chunk on disk %d for stripe %d has unexpected size %d, expected %d",
						d, currentStripeIdx, len(chunk), r.stripeSz)
				}
				for byteIdx := 0; byteIdx < r.stripeSz; byteIdx++ {
					reconstructedChunk[byteIdx] ^= chunk[byteIdx]
				}
			}
			stripeChunks[failedDiskIdx] = reconstructedChunk // Add reconstructed chunk to the map
			logrus.Warnf("Reconstructed chunk for disk %d in stripe %d. Content (first byte): %d",
				failedDiskIdx, currentStripeIdx, reconstructedChunk[0])
		}

		// Assemble the logical data for the current stripe from the data chunks
		currentStripeLogicalData := make([]byte, 0, bytesPerFullStripe)
		for d := 0; d < numDisks; d++ {
			if d == parityDiskIdx {
				continue // Skip the parity chunk; it's not part of the logical user data
			}
			dataChunk, ok := stripeChunks[d]
			if !ok || dataChunk == nil || len(dataChunk) != r.stripeSz {
				// This indicates a severe internal inconsistency or unhandled multiple failure.
				return nil, fmt.Errorf("internal error: failed to retrieve or reconstruct data chunk for disk %d in stripe %d", d, currentStripeIdx)
			}
			currentStripeLogicalData = append(currentStripeLogicalData, dataChunk...)
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

func (r *RAID5Controller) ClearDisk(index int) error {
	if index < 0 || index >= len(r.disks) {
		return fmt.Errorf("disk index %d out of bounds for %d disks", index, len(r.disks))
	}

	r.disks[index].Data = [][]byte{}
	logrus.Infof("Disk %d has been cleared (simulating failure).", index)
	return nil
}

func Raid5SimulationFlow(input string, diskCount int, stripeSz int, clearTarget int) {
	raid, err := NewRAID5Controller(diskCount, stripeSz)
	if err != nil {
		logrus.Errorf("[RAID5] Init Raid5 controller failed: %v", err)
	}
	raid.Write([]byte(input), initialOffset)
	logrus.Infof("[RAID5] Write done: %s", input)

	// First read
	output, err := raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID5] Read failed: %v", err)
	} else {
		logrus.Infof("[RAID5] Recovered string before clear: %s", string(output))
	}

	// Clear disk
	raid.ClearDisk(1)
	logrus.Infof("[RAID5] Disk 1 cleared")

	// Read again
	output, err = raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID5] Read failed after clear: %v", err)
	} else {
		logrus.Infof("[RAID5] Recovered string after clear: %s", string(output))
	}
}
