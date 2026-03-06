package solana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Provider handles Solana RPC interactions
type Provider struct {
	config Config
	client *http.Client
	logger *zerolog.Logger
}

// Config configuration for Solana provider
type Config struct {
	// RPC endpoint (e.g., https://api.mainnet-beta.solana.com)
	RPCEndpoint string
	// Devnet RPC endpoint
	DevnetRPCEndpoint string
	// API key (if using paid RPC service like Helius, QuickNode)
	APIKey string
	// Timeout for RPC requests
	Timeout time.Duration
}

// New creates a new Solana provider
func New(config Config, logger *zerolog.Logger) *Provider {
	if config.RPCEndpoint == "" {
		config.RPCEndpoint = "https://api.mainnet-beta.solana.com"
	}
	if config.DevnetRPCEndpoint == "" {
		config.DevnetRPCEndpoint = "https://api.devnet.solana.com"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	log := logger.With().Str("channel", "solana_provider").Logger()

	return &Provider{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger: &log,
	}
}

// RPCRequest represents a Solana JSON-RPC request
type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// RPCResponse represents a Solana JSON-RPC response
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// callRPC makes a JSON-RPC call to Solana
func (p *Provider) callRPC(ctx context.Context, method string, params []interface{}, isTestnet bool) (json.RawMessage, error) {
	endpoint := p.config.RPCEndpoint
	if isTestnet {
		endpoint = p.config.DevnetRPCEndpoint
	}

	req := RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
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
	if p.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
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

// GetBalance returns the balance of a Solana account in lamports
func (p *Provider) GetBalance(ctx context.Context, address string, isTestnet bool) (uint64, error) {
	result, err := p.callRPC(ctx, "getBalance", []interface{}{address}, isTestnet)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get balance")
	}

	var balanceResp struct {
		Value uint64 `json:"value"`
	}

	if err := json.Unmarshal(result, &balanceResp); err != nil {
		return 0, errors.Wrap(err, "failed to unmarshal balance response")
	}

	return balanceResp.Value, nil
}

// GetTokenBalance returns the balance of an SPL token account
func (p *Provider) GetTokenBalance(ctx context.Context, tokenAccountAddress string, isTestnet bool) (uint64, string, error) {
	result, err := p.callRPC(ctx, "getTokenAccountBalance", []interface{}{tokenAccountAddress}, isTestnet)
	if err != nil {
		return 0, "", errors.Wrap(err, "failed to get token balance")
	}

	var tokenBalanceResp struct {
		Value struct {
			Amount         string `json:"amount"`
			Decimals       int    `json:"decimals"`
			UIAmountString string `json:"uiAmountString"`
		} `json:"value"`
	}

	if err := json.Unmarshal(result, &tokenBalanceResp); err != nil {
		return 0, "", errors.Wrap(err, "failed to unmarshal token balance response")
	}

	// Parse amount
	var amount uint64
	fmt.Sscanf(tokenBalanceResp.Value.Amount, "%d", &amount)

	return amount, tokenBalanceResp.Value.UIAmountString, nil
}

// GetRecentBlockhash returns the latest blockhash (required for transactions)
func (p *Provider) GetRecentBlockhash(ctx context.Context, isTestnet bool) (string, error) {
	result, err := p.callRPC(ctx, "getLatestBlockhash", []interface{}{
		map[string]string{"commitment": "finalized"},
	}, isTestnet)
	if err != nil {
		return "", errors.Wrap(err, "failed to get recent blockhash")
	}

	var blockhashResp struct {
		Value struct {
			Blockhash string `json:"blockhash"`
		} `json:"value"`
	}

	if err := json.Unmarshal(result, &blockhashResp); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal blockhash response")
	}

	return blockhashResp.Value.Blockhash, nil
}

// SendTransaction broadcasts a signed transaction to the Solana network
func (p *Provider) SendTransaction(ctx context.Context, signedTx []byte, isTestnet bool) (string, error) {
	// Encode transaction as base58
	encodedTx := base58.Encode(signedTx)

	result, err := p.callRPC(ctx, "sendTransaction", []interface{}{
		encodedTx,
		map[string]interface{}{
			"encoding": "base58",
		},
	}, isTestnet)
	if err != nil {
		return "", errors.Wrap(err, "failed to send transaction")
	}

	var signature string
	if err := json.Unmarshal(result, &signature); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal transaction signature")
	}

	p.logger.Info().
		Str("signature", signature).
		Bool("testnet", isTestnet).
		Msg("Transaction sent successfully")

	return signature, nil
}

