package processing

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/service/blockchain"
	"github.com/cryptolink/cryptolink/internal/service/email"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/cryptolink/cryptolink/internal/service/transaction"
	"github.com/cryptolink/cryptolink/internal/service/wallet"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"
)

// inProgressTimeout is the maximum time a transaction can stay in inProgress
// before being timed out. Prevents stuck transactions from consuming resources.
const inProgressTimeout = 24 * time.Hour

const revertReason = "transaction reverted on chain"

var (
	ErrInvalidInput = errors.New("invalid incoming input")
	ErrTransaction  = errors.New("transaction error")
)

type Input struct {
	Currency      money.CryptoCurrency
	Amount        money.Money
	SenderAddress string
	TransactionID string
	NetworkID     string
}

func (i Input) validate() error {
	if i.Currency.Ticker == "" {
		return errors.Wrap(ErrInvalidInput, "missing currency")
	}

	if i.Amount.Ticker() == "" {
		return errors.Wrap(ErrInvalidInput, "missing amount")
	}

	if i.SenderAddress == "" {
		return errors.Wrap(ErrInvalidInput, "missing SenderAddress")
	}

	if i.TransactionID == "" {
		return errors.Wrap(ErrInvalidInput, "missing TransactionID")
	}

	if i.NetworkID == "" {
		return errors.Wrap(ErrInvalidInput, "missing networkID")
	}

	return nil
}

// ProcessInboundTransaction processes a detected on-chain transfer against a
// pending or partial invoice.
//
// Partial-fill flow: when the new transfer alone (or the cumulative amount
// across all confirmed fills) is below the invoice's expected amount, the
// transfer is recorded as a `transaction_fill` row and the parent transaction
// is left in StatusPending so the watcher keeps polling the same destination
// address. The payment is flipped to StatusPartial and its expiry is extended
// (capped at original_expires_at + 24h). The customer can then top up from
// any wallet to the same address.
//
// Promotion to StatusInProgress happens only when sum(confirmed fills) +
// new transfer ≥ expected (with the standard $0.10 fiat tolerance). At that
// moment the parent transaction's hash is set to the *triggering* fill's hash
// so the existing receipt-confirmation poller drives it to completion.
func (s *Service) ProcessInboundTransaction(
	ctx context.Context,
	tx *transaction.Transaction,
	wt *wallet.Wallet,
	input Input,
) error {
	if err := input.validate(); err != nil {
		return err
	}

	// What's already been received and confirmed against this invoice.
	prevConfirmed, err := s.transactions.SumConfirmedFills(ctx, tx)
	if err != nil {
		return errors.Wrap(err, "unable to sum confirmed fills")
	}

	// Combine: prev + new. If we cross the expected threshold, promote.
	combined, err := prevConfirmed.Add(input.Amount)
	if err != nil {
		return errors.Wrap(err, "unable to combine fills")
	}

	crosses, err := s.crossesExpected(ctx, tx, combined)
	if err != nil {
		return errors.Wrap(err, "unable to evaluate combined amount vs expected")
	}

	if !crosses {
		return s.recordPartialFill(ctx, tx, wt, input, prevConfirmed)
	}

	return s.promoteFromPartial(ctx, tx, wt, input, prevConfirmed, combined)
}

// crossesExpected returns true when `received` is at or above tx.Amount,
// applying the standard $0.10 fiat rounding tolerance for underpayments.
func (s *Service) crossesExpected(ctx context.Context, tx *transaction.Transaction, received money.Money) (bool, error) {
	if received.GreaterThanOrEqual(tx.Amount) {
		return true, nil
	}

	// Tolerance: $0.10 in the invoice's crypto currency, mirrors
	// determineIncomingStatus's existing rounding allowance.
	tenCents, err := money.USD.MakeAmount("10")
	if err != nil {
		return false, err
	}
	conv, err := s.blockchain.FiatToCrypto(ctx, tenCents, tx.Currency)
	if err != nil {
		return false, err
	}

	withTolerance, err := received.Add(conv.To)
	if err != nil {
		return false, err
	}

	return withTolerance.GreaterThanOrEqual(tx.Amount), nil
}

