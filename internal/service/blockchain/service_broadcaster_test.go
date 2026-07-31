package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/provider/rpc"
	"github.com/ethereum/go-ethereum"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures captured from BSC mainnet on 2026-07-31 — the exact transaction that
// the archive-gating incident stranded (payment 26d5f6e3, tx 1066).
const (
	testTxHash      = "0xd956d180f7533127ec59cb1885c46b16467d3ed890f0a6b89ad88e7933c0d62b"
	testTxBlock     = 0x6bef771
	testLatestBlock = 0x6bef8f1 // 384 blocks later, well past bscConfirmations (15)

	// The verbatim error bsc-rpc.publicnode.com returns for eth_getTransactionReceipt
	// while still answering eth_blockNumber, which is what let it keep passing the
	// dial-time health check.
	archiveGateMessage = "Archive requests require a personal token. Get one at: https://www.allnodes.com/publicnode"
)

const testTxJSON = `{
	"blockHash":"0xe54026f50a41f138538a9452d63f7b8725d98085028668124763b42b1e8f0321",
	"blockNumber":"0x6bef771",
	"from":"0xef3aeff9a5f61c6dda33069c58c1434006e13b20",
	"gas":"0x15f90",
	"gasPrice":"0x77359400",
	"hash":"0xd956d180f7533127ec59cb1885c46b16467d3ed890f0a6b89ad88e7933c0d62b",
	"input":"0xa9059cbb000000000000000000000000dd46c2751155e8411cc84c10c64bf0df2412b52100000000000000000000000000000000000000000000000347547a83401a4000",
	"nonce":"0x6b46c4",
	"to":"0x55d398326f99059ff775485246999027b3197955",
	"transactionIndex":"0x1e",
	"value":"0x0",
	"type":"0x0",
	"chainId":"0x38",
	"v":"0x93",
	"r":"0x75607aa8e5fb29b1a446d5fdf791f5ca65a42adc857114a225907de9af381f0c",
	"s":"0x47443f5b1999ee9882241ba65289790db8ac85a8b0be16af8d31462c2d5cf030"
}`

func testReceiptJSON() string {
	return fmt.Sprintf(`{
		"type":"0x0",
		"status":"0x1",
		"from":"0xef3aeff9a5f61c6dda33069c58c1434006e13b20",
		"to":"0x55d398326f99059ff775485246999027b3197955",
		"cumulativeGasUsed":"0x72058f",
		"gasUsed":"0xc9ab",
		"effectiveGasPrice":"0x77359400",
		"logsBloom":"0x%s",
		"logs":[],
		"contractAddress":null,
		"transactionHash":"%s",
		"blockHash":"0xe54026f50a41f138538a9452d63f7b8725d98085028668124763b42b1e8f0321",
		"blockNumber":"0x6bef771",
		"transactionIndex":"0x1e"
	}`, strings.Repeat("0", 512), testTxHash)
}

// stubOpts configures how an rpcStub answers eth_getTransactionReceipt.
type stubOpts struct {
	// receiptError, when set, is returned as a JSON-RPC error for
	// eth_getTransactionReceipt (simulating the paid-tier archive gate).
	receiptError string
	// receiptNull makes the endpoint answer "not mined yet" (null result),
	// which ethclient surfaces as ethereum.NotFound.
	receiptNull bool
}

// rpcStub is a fake EVM JSON-RPC endpoint that can be told to answer
// eth_blockNumber normally while refusing or nulling the receipt call — the
// asymmetry at the heart of the incident this file guards against.
type rpcStub struct {
	server *httptest.Server
	opts   stubOpts

	mu    sync.Mutex
	calls map[string]int
}

func newRPCStub(t *testing.T, opts stubOpts) *rpcStub {
	t.Helper()

	s := &rpcStub{
		calls: make(map[string]int),
		opts:  opts,
	}

	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.server.Close)

	return s
}

