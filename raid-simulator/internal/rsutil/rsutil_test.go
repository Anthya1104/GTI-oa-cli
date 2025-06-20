package rsutil_test

import (
	"fmt"
	"testing"

	"github.com/Anthya1104/raid-simulator/internal/rsutil"
	"github.com/klauspost/reedsolomon"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func TestEncodeStripeShards(t *testing.T) {
	t.Run("RAID5_FullDataEncode", func(t *testing.T) {
		numDataShards := 2
		numParityShards := 1
		stripeSize := 1

		encoder, err := reedsolomon.New(numDataShards, numParityShards)
		assert.Nil(t, err)

		inputData := []byte("AB")

		shards, err := rsutil.EncodeStripeShards(inputData, stripeSize, encoder, numDataShards, numParityShards)
		assert.Nil(t, err)
		assert.Equal(t, numDataShards+numParityShards, len(shards))
		assert.Equal(t, []byte("A"), shards[0], "The first shard should be 'A'")
		assert.Equal(t, []byte("B"), shards[1], "The second shard should be 'B'")

		assert.NotNil(t, shards[2])
		assert.Equal(t, stripeSize, len(shards[2]))

		// copy the shard, and monitor the case that any shard lost
		testShards := make([][]byte, len(shards))
		for k, s := range shards {
			if s != nil {
				testShards[k] = make([]byte, len(s))
				copy(testShards[k], s)
			}
		}

		testShards[0] = nil // if the first shard ('A') lost

		reconstructErr := rsutil.ReconstructStripeShards(testShards, encoder, numParityShards)
		assert.Nil(t, reconstructErr)

		// check the rconstructed data in same value
		assert.Equal(t, []byte("A"), testShards[0])
		assert.Equal(t, []byte("B"), testShards[1])
	})

	t.Run("RAID6_FullDataEncode", func(t *testing.T) {
		numDataShards := 2
		numParityShards := 2
		stripeSize := 1

		encoder, err := reedsolomon.New(numDataShards, numParityShards)
		assert.Nil(t, err)

		inputData := []byte("CD")

		shards, err := rsutil.EncodeStripeShards(inputData, stripeSize, encoder, numDataShards, numParityShards)
		assert.Nil(t, err)
		assert.Equal(t, numDataShards+numParityShards, len(shards))
		assert.Equal(t, []byte("C"), shards[0], "The first shard should be 'C'")
		assert.Equal(t, []byte("D"), shards[1], "The second shard should be 'D'")

		assert.NotNil(t, shards[2], "the first parity shard (P) should not be nil")
		assert.Equal(t, stripeSize, len(shards[2]))
		assert.NotNil(t, shards[3], "the second parity shard (Q) should not be nil")
		assert.Equal(t, stripeSize, len(shards[3]))

		testShards := make([][]byte, len(shards))
		for k, s := range shards {
			if s != nil {
				testShards[k] = make([]byte, len(s))
				copy(testShards[k], s)
			}
		}

		testShards[0] = nil // if the first shard ('C') lost
		testShards[3] = nil // if the second shard ('Q') lost

		reconstructErr := rsutil.ReconstructStripeShards(testShards, encoder, numParityShards)
		assert.Nil(t, reconstructErr)

		assert.Equal(t, []byte("C"), testShards[0])
		assert.Equal(t, []byte("D"), testShards[1])
	})

	t.Run("PartialDataEncode_ZeroPadding", func(t *testing.T) {
		numDataShards := 2
		numParityShards := 1
		stripeSize := 2

		encoder, err := reedsolomon.New(numDataShards, numParityShards)
		assert.Nil(t, err)

		inputData := []byte("E") // 1 byte could not fill two shards (4 bytes)

		shards, err := rsutil.EncodeStripeShards(inputData, stripeSize, encoder, numDataShards, numParityShards)
		assert.Nil(t, err)
		assert.Equal(t, numDataShards+numParityShards, len(shards))

		assert.Equal(t, []byte{'E', 0x00}, shards[0], "The first shard should be 'E' and fill with 0x00")
		assert.Equal(t, []byte{0x00, 0x00}, shards[1], "The second shard should be fill with 0x00")

		assert.NotNil(t, shards[2])
		assert.Equal(t, stripeSize, len(shards[2]))

		testShards := make([][]byte, len(shards))
		for k, s := range shards {
			if s != nil {
				testShards[k] = make([]byte, len(s))
				copy(testShards[k], s)
			}
		}
		testShards[1] = nil

		reconstructErr := rsutil.ReconstructStripeShards(testShards, encoder, numParityShards)
		assert.Nil(t, reconstructErr)
		assert.Equal(t, []byte{'E', 0x00}, testShards[0])
		assert.Equal(t, []byte{0x00, 0x00}, testShards[1])
	})
}

func TestReconstructStripeShards(t *testing.T) {
	numDataShards := 2
	numParityShards := 2
	stripeSize := 1

	encoder, err := reedsolomon.New(numDataShards, numParityShards)
	assert.Nil(t, err)

	inputData := []byte("XY")
	encodedShards, err := rsutil.EncodeStripeShards(inputData, stripeSize, encoder, numDataShards, numParityShards)
	assert.Nil(t, err)

	originalShards := make([][]byte, len(encodedShards))
	for i, shard := range encodedShards {
		originalShards[i] = make([]byte, stripeSize)
		copy(originalShards[i], shard)
	}

	t.Run("NoMissingShards", func(t *testing.T) {
		shards := make([][]byte, len(originalShards))
		copy(shards, originalShards)

		err := rsutil.ReconstructStripeShards(shards, encoder, numParityShards)
		assert.Nil(t, err)
		assert.Equal(t, originalShards, shards)
	})

	t.Run("OneMissingDataShard", func(t *testing.T) {
		shards := make([][]byte, len(originalShards))
		copy(shards, originalShards)
		shards[0] = nil

		err := rsutil.ReconstructStripeShards(shards, encoder, numParityShards)
		assert.Nil(t, err)
		assert.Equal(t, originalShards[0], shards[0])
		assert.Equal(t, originalShards, shards)
	})

	t.Run("OneMissingParityShard", func(t *testing.T) {
		shards := make([][]byte, len(originalShards))
		copy(shards, originalShards)
		shards[numDataShards] = nil

		err := rsutil.ReconstructStripeShards(shards, encoder, numParityShards)
		assert.Nil(t, err)
		assert.Equal(t, originalShards[numDataShards], shards[numDataShards])
		assert.Equal(t, originalShards, shards)
	})

	t.Run("TwoMissingDataShards", func(t *testing.T) {
		shards := make([][]byte, len(originalShards))
		copy(shards, originalShards)
		shards[0] = nil
		shards[1] = nil

		err := rsutil.ReconstructStripeShards(shards, encoder, numParityShards)
		assert.Nil(t, err)
		assert.Equal(t, originalShards[0], shards[0])
		assert.Equal(t, originalShards[1], shards[1])
		assert.Equal(t, originalShards, shards)
	})

	t.Run("OneDataAndOneParityShardMissing", func(t *testing.T) {
		shards := make([][]byte, len(originalShards))
		copy(shards, originalShards)
		shards[0] = nil
		shards[numDataShards+1] = nil

		err := rsutil.ReconstructStripeShards(shards, encoder, numParityShards)
		assert.Nil(t, err)
		assert.Equal(t, originalShards[0], shards[0])
		assert.Equal(t, originalShards[numDataShards+1], shards[numDataShards+1])
		assert.Equal(t, originalShards, shards)
	})

	t.Run("TooManyMissingShards", func(t *testing.T) {
		shards := make([][]byte, len(originalShards))
		copy(shards, originalShards)
		shards[0] = nil
		shards[1] = nil
		shards[2] = nil

		err := rsutil.ReconstructStripeShards(shards, encoder, numParityShards)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("too many missing shards (%d), only %d parity shards available", 3, numParityShards))
	})
}

