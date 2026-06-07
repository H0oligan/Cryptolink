package processing

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/service/email"
	"github.com/cryptolink/cryptolink/internal/service/transaction"
	"github.com/pkg/errors"
)

// crossCurrencyFiatTolerance is the fiat slack (in the invoice's fiat unit)
// allowed when checking whether a cross-currency payment covers an invoice.
// Mirrors the $0.10 rounding tolerance used elsewhere in the incoming flow.
const crossCurrencyFiatTolerance = 0.10

// crossCurrencyDustFiat is the floor (in fiat) below which an unmatched
// collector transfer is treated as dust/spam: it is logged at debug level and
// neither auto-accepted nor alerted on, so attackers can't spam merchant
// inboxes by dusting a collector contract.
const crossCurrencyDustFiat = 0.50

// UnmatchedCollectorPayment describes an on-chain transfer the watcher observed
// at a merchant's collector contract that did NOT match any pending invoice of
// the same currency (e.g. the customer paid native ETH while the invoice was
// locked in ETH_USDT). The currency is resolved here in processing because the
// watcher has no currency resolver.
type UnmatchedCollectorPayment struct {
	CollectorAddress string
	Blockchain       money.Blockchain
	IsTest           bool

	// IsNative true => the transfer is the chain's native coin (Received event).
	// Otherwise TokenContract identifies the ERC-20 that moved.
	IsNative      bool
	TokenContract string

	// RawAmount is the smallest-unit amount as a decimal string (wei for native).
	RawAmount     string
	TxHash        string
	SenderAddress string
}

// crossCurrencyAmbiguityMargin is the minimum fiat gap by which the best-matching
// open invoice must beat the runner-up before a cross-currency payment is
// attributed automatically. Two invoices whose expected amounts are closer than
// this (notably two invoices of the *same* amount) are indistinguishable, so we
// refuse rather than risk crediting the wrong customer's invoice. Scales with
// the payment size (2%), with a $1.00 floor.
func crossCurrencyAmbiguityMargin(detFiat float64) float64 {
	if m := 0.02 * detFiat; m > 1.0 {
		return m
	}
	return 1.0
}

// chooseCrossCurrencyInvoice attributes a cross-currency payment worth detFiat
// (in the merchant's fiat) to exactly one open invoice by matching the received
// value against each invoice's base fiat price (no volatility markup — the
// caller enforces dust and coverage gating separately).
//
// Pure (no I/O) so the money-sensitive attribution is unit tested in isolation.
// Returns (idx, true) only when one invoice is the unambiguous closest match.
// Returns ok=false when there are no invoices, or when the two closest are too
// near to tell apart (e.g. two invoices of equal amount) — the caller MUST NOT
// auto-credit in that case; it routes to the safety net for human attribution.
func chooseCrossCurrencyInvoice(detFiat float64, expected []float64) (int, bool) {
	if len(expected) == 0 {
		return -1, false
	}

	bestIdx := 0
	for i := 1; i < len(expected); i++ {
		if math.Abs(detFiat-expected[i]) < math.Abs(detFiat-expected[bestIdx]) {
			bestIdx = i
		}
	}
	if len(expected) == 1 {
		return bestIdx, true
	}

	secondIdx := -1
	for i := range expected {
		if i == bestIdx {
			continue
		}
		if secondIdx == -1 || math.Abs(detFiat-expected[i]) < math.Abs(detFiat-expected[secondIdx]) {
			secondIdx = i
		}
	}

	gap := math.Abs(detFiat-expected[secondIdx]) - math.Abs(detFiat-expected[bestIdx])
	if gap >= crossCurrencyAmbiguityMargin(detFiat) {
		return bestIdx, true
	}
	return -1, false // two invoices too close to attribute safely
}

