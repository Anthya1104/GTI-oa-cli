package raid

// based on raid module to read inside fields: controller.disks, controller.stripeSz

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRAID6Controller(t *testing.T) {
	t.Run("ValidCreationMinDisks", func(t *testing.T) {
		controller, err := NewRAID6Controller(4, 4) // 4 disks, 4 bytes per chunk
		assert.Nil(t, err, "Controller creation should not have an error")
		assert.NotNil(t, controller, "Controller should not be nil")
		assert.Equal(t, 4, len(controller.disks), "Number of disks should be 4") // Accessing unexported field 'disks'
		assert.Equal(t, 4, controller.stripeSz, "Stripe size should be 4")       // Accessing unexported field 'stripeSz'
		for i, disk := range controller.disks {                                  // Accessing unexported field 'disks'
			assert.Equal(t, i, disk.ID, fmt.Sprintf("Disk %d ID should be %d", i, i))
			assert.Empty(t, disk.Data, fmt.Sprintf("Disk %d data should be empty initially", i))
		}
	})

	t.Run("ValidCreationMoreDisks", func(t *testing.T) {
		controller, err := NewRAID6Controller(6, 8) // 6 disks, 8 bytes per chunk (4 data + 2 parity)
		assert.Nil(t, err, "Controller creation should not have an error with more disks")
		assert.NotNil(t, controller, "Controller should not be nil")
		assert.Equal(t, 6, len(controller.disks), "Number of disks should be 6") // Accessing unexported field 'disks'
		assert.Equal(t, 8, controller.stripeSz, "Stripe size should be 8")       // Accessing unexported field 'stripeSz'
	})

	t.Run("InvalidDiskCount", func(t *testing.T) {
		controller, err := NewRAID6Controller(3, 4) // 3 disks
		assert.NotNil(t, err, "Should have an error with invalid disk count")
		assert.Nil(t, controller, "Controller should be nil")
		assert.Contains(t, err.Error(), "RAID6 requires at least 4 disks", "Error message should mention disk count requirement")
	})

	t.Run("InvalidStripeSizeZero", func(t *testing.T) {
		controller, err := NewRAID6Controller(4, 0) // Stripe size 0
		assert.NotNil(t, err, "Should have an error with invalid stripe size")
		assert.Nil(t, controller, "Controller should be nil")
		assert.Contains(t, err.Error(), "stripe size (chunk unit size) must be greater than 0", "Error message should mention stripe size requirement")
	})

	t.Run("InvalidStripeSizeNegative", func(t *testing.T) {
		controller, err := NewRAID6Controller(4, -1) // Stripe size -1
		assert.NotNil(t, err, "Should have an error with invalid stripe size")
		assert.Nil(t, controller, "Controller should be nil")
		assert.Contains(t, err.Error(), "stripe size (chunk unit size) must be greater than 0", "Error message should mention stripe size requirement")
	})
}

func TestRAID6_WriteAndRead_Success(t *testing.T) {
	t.Run("WriteAndRead_FullStripes", func(t *testing.T) {
		r, err := NewRAID6Controller(4, 1) // 4 disks, stripe size 1 byte (2 data disks, 2 parity)
		assert.NoError(t, err, "Controller creation should not have an error")

		data := []byte("ABCDEFGH") // 8 bytes. Bytes per full stripe = 1 * (4-2) = 2 bytes. 8 bytes means 4 full stripes.
		err = r.Write(data, 0)
		assert.NoError(t, err, "Writing data should not have an error")

		read, err := r.Read(0, len(data))
		assert.NoError(t, err, "Reading data should not have an error")
		assert.Equal(t, data, read, "Read data should be identical to original data for full stripes")
	})

	t.Run("WriteAndRead_PartialStripe", func(t *testing.T) {
		r, err := NewRAID6Controller(4, 1) // 4 disks, stripe size 1 byte (2 data disks, 2 parity)
		assert.NoError(t, err, "Controller creation should not have an error")

		data := []byte("ABCDEFG") // 7 bytes. 3 full stripes + 1 byte partial.
		err = r.Write(data, 0)
		assert.NoError(t, err, "Writing partial data should not have an error")

		read, err := r.Read(0, len(data))
		assert.NoError(t, err, "Reading partial data should not have an error")
		assert.Equal(t, data, read, "Read data should be identical to original data for partial stripes")
	})

	t.Run("WriteAndRead_RMW", func(t *testing.T) {
		r, err := NewRAID6Controller(4, 2) // 4 disks, stripe size 2 bytes (2 data disks, 2 parity). Bytes per full stripe = 2 * 2 = 4 bytes.
		assert.NoError(t, err, "Controller creation should not have an error")

		initialData := []byte("01234567") // 8 bytes = 2 full stripes
		err = r.Write(initialData, 0)
		assert.NoError(t, err, "Initial write should not have an error")

		// Overwrite "45" (logical offset 4) with "XX". This affects the second stripe.
		// Original: "0123" (Stripe 0) "4567" (Stripe 1)
		// Overwrite:           "XX"
		// Expected: "0123XX67"
		overwritePart := []byte("XX")
		err = r.Write(overwritePart, 4) // Overwrite at logical offset 4 (start of second stripe)
		assert.NoError(t, err, "Overwrite should not have an error")

		expectedData := []byte("0123XX67")
		readData, err := r.Read(0, len(expectedData))
		assert.NoError(t, err, "Reading data after RMW should not have an error")
		assert.Equal(t, expectedData, readData, "Read data after RMW should match expected")
	})
}