// GetTransaction gets transaction details by signature
func (p *Provider) GetTransaction(ctx context.Context, signature string, isTestnet bool) (*TransactionInfo, error) {
	result, err := p.callRPC(ctx, "getTransaction", []interface{}{
		signature,
		map[string]string{
			"encoding":   "json",
			"commitment": "confirmed",
		},
	}, isTestnet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	var txInfo TransactionInfo
	if err := json.Unmarshal(result, &txInfo); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal transaction info")
	}

	return &txInfo, nil
}

// TransactionInfo represents transaction information from Solana
type TransactionInfo struct {
	Slot        uint64          `json:"slot"`
	BlockTime   int64           `json:"blockTime"`
	Meta        json.RawMessage `json:"meta"`
	Transaction json.RawMessage `json:"transaction"`
}

// ConfirmTransaction waits for transaction confirmation
func (p *Provider) ConfirmTransaction(ctx context.Context, signature string, isTestnet bool, maxRetries int) (bool, error) {
	for i := 0; i < maxRetries; i++ {
		result, err := p.callRPC(ctx, "getSignatureStatuses", []interface{}{
			[]string{signature},
		}, isTestnet)
		if err != nil {
			return false, errors.Wrap(err, "failed to get signature status")
		}

		var statusResp struct {
			Value []struct {
				Slot               uint64      `json:"slot"`
				Confirmations      *int        `json:"confirmations"`
				Err                interface{} `json:"err"`
				ConfirmationStatus string      `json:"confirmationStatus"`
			} `json:"value"`
		}

		if err := json.Unmarshal(result, &statusResp); err != nil {
			return false, errors.Wrap(err, "failed to unmarshal status response")
		}

		if len(statusResp.Value) > 0 && statusResp.Value[0].Slot > 0 {
			status := statusResp.Value[0]

			// Check for error
			if status.Err != nil {
				return false, fmt.Errorf("transaction failed: %v", status.Err)
			}

			// Check confirmation status
			if status.ConfirmationStatus == "confirmed" || status.ConfirmationStatus == "finalized" {
				p.logger.Info().
					Str("signature", signature).
					Str("status", status.ConfirmationStatus).
					Msg("Transaction confirmed")
				return true, nil
			}
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(2 * time.Second):
			// Continue loop
		}
	}

	return false, errors.New("transaction confirmation timeout")
}

// GetTokenAccountsByOwner gets all token accounts owned by an address
func (p *Provider) GetTokenAccountsByOwner(ctx context.Context, ownerAddress string, isTestnet bool) ([]TokenAccount, error) {
	result, err := p.callRPC(ctx, "getTokenAccountsByOwner", []interface{}{
		ownerAddress,
		map[string]interface{}{
			"programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", // SPL Token Program
		},
		map[string]interface{}{
			"encoding": "jsonParsed",
		},
	}, isTestnet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get token accounts")
	}

	var accountsResp struct {
		Value []struct {
			Pubkey  string `json:"pubkey"`
			Account struct {
				Data struct {
					Parsed struct {
						Info struct {
							Mint        string `json:"mint"`
							Owner       string `json:"owner"`
							TokenAmount struct {
								Amount         string `json:"amount"`
								Decimals       int    `json:"decimals"`
								UIAmountString string `json:"uiAmountString"`
							} `json:"tokenAmount"`
						} `json:"info"`
					} `json:"parsed"`
				} `json:"data"`
			} `json:"account"`
		} `json:"value"`
	}

	if err := json.Unmarshal(result, &accountsResp); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal token accounts")
	}

	var accounts []TokenAccount
	for _, acc := range accountsResp.Value {
		var amount uint64
		fmt.Sscanf(acc.Account.Data.Parsed.Info.TokenAmount.Amount, "%d", &amount)

		accounts = append(accounts, TokenAccount{
			Address:        acc.Pubkey,
			Mint:           acc.Account.Data.Parsed.Info.Mint,
			Owner:          acc.Account.Data.Parsed.Info.Owner,
			Amount:         amount,
			Decimals:       acc.Account.Data.Parsed.Info.TokenAmount.Decimals,
			UIAmountString: acc.Account.Data.Parsed.Info.TokenAmount.UIAmountString,
		})
	}

	return accounts, nil
}

// TokenAccount represents an SPL token account
type TokenAccount struct {
	Address        string
	Mint           string
	Owner          string
	Amount         uint64
	Decimals       int
	UIAmountString string
}
