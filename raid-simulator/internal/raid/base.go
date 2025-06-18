package raid

import "github.com/sirupsen/logrus"

type RaidType string

var (
	RaidTypeRaid0 RaidType = "raid0"
	RaidTypeRaid1 RaidType = "raid1"
)

// Simulate single Disk
type Disk struct {
	ID   int
	Data [][]byte // keep the data structure as [][]byte to simulate unit stripe/block
}

// Base RAIDController
type RAIDController interface {
	Write(data []byte) error
	Read(start, length int) ([]byte, error)
	ClearDisk(index int) error
}

func RunRAIDSimulation(raidType RaidType, input string) {
	switch raidType {
	case RaidTypeRaid0:
		raid := NewRAID0Controller(3, 4)
		raid.Write([]byte(input))
		logrus.Infof("[RAID0] Write done: %s", input)

		// First read
		output, err := raid.Read(0, len(input))
		if err != nil {
			logrus.Errorf("[RAID0] Read failed: %v", err)
		} else {
			logrus.Infof("[RAID0] Recovered string before clear: %s", string(output))
		}

		// Clear disk
		raid.ClearDisk(1)
		logrus.Infof("[RAID0] Disk 1 cleared")

		// Read again
		output, err = raid.Read(0, len(input))
		if err != nil {
			logrus.Errorf("[RAID0] Read failed after clear: %v", err)
		} else {
			logrus.Infof("[RAID0] Recovered string after clear: %s", string(output))
		}
	case RaidTypeRaid1:
		raid := NewRAID1Controller(2)
		raid.Write([]byte(input))
		logrus.Infof("[RAID1] Write done: %s", input)

		output, err := raid.Read(0, len(input))
		if err != nil {
			logrus.Errorf("[RAID1] Read failed: %v", err)
		} else {
			logrus.Infof("[RAID1] Recovered string before clear: %s", string(output))
		}

		raid.ClearDisk(0)
		logrus.Infof("[RAID1] Disk 0 cleared")

		output, err = raid.Read(0, len(input))
		if err != nil {
			logrus.Errorf("[RAID1] Read failed after clear: %v", err)
		} else {
			logrus.Infof("[RAID1] Recovered string after clear: %s", string(output))
		}
	default:
		logrus.Warnf("Unsupported RAID type: %s", raidType)
	}
}