// recordPartialFill stores the new transfer as a fill, flips the payment to
// StatusPartial, extends expiry, and leaves the parent transaction in
// StatusPending so subsequent top-ups to the same address are detected.
func (s *Service) recordPartialFill(
	ctx context.Context,
	tx *transaction.Transaction,
	wt *wallet.Wallet,
	input Input,
	prevConfirmed money.Money,
) error {
	walletID := int64(0)
	if wt != nil {
		walletID = wt.ID
	}

	// Record the fill. Idempotent on (tx_id, network_id, hash, vout/logidx).
	// vout/logidx is 0 for now — chain-specific detectors that need to
	// distinguish multiple outputs in the same tx (e.g. a BTC payer with
	// two outputs to the same address) can pass distinct values via input.
	if _, err := s.transactions.RecordFill(
		ctx,
		tx,
		input.NetworkID,
		input.TransactionID,
		0,
		input.Amount,
		input.SenderAddress,
		0, // block_number unknown at this layer; fill in a follow-up
		1, // confirmations: detection in the watcher already implies on-chain inclusion
		transaction.FillStatusConfirmed,
	); err != nil {
		return errors.Wrap(err, "unable to record partial fill")
	}

	pt, err := s.payments.MarkPartial(ctx, tx.MerchantID, tx.EntityID)
	if err != nil {
		return errors.Wrap(err, "unable to mark payment partial")
	}

	combined, _ := prevConfirmed.Add(input.Amount)
	remaining, _ := tx.Amount.SubNegative(combined)

	s.logger.Info().
		Int64("wallet_id", walletID).
		Int64("transaction_id", tx.ID).
		Int64("payment_id", pt.ID).
		Str("expected", tx.Amount.String()).
		Str("received_now", input.Amount.String()).
		Str("received_total", combined.String()).
		Str("remaining", remaining.String()).
		Time("expires_at_extended_to", *pt.ExpiresAt).
		Msg("partial payment recorded — awaiting top-up")

	return nil
}

// promoteFromPartial finalizes the parent transaction. It calls Receive with
// the *cumulative* fact_amount (all prior confirmed fills + this triggering
// transfer), sets the parent's hash to this transfer's hash, and flips the
// payment to StatusInProgress so the existing receipt poller takes over.
func (s *Service) promoteFromPartial(
	ctx context.Context,
	tx *transaction.Transaction,
	wt *wallet.Wallet,
	input Input,
	prevConfirmed money.Money,
	combined money.Money,
) error {
	if err := s.determineIncomingStatusFromCombined(ctx, tx, combined); err != nil {
		return err
	}

	// Also record the triggering transfer as a fill so the audit trail is
	// complete (every observed on-chain transfer that funded this invoice
	// has a row in transaction_fills).
	if _, err := s.transactions.RecordFill(
		ctx,
		tx,
		input.NetworkID,
		input.TransactionID,
		0,
		input.Amount,
		input.SenderAddress,
		0,
		1,
		transaction.FillStatusConfirmed,
	); err != nil {
		return errors.Wrap(err, "unable to record finalizing fill")
	}

	tx, err := s.transactions.Receive(ctx, tx.MerchantID, tx.ID, transaction.ReceiveTransaction{
		Status:          tx.Status,
		SenderAddress:   input.SenderAddress,
		TransactionHash: input.TransactionID,
		FactAmount:      combined,
		MetaData:        tx.MetaData,
	})
	if err != nil {
		return errors.Wrap(err, "unable to update transaction")
	}

	walletID := int64(0)
	if wt != nil {
		walletID = wt.ID
	}

	if tx.Status != transaction.StatusInProgress {
		s.logger.Warn().
			Int64("wallet_id", walletID).
			Int64("transaction_id", tx.ID).
			Str("expected_amount", tx.Amount.String()).
			Str("combined_amount", combined.String()).
			Str("prev_confirmed", prevConfirmed.String()).
			Msg("promoted payment did not reach inProgress status")
		return nil
	}

	pt, err := s.payments.GetByID(ctx, tx.MerchantID, tx.EntityID)
	if err != nil {
		return errors.Wrap(err, "unable to get payment")
	}

	if _, err := s.payments.Update(ctx, tx.MerchantID, pt.ID, payment.UpdateProps{Status: payment.StatusInProgress}); err != nil {
		return errors.Wrap(err, "unable to update payment")
	}

	s.logger.Info().
		Int64("transaction_id", tx.ID).
		Int64("payment_id", pt.ID).
		Str("combined_fact_amount", combined.String()).
		Msg("payment promoted to inProgress (partial fills aggregated)")

	return nil
}

