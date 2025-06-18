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
		Raid0SimulationFlow(input, 3, 4, 1)
	case RaidTypeRaid1:
		Raid1SimulationFlow(input, 2, 0)
	default:
		logrus.Warnf("Unsupported RAID type: %s", raidType)
	}
}
