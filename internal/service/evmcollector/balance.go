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

	// Query ERC-20 token balances — include all known tokens (even zero)
	// so the frontend can always display them based on merchant settings.
	for _, token := range knownERC20Tokens[chain] {
		amount := "0"
		hexTokenBal, err := ethCallBalanceOf(ctx, rpcURL, token.Address, contractAddress)
		if err == nil {
			amount = hexToDecimal(hexTokenBal, token.Decimals)
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

// Known TRC-20 tokens on TRON (base58 address → hex address, ticker, decimals)
type tronToken struct {
	Base58   string
	Hex      string // 41-prefixed hex (no 0x)
	Ticker   string
	Decimals int
}

var tronKnownTokensList = []tronToken{
	{Base58: "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", Hex: "41a614f803b6fd780986a42c78ec9c7f77e6ded13c", Ticker: "USDT", Decimals: 6},
	{Base58: "TEkxiTehnzSmSe2XqrBj4w32RUN966rdz8", Hex: "41382bb369637e33ada3072744a29c96e80087810e", Ticker: "USDC", Decimals: 6},
}

type tronGridAccountResponse struct {
	Data []struct {
		Balance int64 `json:"balance"` // TRX in SUN (1 TRX = 1,000,000 SUN)
	} `json:"data"`
}

// tronTriggerResponse is the response from /wallet/triggerconstantcontract
type tronTriggerResponse struct {
	Result struct {
		Result bool `json:"result"`
	} `json:"result"`
	ConstantResult []string `json:"constant_result"`
}

func tronGetBalance(ctx context.Context, base58Address string) (*OnChainBalance, error) {
	bal := &OnChainBalance{
		NativeAmount: "0",
		NativeTicker: "TRX",
	}

	// Convert the collector's base58 address to hex for smart contract calls
	holderHex := tronBase58ToHex(base58Address)

	// 1. Fetch native TRX balance via /v1/accounts/
	trxURL := fmt.Sprintf("https://api.trongrid.io/v1/accounts/%s", base58Address)
	trxReq, err := http.NewRequestWithContext(ctx, http.MethodGet, trxURL, nil)
	if err == nil {
		trxReq.Header.Set("Accept", "application/json")
		client := &http.Client{Timeout: 10 * time.Second}
		trxResp, err := client.Do(trxReq)
		if err == nil {
			defer trxResp.Body.Close()
			var result tronGridAccountResponse
			if err := json.NewDecoder(trxResp.Body).Decode(&result); err == nil && len(result.Data) > 0 {
				bal.NativeAmount = sunToDecimal(result.Data[0].Balance, 6)
			}
		}
	}

	// 2. Fetch TRC-20 token balances via direct balanceOf() calls.
	// The /v1/accounts/ trc20 field does NOT reliably index smart contract
	// token holdings (EIP-1167 clones), so we call each token contract directly.
	for _, token := range tronKnownTokensList {
		amount := tronCallBalanceOf(ctx, token.Hex, holderHex, token.Decimals)
		bal.Tokens = append(bal.Tokens, TokenBalance{
			ContractAddress: token.Base58,
			Ticker:          token.Ticker,
			Amount:          amount,
			Decimals:        token.Decimals,
		})
	}

	return bal, nil
}

// tronCallBalanceOf calls balanceOf(address) on a TRC-20 token contract via
// TronGrid's triggerconstantcontract endpoint (read-only, no energy cost).
func tronCallBalanceOf(ctx context.Context, tokenHex, holderHex string, decimals int) string {
	// ABI-encode: balanceOf(address) — strip 41 prefix from holder for 20-byte address
	holderAddr := strings.TrimPrefix(holderHex, "41")
	parameter := fmt.Sprintf("000000000000000000000000%s", holderAddr)

	payload, _ := json.Marshal(map[string]interface{}{
		"owner_address":    holderHex,
		"contract_address": tokenHex,
		"function_selector": "balanceOf(address)",
		"parameter":         parameter,
		"visible":           false,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.trongrid.io/wallet/triggerconstantcontract", bytes.NewReader(payload))
	if err != nil {
		return "0"
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "0"
	}
	defer resp.Body.Close()

	var result tronTriggerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "0"
	}

	if !result.Result.Result || len(result.ConstantResult) == 0 {
		return "0"
	}

	// constant_result[0] is a hex-encoded uint256
	return hexToDecimal("0x"+result.ConstantResult[0], decimals)
}

// tronBase58ToHex decodes a TRON base58check address to its 41-prefixed hex form.
func tronBase58ToHex(base58Addr string) string {
	decoded := base58Decode(base58Addr)
	if len(decoded) < 21 {
		return ""
	}
	// First 21 bytes are the address (41 prefix + 20-byte address), rest is checksum
	return fmt.Sprintf("%x", decoded[:21])
}

// base58Decode decodes a base58-encoded string (no check).
func base58Decode(input string) []byte {
	alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	result := big.NewInt(0)
	for _, c := range input {
		idx := strings.IndexRune(alphabet, c)
		if idx < 0 {
			return nil
		}
		result.Mul(result, big.NewInt(58))
		result.Add(result, big.NewInt(int64(idx)))
	}
	// Convert to bytes
	bz := result.Bytes()
	// Count leading '1's in input — they represent leading zero bytes
	numLeadingZeros := 0
	for _, c := range input {
		if c != '1' {
			break
		}
		numLeadingZeros++
	}
	out := make([]byte, numLeadingZeros+len(bz))
	copy(out[numLeadingZeros:], bz)
	return out
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
