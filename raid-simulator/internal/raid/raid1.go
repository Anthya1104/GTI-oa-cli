package raid

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type RAID1Controller struct {
	disks []*Disk
}

func NewRAID1Controller(diskCount int) *RAID1Controller {
	disks := make([]*Disk, diskCount)
	for i := range disks {
		disks[i] = &Disk{
			ID:   i,
			Data: [][]byte{},
		}
	}
	return &RAID1Controller{disks: disks}
}

func (r *RAID1Controller) Write(data []byte) error {
	// All disks store the same copy
	for _, disk := range r.disks {
		disk.Data = [][]byte{data}
	}
	return nil
}

func (r *RAID1Controller) Read(start, length int) ([]byte, error) {
	for _, disk := range r.disks {
		if len(disk.Data) == 0 {
			continue
		}
		data := disk.Data[0]
		if start+length > len(data) {
			return nil, fmt.Errorf("read range exceeds data length")
		}
		return data[start : start+length], nil
	}
	return nil, fmt.Errorf("no available disk with data")
}

func (r *RAID1Controller) ClearDisk(index int) error {
	if index < 0 || index >= len(r.disks) {
		return fmt.Errorf("invalid disk index")
	}
	r.disks[index].Data = [][]byte{}
	return nil
}

func Raid1SimulationFlow(input string, diskCount int, clearTarget int) {
	raid := NewRAID1Controller(diskCount)
	raid.Write([]byte(input))
	logrus.Infof("[RAID1] Write done: %s", input)

	output, err := raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID1] Read failed: %v", err)
	} else {
		logrus.Infof("[RAID1] Recovered string before clear: %s", string(output))
	}

	raid.ClearDisk(clearTarget)
	logrus.Infof("[RAID1] Disk 0 cleared")

	output, err = raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("[RAID1] Read failed after clear: %v", err)
	} else {
		logrus.Infof("[RAID1] Recovered string after clear: %s", string(output))
	}
}
