package mock

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cryptolink/cryptolink/internal/service/processing"
	"github.com/cryptolink/cryptolink/internal/service/transaction"
	"github.com/cryptolink/cryptolink/internal/service/wallet"
	"github.com/cryptolink/cryptolink/internal/util"
	"golang.org/x/exp/slices"
)

// ProcessingProxyMock proxies some methods of processing.Service and mocks others
type ProcessingProxyMock struct {
	t                      *testing.T
	service                *processing.Service
	mu                     sync.RWMutex
	incomingCheckCalls     map[string]error
	expirationCheckCalls   map[string]error
}

func NewProcessingProxyMock(t *testing.T, service *processing.Service) *ProcessingProxyMock {
	return &ProcessingProxyMock{
		t:                      t,
		service:                service,
		incomingCheckCalls:     map[string]error{},
		expirationCheckCalls:   map[string]error{},
	}
}

func (m *ProcessingProxyMock) BatchCheckIncomingTransactions(_ context.Context, transactionIDs []int64) error {
	key := idsKey(transactionIDs)

	m.mu.RLock()
	defer m.mu.RUnlock()

	err, exists := m.incomingCheckCalls[key]
	if !exists {
		return fmt.Errorf("unexpected call (*ProcessingProxyMock).BatchCheckIncomingTransactions for %q", key)
	}

	return err
}

func (m *ProcessingProxyMock) BatchExpirePayments(_ context.Context, paymentIDs []int64) error {
	key := idsKey(paymentIDs)

	m.mu.RLock()
	defer m.mu.RUnlock()

	err, exists := m.expirationCheckCalls[key]
	if !exists {
		return fmt.Errorf("unexpected call (*ProcessingProxyMock).BatchExpirePayments for %q", key)
	}

	return err
}

func (m *ProcessingProxyMock) SetupBatchCheckIncomingTransactions(transactionIDs []int64, err error) {
	key := idsKey(transactionIDs)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.incomingCheckCalls[key] = err
}

func (m *ProcessingProxyMock) SetupBatchExpirePayments(paymentsIDs []int64, err error) {
	key := idsKey(paymentsIDs)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.expirationCheckCalls[key] = err
}

func (m *ProcessingProxyMock) ProcessInboundTransaction(
	ctx context.Context,
	tx *transaction.Transaction,
	wt *wallet.Wallet,
	input processing.Input,
) error {
	return m.service.ProcessInboundTransaction(ctx, tx, wt, input)
}

const empty = "[ <empty> ]"

func idsKey(ids []int64) string {
	if len(ids) == 0 {
		return empty
	}

	slices.Sort(ids)

	stringInts := util.MapSlice(ids, func(id int64) string { return strconv.Itoa(int(id)) })

	return "[" + strings.Join(stringInts, ", ") + "]"
}
