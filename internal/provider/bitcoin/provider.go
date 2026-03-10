// Package bitcoin provides BTC transaction broadcasting and address monitoring
// via free public APIs (Blockstream and mempool.space). No Bitcoin Core node required.
package bitcoin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Config for the Bitcoin provider.
type Config struct {
	// Primary API (Blockstream)
	BlockstreamURL     string `yaml:"blockstream_url" env:"BTC_BLOCKSTREAM_URL" env-default:"https://blockstream.info"`
	BlockstreamTestURL string `yaml:"blockstream_test_url" env:"BTC_BLOCKSTREAM_TEST_URL" env-default:"https://blockstream.info/testnet"`

	// Fallback API (mempool.space)
	MempoolURL     string `yaml:"mempool_url" env:"BTC_MEMPOOL_URL" env-default:"https://mempool.space"`
	MempoolTestURL string `yaml:"mempool_test_url" env:"BTC_MEMPOOL_TEST_URL" env-default:"https://mempool.space/testnet"`
}

// Provider interacts with Bitcoin blockchain via free public APIs.
type Provider struct {
	config     Config
	logger     *zerolog.Logger
	httpClient *http.Client
}

// AddressInfo contains balance and transaction info for a BTC address.
type AddressInfo struct {
	Address       string
	FundedSum     int64 // total satoshis received
	SpentSum      int64 // total satoshis spent
	Balance       int64 // funded - spent (satoshis)
	TxCount       int64
	MempoolTxs    int64 // unconfirmed transaction count
	MempoolFunded int64 // unconfirmed received satoshis
}

// TransactionInfo contains details about a BTC transaction.
type TransactionInfo struct {
	TxID          string
	Confirmed     bool
	BlockHeight   int64
	Confirmations int64
	Fee           int64 // satoshis
	Inputs        []TxIO
	Outputs       []TxIO
}

// TxIO represents a transaction input or output.
type TxIO struct {
	Address string
	Value   int64 // satoshis
}

func New(config Config, logger *zerolog.Logger) *Provider {
	log := logger.With().Str("channel", "bitcoin_provider").Logger()

	if config.BlockstreamURL == "" {
		config.BlockstreamURL = "https://blockstream.info"
	}
	if config.BlockstreamTestURL == "" {
		config.BlockstreamTestURL = "https://blockstream.info/testnet"
	}
	if config.MempoolURL == "" {
		config.MempoolURL = "https://mempool.space"
	}
	if config.MempoolTestURL == "" {
		config.MempoolTestURL = "https://mempool.space/testnet"
	}

	return &Provider{
		config: config,
		logger: &log,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     60 * time.Second,
			},
		},
	}
}

// BroadcastTransaction broadcasts a raw signed BTC transaction hex.
// Tries Blockstream first, falls back to mempool.space.
func (p *Provider) BroadcastTransaction(ctx context.Context, rawTxHex string, isTest bool) (string, error) {
	// Try Blockstream
	txID, err := p.broadcastViaBlockstream(ctx, rawTxHex, isTest)
	if err == nil {
		return txID, nil
	}
	p.logger.Warn().Err(err).Msg("Blockstream broadcast failed, trying mempool.space")

	// Fallback to mempool.space
	txID, err = p.broadcastViaMempool(ctx, rawTxHex, isTest)
	if err != nil {
		return "", errors.Wrap(err, "all BTC broadcast endpoints failed")
	}

	return txID, nil
}

// GetAddressInfo returns balance and tx info for a BTC address.
func (p *Provider) GetAddressInfo(ctx context.Context, address string, isTest bool) (*AddressInfo, error) {
	info, err := p.getAddressInfoBlockstream(ctx, address, isTest)
	if err == nil {
		return info, nil
	}
	p.logger.Debug().Err(err).Str("address", address).Msg("Blockstream address info failed, trying mempool.space")

	info, err = p.getAddressInfoMempool(ctx, address, isTest)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get BTC address info from any source")
	}

	return info, nil
}

// GetTransaction returns details about a BTC transaction.
func (p *Provider) GetTransaction(ctx context.Context, txID string, isTest bool) (*TransactionInfo, error) {
	info, err := p.getTransactionBlockstream(ctx, txID, isTest)
	if err == nil {
		return info, nil
	}
	p.logger.Debug().Err(err).Str("txid", txID).Msg("Blockstream tx info failed, trying mempool.space")

	info, err = p.getTransactionMempool(ctx, txID, isTest)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get BTC transaction from any source")
	}

	return info, nil
}