func TestRAID6_ClearDisk_Success(t *testing.T) {
	controller, err := NewRAID6Controller(4, 1)
	assert.Nil(t, err)

	data := []byte("ABCDEFGH")
	err = controller.Write(data, 0)
	assert.Nil(t, err)

	t.Run("ClearDisk0", func(t *testing.T) {
		err := controller.ClearDisk(0)
		assert.Nil(t, err, "Clearing disk 0 should not have an error")
		assert.Empty(t, controller.disks[0].Data, "Disk 0's data should be empty after clearing")         // Accessing unexported field 'disks'
		assert.Equal(t, 4, len(controller.disks[1].Data), "Disk 1's block count should remain unchanged") // Accessing unexported field 'disks'
		assert.Equal(t, 4, len(controller.disks[2].Data), "Disk 2's block count should remain unchanged") // Accessing unexported field 'disks'
		assert.Equal(t, 4, len(controller.disks[3].Data), "Disk 3's block count should remain unchanged") // Accessing unexported field 'disks'
	})

	t.Run("ClearNonExistentDisk", func(t *testing.T) {
		err := controller.ClearDisk(5) // Index out of bounds
		assert.NotNil(t, err, "Clearing a non-existent disk should return an error")
		assert.Contains(t, err.Error(), "disk index 5 out of bounds", "Error message should indicate index out of bounds")
	})
}

func TestRAID6_Read_SingleDiskFailure_Reconstruction(t *testing.T) {
	data := []byte("The quick brown fox jumps over the lazy dog.") // Sample data

	// Test Disk 0 failure (data disk)
	t.Run("Disk0Failure", func(t *testing.T) {
		ctrl, err := NewRAID6Controller(4, 4) // 4 disks, stripe size 4 bytes (2 data, 2 parity)
		assert.Nil(t, err)
		err = ctrl.Write(data, 0)
		assert.Nil(t, err)

		err = ctrl.ClearDisk(0)
		assert.Nil(t, err, "Clearing disk 0 should not have an error")

		readData, err := ctrl.Read(0, len(data))
		assert.Nil(t, err, "Reading data after disk 0 failure should not have an error")
		assert.Equal(t, data, readData, "Data should be correctly reconstructed after disk 0 failure")
	})

	// Test Disk 2 failure (P parity disk)
	t.Run("Disk2Failure", func(t *testing.T) {
		ctrl, err := NewRAID6Controller(4, 4)
		assert.Nil(t, err)
		err = ctrl.Write(data, 0)
		assert.Nil(t, err)

		err = ctrl.ClearDisk(2) // Disk 2 is the P parity disk
		assert.Nil(t, err, "Clearing disk 2 should not have an error")

		readData, err := ctrl.Read(0, len(data))
		assert.Nil(t, err, "Reading data after disk 2 failure should not have an error")
		assert.Equal(t, data, readData, "Data should be correctly reconstructed after disk 2 failure")
	})

	// Test Disk 3 failure (Q parity disk)
	t.Run("Disk3Failure", func(t *testing.T) {
		ctrl, err := NewRAID6Controller(4, 4)
		assert.Nil(t, err)
		err = ctrl.Write(data, 0)
		assert.Nil(t, err)

		err = ctrl.ClearDisk(3) // Disk 3 is the Q parity disk
		assert.Nil(t, err, "Clearing disk 3 should not have an error")

		readData, err := ctrl.Read(0, len(data))
		assert.Nil(t, err, "Reading data after disk 3 failure should not have an error")
		assert.Equal(t, data, readData, "Data should be correctly reconstructed after disk 3 failure")
	})
}