// ResolveUnmatchedCollectorPayment attributes an on-chain transfer that arrived
// at a collector contract in a currency the customer was not invoiced in.
//
// Auto-accept path (an unambiguous closest-matching invoice whose base fiat
// price is covered): a fresh pending incoming transaction in the *received*
// currency is created against the invoice's payment, the stale (wrong-currency)
// lock is canceled, and the transfer is run through the normal
// ProcessInboundTransaction → confirm flow so the merchant is credited the
// actual amount received and the standard webhook fires. Anything else (no open
// invoices, two invoices too close to tell apart, or a fiat underpayment) hits
// the safety net: a structured error log plus a best-effort merchant email,
// leaving the funds reconcilable via the admin tools.
func (s *Service) ResolveUnmatchedCollectorPayment(ctx context.Context, p UnmatchedCollectorPayment) error {
	// 1. Resolve the currency that actually moved on-chain.
	currency, err := s.resolveUnmatchedCurrency(p)
	if err != nil {
		s.logger.Error().Err(err).
			Str("collector", p.CollectorAddress).
			Str("blockchain", p.Blockchain.String()).
			Str("tx_hash", p.TxHash).
			Bool("is_native", p.IsNative).
			Str("token_contract", p.TokenContract).
			Msg("cross-currency: unable to resolve currency for unmatched collector payment")
		return nil // don't wedge the watcher; this is logged for manual review
	}

	networkID := currency.ChooseNetwork(p.IsTest)

	amountInt, ok := new(big.Int).SetString(p.RawAmount, 10)
	if !ok || amountInt.Sign() <= 0 {
		s.logger.Error().
			Str("collector", p.CollectorAddress).
			Str("raw_amount", p.RawAmount).
			Str("tx_hash", p.TxHash).
			Msg("cross-currency: invalid raw amount on unmatched collector payment")
		return nil
	}
	amount, err := money.NewFromBigInt(money.Crypto, currency.Ticker, amountInt, currency.Decimals)
	if err != nil {
		return errors.Wrap(err, "cross-currency: unable to construct amount")
	}

	// 2. Idempotency: if this hash is already bound to a tx at this collector,
	//    it's been handled (by us on a prior cycle or by the normal path).
	if existing, lookupErr := s.transactions.GetByHashAndRecipient(ctx, networkID, p.TxHash, p.CollectorAddress); lookupErr == nil && existing != nil {
		return nil
	}

	// 3. Find the open invoices sharing this collector address. A collector is
	//    per-merchant per-chain, so every match belongs to the same merchant.
	invoices, err := s.transactions.ListByFilter(ctx, transaction.Filter{
		RecipientAddress: p.CollectorAddress,
		Types:            []transaction.Type{transaction.TypeIncoming},
		Statuses:         []transaction.Status{transaction.StatusPending},
		HashIsEmpty:      true,
	}, 50)
	if err != nil {
		return errors.Wrap(err, "cross-currency: unable to list open collector invoices")
	}

	// 4. Value every open invoice in its (shared, per-merchant) fiat. A collector
	//    is per-merchant per-chain, so all invoices share one fiat; the base
	//    price drives attribution (the volatility markup is only a buffer).
	expected := make([]float64, len(invoices))
	fiatCode := ""
	for i, inv := range invoices {
		pt, ptErr := s.payments.GetByID(ctx, inv.MerchantID, inv.EntityID)
		if ptErr != nil {
			return errors.Wrap(ptErr, "cross-currency: unable to load invoice payment")
		}
		expected[i], _ = pt.Price.FiatToFloat64()
		if fiatCode == "" {
			fiatCode = pt.Price.Ticker()
		}
	}
	if fiatCode == "" {
		fiatCode = money.USD.String()
	}

	// 5. Value the received crypto in that fiat to drive attribution and gating.
	detFiat, err := s.cryptoToFiatFloat(ctx, amount, fiatCode)
	if err != nil {
		// Can't value the payment — fall back to the safety net so it isn't lost.
		s.logger.Error().Err(err).
			Str("collector", p.CollectorAddress).
			Str("ticker", currency.Ticker).
			Str("tx_hash", p.TxHash).
			Msg("cross-currency: unable to value received crypto in fiat; routing to safety net")
		s.alertUnmatchedCollectorPayment(ctx, invoices, p, currency, amount, 0, 0)
		return nil
	}

	// 6. Dust: ignore sub-threshold transfers so a collector can't be spammed.
	if detFiat < crossCurrencyDustFiat {
		s.logger.Debug().
			Str("collector", p.CollectorAddress).
			Str("tx_hash", p.TxHash).
			Float64("fiat_value", detFiat).
			Msg("cross-currency: ignoring sub-threshold unmatched collector transfer (dust)")
		return nil
	}

	// 7. Attribute to the unambiguous closest-matching invoice. No match (zero
	//    invoices) or two invoices too close to tell apart → safety net.
	idx, ok := chooseCrossCurrencyInvoice(detFiat, expected)
	if !ok {
		s.logger.Error().
			Str("collector", p.CollectorAddress).
			Str("tx_hash", p.TxHash).
			Str("ticker", currency.Ticker).
			Str("amount", amount.String()).
			Float64("fiat_value", detFiat).
			Int("open_invoices", len(invoices)).
			Msg("SAFETY NET: unmatched collector payment could not be attributed (no/ambiguous invoice) — manual review required")
		s.alertUnmatchedCollectorPayment(ctx, invoices, p, currency, amount, detFiat, 0)
		return nil
	}

	// 8. Coverage gate: never auto-credit a fiat underpayment, even to a match.
	if detFiat+crossCurrencyFiatTolerance < expected[idx] {
		s.logger.Error().
			Str("collector", p.CollectorAddress).
			Str("tx_hash", p.TxHash).
			Str("ticker", currency.Ticker).
			Str("amount", amount.String()).
			Float64("fiat_value", detFiat).
			Float64("invoice_price", expected[idx]).
			Int("open_invoices", len(invoices)).
			Msg("SAFETY NET: cross-currency payment is below the matched invoice price (underpaid) — manual review required")
		s.alertUnmatchedCollectorPayment(ctx, invoices, p, currency, amount, detFiat, expected[idx])
		return nil
	}

	// 9. Auto-accept: re-lock the chosen invoice in the received currency and
	//    drive it through the normal confirm flow.
	return s.autoAcceptCrossCurrency(ctx, invoices[idx], currency, amount, networkID, p)
}

