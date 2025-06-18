package raid

import "fmt"

// RAID0
// data would be splitted into to multiple disks
type RAID0Controller struct {
	disks    []*Disk
	stripeSz int // the data size for each stripe
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

func (r *RAID0Controller) Write(data []byte) error {
	offset := 0
	diskIndex := 0
	for offset < len(data) {
		end := offset + r.stripeSz
		if end > len(data) {
			end = len(data)
		}
		stripe := make([]byte, r.stripeSz)
		copy(stripe, data[offset:end])
		r.disks[diskIndex].Data = append(r.disks[diskIndex].Data, stripe)
		offset = end
		diskIndex = (diskIndex + 1) % len(r.disks)
	}
	return nil
}

func (r *RAID0Controller) Read(start, length int) ([]byte, error) {
	result := make([]byte, 0, length)
	readCount := 0
	stripeCount := 0
	totalStripes := (start + length + r.stripeSz - 1) / r.stripeSz
	for stripeCount < totalStripes {
		diskIndex := stripeCount % len(r.disks)
		chunkIndex := stripeCount / len(r.disks)
		if chunkIndex >= len(r.disks[diskIndex].Data) {
			return nil, fmt.Errorf("missing stripe data at disk %d, chunk %d", diskIndex, chunkIndex)
		}
		chunk := r.disks[diskIndex].Data[chunkIndex]
		remain := length - readCount
		if remain >= len(chunk) {
			result = append(result, chunk...)
			readCount += len(chunk)
		} else {
			result = append(result, chunk[:remain]...)
			readCount += remain
		}
		stripeCount++
	}
	return result[start%r.stripeSz:], nil
}

func (r *RAID0Controller) ClearDisk(index int) error {
	if index < 0 || index >= len(r.disks) {
		return fmt.Errorf("invalid disk index")
	}
	r.disks[index].Data = [][]byte{}
	return nil
}