func TestRAID6_Read_TwoDiskFailures_Reconstruction(t *testing.T) {
	data := []byte("RAID6 can survive two simultaneous disk failures and reconstruct all data!")

	t.Run("TwoDataDisksFailure", func(t *testing.T) {
		ctrl, err := NewRAID6Controller(4, 4) // 4 disks, stripe size 4 bytes (2 data, 2 parity)
		assert.Nil(t, err)
		err = ctrl.Write(data, 0)
		assert.Nil(t, err)

		err = ctrl.ClearDisk(0) // Clear Data Disk 0
		assert.Nil(t, err)
		err = ctrl.ClearDisk(1) // Clear Data Disk 1
		assert.Nil(t, err)

		readData, err := ctrl.Read(0, len(data))
		assert.Nil(t, err, "Reading data after two data disk failures should not have an error")
		assert.Equal(t, data, readData, "Data should be correctly reconstructed after two data disk failures")
	})

	t.Run("DataAndPParityDiskFailure", func(t *testing.T) {
		ctrl, err := NewRAID6Controller(4, 4)
		assert.Nil(t, err)
		err = ctrl.Write(data, 0)
		assert.Nil(t, err)

		err = ctrl.ClearDisk(0) // Clear Data Disk 0
		assert.Nil(t, err)
		err = ctrl.ClearDisk(2) // Clear P Parity Disk (Disk 2)
		assert.Nil(t, err)

		readData, err := ctrl.Read(0, len(data))
		assert.Nil(t, err, "Reading data after data and P parity disk failures should not have an error")
		assert.Equal(t, data, readData, "Data should be correctly reconstructed after data and P parity disk failures")
	})

	t.Run("BothParityDisksFailure", func(t *testing.T) {
		ctrl, err := NewRAID6Controller(4, 4)
		assert.Nil(t, err)
		err = ctrl.Write(data, 0)
		assert.Nil(t, err)

		err = ctrl.ClearDisk(2) // Clear P Parity Disk (Disk 2)
		assert.Nil(t, err)
		err = ctrl.ClearDisk(3) // Clear Q Parity Disk (Disk 3)
		assert.Nil(t, err)

		readData, err := ctrl.Read(0, len(data))
		assert.Nil(t, err, "Reading data after both parity disk failures should not have an error")
		assert.Equal(t, data, readData, "Data should be correctly reconstructed after both parity disk failures")
	})
}

func TestRAID6_Read_MultipleDiskFailures(t *testing.T) {
	data := []byte("This data should not be recoverable.")

	t.Run("ThreeDisksFailure", func(t *testing.T) {
		ctrl, err := NewRAID6Controller(4, 1) // 4 disks, stripe size 1 (2 data, 2 parity)
		assert.Nil(t, err)
		err = ctrl.Write(data, 0)
		assert.Nil(t, err)

		err = ctrl.ClearDisk(0) // Clear Disk 0
		assert.Nil(t, err)
		err = ctrl.ClearDisk(1) // Clear Disk 1
		assert.Nil(t, err)
		err = ctrl.ClearDisk(2) // Clear Disk 2
		assert.Nil(t, err)

		readData, readErr := ctrl.Read(0, len(data))
		assert.NotNil(t, readErr, "Three disk failures should return an error")
		assert.Contains(t, readErr.Error(), "too many missing shards", "Error message should indicate too many missing shards")
		assert.Empty(t, readData, "Three disk failures should not return any data")
	})
}

func TestRAID6_Read_OutOfBounds(t *testing.T) {
	controller, err := NewRAID6Controller(4, 1) // 4 disks, stripe size 1 (2 data, 2 parity). Bytes per full stripe = 2.
	assert.Nil(t, err)

	data := []byte("ABCDEFGH") // Total 8 bytes = 4 stripes of 2 data bytes
	err = controller.Write(data, 0)
	assert.Nil(t, err)

	// Read start offset beyond total data
	t.Run("ReadStartBeyondData", func(t *testing.T) {
		readData, readErr := controller.Read(10, 2)
		assert.NotNil(t, readErr, "Read should return an error when start offset is beyond data")
		assert.Contains(t, readErr.Error(), "read start offset 10 is beyond total data stored 8", "Error message should indicate start offset out of bounds")
		assert.Empty(t, readData, "Should not return any data when start offset is beyond data")
	})

	// Read partial length beyond total data (should truncate)
	t.Run("ReadPartialBeyondData", func(t *testing.T) {
		readData, readErr := controller.Read(6, 4)                                                              // Request 4 bytes from offset 6. Total data is 8 bytes.
		assert.Nil(t, readErr, "Partial read beyond data should not return an error")                           // Should be a warning (log), not an error
		assert.Equal(t, []byte("GH"), readData, "Partial read beyond data should return truncated data ('GH')") // Should only return "GH" (2 bytes)
	})

	// Read zero length
	t.Run("ReadZeroLength", func(t *testing.T) {
		readData, readErr := controller.Read(0, 0)
		assert.Nil(t, readErr, "Read should not have an error when length is zero")
		assert.Empty(t, readData, "Should return empty data when length is zero")
	})

	// Read negative length
	t.Run("ReadNegativeLength", func(t *testing.T) {
		readData, readErr := controller.Read(0, -1)
		assert.NotNil(t, readErr, "Read should return an error when length is negative")
		assert.Contains(t, readErr.Error(), "read start and length must be non-negative", "Error message should indicate negative length")
		assert.Empty(t, readData, "Should not return any data when length is negative")
	})
}

func TestRAID6_Read_NoDataWritten(t *testing.T) {
	controller, err := NewRAID6Controller(4, 1)
	assert.Nil(t, err)

	t.Run("ReadFromEmptyRAID", func(t *testing.T) {
		readData, readErr := controller.Read(0, 5)
		assert.NotNil(t, readErr, "Read from empty RAID should return an error")
		assert.Contains(t, readErr.Error(), "no data has been written to the RAID array yet", "Error message should indicate no data written")
		assert.Empty(t, readData, "Should not return any data when reading from empty RAID")
	})
}