// determineIncomingStatusFromCombined chooses StatusInProgress vs the
// (legacy, no longer reached in normal flow) StatusInProgressInvalid based on
// the cumulative combined amount rather than a single transfer. Kept for
// belt-and-braces — promoteFromPartial only fires when crossesExpected is
// already true, so this should always pick StatusInProgress.
func (s *Service) determineIncomingStatusFromCombined(ctx context.Context, tx *transaction.Transaction, combined money.Money) error {
	if combined.GreaterThan(tx.Amount) {
		tx.Status = transaction.StatusInProgress
		tx.MetaData[transaction.MetaComment] = "cumulative incoming amount is higher than expected"
		return nil
	}
	if combined.Equals(tx.Amount) {
		tx.Status = transaction.StatusInProgress
		return nil
	}

	crosses, err := s.crossesExpected(ctx, tx, combined)
	if err != nil {
		return err
	}
	if crosses {
		tx.Status = transaction.StatusInProgress
		return nil
	}

	tx.Status = transaction.StatusInProgressInvalid
	tx.MetaData[transaction.MetaErrorReason] = "cumulative incoming amount is less than expected"
	return nil
}

func (s *Service) createUnexpectedTransaction(ctx context.Context, wt *wallet.Wallet, input Input) error {
	isTest := input.Currency.NetworkID != input.NetworkID

	conv, err := s.blockchain.CryptoToFiat(ctx, input.Amount, money.USD)
	if err != nil {
		return errors.Wrapf(err, "unable to convert %s to USD", input.Currency.Ticker)
	}

	params := transaction.CreateTransaction{
		Type:            transaction.TypeIncoming,
		SenderAddress:   input.SenderAddress,
		RecipientWallet: wt,
		TransactionHash: input.TransactionID,
		Currency:        input.Currency,
		Amount:          input.Amount,
		USDAmount:       conv.To,
		IsTest:          isTest,
	}

	_, err = s.transactions.Create(ctx, transaction.SystemMerchantID, params, transaction.IncomingUnexpected())
	if err != nil {
		return errors.Wrap(err, "unable to store unexpected transaction")
	}

	return nil
}

// (legacy determineIncomingStatus removed — partial-fill flow in
// ProcessInboundTransaction supersedes it. Cumulative-amount evaluation now
// happens in determineIncomingStatusFromCombined.)

// reorgGracePeriod is the minimum age a fill must reach before the reorg
// recheck is willing to flag it as reorged. Buffers against transient RPC
// flakes (returning ErrNotFound briefly when the node is catching up) and
// against newly-broadcast fills that haven't propagated to every node yet.
const reorgGracePeriod = 30 * time.Minute

// RecheckPartialFills iterates every fill on every payment currently in
// StatusPartial and re-verifies each fill's hash on chain. Fills whose
// on-chain receipt no longer confirms (and which are at least
// reorgGracePeriod old) are flipped to status='reorged' so they stop
// counting toward the cumulative confirmed sum. Run on a periodic
// scheduler — does NOT block the main watcher loop.
func (s *Service) RecheckPartialFills(ctx context.Context) error {
	txIDs, err := s.transactions.ListPartialPaymentTxIDs(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to list partial-payment tx ids")
	}
	if len(txIDs) == 0 {
		return nil
	}

	now := time.Now()
	checked := 0
	flagged := 0

	for _, txID := range txIDs {
		tx, err := s.transactions.GetByID(ctx, transaction.MerchantIDWildcard, txID)
		if err != nil {
			s.logger.Warn().Err(err).Int64("tx_id", txID).Msg("recheck: tx fetch failed")
			continue
		}

		fills, err := s.transactions.ListFills(ctx, tx)
		if err != nil {
			s.logger.Warn().Err(err).Int64("tx_id", txID).Msg("recheck: list fills failed")
			continue
		}

		for _, f := range fills {
			checked++
			if f.Status != transaction.FillStatusConfirmed {
				continue
			}
			if now.Sub(f.ObservedAt) < reorgGracePeriod {
				continue
			}

			receipt, recErr := s.blockchain.GetTransactionReceipt(ctx, tx.Currency.Blockchain, f.TransactionHash, tx.IsTest)
			// Treat any "tx no longer on chain" signal as reorg evidence.
			// Transient RPC errors don't get past the grace-period gate
			// because they'd resolve on the next scheduler tick.
			if recErr != nil || receipt == nil || !receipt.IsConfirmed {
				if mErr := s.transactions.MarkFillReorged(ctx, f.ID); mErr != nil {
					s.logger.Error().Err(mErr).Int64("fill_id", f.ID).
						Msg("recheck: failed to mark fill reorged")
					continue
				}
				flagged++
				s.logger.Warn().
					Int64("fill_id", f.ID).
					Int64("tx_id", txID).
					Str("hash", f.TransactionHash).
					Msg("partial fill reorged off-chain — marked reorged, sum decremented")
			}
		}
	}

	s.logger.Info().
		Int("partial_txs", len(txIDs)).
		Int("fills_checked", checked).
		Int("fills_flagged_reorged", flagged).
		Msg("partial-fill reorg recheck completed")

	return nil
}

