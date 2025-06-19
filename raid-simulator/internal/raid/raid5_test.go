package raid

// based on raid module to get internal info(disks, stripeSz etc.) for test

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func TestNewRAID5Controller(t *testing.T) {
	t.Run("ValidCreation", func(t *testing.T) {
		controller, err := NewRAID5Controller(3, 4) //3 disk, 4 bytes across each stripe
		assert.Nil(t, err)
		assert.NotNil(t, controller)
		assert.Equal(t, 3, len(controller.disks))
		assert.Equal(t, 4, controller.stripeSz)
		for i, disk := range controller.disks {
			assert.Equal(t, i, disk.ID)
			assert.Empty(t, disk.Data)
		}
	})

	t.Run("InvalidDiskCount", func(t *testing.T) {
		controller, err := NewRAID5Controller(1, 4) // 1 disk
		assert.NotNil(t, err)
		assert.Nil(t, controller)
		assert.Contains(t, err.Error(), "RAID5 requires at least 3 disks")
	})

	t.Run("InvalidStripeSizeZero", func(t *testing.T) {
		controller, err := NewRAID5Controller(3, 0)
		assert.NotNil(t, err)
		assert.Nil(t, controller)
		assert.Contains(t, err.Error(), "stripe size (chunk unit size) must be greater than 0")
	})

	t.Run("InvalidStripeSizeNegative", func(t *testing.T) {
		controller, err := NewRAID5Controller(3, -1)
		assert.NotNil(t, err)
		assert.Nil(t, controller)
		assert.Contains(t, err.Error(), "stripe size (chunk unit size) must be greater than 0")
	})
}

func TestRAID5_Write_FullStripes(t *testing.T) {
	t.Run("3Disks_StripeSz1_8Bytes", func(t *testing.T) {
		controller, err := NewRAID5Controller(3, 1)
		assert.Nil(t, err)

		data := []byte("ABCDEFGH") // 8 位元組
		err = controller.Write(data, 0)
		assert.Nil(t, err)

		// Stripe 0:
		// Disk 0: [P0]
		// Disk 1: [A]
		// Disk 2: [B]

		// Stripe 1:
		// Disk 0: [P0, C]
		// Disk 1: [A, P1]
		// Disk 2: [B, D]

		// Stripe 2:
		// Disk 0: [P0, C, E]
		// Disk 1: [A, P1, F]
		// Disk 2: [B, D, P2]

		// Stripe 3:
		// Disk 0: [P0, C, E, P3]
		// Disk 1: [A, P1, F, G]
		// Disk 2: [B, D, P2, H]

		// Check Disk 0
		assert.Equal(t, []byte{0x41 ^ 0x42}, controller.disks[0].Data[0])
		assert.Equal(t, []byte{'C'}, controller.disks[0].Data[1])
		assert.Equal(t, []byte{'E'}, controller.disks[0].Data[2])
		assert.Equal(t, []byte{0x47 ^ 0x48}, controller.disks[0].Data[3])

		// Check Disk 1
		assert.Equal(t, []byte{'A'}, controller.disks[1].Data[0])
		assert.Equal(t, []byte{0x43 ^ 0x44}, controller.disks[1].Data[1])
		assert.Equal(t, []byte{'F'}, controller.disks[1].Data[2])
		assert.Equal(t, []byte{'G'}, controller.disks[1].Data[3])

		// Check Disk 2
		assert.Equal(t, []byte{'B'}, controller.disks[2].Data[0])
		assert.Equal(t, []byte{'D'}, controller.disks[2].Data[1])
		assert.Equal(t, []byte{0x45 ^ 0x46}, controller.disks[2].Data[2])
		assert.Equal(t, []byte{'H'}, controller.disks[2].Data[3])

		// make sure same distributed-blocks in each disk
		assert.Equal(t, 4, len(controller.disks[0].Data))
		assert.Equal(t, 4, len(controller.disks[1].Data))
		assert.Equal(t, 4, len(controller.disks[2].Data))
	})

	t.Run("4Disks_StripeSz2_12Bytes", func(t *testing.T) {
		controller, err := NewRAID5Controller(4, 2)
		assert.Nil(t, err)

		data := []byte("0123456789AB") // 12 bytes
		err = controller.Write(data, 0)
		assert.Nil(t, err)

		// Stripe 0:
		// D0: P0 (01^23^45)
		// D1: 01
		// D2: 23
		// D3: 45

		// Stripe 1:
		// D0: 67
		// D1: P1 (67^89^AB)
		// D2: 89
		// D3: AB

		// Check Disk 0
		assert.Equal(t, []byte{'0' ^ '2' ^ '4', '1' ^ '3' ^ '5'}, controller.disks[0].Data[0])
		assert.Equal(t, []byte{'6', '7'}, controller.disks[0].Data[1])

		// Check Disk 1
		assert.Equal(t, []byte{'0', '1'}, controller.disks[1].Data[0])
		assert.Equal(t, []byte{'6' ^ '8' ^ 'A', '7' ^ '9' ^ 'B'}, controller.disks[1].Data[1])

		// Check Disk 2
		assert.Equal(t, []byte{'2', '3'}, controller.disks[2].Data[0])
		assert.Equal(t, []byte{'8', '9'}, controller.disks[2].Data[1])

		// Check Disk 3
		assert.Equal(t, []byte{'4', '5'}, controller.disks[3].Data[0])
		assert.Equal(t, []byte{'A', 'B'}, controller.disks[3].Data[1])

		assert.Equal(t, 2, len(controller.disks[0].Data))
		assert.Equal(t, 2, len(controller.disks[1].Data))
		assert.Equal(t, 2, len(controller.disks[2].Data))
		assert.Equal(t, 2, len(controller.disks[3].Data))
	})
}

