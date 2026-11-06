package repository_test

import (
	"context"
	"math/big"
	"math/rand"
	"testing"

	"github.com/cryptolink/cryptolink/internal/db/repository"
	"github.com/cryptolink/cryptolink/internal/test"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Amounts are stored as raw integers in the smallest unit, so a numeric that
// does not survive the driver exactly would corrupt balances.
func TestNumericRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := test.NewDB(ctx)
	t.Cleanup(db.TearDown)

	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	values := []string{
		"0", "1", "-1", "10", "1000000", "-1000000", "1000000000000000000",
		"123456789012345678901234567890", "1000000000000000000000000000000000000000",
		maxUint256.String(), "-" + maxUint256.String(),
	}

	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			want, ok := new(big.Int).SetString(value, 10)
			require.True(t, ok)

			// Encode with the helper, then let Postgres render the stored value.
			var asText string
			err := db.Conn().QueryRow(ctx, "select $1::numeric::text", repository.BigIntToNumeric(want)).Scan(&asText)
			require.NoError(t, err)
			assert.Equal(t, value, asText)

			// Decode a value Postgres produced from its text form.
			var decoded pgtype.Numeric
			err = db.Conn().QueryRow(ctx, "select $1::text::numeric", value).Scan(&decoded)
			require.NoError(t, err)

			got, err := repository.NumericToBigInt(decoded)
			require.NoError(t, err)
			assert.Zero(t, want.Cmp(got), "want %s, got %s", want, got)
		})
	}

	t.Run("random", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1))

		for i := 0; i < 5000; i++ {
			// Up to uint256, with trailing zeros that decode to a positive Exp.
			want := new(big.Int).Rand(rng, new(big.Int).Lsh(big.NewInt(1), uint(rng.Intn(257))))
			want.Mul(want, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(rng.Intn(20))), nil))
			if rng.Intn(2) == 0 {
				want.Neg(want)
			}

			var decoded pgtype.Numeric
			var asText string
			err := db.Conn().QueryRow(ctx, "select $1::numeric, $1::numeric::text", repository.BigIntToNumeric(want)).Scan(&decoded, &asText)
			require.NoError(t, err)
			require.Equal(t, want.String(), asText)

			got, err := repository.NumericToBigInt(decoded)
			require.NoError(t, err)
			require.Zero(t, want.Cmp(got), "want %s, got %s", want, got)
		}
	})

	t.Run("null", func(t *testing.T) {
		var decoded pgtype.Numeric
		require.NoError(t, db.Conn().QueryRow(ctx, "select null::numeric").Scan(&decoded))
		assert.False(t, decoded.Valid)

		_, err := repository.NumericToBigInt(decoded)
		assert.Error(t, err)

		var isNull bool
		require.NoError(t, db.Conn().QueryRow(ctx, "select $1::numeric is null", pgtype.Numeric{}).Scan(&isNull))
		assert.True(t, isNull)
	})
}

// Nil bytes must map to SQL NULL rather than to JSON null or an encoding error.
func TestJSONBRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := test.NewDB(ctx)
	t.Cleanup(db.TearDown)

	var isSQLNull bool
	require.NoError(t, db.Conn().QueryRow(ctx, "select $1::jsonb is null", []byte(nil)).Scan(&isSQLNull))
	assert.True(t, isSQLNull)

	var decoded []byte
	require.NoError(t, db.Conn().QueryRow(ctx, "select null::jsonb").Scan(&decoded))
	assert.Nil(t, decoded)

	require.NoError(t, db.Conn().QueryRow(ctx, "select $1::jsonb", []byte(`{"a": 1}`)).Scan(&decoded))
	assert.JSONEq(t, `{"a": 1}`, string(decoded))
}