// firstFillHash returns the hash of the earliest observed fill for a tx, or
// empty string when no fills exist or the lookup fails. Used at expiry time
// to attach a representative on-chain hash to a partial-then-expired tx so
// the merchant has a starting point for manual reconciliation.
func firstFillHash(ctx context.Context, s *Service, tx *transaction.Transaction) string {
	fills, err := s.transactions.ListFills(ctx, tx)
	if err != nil || len(fills) == 0 {
		return ""
	}
	return fills[0].TransactionHash
}

func firstFillSender(ctx context.Context, s *Service, tx *transaction.Transaction) string {
	fills, err := s.transactions.ListFills(ctx, tx)
	if err != nil || len(fills) == 0 {
		return ""
	}
	if fills[0].SenderAddress != nil {
		return *fills[0].SenderAddress
	}
	return ""
}

func (s *Service) BatchCheckIncomingTransactions(ctx context.Context, transactionIDs []int64) error {
	var (
		group     errgroup.Group
		checked   int64
		failedTXs []int64
		mu        sync.Mutex
	)

	group.SetLimit(8)

	for i := range transactionIDs {
		txID := transactionIDs[i]
		group.Go(func() error {
			if err := s.checkIncomingTransaction(ctx, txID); err != nil {
				mu.Lock()
				failedTXs = append(failedTXs, txID)
				mu.Unlock()

				return err
			}

			atomic.AddInt64(&checked, 1)

			return nil
		})
	}

	err := group.Wait()

	evt := s.logger.Info()
	if err != nil {
		evt = s.logger.Error().Err(err)
	}

	evt.Int64("checked_transactions_count", checked).
		Ints64("transaction_ids", transactionIDs).
		Ints64("failed_transaction_ids", failedTXs).
		Msg("Checked incoming transactions")

	return err
}

func (s *Service) checkIncomingTransaction(ctx context.Context, txID int64) error {
	tx, err := s.transactions.GetByID(ctx, transaction.MerchantIDWildcard, txID)
	if err != nil {
		return errors.Wrap(err, "unable to get transaction")
	}

	switch {
	case tx.Type != transaction.TypeIncoming:
		return errors.New("invalid transaction type")
	case tx.HashID == nil:
		return errors.New("empty transaction hash")
	case tx.SenderAddress == nil:
		return errors.New("empty sender address")
	case tx.RecipientWalletID == nil && tx.RecipientAddress == "":
		return errors.New("empty recipient wallet id and address")
	case !tx.IsInProgress():
		return nil
	}

	receipt, err := s.blockchain.GetTransactionReceipt(ctx, tx.Currency.Blockchain, *tx.HashID, tx.IsTest)
	if err != nil {
		// Defense-in-depth: a persistent receipt-fetch failure (most commonly
		// a 404 because the tx vanished from the mempool before mining, or a
		// chain reorg orphaned it) used to keep the payment in inProgress
		// forever because we returned the error before reaching the timeout
		// check below. Honour the same 24h ceiling here so stuck transactions
		// always self-cancel instead of spamming the scheduler indefinitely.
		if time.Since(tx.UpdatedAt) > inProgressTimeout {
			s.logger.Warn().Err(err).
				Int64("transaction_id", tx.ID).
				Str("hash", *tx.HashID).
				Msg("transaction timed out after 24h with persistent receipt-fetch failure")
			return s.cancelIncomingTransaction(ctx, tx)
		}
		return errors.Wrap(err, "unable to get transaction receipt")
	}

	if !receipt.IsConfirmed {
		// Timeout stuck inProgress transactions after 24h
		if time.Since(tx.UpdatedAt) > inProgressTimeout {
			s.logger.Warn().
				Int64("transaction_id", tx.ID).
				Str("hash", *tx.HashID).
				Msg("transaction timed out after 24h without confirmation")
			return s.cancelIncomingTransaction(ctx, tx)
		}
		// check later
		return nil
	}

	if !receipt.Success {
		return s.cancelIncomingTransaction(ctx, tx)
	}

	return s.confirmIncomingTransaction(ctx, tx, receipt)
}