func TestRAID5_Write_PartialStripe_RMW(t *testing.T) {
	t.Run("InitialPartialWrite_1Byte", func(t *testing.T) {
		controller, err := NewRAID5Controller(3, 1)
		assert.Nil(t, err)

		data := []byte("A") // 1 bytes, partial stripe
		err = controller.Write(data, 0)
		assert.Nil(t, err)

		// Disk 0 (strip 1 parity):  A^0 = A
		// Disk 1: A
		// Disk 2: 0

		// Check Disk 0
		assert.Equal(t, []byte{'A' ^ 0x00}, controller.disks[0].Data[0])

		// Check Disk 1
		assert.Equal(t, []byte{'A'}, controller.disks[1].Data[0])

		// Check Disk 2
		assert.Equal(t, []byte{0x00}, controller.disks[2].Data[0])

		assert.Equal(t, 1, len(controller.disks[0].Data))
		assert.Equal(t, 1, len(controller.disks[1].Data))
		assert.Equal(t, 1, len(controller.disks[2].Data))
	})

	t.Run("AppendPartialWrite_1Byte", func(t *testing.T) {
		controller, err := NewRAID5Controller(3, 1)
		assert.Nil(t, err)

		dataInitial := []byte("ABCDEF") // 6 bytes
		err = controller.Write(dataInitial, 0)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(controller.disks[0].Data))

		dataAppend := []byte("G") // 1 bytes, partial stripe
		err = controller.Write(dataAppend, 6)
		assert.Nil(t, err)

		assert.Equal(t, 4, len(controller.disks[0].Data))
		assert.Equal(t, 4, len(controller.disks[1].Data))
		assert.Equal(t, 4, len(controller.disks[2].Data))

		assert.Equal(t, []byte{'G' ^ 0x00}, controller.disks[0].Data[3])
		assert.Equal(t, []byte{'G'}, controller.disks[1].Data[3])
		assert.Equal(t, []byte{0x00}, controller.disks[2].Data[3])
	})
}

func TestRAID5_WriteAndRead_Success(t *testing.T) {

	t.Run("WriteAndRead_Without_Partial_Writing", func(t *testing.T) {
		r, err := NewRAID5Controller(3, 1)
		assert.NoError(t, err)

		data := []byte("ABCDEFGH") // 8 bytes
		err = r.Write(data, 0)
		assert.NoError(t, err)

		read, err := r.Read(0, len(data))
		assert.NoError(t, err)
		assert.Equal(t, data, read)
	})

	t.Run("WriteAndRead_With_Partial_Writing", func(t *testing.T) {
		r, err := NewRAID5Controller(3, 1)
		assert.NoError(t, err)

		data := []byte("ABCDEFG") // 7 bytes
		err = r.Write(data, 0)
		assert.NoError(t, err)

		read, err := r.Read(0, len(data))
		assert.NoError(t, err)
		assert.Equal(t, data, read)
	})
}

