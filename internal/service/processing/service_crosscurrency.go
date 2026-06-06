package processing

import (
	"context"
	"fmt"
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

// decideCrossCurrencyAccept decides whether an unmatched cross-currency payment
// should be auto-credited to the single open invoice at the collector.
//
// It is intentionally pure (no I/O) so the money-sensitive gating is unit
// tested in isolation. Auto-accept requires:
//   - exactly one open invoice at the collector (no ambiguity), and
//   - the detected fiat value covers the invoice's base price (minus tolerance).
//
// detFiat is the current fiat value of the received crypto. invoicePrice is the
// invoice's base fiat price (no volatility-fee markup — the markup is a buffer,
// not a hard requirement, so we don't force the customer to cover it).
func decideCrossCurrencyAccept(openInvoices int, detFiat, invoicePrice float64) (accept, dust, underpaid bool) {
	if detFiat < crossCurrencyDustFiat {
		return false, true, false
	}
	if openInvoices != 1 {
		return false, false, false
	}
	if detFiat+crossCurrencyFiatTolerance >= invoicePrice {
		return true, false, false
	}
	return false, false, true
}

// ResolveUnmatchedCollectorPayment attributes an on-chain transfer that arrived
// at a collector contract in a currency the customer was not invoiced in.
//
// Auto-accept path (single open invoice, fiat value covers it): a fresh pending
// incoming transaction in the *received* currency is created against the
// invoice's payment, the stale (wrong-currency) lock is canceled, and the
// transfer is run through the normal ProcessInboundTransaction → confirm flow
// so the merchant is credited the actual amount received and the standard
// webhook fires. Anything else (no/multiple open invoices, or a fiat
// underpayment) hits the safety net: a structured error log plus a best-effort
// merchant email, leaving the funds reconcilable via the admin tools.
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

	// 4. Value the received crypto in the invoice's fiat to gate the decision.
	var invoicePrice float64
	var fiatCode string
	if len(invoices) == 1 {
		pt, ptErr := s.payments.GetByID(ctx, invoices[0].MerchantID, invoices[0].EntityID)
		if ptErr != nil {
			return errors.Wrap(ptErr, "cross-currency: unable to load invoice payment")
		}
		invoicePrice, _ = pt.Price.FiatToFloat64()
		fiatCode = pt.Price.Ticker()
	}
	if fiatCode == "" {
		fiatCode = money.USD.String()
	}

	detFiat, err := s.cryptoToFiatFloat(ctx, amount, fiatCode)
	if err != nil {
		// Can't value the payment — fall back to the safety net so it isn't lost.
		s.logger.Error().Err(err).
			Str("collector", p.CollectorAddress).
			Str("ticker", currency.Ticker).
			Str("tx_hash", p.TxHash).
			Msg("cross-currency: unable to value received crypto in fiat; routing to safety net")
		s.alertUnmatchedCollectorPayment(ctx, invoices, p, currency, amount, 0, invoicePrice)
		return nil
	}

	accept, dust, underpaid := decideCrossCurrencyAccept(len(invoices), detFiat, invoicePrice)

	switch {
	case dust:
		s.logger.Debug().
			Str("collector", p.CollectorAddress).
			Str("tx_hash", p.TxHash).
			Float64("fiat_value", detFiat).
			Msg("cross-currency: ignoring sub-threshold unmatched collector transfer (dust)")
		return nil
	case !accept:
		s.logger.Error().
			Str("collector", p.CollectorAddress).
			Str("tx_hash", p.TxHash).
			Str("ticker", currency.Ticker).
			Str("amount", amount.String()).
			Float64("fiat_value", detFiat).
			Float64("invoice_price", invoicePrice).
			Int("open_invoices", len(invoices)).
			Bool("underpaid", underpaid).
			Msg("SAFETY NET: unmatched collector payment could not be auto-credited — manual review required")
		s.alertUnmatchedCollectorPayment(ctx, invoices, p, currency, amount, detFiat, invoicePrice)
		return nil
	}

	// 5. Auto-accept: re-lock the single open invoice in the received currency
	//    and drive it through the normal confirm flow.
	return s.autoAcceptCrossCurrency(ctx, invoices[0], currency, amount, networkID, p)
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