func (s *Service) confirmIncomingTransaction(
	ctx context.Context,
	tx *transaction.Transaction,
	receipt *blockchain.TransactionReceipt,
) error {
	s.logger.Info().Int64("transaction_id", tx.ID).Msg("confirming incoming transaction")

	setTXStatus := transaction.StatusCompleted
	setPaymentStatus := payment.StatusSuccess

	if tx.Status == transaction.StatusInProgressInvalid {
		setTXStatus = transaction.StatusCompletedInvalid
		// Underpayment confirmed on-chain → merchant decides to accept or decline
		setPaymentStatus = payment.StatusUnderpaid
	}

	confirmation := transaction.ConfirmTransaction{
		Status:          setTXStatus,
		SenderAddress:   *tx.SenderAddress,
		TransactionHash: *tx.HashID,
		FactAmount:      *tx.FactAmount,
		NetworkFee:      receipt.NetworkFee,
		MetaData:        tx.MetaData,
	}

	confirmation.AllowZeroNetworkFee()

	tx, err := s.transactions.Confirm(ctx, tx.MerchantID, tx.ID, confirmation)
	if err != nil {
		return errors.Wrap(err, "unable to confirm transaction")
	}

	if tx.MerchantID == transaction.SystemMerchantID {
		s.logger.Info().
			Int64("transaction_id", tx.ID).
			Str("transaction_status", string(tx.Status)).
			Msg("processed unexpected incoming transaction")

		return nil
	}

	paymentID := tx.EntityID

	pt, err := s.payments.GetByID(ctx, tx.MerchantID, paymentID)
	if err != nil {
		return errors.Wrap(err, "unable to get payment")
	}

	pt, err = s.payments.Update(ctx, tx.MerchantID, pt.ID, payment.UpdateProps{Status: setPaymentStatus})
	if err != nil {
		return errors.Wrap(err, "unable to update payment")
	}

	s.logger.Info().
		Int64("transaction_id", tx.ID).
		Int64("payment_id", paymentID).
		Str("transaction_status", string(tx.Status)).
		Str("payment_status", string(pt.Status)).
		Msg("processed payment")

	// Increment subscription usage counters (best-effort, non-blocking)
	if setPaymentStatus == payment.StatusSuccess && s.subscriptions != nil {
		volumeUSD := decimal.Zero
		if tx.USDAmount.String() != "" {
			volumeUSD, _ = decimal.NewFromString(tx.USDAmount.StringRaw())
		}
		if err := s.subscriptions.IncrementPaymentUsage(ctx, tx.MerchantID, volumeUSD); err != nil {
			s.logger.Warn().Err(err).Int64("merchant_id", tx.MerchantID).Msg("failed to increment payment usage")
		}
	}

	// Send email notifications (best-effort, non-blocking)
	if setPaymentStatus == payment.StatusSuccess && s.emailService != nil {
		go s.sendConfirmationEmails(context.Background(), tx, pt)
	}

	// Send underpaid notification to merchant
	if setPaymentStatus == payment.StatusUnderpaid && s.emailService != nil {
		go s.sendUnderpaidEmail(context.Background(), tx, pt)
	}

	return nil
}