func TestRAID5_Read_Success(t *testing.T) {
	controller, err := NewRAID5Controller(3, 1)
	assert.Nil(t, err)

	data := []byte("ABCDEFGH") // 8 bytes
	err = controller.Write(data, 0)
	assert.Nil(t, err)
	t.Run("ReadFullData", func(t *testing.T) {
		readData, err := controller.Read(0, 8)
		assert.Nil(t, err)
		assert.Equal(t, data, readData)
	})

	// partial read from middle index
	t.Run("ReadPartialData_Middle", func(t *testing.T) {
		readData, err := controller.Read(2, 4)
		assert.Nil(t, err)
		assert.Equal(t, []byte("CDEF"), readData)
	})

	// partial read from the final part
	t.Run("ReadPartialData_End", func(t *testing.T) {
		readData, err := controller.Read(6, 2)
		assert.Nil(t, err)
		assert.Equal(t, []byte("GH"), readData)
	})

	// offset partial read
	t.Run("ReadWithOffset", func(t *testing.T) {
		readData, err := controller.Read(1, 3)
		assert.Nil(t, err)
		assert.Equal(t, []byte("BCD"), readData)
	})
}

func TestRAID5_ClearDisk_Success(t *testing.T) {
	controller, err := NewRAID5Controller(3, 1)
	assert.Nil(t, err)

	data := []byte("ABCDEFGH")
	err = controller.Write(data, 0)
	assert.Nil(t, err)

	t.Run("ClearDisk0", func(t *testing.T) {
		err := controller.ClearDisk(0)
		assert.Nil(t, err)
		assert.Empty(t, controller.disks[0].Data)
		assert.Equal(t, 4, len(controller.disks[1].Data))
		assert.Equal(t, 4, len(controller.disks[2].Data))
	})

	t.Run("ClearNonExistentDisk", func(t *testing.T) {
		err := controller.ClearDisk(5)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "disk index 5 out of bounds")
	})
}

func TestRAID5_Read_SingleDiskFailure_Reconstruction(t *testing.T) {
	controller, err := NewRAID5Controller(3, 1)
	assert.Nil(t, err)

	data := []byte("ABCDEFGH")
	err = controller.Write(data, 0)
	assert.Nil(t, err)

	t.Run("Disk0Failure", func(t *testing.T) {
		ctrl, _ := NewRAID5Controller(3, 1)
		ctrl.Write(data, 0)

		err := ctrl.ClearDisk(0)
		assert.Nil(t, err)

		readData, err := ctrl.Read(0, 8)
		assert.Nil(t, err)
		assert.Equal(t, data, readData) // "ABCDEFGH"
	})

	t.Run("Disk1Failure", func(t *testing.T) {
		ctrl, _ := NewRAID5Controller(3, 1)
		ctrl.Write(data, 0)

		err := ctrl.ClearDisk(1)
		assert.Nil(t, err)

		readData, err := ctrl.Read(0, 8)
		assert.Nil(t, err)
		assert.Equal(t, data, readData) // "ABCDEFGH"
	})

	t.Run("Disk2Failure", func(t *testing.T) {
		ctrl, _ := NewRAID5Controller(3, 1)
		ctrl.Write(data, 0)

		err := ctrl.ClearDisk(2)
		assert.Nil(t, err)

		readData, err := ctrl.Read(0, 8)
		assert.Nil(t, err)
		assert.Equal(t, data, readData) //"ABCDEFGH"
	})
}

func TestRAID5_Read_MultipleDiskFailures(t *testing.T) {
	controller, err := NewRAID5Controller(3, 1)
	assert.Nil(t, err)

	data := []byte("ABCDEFGH")
	err = controller.Write(data, 0)
	assert.Nil(t, err)

	err = controller.ClearDisk(0)
	assert.Nil(t, err)
	err = controller.ClearDisk(1)
	assert.Nil(t, err)

	t.Run("TwoDiskFailure", func(t *testing.T) {
		readData, readErr := controller.Read(0, 8)
		assert.NotNil(t, readErr)
		assert.Contains(t, readErr.Error(), "multiple disk failures")
		assert.Empty(t, readData)
	})
}
