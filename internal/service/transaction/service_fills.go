package transaction

import (
	"context"
	"database/sql"
	"math/big"
	"time"

	"github.com/jackc/pgtype"
	"github.com/cryptolink/cryptolink/internal/db/repository"
	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/pkg/errors"
)

// Fill represents one on-chain transfer that contributed to an invoice.
// A single invoice can have many fills when a customer underpays then tops up.
type Fill struct {
	ID              int64
	TransactionID   int64
	NetworkID       string
	TransactionHash string
	VoutOrLogIdx    int32
	Amount          money.Money
	SenderAddress   *string
	BlockNumber     *int64
	Confirmations   int32
	Status          string // observed | confirmed | reorged
	ObservedAt      time.Time
	ConfirmedAt     *time.Time
}

const (
	FillStatusObserved  = "observed"
	FillStatusConfirmed = "confirmed"
	FillStatusReorged   = "reorged"
)

// RecordFill inserts a fill against a parent transaction. Idempotent on
// (transaction_id, network_id, transaction_hash, vout_or_logidx) — repeat
// detections during watcher polling cycles or block reorgs that re-include
// the same tx are absorbed without double-counting.
//
// confirmations is the on-chain confirmation count at time of detection.
// When confirmations >= chain's required threshold, pass status=confirmed so
// the fill counts toward the cumulative paid amount.
func (s *Service) RecordFill(
	ctx context.Context,
	parentTx *Transaction,
	networkID, txHash string,
	voutOrLogIdx int32,
	amount money.Money,
	senderAddress string,
	blockNumber int64,
	confirmations int32,
	status string,
) (*Fill, error) {
	if !parentTx.Amount.CompatibleTo(amount) {
		return nil, errors.New("fill amount currency mismatch with parent transaction")
	}

	senderNS := sql.NullString{String: senderAddress, Valid: senderAddress != ""}
	blockNS := sql.NullInt64{Int64: blockNumber, Valid: blockNumber > 0}

	row, err := s.store.InsertTransactionFill(ctx, repository.InsertTransactionFillParams{
		TransactionID:   parentTx.ID,
		NetworkID:       networkID,
		TransactionHash: txHash,
		VoutOrLogIdx:    voutOrLogIdx,
		Amount:          repository.MoneyToNumeric(amount),
		SenderAddress:   senderNS,
		BlockNumber:     blockNS,
		Confirmations:   confirmations,
		Status:          status,
		ObservedAt:      time.Now(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to insert transaction fill")
	}

	return s.fillFromRepo(parentTx, row), nil
}

// SumConfirmedFills returns the cumulative confirmed amount across all fills
// for a given transaction. Used to drive the partial→full promotion check.
func (s *Service) SumConfirmedFills(ctx context.Context, parentTx *Transaction) (money.Money, error) {
	num, err := s.store.SumConfirmedFillsForTx(ctx, parentTx.ID)
	if err != nil {
		return money.Money{}, errors.Wrap(err, "unable to sum confirmed fills")
	}
	return numericToCryptoSameDecimals(num, parentTx.Amount)
}

// SumAllFills returns the cumulative amount across all observed (non-reorged)
// fills, including unconfirmed ones. Surfaced on the customer-facing payment
// page so a freshly broadcast top-up is visible immediately even before its
// confirmations land.
func (s *Service) SumAllFills(ctx context.Context, parentTx *Transaction) (money.Money, error) {
	num, err := s.store.SumAllFillsForTx(ctx, parentTx.ID)
	if err != nil {
		return money.Money{}, errors.Wrap(err, "unable to sum fills")
	}
	return numericToCryptoSameDecimals(num, parentTx.Amount)
}

// ListFills returns the per-fill history for a transaction in observation order.
// Empty slice when no fills have been recorded yet.
func (s *Service) ListFills(ctx context.Context, parentTx *Transaction) ([]*Fill, error) {
	rows, err := s.store.ListTransactionFills(ctx, parentTx.ID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to list fills")
	}

	out := make([]*Fill, 0, len(rows))
	for i := range rows {
		out = append(out, s.fillFromRepo(parentTx, rows[i]))
	}
	return out, nil
}

// FillExistsByHashAndRecipient is the watcher dedup hook. Returns true if any
// pending or partial invoice has already recorded a fill at the given
// (network_id, transaction_hash, recipient) — preventing the same on-chain
// transfer from being credited twice when re-detected on a later poll cycle.
func (s *Service) FillExistsByHashAndRecipient(ctx context.Context, networkID, txHash, recipient string) (bool, error) {
	return s.store.FillExistsByHashAndRecipient(ctx, networkID, txHash, recipient)
}

// MarkFillReorged flips a fill to reorged so it stops counting toward the
// cumulative confirmed sum. Used by the periodic reorg recheck.
func (s *Service) MarkFillReorged(ctx context.Context, fillID int64) error {
	return s.store.MarkFillReorged(ctx, fillID)
}

// ListPartialPaymentTxIDs returns transaction ids belonging to invoices
// currently in payment.StatusPartial. The reorg recheck iterates them.
func (s *Service) ListPartialPaymentTxIDs(ctx context.Context) ([]int64, error) {
	return s.store.ListPartialPaymentTxIDs(ctx)
}

// ConfirmedFillSumsByTxIDs returns the cumulative confirmed-fill amount for
// each input transaction id as a raw big.Int (in the smallest unit of the
// per-tx currency). Caller is responsible for interpreting the value with
// the tx's own decimals — used by the watcher to compute remaining-amount
// for the matcher without doing per-tx round trips. tx ids without any
// fills do not appear in the returned map (caller treats absence as zero).
func (s *Service) ConfirmedFillSumsByTxIDs(ctx context.Context, txIDs []int64) (map[int64]*big.Int, error) {
	out := make(map[int64]*big.Int, len(txIDs))
	for _, id := range txIDs {
		num, err := s.store.SumConfirmedFillsForTx(ctx, id)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to sum confirmed fills for tx %d", id)
		}
		bi, biErr := repository.NumericToBigInt(num)
		if biErr != nil || bi == nil || bi.Sign() == 0 {
			continue
		}
		out[id] = bi
	}
	return out, nil
}

func (s *Service) fillFromRepo(parentTx *Transaction, r repository.TransactionFill) *Fill {
	amount, _ := numericToCryptoSameDecimals(r.Amount, parentTx.Amount)

	var sender *string
	if r.SenderAddress.Valid {
		v := r.SenderAddress.String
		sender = &v
	}

	var block *int64
	if r.BlockNumber.Valid {
		v := r.BlockNumber.Int64
		block = &v
	}

	var confirmedAt *time.Time
	if r.ConfirmedAt.Valid {
		v := r.ConfirmedAt.Time
		confirmedAt = &v
	}

	return &Fill{
		ID:              r.ID,
		TransactionID:   r.TransactionID,
		NetworkID:       r.NetworkID,
		TransactionHash: r.TransactionHash,
		VoutOrLogIdx:    r.VoutOrLogIdx,
		Amount:          amount,
		SenderAddress:   sender,
		BlockNumber:     block,
		Confirmations:   r.Confirmations,
		Status:          r.Status,
		ObservedAt:      r.ObservedAt,
		ConfirmedAt:     confirmedAt,
	}
}

// numericToCryptoSameDecimals turns a raw numeric (no decimal scaling) into a
// money.Money using the same currency/decimals as the parent transaction.
// repository.NumericToMoney expects the value to be the raw integer amount in
// the smallest unit, which is exactly how transaction_fills.amount is stored.
func numericToCryptoSameDecimals(num pgtype.Numeric, ref money.Money) (money.Money, error) {
	bigInt := new(big.Int)
	if num.Status == pgtype.Present {
		var err error
		bigInt, err = repository.NumericToBigInt(num)
		if err != nil {
			return money.Money{}, err
		}
	}
	return money.NewFromBigInt(money.Crypto, ref.Ticker(), bigInt, ref.Decimals())
}
