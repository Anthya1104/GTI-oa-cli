package raid

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type RAID10Controller struct {
	mirrors  [][]*Disk // raid1 mirror pair
	stripeSz int
}

func NewRAID10Controller(totalDisks int, stripeSz int) *RAID10Controller {
	if totalDisks%2 != 0 {
		panic("RAID10 requires even number of disks")
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
	}
}

func (r *RAID10Controller) Write(data []byte, offset int) error {
	if len(data) < r.stripeSz {
		return fmt.Errorf("data length: %v should larger than stripe size: %v", len(data), r.stripeSz)
	}
	mirrorIndex := 0
	for offset < len(data) {
		end := offset + r.stripeSz
		if end > len(data) {
			end = len(data)
		}
		stripe := make([]byte, r.stripeSz)
		copy(stripe, data[offset:end])

		primary := r.mirrors[mirrorIndex][0]
		backup := r.mirrors[mirrorIndex][1]

		primary.Data = append(primary.Data, stripe)
		backup.Data = append(backup.Data, stripe)

		offset = end
		mirrorIndex = (mirrorIndex + 1) % len(r.mirrors)
	}
	return nil
}

func (r *RAID10Controller) Read(start, length int) ([]byte, error) {
	result := make([]byte, 0, length)
	end := start + length

	for i := start; i < end; {
		stripeIdx := i / r.stripeSz
		offsetInStripe := i % r.stripeSz

		mirror := r.mirrors[stripeIdx%len(r.mirrors)]
		chunkIndex := stripeIdx / len(r.mirrors)

		var chunk []byte
		for _, disk := range mirror {
			if chunkIndex < len(disk.Data) {
				chunk = disk.Data[chunkIndex]
				break
			}
		}
		if chunk == nil {
			return nil, fmt.Errorf("missing stripe at mirror %d", stripeIdx%len(r.mirrors))
		}

		remain := end - i
		if offsetInStripe >= len(chunk) {
			return nil, fmt.Errorf("invalid offset in stripe")
		}
		readLen := r.stripeSz - offsetInStripe
		if readLen > remain {
			readLen = remain
		}

		result = append(result, chunk[offsetInStripe:offsetInStripe+readLen]...)
		i += readLen
	}

	return result, nil
}

func (r *RAID10Controller) ClearDisk(index int) error {
	for _, mirror := range r.mirrors {
		for _, disk := range mirror {
			if disk.ID == index {
				disk.Data = [][]byte{}
				return nil
			}
		}
	}
	return fmt.Errorf("disk %d not found", index)
}

func Raid10SimulationFlow(input string, diskCount int, stripeSz int, clearTarget int) {
	raid := NewRAID10Controller(diskCount, stripeSz)

	raid.Write([]byte(input), 0)
	logrus.Infof("[RAID10] Write done: %s", input)

	output, err := raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID10] Read failed: %v", err)
	} else {
		logrus.Infof("[RAID10] Recovered string before clear: %s", string(output))
	}

	raid.ClearDisk(clearTarget)
	logrus.Infof("[RAID10] Disk %d cleared", clearTarget)

	output, err = raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID10] Read failed after clear: %v", err)
	} else {
		logrus.Infof("[RAID10] Recovered string after clear: %s", string(output))
	}
}
