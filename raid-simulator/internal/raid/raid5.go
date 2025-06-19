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
		logrus.Debugf("[RAID5] stripe %d (data bytes %d-%d) - parityDisk: %d, chunks: %v, parity: %v",
			i, currentDataOffset, currentDataOffset+bytesPerFullStripe-1, parityDiskIdx, chunksForThisStripe, parityChunk)

		currentDataOffset += bytesPerFullStripe // Advance the offset to the beginning of the next full stripe
	}

	// Start processing Partial Write (Read-Modify-Write)
	if remainingBytes > 0 {

		absolutePartialStripeIndex := offset / bytesPerFullStripe

		return r.handlePartialWrite(data, currentDataOffset, remainingBytes, absolutePartialStripeIndex)
	}

	return nil
}

func (r *RAID5Controller) handlePartialWrite(data []byte, currentDataOffset int, remainingBytes int, partialStripeIdx int) error {
	logrus.Infof("RAID5: Handling partial write of %d bytes using Read-Modify-Write.", remainingBytes)

	numDisks := len(r.disks)
	numDataDisks := numDisks - 1
	bytesPerFullStripe := r.stripeSz * numDataDisks

	// Ensure the disks are long enough to hold this partial stripe's data.
	// If this is the very first write and it's a partial one, disks might be empty.
	if partialStripeIdx >= len(r.disks[0].Data) {
		// If it's a new stripe, initialize chunks with zeros to represent empty space
		for _, disk := range r.disks {
			disk.Data = append(disk.Data, make([]byte, r.stripeSz)) // Append zero-filled chunk
		}
	}

	// 1. Read the affected stripe (current partialStripeIdx)
	currentStripeChunks := make([][]byte, numDisks)
	var failedDiskInStripe int = -1 // Track a single failed disk for reconstruction

	for d := 0; d < numDisks; d++ {
		if partialStripeIdx < len(r.disks[d].Data) && r.disks[d].Data[partialStripeIdx] != nil && len(r.disks[d].Data[partialStripeIdx]) > 0 {
			// Copy existing chunk data.
			chunkCopy := make([]byte, r.stripeSz)
			copy(chunkCopy, r.disks[d].Data[partialStripeIdx])
			currentStripeChunks[d] = chunkCopy
		} else {
			// Disk is missing this chunk (e.g., cleared or not yet written to this depth).
			// In a RMW, if a disk is truly failed, we'd reconstruct it first or rely on Read.
			// For this simplified RMW, we assume data exists or can be reconstructed if one disk is down.
			// If we find an empty slot, let's treat it as a potential failed disk for reconstruction logic
			// if exactly one is found.
			currentStripeChunks[d] = make([]byte, r.stripeSz) // Initialize with zeros if missing
			if failedDiskInStripe == -1 {
				failedDiskInStripe = d
			} else {
				// More than one missing chunk in this stripe means unrecoverable for RMW
				return fmt.Errorf("RAID5: Multiple disk failures detected in stripe %d during partial write RMW. Data unrecoverable", partialStripeIdx)
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
		logrus.Warnf("RAID5: Reconstructed existing data for disk %d in stripe %d for RMW.", failedDiskInStripe, partialStripeIdx)
	}

	// 2. Modify Data (overlay new partial data onto the existing data chunks)

	newPartialData := data[currentDataOffset : currentDataOffset+remainingBytes]

	dataChunkIdx := 0
	bytesCopiedToStripe := 0
	for d := 0; d < numDisks; d++ {
		if d == partialStripeIdx%numDisks { // Skip the parity disk for this stripe
			continue
		}

		// Calculate the logical offset within the `bytesPerFullStripe` for this chunk.
		// This tells us where this chunk's data starts in the "virtual" full stripe.
		logicalChunkStartInStripe := dataChunkIdx * r.stripeSz

		// Determine how much of `newPartialData` will go into this data chunk.
		copyStartInNewPartial := 0
		if logicalChunkStartInStripe > len(newPartialData) {
			// If this data chunk is entirely beyond the new partial data, skip.
			dataChunkIdx++
			continue
		}
		if currentDataOffset%bytesPerFullStripe > logicalChunkStartInStripe {
			// Calculate how much we need to skip in `newPartialData`.
			copyStartInNewPartial = currentDataOffset%bytesPerFullStripe - logicalChunkStartInStripe
		}

		// Determine how many bytes to copy into this chunk from `newPartialData`.
		bytesToCopy := r.stripeSz - copyStartInNewPartial
		if bytesToCopy > (len(newPartialData) - bytesCopiedToStripe) {
			bytesToCopy = (len(newPartialData) - bytesCopiedToStripe)
		}
		if bytesToCopy <= 0 {
			break // No more bytes to copy from newPartialData
		}

		if currentStripeChunks[d] == nil || len(currentStripeChunks[d]) < r.stripeSz {
			currentStripeChunks[d] = make([]byte, r.stripeSz) // Initialize if not already
		}

		// Perform the overlay: copy new data into the relevant part of the existing chunk.
		copy(currentStripeChunks[d][copyStartInNewPartial:copyStartInNewPartial+bytesToCopy], newPartialData[bytesCopiedToStripe:bytesCopiedToStripe+bytesToCopy])
		bytesCopiedToStripe += bytesToCopy
		dataChunkIdx++
	}

	// 3. Recalculate Parity for the modified stripe
	newParityChunk := make([]byte, r.stripeSz)
	for byteIdx := 0; byteIdx < r.stripeSz; byteIdx++ {
		for d := 0; d < numDisks; d++ {
			if d == partialStripeIdx%numDisks { // Skip the parity disk
				continue
			}
			if currentStripeChunks[d] != nil && len(currentStripeChunks[d]) > byteIdx {
				newParityChunk[byteIdx] ^= currentStripeChunks[d][byteIdx]
			}
		}
	}

	// 4. Write new data chunks and new parity chunk back to disks
	dataChunkIdx = 0
	for d := 0; d < numDisks; d++ {
		if d == partialStripeIdx%numDisks { // This is the parity disk for this stripe
			// Update parity chunk
			if partialStripeIdx < len(r.disks[d].Data) {
				r.disks[d].Data[partialStripeIdx] = newParityChunk
			} else {
				r.disks[d].Data = append(r.disks[d].Data, newParityChunk)
			}
		} else { // This is a data disk
			// Update data chunk
			if partialStripeIdx < len(r.disks[d].Data) {
				r.disks[d].Data[partialStripeIdx] = currentStripeChunks[d] // Write the modified chunk
			} else {
				r.disks[d].Data = append(r.disks[d].Data, currentStripeChunks[d])
			}
			dataChunkIdx++
		}
	}

	logrus.Infof("[RAID5] Partial write handled for stripe %d. New parity: %v", partialStripeIdx, newParityChunk)
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

	numDisks := len(r.disks) // Corrected: r.disks -> r.disks
	numDataDisks := numDisks - 1
	bytesPerFullStripe := r.stripeSz * numDataDisks // Corrected: r.stripeSz -> r.stripeSz

	if bytesPerFullStripe == 0 {
		return nil, fmt.Errorf("invalid RAID5 configuration: bytes per full stripe is zero (check StripeSz or diskCount)")
	}

	// Determine the maximum logical stripe index that has ever been written across the array.
	// This should not be limited by a single failed disk's zero length, as data can be reconstructed.
	maxWrittenLogicalStripeIdx := -1
	for _, disk := range r.disks { // Corrected: r.disks -> r.disks
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

		// Map to temporarily store the chunks (data or parity) for the current stripe.
		// This map uses disk ID as key and the byte slice (chunk data) as value.
		stripeChunks := make(map[int][]byte)

		// First pass: identify failed disks and collect available chunks
		for d := 0; d < numDisks; d++ {
			// A disk is "failed" for this stripe if it doesn't have a chunk at currentStripeIdx
			// (e.g., if it was cleared, or not enough data was written to it).
			if currentStripeIdx >= len(r.disks[d].Data) || r.disks[d].Data[currentStripeIdx] == nil || len(r.disks[d].Data[currentStripeIdx]) == 0 { // Corrected: r.disks -> r.disks
				failedDisksInStripe = append(failedDisksInStripe, d)
				logrus.Debugf("Disk %d considered failed for stripe %d", d, currentStripeIdx)
			} else {
				// Copy the chunk to prevent memory aliasing, especially important if Read is part of a modify-write cycle.
				// For this simulator, it's good practice for isolated chunk handling.
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

		// Determine the segment of `currentStripeLogicalData` to append to the result.
		// This handles partial reads at the beginning and end of the overall requested range.
		startCopyOffset := 0
		endCopyOffset := len(currentStripeLogicalData) // Default to full stripe length

		if currentStripeIdx == startStripeIdx {
			startCopyOffset = startOffsetInFirstStripe
		}
		if currentStripeIdx == endStripeIdx {
			endCopyOffset = endOffsetInLastStripe + 1 // +1 because slice end index is exclusive
		}

		// Ensure the copy offsets are within valid bounds of the current stripe's logical data
		if startCopyOffset < 0 {
			startCopyOffset = 0
		}
		if endCopyOffset > len(currentStripeLogicalData) {
			endCopyOffset = len(currentStripeLogicalData)
		}

		// Only append if there's actual data to copy from this stripe within the requested range
		if startCopyOffset < endCopyOffset {
			dataToAppend := currentStripeLogicalData[startCopyOffset:endCopyOffset]
			result = append(result, dataToAppend...)
		}
	}

	// Final check: if the collected result is longer than requested (due to partial stripe math),
	// truncate it. This usually shouldn't happen if the logic is perfect.
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
