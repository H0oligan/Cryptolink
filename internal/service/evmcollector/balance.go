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
	"ETH":      "https://eth.llamarpc.com",
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

// FetchBalance queries the on-chain native ETH/native balance for a collector contract.
// Token balances are not fetched server-side (done client-side via wallet provider).
func (s *Service) FetchBalance(ctx context.Context, blockchain, contractAddress string) (*OnChainBalance, error) {
	chain := strings.ToUpper(blockchain)

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

	return &OnChainBalance{
		NativeAmount: weiToDecimal(hexBal),
		NativeTicker: ticker,
		Tokens:       nil,
	}, nil
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