func TestRsutilEdgeCases(t *testing.T) {
	t.Run("StripeSize1", func(t *testing.T) {
		numDataShards := 2
		numParityShards := 2
		stripeSize := 1
		encoder, err := reedsolomon.New(numDataShards, numParityShards)
		assert.Nil(t, err)

		inputData := []byte("FG")
		shards, err := rsutil.EncodeStripeShards(inputData, stripeSize, encoder, numDataShards, numParityShards)
		assert.Nil(t, err)

		shardsCopy := make([][]byte, len(shards))
		for i, s := range shards {
			shardsCopy[i] = make([]byte, stripeSize)
			copy(shardsCopy[i], s)
		}

		shardsCopy[0] = nil
		shardsCopy[2] = nil

		err = rsutil.ReconstructStripeShards(shardsCopy, encoder, numParityShards)
		assert.Nil(t, err)
		assert.Equal(t, shards, shardsCopy, "stripeSize=1 should be reconstructed")
	})

	t.Run("EncodeEmptyInputData", func(t *testing.T) {
		numDataShards := 2
		numParityShards := 1
		stripeSize := 4
		encoder, err := reedsolomon.New(numDataShards, numParityShards)
		assert.Nil(t, err)

		inputData := []byte{}
		shards, err := rsutil.EncodeStripeShards(inputData, stripeSize, encoder, numDataShards, numParityShards)
		assert.Nil(t, err)

		assert.Equal(t, []byte{0, 0, 0, 0}, shards[0])
		assert.Equal(t, []byte{0, 0, 0, 0}, shards[1])
		assert.Equal(t, []byte{0, 0, 0, 0}, shards[2])
	})

	t.Run("ReconstructAllNilShards", func(t *testing.T) {
		numDataShards := 2
		numParityShards := 2
		encoder, err := reedsolomon.New(numDataShards, numParityShards)
		assert.Nil(t, err)

		allNilShards := make([][]byte, numDataShards+numParityShards)

		err = rsutil.ReconstructStripeShards(allNilShards, encoder, numParityShards)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("too many missing shards (%d), only %d parity shards available", len(allNilShards), numParityShards))
	})
}