// resolveUnmatchedCurrency maps the watcher's raw signal to a CryptoCurrency.
func (s *Service) resolveUnmatchedCurrency(p UnmatchedCollectorPayment) (money.CryptoCurrency, error) {
	if p.IsNative {
		return s.blockchain.GetNativeCoin(p.Blockchain)
	}
	// Token path: resolve by contract. NetworkID for token lookup uses the
	// mainnet/testnet id of the native coin on this chain.
	native, err := s.blockchain.GetNativeCoin(p.Blockchain)
	if err != nil {
		return money.CryptoCurrency{}, err
	}
	networkID := native.ChooseNetwork(p.IsTest)
	return s.blockchain.GetCurrencyByBlockchainAndContract(p.Blockchain, networkID, p.TokenContract)
}

// cryptoToFiatFloat values a crypto amount in the given fiat code as a float.
func (s *Service) cryptoToFiatFloat(ctx context.Context, amount money.Money, fiatCode string) (float64, error) {
	fiatCur, err := money.MakeFiatCurrency(fiatCode)
	if err != nil {
		return 0, errors.Wrapf(err, "unknown fiat %q", fiatCode)
	}
	conv, err := s.blockchain.CryptoToFiat(ctx, amount, fiatCur)
	if err != nil {
		return 0, err
	}
	return conv.To.FiatToFloat64()
}