func (s *rpcStub) handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.calls[req.Method]++
	s.mu.Unlock()

	id := string(req.ID)
	if id == "" {
		id = "1"
	}

	w.Header().Set("Content-Type", "application/json")

	switch req.Method {
	case "eth_blockNumber":
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":"%#x"}`, id, testLatestBlock)
	case "eth_chainId":
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":"0x38"}`, id)
	case "eth_getTransactionByHash":
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, id, testTxJSON)
	case "eth_getTransactionReceipt":
		switch {
		case s.opts.receiptError != "":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32602,"message":%q}}`, id, s.opts.receiptError)
		case s.opts.receiptNull:
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":null}`, id)
		default:
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, id, testReceiptJSON())
		}
	default:
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32601,"message":"unsupported method %s"}}`, id, req.Method)
	}
}

func (s *rpcStub) url() string { return s.server.URL }

func (s *rpcStub) callCount(method string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.calls[method]
}

// newTestService wires a Service whose BSC endpoints are the given stubs, in
// order. Every ChainRPC slot is filled so applyDefaults never substitutes a
// real public endpoint and the test stays offline.
func newTestService(t *testing.T, endpoints ...string) *Service {
	t.Helper()
	require.GreaterOrEqual(t, len(endpoints), 2, "need a primary and at least one failover")

	resolver := NewCurrencies()
	require.NoError(t, DefaultSetup(resolver))

	logger := zerolog.Nop()

	cfg := rpc.Config{
		BSC: rpc.ChainRPC{
			Mainnet:  endpoints[0],
			Testnet:  endpoints[0],
			Fallback: endpoints[1],
			Extra:    endpoints[2:],
		},
		ConnTimeout: 5,
	}
	// Extra must be non-empty or applyChainDefaults refills it with the real
	// public endpoint list.
	if len(cfg.BSC.Extra) == 0 {
		cfg.BSC.Extra = []string{endpoints[1]}
	}

	return &Service{
		CurrencyResolver: resolver,
		providers:        Providers{RPC: rpc.New(cfg, &logger)},
		logger:           &logger,
	}
}

// An endpoint that passes the dial-time eth_blockNumber health check but refuses
// eth_getTransactionReceipt must be demoted and the lookup retried on the
// failover *within the same call* — otherwise the payment sits in inProgress
// until the 24h timeout cancels it, which is exactly what happened to BSC
// payments on 2026-07-31.
func TestGetEVMReceiptRotatesPastArchiveGatedEndpoint(t *testing.T) {
	gated := newRPCStub(t, stubOpts{receiptError: archiveGateMessage})
	healthy := newRPCStub(t, stubOpts{})

	svc := newTestService(t, gated.url(), healthy.url())

	nativeCoin, err := svc.GetNativeCoin(money.Blockchain("BSC"))
	require.NoError(t, err)

	receipt, err := svc.getEVMReceipt(
		context.Background(),
		svc.providers.RPC.BinanceSmartChainRPC,
		nativeCoin,
		testTxHash,
		15,
		false,
	)

	require.NoError(t, err, "lookup must survive an archive-gated primary")
	require.NotNil(t, receipt)

	assert.True(t, receipt.Success, "on-chain status was 0x1")
	assert.True(t, receipt.IsConfirmed, "%d confirmations is past the 15 BSC needs", receipt.Confirmations)
	assert.Equal(t, int64(testLatestBlock-testTxBlock), receipt.Confirmations)

	assert.Equal(t, 1, gated.callCount("eth_getTransactionReceipt"),
		"gated endpoint should be tried exactly once, then demoted")
	assert.Equal(t, 1, healthy.callCount("eth_getTransactionReceipt"),
		"lookup should have rotated to the failover")
}

// A demoted endpoint must stay demoted for subsequent lookups, so a stuck
// endpoint costs one wasted attempt, not one per transaction per tick.
func TestGetEVMReceiptKeepsGatedEndpointDemoted(t *testing.T) {
	gated := newRPCStub(t, stubOpts{receiptError: archiveGateMessage})
	healthy := newRPCStub(t, stubOpts{})

	svc := newTestService(t, gated.url(), healthy.url())

	nativeCoin, err := svc.GetNativeCoin(money.Blockchain("BSC"))
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err := svc.getEVMReceipt(
			context.Background(),
			svc.providers.RPC.BinanceSmartChainRPC,
			nativeCoin,
			testTxHash,
			15,
			false,
		)
		require.NoError(t, err, "lookup %d", i+1)
	}

	assert.Equal(t, 1, gated.callCount("eth_getTransactionReceipt"),
		"gated endpoint should not be re-tried within the health recovery interval")
	assert.Equal(t, 3, healthy.callCount("eth_getTransactionReceipt"))
}

// "Not mined yet" is a correct answer, not an endpoint failure. Rotating on it
// would make every pending transaction hammer all endpoints every 30s tick and
// demote perfectly healthy ones.
func TestGetEVMReceiptDoesNotRotateOnNotFound(t *testing.T) {
	primary := newRPCStub(t, stubOpts{receiptNull: true})
	failover := newRPCStub(t, stubOpts{receiptNull: true})

	svc := newTestService(t, primary.url(), failover.url())

	nativeCoin, err := svc.GetNativeCoin(money.Blockchain("BSC"))
	require.NoError(t, err)

	_, err = svc.getEVMReceipt(
		context.Background(),
		svc.providers.RPC.BinanceSmartChainRPC,
		nativeCoin,
		testTxHash,
		15,
		false,
	)

	require.ErrorIs(t, err, ethereum.NotFound)
	assert.Equal(t, 1, primary.callCount("eth_getTransactionReceipt"))
	assert.Equal(t, 0, failover.callCount("eth_getTransactionReceipt"),
		"a pending transaction must not fan out across endpoints")
}
