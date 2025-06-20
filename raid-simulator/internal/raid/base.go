package raid

import "github.com/sirupsen/logrus"

const (
	initialOffset = 0
)

type RaidType string

var (
	RaidTypeRaid0  RaidType = "raid0"
	RaidTypeRaid1  RaidType = "raid1"
	RaidTypeRaid10 RaidType = "raid10"
	RaidTypeRaid5  RaidType = "raid5"
	RaidTypeRaid6  RaidType = "raid6"
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
		diskCount := 3
		stripeSz := 4
		clearTarget := 1
		Raid0SimulationFlow(input, diskCount, stripeSz, clearTarget)
	case RaidTypeRaid1:
		diskCount := 2
		stripeSz := 1
		clearTarget := 0
		Raid1SimulationFlow(input, diskCount, stripeSz, clearTarget)
	case RaidTypeRaid10:
		totalDisks := 4
		stripeSz := 4
		clearTarget := 2
		Raid10SimulationFlow(input, totalDisks, stripeSz, clearTarget)
	case RaidTypeRaid5:
		totalDisks := 3
		stripeSz := 1
		clearTarget := 0
		Raid5SimulationFlow(input, totalDisks, stripeSz, clearTarget)
	case RaidTypeRaid6:
		totalDisks := 4
		stripeSz := 1
		clearTarget := []int{0, 1}
		Raid6SimulationFlow(input, totalDisks, stripeSz, clearTarget)
	default:
		logrus.Warnf("Unsupported RAID type: %s", raidType)
	}
}
