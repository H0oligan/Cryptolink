package evmcollector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// publicRPCFallbacks provides fallback public RPC endpoints for known chains.
var publicRPCFallbacks = map[string]string{
	"ETH":      "https://ethereum-rpc.publicnode.com",
	"MATIC":    "https://polygon-rpc.com",
	"BSC":      "https://bsc-dataseed.binance.org",
	"ARBITRUM": "https://arb1.arbitrum.io/rpc",
	"AVAX":     "https://api.avax.network/ext/bc/C/rpc",
	"BASE":     "https://mainnet.base.org",
}

var chainNativeTickers = map[string]string{
	"ETH":      "ETH",
	"MATIC":    "MATIC",
	"BSC":      "BNB",
	"ARBITRUM": "ETH",
	"AVAX":     "AVAX",
	"BASE":     "ETH",
}

// Known ERC-20 tokens per chain — matches frontend KNOWN_TOKENS in merchant-collector.ts
type knownToken struct {
	Address  string
	Ticker   string
	Decimals int
}

var knownERC20Tokens = map[string][]knownToken{
	"ETH": {
		{Address: "0xdAC17F958D2ee523a2206206994597C13D831ec7", Ticker: "USDT", Decimals: 6},
		{Address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", Ticker: "USDC", Decimals: 6},
	},
	"MATIC": {
		{Address: "0xc2132D05D31c914a87C6611C10748AEb04B58e8F", Ticker: "USDT", Decimals: 6},
		{Address: "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174", Ticker: "USDC", Decimals: 6},
	},
	"BSC": {
		{Address: "0x55d398326f99059fF775485246999027B3197955", Ticker: "USDT", Decimals: 18},
		{Address: "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d", Ticker: "USDC", Decimals: 18},
	},
	"ARBITRUM": {
		{Address: "0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9", Ticker: "USDT", Decimals: 6},
		{Address: "0xFF970A61A04b1cA14834A43f5dE4533eBDDB5CC8", Ticker: "USDC.e", Decimals: 6},
		{Address: "0xaf88d065e77c8cC2239327C5EDb3A432268e5831", Ticker: "USDC", Decimals: 6},
	},
	"AVAX": {
		{Address: "0x9702230A8Ea53601f5cD2dc00fDBc13d4dF4A8c7", Ticker: "USDT", Decimals: 6},
		{Address: "0xB97EF9Ef8734C71904D8002F8b6Bc66Dd9c48a6E", Ticker: "USDC", Decimals: 6},
	},
}

// FetchBalance queries the on-chain native balance for a collector contract.
// Token balances are not fetched server-side (done client-side via wallet provider).
func (s *Service) FetchBalance(ctx context.Context, blockchain, contractAddress string) (*OnChainBalance, error) {
	chain := strings.ToUpper(blockchain)

	// TRON uses REST API (TronGrid), not EVM JSON-RPC
	if chain == "TRON" {
		return tronGetBalance(ctx, contractAddress)
	}

	rpcURL := ""
	if cfg, ok := s.config.Chains[chain]; ok && cfg.RPCEndpoint != "" {
		rpcURL = cfg.RPCEndpoint
	}
	if rpcURL == "" {
		rpcURL = publicRPCFallbacks[chain]
	}

	ticker := chainNativeTickers[chain]
	if ticker == "" {
		ticker = chain
	}

	if rpcURL == "" {
		return &OnChainBalance{NativeAmount: "0", NativeTicker: ticker}, nil
	}

	hexBal, err := ethGetBalance(ctx, rpcURL, contractAddress)
	if err != nil {
		return nil, fmt.Errorf("eth_getBalance: %w", err)
	}

	bal := &OnChainBalance{
		NativeAmount: weiToDecimal(hexBal),
		NativeTicker: ticker,
	}

	// Query ERC-20 token balances
	for _, token := range knownERC20Tokens[chain] {
		hexTokenBal, err := ethCallBalanceOf(ctx, rpcURL, token.Address, contractAddress)
		if err != nil {
			continue // skip failed queries, don't block the whole response
		}
		amount := hexToDecimal(hexTokenBal, token.Decimals)
		if amount == "0" {
			continue
		}
		bal.Tokens = append(bal.Tokens, TokenBalance{
			ContractAddress: token.Address,
			Ticker:          token.Ticker,
			Amount:          amount,
			Decimals:        token.Decimals,
		})
	}

	return bal, nil
}

// ────────────────────────────────────────────────────────────────────────────
// JSON-RPC helpers
// ────────────────────────────────────────────────────────────────────────────

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type rpcResponse struct {
	Result string `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ethCallBalanceOf calls ERC-20 balanceOf(address) via eth_call.
// Returns the hex-encoded uint256 balance.
func ethCallBalanceOf(ctx context.Context, rpcURL, tokenContract, holder string) (string, error) {
	// balanceOf(address) selector = 0x70a08231 + left-padded address (32 bytes)
	paddedAddr := fmt.Sprintf("000000000000000000000000%s", strings.TrimPrefix(strings.ToLower(holder), "0x"))
	data := "0x70a08231" + paddedAddr

	payload, _ := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_call",
		Params: []interface{}{
			map[string]string{
				"to":   tokenContract,
				"data": data,
			},
			"latest",
		},
		ID: 1,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(payload))
	if err != nil {
		return "0x0", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "0x0", err
	}
	defer resp.Body.Close()

	var result rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "0x0", err
	}
	if result.Error != nil {
		return "0x0", fmt.Errorf("rpc: %s", result.Error.Message)
	}

	return result.Result, nil
}

// hexToDecimal converts a hex uint256 to a human-readable decimal string with given decimals.
func hexToDecimal(hexVal string, decimals int) string {
	hex := strings.TrimPrefix(hexVal, "0x")
	if hex == "" || hex == "0" {
		return "0"
	}

	n := new(big.Int)
	if _, ok := n.SetString(hex, 16); !ok {
		return "0"
	}
	if n.Sign() == 0 {
		return "0"
	}

	return bigIntToDecimal(n, decimals)
}

func ethGetBalance(ctx context.Context, rpcURL, address string) (string, error) {
	payload, _ := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_getBalance",
		Params:  []interface{}{address, "latest"},
		ID:      1,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(payload))
	if err != nil {
		return "0x0", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "0x0", err
	}
	defer resp.Body.Close()

	var result rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "0x0", err
	}
	if result.Error != nil {
		return "0x0", fmt.Errorf("rpc: %s", result.Error.Message)
	}

	return result.Result, nil
}

// ────────────────────────────────────────────────────────────────────────────
// TRON balance via TronGrid REST API
// ────────────────────────────────────────────────────────────────────────────

// Known TRC-20 tokens to check balance for
var tronKnownTokens = map[string]string{
	"TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t": "USDT",
	"TEkxiTehnzSmSe2XqrBj4w32RUN966rdz8": "USDC",
}

type tronGridResponse struct {
	Data []struct {
		Balance int64 `json:"balance"` // TRX in SUN (1 TRX = 1,000,000 SUN)
		TRC20   []map[string]string `json:"trc20"`
	} `json:"data"`
}

func tronGetBalance(ctx context.Context, base58Address string) (*OnChainBalance, error) {
	url := fmt.Sprintf("https://api.trongrid.io/v1/accounts/%s", base58Address)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("trongrid request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("trongrid fetch: %w", err)
	}
	defer resp.Body.Close()

	var result tronGridResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("trongrid decode: %w", err)
	}

	bal := &OnChainBalance{
		NativeAmount: "0",
		NativeTicker: "TRX",
	}

	if len(result.Data) == 0 {
		return bal, nil
	}

	account := result.Data[0]

	// TRX balance: SUN to TRX (6 decimals)
	bal.NativeAmount = sunToDecimal(account.Balance, 6)

	// TRC-20 token balances
	for _, tokenMap := range account.TRC20 {
		for contractAddr, rawAmount := range tokenMap {
			ticker, known := tronKnownTokens[contractAddr]
			if !known {
				continue
			}
			bal.Tokens = append(bal.Tokens, TokenBalance{
				ContractAddress: contractAddr,
				Ticker:          ticker,
				Amount:          sunToDecimal(0, 6), // parsed below
			})
			// Parse the string amount
			n := new(big.Int)
			if _, ok := n.SetString(rawAmount, 10); ok {
				bal.Tokens[len(bal.Tokens)-1].Amount = bigIntToDecimal(n, 6)
			}
		}
	}

	return bal, nil
}

// sunToDecimal converts a SUN int64 value to a decimal string (1 TRX = 10^6 SUN).
func sunToDecimal(sun int64, decimals int) string {
	return bigIntToDecimal(big.NewInt(sun), decimals)
}

// bigIntToDecimal converts a big.Int to a decimal string given the number of decimals.
func bigIntToDecimal(n *big.Int, decimals int) string {
	if n.Sign() == 0 {
		return "0"
	}
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	val := new(big.Float).Quo(new(big.Float).SetPrec(128).SetInt(n), divisor)
	return val.Text('f', 6)
}

// weiToDecimal converts a hex wei value (e.g. "0x38d7ea4c68000") to a decimal
// string with up to 6 significant decimal places (e.g. "0.001000").
func weiToDecimal(hexWei string) string {
	hex := strings.TrimPrefix(hexWei, "0x")
	if hex == "" || hex == "0" {
		return "0"
	}

	n := new(big.Int)
	n.SetString(hex, 16)

	if n.Sign() == 0 {
		return "0"
	}

	// 10^18
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	ethVal := new(big.Float).Quo(new(big.Float).SetPrec(128).SetInt(n), divisor)

	return ethVal.Text('f', 8)
}
