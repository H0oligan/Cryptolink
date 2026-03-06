package monero

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Provider handles Monero wallet-RPC interactions
// Requires monero-wallet-rpc to be running
type Provider struct {
	config Config
	client *http.Client
	logger *zerolog.Logger
}

// Config configuration for Monero provider
type Config struct {
	// Wallet RPC endpoint (e.g., http://localhost:18082/json_rpc)
	WalletRPCEndpoint string
	// Testnet wallet RPC endpoint
	TestnetWalletRPCEndpoint string
	// RPC username (if authentication is enabled)
	RPCUsername string
	// RPC password (if authentication is enabled)
	RPCPassword string
	// Timeout for RPC requests
	Timeout time.Duration
}

// New creates a new Monero provider
func New(config Config, logger *zerolog.Logger) *Provider {
	if config.WalletRPCEndpoint == "" {
		config.WalletRPCEndpoint = "http://localhost:18082/json_rpc"
	}
	if config.TestnetWalletRPCEndpoint == "" {
		config.TestnetWalletRPCEndpoint = "http://localhost:28082/json_rpc"
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second // Monero operations can be slow
	}

	log := logger.With().Str("channel", "monero_provider").Logger()

	return &Provider{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger: &log,
	}
}

// RPCRequest represents a Monero JSON-RPC request
type RPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// RPCResponse represents a Monero JSON-RPC response
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("Monero RPC error %d: %s", e.Code, e.Message)
}

// callRPC makes a JSON-RPC call to monero-wallet-rpc
func (p *Provider) callRPC(ctx context.Context, method string, params interface{}, isTestnet bool) (json.RawMessage, error) {
	endpoint := p.config.WalletRPCEndpoint
	if isTestnet {
		endpoint = p.config.TestnetWalletRPCEndpoint
	}

	req := RPCRequest{
		JSONRPC: "2.0",
		ID:      "0",
		Method:  method,
		Params:  params,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request")
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create HTTP request")
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Add basic auth if configured
	if p.config.RPCUsername != "" && p.config.RPCPassword != "" {
		httpReq.SetBasicAuth(p.config.RPCUsername, p.config.RPCPassword)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, errors.Wrap(err, "RPC request failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	var rpcResp RPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}

	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}

	return rpcResp.Result, nil
}

// GetBalance returns the wallet balance in atomic units (piconero)
// 1 XMR = 1,000,000,000,000 piconero
func (p *Provider) GetBalance(ctx context.Context, accountIndex uint32, isTestnet bool) (uint64, uint64, error) {
	params := map[string]interface{}{
		"account_index": accountIndex,
	}

	result, err := p.callRPC(ctx, "get_balance", params, isTestnet)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to get balance")
	}

	var balanceResp struct {
		Balance          uint64 `json:"balance"`
		UnlockedBalance  uint64 `json:"unlocked_balance"`
		MultisigImported uint64 `json:"multisig_imported_amount"`
	}

	if err := json.Unmarshal(result, &balanceResp); err != nil {
		return 0, 0, errors.Wrap(err, "failed to unmarshal balance response")
	}

	return balanceResp.Balance, balanceResp.UnlockedBalance, nil
}

// GetAddress returns the primary address for an account
func (p *Provider) GetAddress(ctx context.Context, accountIndex uint32, isTestnet bool) (string, error) {
	params := map[string]interface{}{
		"account_index": accountIndex,
	}

	result, err := p.callRPC(ctx, "get_address", params, isTestnet)
	if err != nil {
		return "", errors.Wrap(err, "failed to get address")
	}

	var addressResp struct {
		Address string `json:"address"`
	}

	if err := json.Unmarshal(result, &addressResp); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal address response")
	}

	return addressResp.Address, nil
}

// CreateAccount creates a new account in the wallet
func (p *Provider) CreateAccount(ctx context.Context, label string, isTestnet bool) (uint32, string, error) {
	params := map[string]interface{}{
		"label": label,
	}

	result, err := p.callRPC(ctx, "create_account", params, isTestnet)
	if err != nil {
		return 0, "", errors.Wrap(err, "failed to create account")
	}

	var accountResp struct {
		AccountIndex uint32 `json:"account_index"`
		Address      string `json:"address"`
	}

	if err := json.Unmarshal(result, &accountResp); err != nil {
		return 0, "", errors.Wrap(err, "failed to unmarshal account response")
	}

	p.logger.Info().
		Uint32("account_index", accountResp.AccountIndex).
		Str("address", accountResp.Address).
		Msg("Created Monero account")

	return accountResp.AccountIndex, accountResp.Address, nil
}

