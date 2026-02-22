package blockchain_test

import (
	"testing"

	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/service/blockchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBitcoinSupport(t *testing.T) {
	// Setup currency resolver
	resolver := blockchain.NewCurrencies()
	err := blockchain.DefaultSetup(resolver)
	require.NoError(t, err, "Failed to setup currencies")

	// Test 1: Bitcoin currency is loaded
	btc, err := resolver.GetCurrencyByTicker("BTC")
	require.NoError(t, err, "BTC should be available")
	assert.Equal(t, "BTC", btc.Ticker)
	assert.Equal(t, "Bitcoin", btc.BlockchainName)
	assert.Equal(t, money.Blockchain("BTC"), btc.Blockchain)
	assert.Equal(t, int64(8), btc.Decimals)
	assert.Equal(t, money.Coin, btc.Type)

	// Test 2: Bitcoin is in supported currencies list
	currencies := resolver.ListSupportedCurrencies(false)
	found := false
	for _, c := range currencies {
		if c.Ticker == "BTC" {
			found = true
			break
		}
	}
	assert.True(t, found, "BTC should be in supported currencies")

	// Test 3: Bitcoin is the native coin for BTC blockchain
	nativeCoin, err := resolver.GetNativeCoin(money.Blockchain("BTC"))
	require.NoError(t, err, "Should get BTC native coin")
	assert.Equal(t, "BTC", nativeCoin.Ticker)

	// Test 4: Payment link generation
	addr := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	// 0.001 BTC = 100000 satoshis
	amount, err := money.CryptoFromStringFloat(money.Blockchain("BTC"), "BTC", "0.001", 8)
	require.NoError(t, err)

	link, err := blockchain.CreatePaymentLink(addr, btc, amount, false)
	require.NoError(t, err, "Should create payment link")
	assert.Equal(t, "bitcoin:1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa?amount=0.001", link)

	// Test 5: Payment link for testnet
	linkTestnet, err := blockchain.CreatePaymentLink(addr, btc, amount, true)
	require.NoError(t, err, "Should create testnet payment link")
	assert.Equal(t, "bitcoin:1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa?amount=0.001", linkTestnet)

	// Test 6: Explorer link for mainnet
	txID := "f4184fc596403b9d638783cf57adfe4c75c605f6356fbc91338530e9831e9e16"
	explorerLink, err := blockchain.CreateExplorerTXLink(money.Blockchain("BTC"), "mainnet", txID)
	require.NoError(t, err, "Should create explorer link")
	assert.Equal(t, "https://blockchair.com/bitcoin/transaction/"+txID, explorerLink)

	// Test 7: Explorer link for testnet
	explorerLinkTestnet, err := blockchain.CreateExplorerTXLink(money.Blockchain("BTC"), "testnet", txID)
	require.NoError(t, err, "Should create testnet explorer link")
	assert.Equal(t, "https://blockchair.com/bitcoin/testnet/transaction/"+txID, explorerLinkTestnet)
}