// GetBlockHeight returns the current Bitcoin block height.
func (p *Provider) GetBlockHeight(ctx context.Context, isTest bool) (int64, error) {
	baseURL := p.blockstreamBase(isTest)
	url := fmt.Sprintf("%s/api/blocks/tip/height", baseURL)

	body, err := p.doGet(ctx, url)
	if err != nil {
		// Try mempool.space fallback
		baseURL = p.mempoolBase(isTest)
		url = fmt.Sprintf("%s/api/blocks/tip/height", baseURL)
		body, err = p.doGet(ctx, url)
		if err != nil {
			return 0, errors.Wrap(err, "unable to get BTC block height")
		}
	}

	var height int64
	if err := json.Unmarshal(body, &height); err != nil {
		return 0, errors.Wrap(err, "unable to parse block height")
	}

	return height, nil
}

// --- Blockstream API methods ---

func (p *Provider) broadcastViaBlockstream(ctx context.Context, rawTxHex string, isTest bool) (string, error) {
	baseURL := p.blockstreamBase(isTest)
	url := fmt.Sprintf("%s/api/tx", baseURL)
	return p.postRawTx(ctx, url, rawTxHex)
}

func (p *Provider) getAddressInfoBlockstream(ctx context.Context, address string, isTest bool) (*AddressInfo, error) {
	baseURL := p.blockstreamBase(isTest)
	url := fmt.Sprintf("%s/api/address/%s", baseURL, address)
	return p.parseAddressResponse(ctx, url, address)
}

func (p *Provider) getTransactionBlockstream(ctx context.Context, txID string, isTest bool) (*TransactionInfo, error) {
	baseURL := p.blockstreamBase(isTest)
	url := fmt.Sprintf("%s/api/tx/%s", baseURL, txID)
	return p.parseTransactionResponse(ctx, url, txID, isTest)
}

// --- mempool.space API methods ---

func (p *Provider) broadcastViaMempool(ctx context.Context, rawTxHex string, isTest bool) (string, error) {
	baseURL := p.mempoolBase(isTest)
	url := fmt.Sprintf("%s/api/tx", baseURL)
	return p.postRawTx(ctx, url, rawTxHex)
}

func (p *Provider) getAddressInfoMempool(ctx context.Context, address string, isTest bool) (*AddressInfo, error) {
	baseURL := p.mempoolBase(isTest)
	url := fmt.Sprintf("%s/api/address/%s", baseURL, address)
	return p.parseAddressResponse(ctx, url, address)
}

func (p *Provider) getTransactionMempool(ctx context.Context, txID string, isTest bool) (*TransactionInfo, error) {
	baseURL := p.mempoolBase(isTest)
	url := fmt.Sprintf("%s/api/tx/%s", baseURL, txID)
	return p.parseTransactionResponse(ctx, url, txID, isTest)
}

// --- Shared helpers ---

func (p *Provider) postRawTx(ctx context.Context, url, rawTxHex string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(rawTxHex))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "broadcast request failed")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("broadcast returned %d: %s", resp.StatusCode, string(body))
	}

	txID := strings.TrimSpace(string(body))
	return txID, nil
}

// blockstreamAddressResponse matches Blockstream/mempool.space /api/address/:addr JSON.
type blockstreamAddressResponse struct {
	Address    string `json:"address"`
	ChainStats struct {
		FundedTxoCount int64 `json:"funded_txo_count"`
		FundedTxoSum   int64 `json:"funded_txo_sum"`
		SpentTxoCount  int64 `json:"spent_txo_count"`
		SpentTxoSum    int64 `json:"spent_txo_sum"`
		TxCount        int64 `json:"tx_count"`
	} `json:"chain_stats"`
	MempoolStats struct {
		FundedTxoCount int64 `json:"funded_txo_count"`
		FundedTxoSum   int64 `json:"funded_txo_sum"`
		SpentTxoCount  int64 `json:"spent_txo_count"`
		SpentTxoSum    int64 `json:"spent_txo_sum"`
		TxCount        int64 `json:"tx_count"`
	} `json:"mempool_stats"`
}

func (p *Provider) parseAddressResponse(ctx context.Context, url, address string) (*AddressInfo, error) {
	body, err := p.doGet(ctx, url)
	if err != nil {
		return nil, err
	}

	var resp blockstreamAddressResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errors.Wrap(err, "unable to parse address response")
	}

	return &AddressInfo{
		Address:       address,
		FundedSum:     resp.ChainStats.FundedTxoSum,
		SpentSum:      resp.ChainStats.SpentTxoSum,
		Balance:       resp.ChainStats.FundedTxoSum - resp.ChainStats.SpentTxoSum + resp.MempoolStats.FundedTxoSum - resp.MempoolStats.SpentTxoSum,
		TxCount:       resp.ChainStats.TxCount,
		MempoolTxs:    resp.MempoolStats.TxCount,
		MempoolFunded: resp.MempoolStats.FundedTxoSum,
	}, nil
}