// autoAcceptCrossCurrency re-locks the invoice in the received currency and
// processes the transfer through the standard inbound flow.
func (s *Service) autoAcceptCrossCurrency(
	ctx context.Context,
	inv *transaction.Transaction,
	currency money.CryptoCurrency,
	amount money.Money,
	networkID string,
	p UnmatchedCollectorPayment,
) error {
	pt, err := s.payments.GetByID(ctx, inv.MerchantID, inv.EntityID)
	if err != nil {
		return errors.Wrap(err, "cross-currency: unable to load payment for auto-accept")
	}

	zeroFee, err := money.NewFromBigInt(money.Crypto, currency.Ticker, big.NewInt(0), currency.Decimals)
	if err != nil {
		return errors.Wrap(err, "cross-currency: unable to build zero service fee")
	}

	// usd_amount drives subscription volume tracking and is always stored in USD,
	// matching the normal lock-creation path (FiatToFiat(price, USD)). For a
	// non-USD merchant (EUR, GBP, ...) this converts the invoice's fiat price to
	// USD; falls back to the raw price if conversion is unavailable.
	usdAmount := pt.Price
	if conv, convErr := s.blockchain.FiatToFiat(ctx, pt.Price, money.USD); convErr == nil {
		usdAmount = conv.To
	}

	// New pending lock in the received currency, amount == exactly what arrived
	// so the inbound flow promotes immediately and credits the real amount.
	newTx, err := s.transactions.Create(ctx, inv.MerchantID, transaction.CreateTransaction{
		Type:             transaction.TypeIncoming,
		EntityID:         inv.EntityID,
		RecipientAddress: p.CollectorAddress,
		Currency:         currency,
		Amount:           amount,
		ServiceFee:       zeroFee,
		USDAmount:        usdAmount,
		IsTest:           p.IsTest,
	})
	if err != nil {
		return errors.Wrap(err, "cross-currency: unable to create re-locked transaction")
	}

	// Cancel the stale wrong-currency lock so the payment has a single live tx.
	if cancelErr := s.transactions.Cancel(
		ctx, inv, transaction.StatusCancelled,
		"superseded by cross-currency payment in "+currency.Ticker, nil,
	); cancelErr != nil {
		s.logger.Warn().Err(cancelErr).
			Int64("stale_tx_id", inv.ID).
			Int64("payment_id", inv.EntityID).
			Msg("cross-currency: unable to cancel stale lock (continuing; it will expire)")
	}

	sender := p.SenderAddress
	if sender == "" {
		sender = "cross-currency-reconcile"
	}

	s.logger.Warn().
		Int64("payment_id", inv.EntityID).
		Int64("merchant_id", inv.MerchantID).
		Int64("new_tx_id", newTx.ID).
		Int64("canceled_tx_id", inv.ID).
		Str("received_ticker", currency.Ticker).
		Str("received_amount", amount.String()).
		Str("tx_hash", p.TxHash).
		Msg("AUDIT: auto-accepting cross-currency collector payment")

	input := Input{
		Currency:      currency,
		Amount:        amount,
		SenderAddress: sender,
		TransactionID: p.TxHash,
		NetworkID:     networkID,
	}
	return s.ProcessInboundTransaction(ctx, newTx, nil, input)
}

// alertUnmatchedCollectorPayment is the safety net: notify the merchant so a
// human can reconcile via the admin tools. Best-effort — never fails the caller.
func (s *Service) alertUnmatchedCollectorPayment(
	ctx context.Context,
	invoices []*transaction.Transaction,
	p UnmatchedCollectorPayment,
	currency money.CryptoCurrency,
	amount money.Money,
	detFiat, invoicePrice float64,
) {
	if s.emailService == nil || len(invoices) == 0 {
		return
	}
	merchantID := invoices[0].MerchantID

	merchantEmail, err := s.emailService.GetMerchantEmail(ctx, merchantID)
	if err != nil || merchantEmail == "" {
		return
	}

	subject := "Action needed: unmatched payment to your collector"
	body := fmt.Sprintf(
		"We detected a payment to your %s collector contract (%s) that we could not "+
			"automatically match to one of your invoices.\n\n"+
			"Received: %s %s (~%.2f %s)\n"+
			"Transaction: %s\n"+
			"Open invoices at this address: %d\n\n"+
			"This usually happens when a customer pays in a different currency than the "+
			"one the invoice was created in. The funds are safe in your collector "+
			"contract. Please review and reconcile this payment from your dashboard.",
		p.Blockchain.String(), p.CollectorAddress,
		amount.String(), currency.Ticker, detFiat, currency.Ticker,
		p.TxHash, len(invoices),
	)

	if sendErr := s.emailService.SendEmail(ctx, email.SendEmailParams{
		To:      merchantEmail,
		Subject: subject,
		Body:    body,
	}); sendErr != nil {
		s.logger.Warn().Err(sendErr).Int64("merchant_id", merchantID).
			Msg("cross-currency: unable to send unmatched-payment alert email")
	}
}
