package xpub

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/btcutil/v2/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Seed of the standard "abandon ... about" BIP39 test mnemonic, no passphrase.
const testSeedHex = "5eb00bbddcf069084889a8ab9155568165f5c453ccb85e70811aaed6f6da5fc1" +
	"9a5ac40b389cd370d086206dec8aa6c43daea6690f20ad3d8d48b2d2ce9e38e4"

// Published BIP84 account key for the same mnemonic.
const bip84Zpub = "zpub6rFR7y4Q2AijBEqTUquhVz398htDFrtymD9xYYfG1m4wAcvPhXNfE3EfH1r1ADqtfSdVCToUG868RvUUkgDKf31mGDtKsAYz2oz2AGutZYs"

func accountXpub(t *testing.T, purpose uint32) string {
	t.Helper()

	seed, err := hex.DecodeString(testSeedHex)
	require.NoError(t, err)

	key, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	require.NoError(t, err)

	for _, child := range []uint32{purpose, 0, 0} {
		key, err = key.Derive(hdkeychain.HardenedKeyStart + child)
		require.NoError(t, err)
	}

	pub, err := key.Neuter()
	require.NoError(t, err)

	return pub.String()
}

// Customers pay to these addresses and merchants recover funds from their own
// wallets, so derivation must never change across library upgrades.
func TestDerivedAddressesAreStable(t *testing.T) {
	s := &Service{}

	testCases := []struct {
		name        string
		purpose     uint32
		path        string
		first       string
		fingerprint string
	}{
		{"BIP44", 44, "m/44'/0'/0'", "1LqBGSKuX5yYUonjxT5qGfpUsXKYYWeabA", "2e535a4e60263aac294d919b4e3e2cad8c034de8dce6bd8a7da8bc1d9319354e"},
		{"BIP49", 49, "m/49'/0'/0'", "37VucYSaXLCAsxYyAPfbSi9eh4iEcbShgf", "97c2c3c33500f0c8dd3e117024b0c81103b22208dbd96430aaed0c0b9f4e3183"},
		{"BIP84", 84, "m/84'/0'/0'", "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu", "c2d3619421a341496480d7745d9e8ad06ffcff655ec02e7d77ae441e94fe3e71"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			xpub := accountXpub(t, tc.purpose)

			first, _, err := s.deriveAddressFromXpub(xpub, "BTC", tc.path, 0)
			require.NoError(t, err)
			assert.Equal(t, tc.first, first)

			hash := sha256.New()
			for i := 0; i < 2000; i++ {
				address, pubKey, err := s.deriveAddressFromXpub(xpub, "BTC", tc.path, i)
				require.NoError(t, err)
				fmt.Fprintf(hash, "%d:%s:%s\n", i, address, pubKey)
			}
			assert.Equal(t, tc.fingerprint, hex.EncodeToString(hash.Sum(nil)))
		})
	}
}

func TestZpubConversionMatchesBIP84Vectors(t *testing.T) {
	converted, path, err := convertToXpub(bip84Zpub)
	require.NoError(t, err)
	assert.Equal(t, "m/84'/0'/0'", path)

	s := &Service{}
	for index, want := range map[int]string{
		0: "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu",
		1: "bc1qnjg0jd8228aq7egyzacy8cys3knf9xvrerkf9g",
	} {
		address, _, err := s.deriveAddressFromXpub(converted, "BTC", path, index)
		require.NoError(t, err)
		assert.Equal(t, want, address)
	}

	_, _, err = convertToXpub(bip84Zpub[:len(bip84Zpub)-1] + "t")
	assert.Error(t, err, "a corrupted key must fail the checksum")
}