// blockstreamTxResponse matches Blockstream/mempool.space /api/tx/:txid JSON.
type blockstreamTxResponse struct {
	TxID   string `json:"txid"`
	Fee    int64  `json:"fee"`
	Status struct {
		Confirmed   bool  `json:"confirmed"`
		BlockHeight int64 `json:"block_height"`
	} `json:"status"`
	Vin []struct {
		Prevout struct {
			ScriptPubKeyAddress string `json:"scriptpubkey_address"`
			Value               int64  `json:"value"`
		} `json:"prevout"`
	} `json:"vin"`
	Vout []struct {
		ScriptPubKeyAddress string `json:"scriptpubkey_address"`
		Value               int64  `json:"value"`
	} `json:"vout"`
}

func (p *Provider) parseTransactionResponse(ctx context.Context, url, txID string, isTest bool) (*TransactionInfo, error) {
	body, err := p.doGet(ctx, url)
	if err != nil {
		return nil, err
	}

	var resp blockstreamTxResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errors.Wrap(err, "unable to parse transaction response")
	}

	info := &TransactionInfo{
		TxID:        resp.TxID,
		Confirmed:   resp.Status.Confirmed,
		BlockHeight: resp.Status.BlockHeight,
		Fee:         resp.Fee,
	}

	// Calculate confirmations
	if resp.Status.Confirmed {
		height, err := p.GetBlockHeight(ctx, isTest)
		if err == nil && height > 0 && resp.Status.BlockHeight > 0 {
			info.Confirmations = height - resp.Status.BlockHeight + 1
		}
	}

	for _, vin := range resp.Vin {
		info.Inputs = append(info.Inputs, TxIO{
			Address: vin.Prevout.ScriptPubKeyAddress,
			Value:   vin.Prevout.Value,
		})
	}

	for _, vout := range resp.Vout {
		info.Outputs = append(info.Outputs, TxIO{
			Address: vout.ScriptPubKeyAddress,
			Value:   vout.Value,
		})
	}

	return info, nil
}

// GetRecentTransactions returns recent transactions for a BTC address (most recent first).
// Uses Blockstream/mempool.space /api/address/:addr/txs endpoint.
func (p *Provider) GetRecentTransactions(ctx context.Context, address string, isTest bool) ([]*TransactionInfo, error) {
	baseURL := p.blockstreamBase(isTest)
	url := fmt.Sprintf("%s/api/address/%s/txs", baseURL, address)

	body, err := p.doGet(ctx, url)
	if err != nil {
		// Fallback to mempool
		baseURL = p.mempoolBase(isTest)
		url = fmt.Sprintf("%s/api/address/%s/txs", baseURL, address)
		body, err = p.doGet(ctx, url)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get recent transactions")
		}
	}

	var txResponses []blockstreamTxResponse
	if err := json.Unmarshal(body, &txResponses); err != nil {
		return nil, errors.Wrap(err, "unable to parse transactions response")
	}

	var result []*TransactionInfo
	for _, resp := range txResponses {
		info := &TransactionInfo{
			TxID:        resp.TxID,
			Confirmed:   resp.Status.Confirmed,
			BlockHeight: resp.Status.BlockHeight,
			Fee:         resp.Fee,
		}
		for _, vin := range resp.Vin {
			info.Inputs = append(info.Inputs, TxIO{
				Address: vin.Prevout.ScriptPubKeyAddress,
				Value:   vin.Prevout.Value,
			})
		}
		for _, vout := range resp.Vout {
			info.Outputs = append(info.Outputs, TxIO{
				Address: vout.ScriptPubKeyAddress,
				Value:   vout.Value,
			})
		}
		result = append(result, info)
	}

	return result, nil
}

func (p *Provider) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "request failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read response body")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (p *Provider) blockstreamBase(isTest bool) string {
	if isTest {
		return p.config.BlockstreamTestURL
	}
	return p.config.BlockstreamURL
}

func (p *Provider) mempoolBase(isTest bool) string {
	if isTest {
		return p.config.MempoolTestURL
	}
	return p.config.MempoolURL
}