// Transfer creates and sends a Monero transaction
func (p *Provider) Transfer(ctx context.Context, params TransferParams, isTestnet bool) (*TransferResult, error) {
	if err := params.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid transfer parameters")
	}

	rpcParams := map[string]interface{}{
		"destinations": []map[string]interface{}{
			{
				"amount":  params.Amount,
				"address": params.Destination,
			},
		},
		"account_index": params.AccountIndex,
		"priority":      params.Priority,
		"get_tx_key":    true,
		"get_tx_hex":    true,
	}

	if params.PaymentID != "" {
		rpcParams["payment_id"] = params.PaymentID
	}

	if params.UnlockTime > 0 {
		rpcParams["unlock_time"] = params.UnlockTime
	}

	result, err := p.callRPC(ctx, "transfer", rpcParams, isTestnet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to transfer")
	}

	var transferResp TransferResult
	if err := json.Unmarshal(result, &transferResp); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal transfer response")
	}

	p.logger.Info().
		Str("tx_hash", transferResp.TxHash).
		Uint64("amount", params.Amount).
		Str("destination", params.Destination).
		Msg("Monero transfer created")

	return &transferResp, nil
}

// TransferParams parameters for creating a Monero transfer
type TransferParams struct {
	Destination  string // Recipient address
	Amount       uint64 // Amount in piconero
	AccountIndex uint32 // Account to send from
	Priority     uint32 // 0=default, 1=unimportant, 2=normal, 3=elevated, 4=priority
	PaymentID    string // Optional payment ID (hex string)
	UnlockTime   uint64 // Optional unlock time (0 for standard)
}

func (t TransferParams) Validate() error {
	if t.Destination == "" {
		return errors.New("destination address required")
	}
	if t.Amount == 0 {
		return errors.New("amount must be greater than 0")
	}
	if t.Priority > 4 {
		return errors.New("priority must be between 0-4")
	}
	// Basic address validation (Monero addresses start with 4 or 8)
	if len(t.Destination) != 95 || (t.Destination[0] != '4' && t.Destination[0] != '8') {
		return errors.New("invalid Monero address format")
	}
	return nil
}

// TransferResult result of a Monero transfer
type TransferResult struct {
	Amount      uint64   `json:"amount"`
	Fee         uint64   `json:"fee"`
	TxBlob      string   `json:"tx_blob"`
	TxHash      string   `json:"tx_hash"`
	TxKey       string   `json:"tx_key"`
	TxMetadata  string   `json:"tx_metadata"`
	MultisigTxs []string `json:"multisig_txs"`
}

// GetTransfers retrieves incoming and outgoing transfers
func (p *Provider) GetTransfers(ctx context.Context, accountIndex uint32, isTestnet bool) (*TransferHistory, error) {
	params := map[string]interface{}{
		"in":            true,
		"out":           true,
		"pending":       true,
		"failed":        true,
		"pool":          true,
		"account_index": accountIndex,
	}

	result, err := p.callRPC(ctx, "get_transfers", params, isTestnet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transfers")
	}

	var history TransferHistory
	if err := json.Unmarshal(result, &history); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal transfer history")
	}

	return &history, nil
}

// TransferHistory represents transfer history
type TransferHistory struct {
	In      []Transfer `json:"in"`
	Out     []Transfer `json:"out"`
	Pending []Transfer `json:"pending"`
	Failed  []Transfer `json:"failed"`
	Pool    []Transfer `json:"pool"`
}

// Transfer represents a single Monero transfer
type Transfer struct {
	TxID            string `json:"txid"`
	PaymentID       string `json:"payment_id"`
	Height          uint64 `json:"height"`
	Timestamp       uint64 `json:"timestamp"`
	Amount          uint64 `json:"amount"`
	Fee             uint64 `json:"fee"`
	Note            string `json:"note"`
	Type            string `json:"type"`
	UnlockTime      uint64 `json:"unlock_time"`
	Address         string `json:"address"`
	DoubleSpendSeen bool   `json:"double_spend_seen"`
	Confirmations   uint64 `json:"confirmations"`
}

// ValidateAddress validates a Monero address
func (p *Provider) ValidateAddress(ctx context.Context, address string, isTestnet bool) (bool, bool, error) {
	params := map[string]interface{}{
		"address": address,
	}

	result, err := p.callRPC(ctx, "validate_address", params, isTestnet)
	if err != nil {
		return false, false, errors.Wrap(err, "failed to validate address")
	}

	var validateResp struct {
		Valid     bool `json:"valid"`
		Integrated bool `json:"integrated"`
		Subaddress bool `json:"subaddress"`
		Nettype   string `json:"nettype"`
	}

	if err := json.Unmarshal(result, &validateResp); err != nil {
		return false, false, errors.Wrap(err, "failed to unmarshal validation response")
	}

	return validateResp.Valid, validateResp.Integrated, nil
}

// Helper functions

// XMRToPiconero converts XMR to piconero (atomic units)
func XMRToPiconero(xmr float64) uint64 {
	return uint64(xmr * 1_000_000_000_000)
}

// PiconeroToXMR converts piconero to XMR
func PiconeroToXMR(piconero uint64) float64 {
	return float64(piconero) / 1_000_000_000_000
}

// GetMoneroExplorerURL returns the block explorer URL for a transaction
func GetMoneroExplorerURL(txHash string, isTestnet bool) string {
	if isTestnet {
		return fmt.Sprintf("https://testnet.xmrchain.net/tx/%s", txHash)
	}
	return fmt.Sprintf("https://xmrchain.net/tx/%s", txHash)
}
