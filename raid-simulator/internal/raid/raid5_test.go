package raid

// based on raid module to read inside fields: controller.disks, controller.stripeSz

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
		controller, err := NewRAID5Controller(3, 4) // 3 disks, 4 bytes across each stripe
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

		data := []byte("ABCDEFGH") // 8 bytes
		err = controller.Write(data, 0)
		assert.Nil(t, err)

		// After writing, read the data back and verify its content
		readData, err := controller.Read(0, len(data))
		assert.Nil(t, err, "Should not have an error when reading data")
		assert.Equal(t, data, readData, "Read data should be consistent with original data")

		// Verify the length of each disk, no longer checking specific content
		assert.Equal(t, 4, len(controller.disks[0].Data), "Disk 0 should have 4 stripes")
		assert.Equal(t, 4, len(controller.disks[1].Data), "Disk 1 should have 4 stripes")
		assert.Equal(t, 4, len(controller.disks[2].Data), "Disk 2 should have 4 stripes")
	})

	t.Run("4Disks_StripeSz2_12Bytes", func(t *testing.T) {
		controller, err := NewRAID5Controller(4, 2)
		assert.Nil(t, err)

		data := []byte("0123456789AB") // 12 bytes
		err = controller.Write(data, 0)
		assert.Nil(t, err)

		// After writing, read the data back and verify its content
		readData, err := controller.Read(0, len(data))
		assert.Nil(t, err, "Should not have an error when reading data")
		assert.Equal(t, data, readData, "Read data should be consistent with original data")

		// Verify the length of each disk, no longer checking specific content
		assert.Equal(t, 2, len(controller.disks[0].Data), "Disk 0 should have 2 stripes")
		assert.Equal(t, 2, len(controller.disks[1].Data), "Disk 1 should have 2 stripes")
		assert.Equal(t, 2, len(controller.disks[2].Data), "Disk 2 should have 2 stripes")
		assert.Equal(t, 2, len(controller.disks[3].Data), "Disk 3 should have 2 stripes")
	})
}

func TestRAID5_Write_PartialStripe_RMW(t *testing.T) {
	t.Run("InitialPartialWrite_1Byte", func(t *testing.T) {
		controller, err := NewRAID5Controller(3, 1)
		assert.Nil(t, err)

		data := []byte("A") // 1 bytes, partial stripe
		err = controller.Write(data, 0)
		assert.Nil(t, err)

		// After writing, read the data back and verify its content
		readData, err := controller.Read(0, len(data))
		assert.Nil(t, err, "Should not have an error when reading data")
		assert.Equal(t, data, readData, "Read data should be consistent with original data")

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

		// After writing, read all data (dataInitial + dataAppend) and verify its content
		expectedData := append(dataInitial, dataAppend...)
		readData, err := controller.Read(0, len(expectedData))
		assert.Nil(t, err, "Should not have an error when reading data")
		assert.Equal(t, expectedData, readData, "Read data should be consistent with original data")

		assert.Equal(t, 4, len(controller.disks[0].Data))
		assert.Equal(t, 4, len(controller.disks[1].Data))
		assert.Equal(t, 4, len(controller.disks[2].Data))
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
	data := []byte("ABCDEFGH") // 8 bytes

	t.Run("Disk0Failure", func(t *testing.T) {
		ctrl, err := NewRAID5Controller(3, 1)
		assert.Nil(t, err)
		err = ctrl.Write(data, 0)
		assert.Nil(t, err)

		err = ctrl.ClearDisk(0)
		assert.Nil(t, err)

		readData, err := ctrl.Read(0, len(data))
		assert.Nil(t, err)
		assert.Equal(t, data, readData, "Should correctly read and reconstruct data after clearing disk 0")
	})

	t.Run("Disk1Failure", func(t *testing.T) {
		ctrl, err := NewRAID5Controller(3, 1)
		assert.Nil(t, err)
		err = ctrl.Write(data, 0)
		assert.Nil(t, err)

		err = ctrl.ClearDisk(1)
		assert.Nil(t, err)

		readData, err := ctrl.Read(0, len(data))
		assert.Nil(t, err)
		assert.Equal(t, data, readData, "Should correctly read and reconstruct data after clearing disk 1")
	})

	t.Run("Disk2Failure", func(t *testing.T) {
		ctrl, err := NewRAID5Controller(3, 1)
		assert.Nil(t, err)
		err = ctrl.Write(data, 0)
		assert.Nil(t, err)

		err = ctrl.ClearDisk(2)
		assert.Nil(t, err)

		readData, err := ctrl.Read(0, len(data))
		assert.Nil(t, err)
		assert.Equal(t, data, readData, "Should correctly read and reconstruct data after clearing disk 2")
	})
}

func TestRAID5_Read_MultipleDiskFailures(t *testing.T) {
	controller, err := NewRAID5Controller(3, 1)
	assert.Nil(t, err)

	data := []byte("ABCDEFGH")
	err = controller.Write(data, 0)
	assert.Nil(t, err)

	// Note: This test requires an independent controller instance because the clearing operation above
	// would affect subsequent tests. Alternatively, re-initialize the controller inside each sub-test.
	// To ensure test isolation, the controller is re-initialized within the sub-test here.
	t.Run("TwoDiskFailure", func(t *testing.T) {
		ctrl, err := NewRAID5Controller(3, 1)
		assert.Nil(t, err)
		err = ctrl.Write(data, 0)
		assert.Nil(t, err)

		err = ctrl.ClearDisk(0)
		assert.Nil(t, err)
		err = ctrl.ClearDisk(1)
		assert.Nil(t, err)

		readData, readErr := ctrl.Read(0, 8)
		assert.NotNil(t, readErr)
		assert.Contains(t, readErr.Error(), "too many missing shards", "Should return error for too many missing shards")
		assert.Empty(t, readData)
	})
}
