package raid_test

import (
	"testing"

	"github.com/Anthya1104/raid-simulator/internal/raid"
	"github.com/stretchr/testify/assert"
)

func TestRAID1_WriteRead(t *testing.T) {
	r := raid.NewRAID1Controller(3)
	data := []byte("HELLO_RAID1")
	err := r.Write(data)
	assert.NoError(t, err)

	read, err := r.Read(0, len(data))
	assert.NoError(t, err)
	assert.Equal(t, data, read)
}

func TestRAID1_ReadAfterSingleDiskClear(t *testing.T) {
	r := raid.NewRAID1Controller(3)
	data := []byte("HELLO_RAID1")
	err := r.Write(data)
	assert.NoError(t, err)

	err = r.ClearDisk(1)
	assert.NoError(t, err)

	read, err := r.Read(0, len(data))
	assert.NoError(t, err)
	assert.Equal(t, data, read)
}

func TestRAID1_ReadAfterAllDiskClear(t *testing.T) {
	r := raid.NewRAID1Controller(3)
	data := []byte("HELLO_RAID1")
	err := r.Write(data)
	assert.NoError(t, err)

	err = r.ClearDisk(0)
	assert.NoError(t, err)
	err = r.ClearDisk(1)
	assert.NoError(t, err)
	err = r.ClearDisk(2)
	assert.NoError(t, err)

	_, err = r.Read(0, len(data))
	assert.Error(t, err)
}

func TestRAID1_PartialRead(t *testing.T) {
	r := raid.NewRAID1Controller(3)
	data := []byte("HELLO_RAID1")
	err := r.Write(data)
	assert.NoError(t, err)

	read, err := r.Read(6, 5) // Expecting "RAID1"
	assert.NoError(t, err)
	assert.Equal(t, []byte("RAID1"), read)
}