// sendUnderpaidEmail notifies the merchant that a payment was underpaid.
func (s *Service) sendUnderpaidEmail(ctx context.Context, tx *transaction.Transaction, pt *payment.Payment) {
	mt, err := s.merchants.GetByID(ctx, tx.MerchantID, false)
	if err != nil {
		s.logger.Warn().Err(err).Int64("merchant_id", tx.MerchantID).Msg("unable to get merchant for underpaid email")
		return
	}

	merchantEmail, err := s.emailService.GetMerchantEmail(ctx, tx.MerchantID)
	if err != nil || merchantEmail == "" {
		return
	}

	fiatCode := mt.Settings().FiatCurrency()
	fiatSymbol := money.FiatSymbol(money.FiatCurrency(fiatCode))

	// Display precision: ETH/EVM fill amounts arrive with 18 native decimals;
	// truncate to MaxDisplayDecimals so the email body reads "0.02521316" not
	// "0.025213162071772454".
	maxDisplay := tx.Currency.MaxDisplayDecimals()

	factAmount := "0"
	if tx.FactAmount != nil {
		factAmount = tx.FactAmount.TruncateDecimals(maxDisplay).String()
	}

	fiatPrice, _ := pt.Price.FiatToFloat64()

	s.emailService.SendUnderpaidNotification(ctx, email.UnderpaidParams{
		MerchantEmail:  merchantEmail,
		MerchantName:   mt.Name,
		PaymentID:      pt.PublicID.String(),
		AmountExpected: tx.Amount.TruncateDecimals(maxDisplay).String(),
		AmountReceived: factAmount,
		Ticker:         tx.Currency.Ticker,
		FiatSymbol:     fiatSymbol,
		FiatCode:       fiatCode,
		FiatAmount:     fmt.Sprintf("%.2f", fiatPrice),
		Network:        tx.Currency.BlockchainName,
	})
}

// sendConfirmationEmails sends payment notification emails to the merchant and customer.
// Best-effort: errors are logged but never propagated.
func (s *Service) sendConfirmationEmails(ctx context.Context, tx *transaction.Transaction, pt *payment.Payment) {
	// --- Merchant notification ---
	mt, err := s.merchants.GetByID(ctx, tx.MerchantID, false)
	if err != nil {
		s.logger.Warn().Err(err).Int64("merchant_id", tx.MerchantID).Msg("unable to get merchant for payment email")
		return
	}

	// Resolve merchant's fiat currency for email display
	fiatCode := mt.Settings().FiatCurrency()
	fiatSymbol := money.FiatSymbol(money.FiatCurrency(fiatCode))

	// Compute fiat amounts for emails:
	// - Merchant sees the REAL value received (invoice price + fee markup)
	// - Customer sees the original invoice amount (no fees)
	invoiceFiatStr := tx.USDAmount.String() // fallback
	merchantFiatStr := tx.USDAmount.String()
	if fiatPrice, fiatErr := pt.Price.FiatToFloat64(); fiatErr == nil {
		invoiceFiatStr = fmt.Sprintf("%.2f", fiatPrice)
		feePercent := mt.Settings().GlobalFeePercent()
		if feePercent > 0 {
			merchantFiatStr = fmt.Sprintf("%.2f", fiatPrice*(1+feePercent/100))
		} else {
			merchantFiatStr = invoiceFiatStr
		}
	}

	merchantEmail, err := s.emailService.GetMerchantEmail(ctx, tx.MerchantID)
	if err != nil || merchantEmail == "" {
		s.logger.Warn().Err(err).Int64("merchant_id", tx.MerchantID).Msg("no merchant email found for payment notification")
	} else {
		explorerLink := ""
		if link, linkErr := tx.ExplorerLink(); linkErr == nil {
			explorerLink = link
		}

		senderAddr := ""
		if tx.SenderAddress != nil {
			senderAddr = *tx.SenderAddress
		}

		txHash := ""
		if tx.HashID != nil {
			txHash = *tx.HashID
		}

		maxDisplay := tx.Currency.MaxDisplayDecimals()
		factAmount := tx.Amount.TruncateDecimals(maxDisplay).String()
		if tx.FactAmount != nil {
			factAmount = tx.FactAmount.TruncateDecimals(maxDisplay).String()
		}

		// Fetch payer email (best-effort)
		customerEmail, _ := s.emailService.GetCustomerEmail(ctx, pt.ID)

		s.emailService.SendPaymentReceived(ctx, email.PaymentReceivedParams{
			MerchantEmail:    merchantEmail,
			MerchantName:     mt.Name,
			TxHash:           txHash,
			Amount:           factAmount,
			Ticker:           tx.Currency.Ticker,
			USDAmount:        merchantFiatStr,
			FiatSymbol:       fiatSymbol,
			FiatCode:         fiatCode,
			SenderAddress:    senderAddr,
			RecipientAddress: tx.RecipientAddress,
			ExplorerLink:     explorerLink,
			Network:          tx.Currency.BlockchainName,
			ReceivedAt:       tx.CreatedAt,
			CustomerEmail:    customerEmail,
		})
	}

	// --- Customer notification ---
	customerEmail, err := s.emailService.GetCustomerEmail(ctx, pt.ID)
	if err != nil || customerEmail == "" {
		// No customer email — this is normal for payments without customer info
		return
	}

	explorerLink := ""
	if link, linkErr := tx.ExplorerLink(); linkErr == nil {
		explorerLink = link
	}

	txHash := ""
	if tx.HashID != nil {
		txHash = *tx.HashID
	}

	maxDisplay := tx.Currency.MaxDisplayDecimals()
	factAmount := tx.Amount.TruncateDecimals(maxDisplay).String()
	if tx.FactAmount != nil {
		factAmount = tx.FactAmount.TruncateDecimals(maxDisplay).String()
	}

	// Customer email shows the original invoice amount (no fees)
	s.emailService.SendCustomerPaymentConfirmation(ctx, email.CustomerPaymentConfirmParams{
		CustomerEmail: customerEmail,
		MerchantName:  mt.Name,
		Amount:        factAmount,
		Ticker:        tx.Currency.Ticker,
		USDAmount:     invoiceFiatStr,
		FiatSymbol:    fiatSymbol,
		FiatCode:      fiatCode,
		TxHash:        txHash,
		ExplorerLink:  explorerLink,
		Network:       tx.Currency.BlockchainName,
		ReceivedAt:    tx.CreatedAt,
	})
}

