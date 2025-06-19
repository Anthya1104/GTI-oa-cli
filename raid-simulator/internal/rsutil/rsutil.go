package rsutil

import (
	"fmt"

	"github.com/klauspost/reedsolomon"
)

func EncodeStripeShards(inputData []byte, stripeSize int, encoder reedsolomon.Encoder, numDataShards, numParityShards int) ([][]byte, error) {

	shards := make([][]byte, numDataShards+numParityShards)

	for i := 0; i < numDataShards; i++ {
		shards[i] = make([]byte, stripeSize)

		chunkStart := i * stripeSize

		if chunkStart < len(inputData) {

			copy(shards[i], inputData[chunkStart:])
		}

	}

	for i := 0; i < numParityShards; i++ {
		shards[numDataShards+i] = make([]byte, stripeSize)
	}

	err := encoder.Encode(shards)
	if err != nil {
		return nil, fmt.Errorf("failed to encode shards: %w", err)
	}
	return shards, nil
}

func ReconstructStripeShards(shards [][]byte, encoder reedsolomon.Encoder, numParityShards int) error {
	missingShardCount := 0
	for _, shard := range shards {
		if shard == nil {
			missingShardCount++
		}
	}

	if missingShardCount == 0 {
		return nil
	}

	if missingShardCount > numParityShards {
		return fmt.Errorf("too many missing shards (%d), only %d parity shards available", missingShardCount, numParityShards)
	}

	err := encoder.Reconstruct(shards)
	if err != nil {
		return fmt.Errorf("failed to reconstruct shards: %w", err)
	}
	return nil
}
