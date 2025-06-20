package raid_test

import (
	"testing"

	"github.com/Anthya1104/raid-simulator/internal/raid"
	"github.com/stretchr/testify/assert"
)

func TestRAID10_WriteAndRead_Success(t *testing.T) {
	r, _ := raid.NewRAID10Controller(4, 2) // 2 mirrors, each with 2 disks
	data := []byte("ABCDEFGHIJKLMNOP")
	err := r.Write(data, 0)
	assert.NoError(t, err)

	read, err := r.Read(0, len(data))
	assert.NoError(t, err)
	assert.Equal(t, data, read)
}

func TestRAID10_PartialRead(t *testing.T) {
	r, _ := raid.NewRAID10Controller(4, 2)
	data := []byte("1234567890")
	err := r.Write(data, 0)
	assert.NoError(t, err)

	read, err := r.Read(4, 4)
	assert.NoError(t, err)
	assert.Equal(t, []byte("5678"), read)
}

func TestRAID10_ReadAfterDiskClear(t *testing.T) {
	r, _ := raid.NewRAID10Controller(4, 2)
	data := []byte("ABCDEFGH")
	err := r.Write(data, 0)
	assert.NoError(t, err)

	err = r.ClearDisk(0) // clear one primary
	assert.NoError(t, err)

	read, err := r.Read(0, len(data))
	assert.NoError(t, err)
	assert.Equal(t, data, read)
}
