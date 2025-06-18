package raid_test

import (
	"testing"

	"github.com/Anthya1104/raid-simulator/internal/raid"
	"github.com/stretchr/testify/assert"
)

func TestRAID0_WriteAndRead_Success(t *testing.T) {
	r := raid.NewRAID0Controller(3, 4)
	data := []byte("ABCDEFGH")
	err := r.Write(data, 0, 0)
	assert.NoError(t, err)

	read, err := r.Read(0, len(data))
	assert.NoError(t, err)
	assert.Equal(t, data, read)
}

func TestRAID0_ReadAfterClear_Fail(t *testing.T) {
	r := raid.NewRAID0Controller(3, 4)
	data := []byte("ABCDEFGHIJK")
	err := r.Write(data, 0, 0)
	assert.NoError(t, err)

	err = r.ClearDisk(0)
	assert.NoError(t, err)

	read, err := r.Read(0, len(data))
	assert.Error(t, err)
	assert.Nil(t, read)
}

func TestRAID0_ClearInvalidDisk(t *testing.T) {
	r := raid.NewRAID0Controller(3, 4)
	err := r.ClearDisk(-1)
	assert.Error(t, err)

	err = r.ClearDisk(3) // Index out of range
	assert.Error(t, err)
}

func TestRAID0_ReadPartialStripe(t *testing.T) {
	r := raid.NewRAID0Controller(3, 4)
	data := []byte("ABCDEF")
	err := r.Write(data, 0, 0)
	assert.NoError(t, err)

	read, err := r.Read(0, 3)
	assert.NoError(t, err)
	assert.Equal(t, []byte("ABC"), read)
}

func TestRAID0_ReadOffsetInsideStripe(t *testing.T) {
	r := raid.NewRAID0Controller(3, 4)
	data := []byte("ABCDEFGH")
	err := r.Write(data, 0, 0)
	assert.NoError(t, err)

	read, err := r.Read(2, 4) // Expecting "CDEF"
	assert.NoError(t, err)
	assert.Equal(t, []byte("CD"), read)
}