func (s *Service) cancelIncomingTransaction(ctx context.Context, tx *transaction.Transaction) error {
	err := s.transactions.Cancel(ctx, tx, transaction.StatusFailed, revertReason, nil)
	if err != nil {
		return errors.Wrap(err, "unable to cancel transaction")
	}

	if tx.MerchantID == transaction.SystemMerchantID {
		s.logger.Info().
			Int64("transaction_id", tx.ID).
			Str("transaction_status", string(tx.Status)).
			Msg("canceled unexpected incoming transaction")

		return nil
	}

	paymentID := tx.EntityID

	_, err = s.payments.Update(ctx, tx.MerchantID, paymentID, payment.UpdateProps{Status: payment.StatusFailed})
	if err != nil {
		return errors.Wrap(err, "unable to update payment")
	}

	s.logger.Error().
		Int64("transaction_id", tx.ID).
		Int64("payment_id", paymentID).
		Str("transaction_hash", *tx.HashID).
		Msg("incoming transaction has failed")

	return nil
}

func (s *Service) BatchExpirePayments(ctx context.Context, paymentsIDs []int64) error {
	var (
		group        errgroup.Group
		expiredCount int64
		failedIDs    []int64
		mu           sync.Mutex
	)

	group.SetLimit(8)

	for i := range paymentsIDs {
		paymentID := paymentsIDs[i]
		group.Go(func() error {
			if err := s.expirePayment(ctx, paymentID); err != nil {
				mu.Lock()
				failedIDs = append(failedIDs, paymentID)
				mu.Unlock()

				return err
			}

			atomic.AddInt64(&expiredCount, 1)

			return nil
		})
	}

	err := group.Wait()

	evt := s.logger.Info()
	if err != nil {
		evt = s.logger.Error().Err(err)
	}

	evt.Int64("expired_payments_count", expiredCount).
		Ints64("payments_ids", paymentsIDs).
		Ints64("failed_payments_ids", failedIDs).
		Msg("canceled expired payments")

	return err
}

// gracePeriod is the extra time a locked payment (with a pending transaction) gets
// after its nominal expiration. This covers late sends — the watcher keeps polling
// the blockchain address during this window.
const gracePeriod = 30 * time.Minute

func (s *Service) expirePayment(ctx context.Context, paymentID int64) error {
	pt, err := s.payments.GetByID(ctx, payment.MerchantIDWildcard, paymentID)
	if err != nil {
		return errors.Wrap(err, "unable to get payment")
	}

	if pt.Type != payment.TypePayment {
		return errors.Errorf("invalid payment type %q", pt.Type)
	}

	if pt.Status != payment.StatusPending && pt.Status != payment.StatusLocked && pt.Status != payment.StatusPartial {
		return errors.Errorf("invalid payment status %q", pt.Status)
	}

	// 1. Check if tx exists
	tx, err := s.transactions.GetLatestByPaymentID(ctx, pt.ID)
	switch {
	case errors.Is(err, transaction.ErrNotFound):
		// no transaction yet — nothing to wait for
	case err != nil:
		return errors.Wrap(err, "unable to get transaction")
	}

	// 2. Grace period: if a pending transaction exists (customer selected crypto but hasn't
	// paid yet or is paying late), extend the expiry by 30 minutes to catch late payments.
	if tx != nil && tx.Status == transaction.StatusPending && (pt.Status == payment.StatusLocked || pt.Status == payment.StatusPartial) {
		if pt.ExpiresAt != nil && time.Since(*pt.ExpiresAt) < gracePeriod {
			s.logger.Info().
				Int64("payment_id", paymentID).
				Time("expires_at", *pt.ExpiresAt).
				Msg("payment in grace period — skipping expiration, watcher still monitoring")
			return nil
		}
	}

	// 3. Partial payments that hit their hard-capped expiry split into two
	// outcomes by whether anything actually confirmed on-chain:
	//   - confirmedSum > 0 → flip to `underpaid` so the merchant can resolve
	//     via the manual-resolve flow (credits balance with fact_amount).
	//     Parent tx status moves to inProgressInvalid with the cumulative
	//     confirmed amount as fact_amount.
	//   - confirmedSum == 0 → no on-chain receipt actually settled. Cancel
	//     the tx and fail the payment exactly like a never-paid invoice.
	if pt.Status == payment.StatusPartial {
		var confirmedSum money.Money
		if tx != nil {
			cs, sumErr := s.transactions.SumConfirmedFills(ctx, tx)
			if sumErr == nil {
				confirmedSum = cs
			}
		}

		if !confirmedSum.IsZero() && tx != nil {
			if _, updErr := s.transactions.Receive(ctx, tx.MerchantID, tx.ID, transaction.ReceiveTransaction{
				Status:          transaction.StatusInProgressInvalid,
				SenderAddress:   firstFillSender(ctx, s, tx),
				TransactionHash: firstFillHash(ctx, s, tx),
				FactAmount:      confirmedSum,
				MetaData:        tx.MetaData,
			}); updErr != nil {
				s.logger.Error().Err(updErr).Int64("payment_id", paymentID).
					Msg("unable to mark partial transaction as inProgressInvalid on expiry")
			}
			if _, updErr := s.payments.Update(ctx, pt.MerchantID, pt.ID, payment.UpdateProps{Status: payment.StatusUnderpaid}); updErr != nil {
				return errors.Wrap(updErr, "unable to mark partial payment as underpaid on expiry")
			}
			s.logger.Info().Int64("payment_id", paymentID).Str("received", confirmedSum.String()).
				Msg("partial payment expired with confirmed fills — flipped to underpaid for merchant review")
			return nil
		}

		// No confirmed fills — treat exactly like a never-paid expiry.
		if tx != nil && tx.Status != transaction.StatusCancelled {
			if errCancel := s.transactions.Cancel(ctx, tx, transaction.StatusCancelled, "payment expired (partial, zero confirmed)", nil); errCancel != nil {
				return errors.Wrap(errCancel, "unable to cancel partial-zero transaction")
			}
		}
		if errFail := s.payments.Fail(ctx, pt); errFail != nil {
			return errors.Wrap(errFail, "unable to fail partial-zero payment")
		}
		s.logger.Info().Int64("payment_id", paymentID).
			Msg("partial payment expired with zero confirmed fills — failed (no receipt to reconcile)")
		return nil
	}

	// 4. Cancel transaction if exists
	if tx != nil && tx.Status != transaction.StatusCancelled {
		errCancel := s.transactions.Cancel(ctx, tx, transaction.StatusCancelled, "payment expired", nil)
		if errCancel != nil {
			return errors.Wrap(errCancel, "unable to cancel transaction")
		}
	}

	// 5. Cancel payment itself
	if errFail := s.payments.Fail(ctx, pt); errFail != nil {
		return errors.Wrap(errFail, "unable to expire payment")
	}

	return nil
}
