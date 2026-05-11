// package processing implements methods for invoices processing
package processing

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/cryptolink/cryptolink/internal/bus"
	"github.com/cryptolink/cryptolink/internal/lock"
	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/service/blockchain"
	"github.com/cryptolink/cryptolink/internal/service/email"
	"github.com/cryptolink/cryptolink/internal/service/evmcollector"
	"github.com/cryptolink/cryptolink/internal/service/merchant"
	"github.com/cryptolink/cryptolink/internal/service/payment"
	"github.com/cryptolink/cryptolink/internal/service/subscription"
	"github.com/cryptolink/cryptolink/internal/service/transaction"
	"github.com/cryptolink/cryptolink/internal/service/wallet"
	"github.com/cryptolink/cryptolink/internal/service/xpub"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type BlockchainService interface {
	blockchain.Resolver
	blockchain.Convertor
	blockchain.Broadcaster
	blockchain.FeeCalculator
}

type Service struct {
	config           Config
	wallets          *wallet.Service
	merchants        *merchant.Service
	payments         *payment.Service
	transactions     *transaction.Service
	xpubService      *xpub.Service
	evmCollector     *evmcollector.Service
	emailService     *email.Service
	subscriptions    *subscription.Service
	blockchain       BlockchainService
	publisher        bus.Publisher
	locker           *lock.Locker
	logger           *zerolog.Logger
}

type Config struct {
	WebhookBasePath         string `yaml:"webhook_base_path" env:"PROCESSING_WEBHOOK_BASE_PATH" env-description:"Base path for webhooks (sub)domain. Example: https://pay.site.com"`
	PaymentFrontendBasePath string `yaml:"payment_frontend_base_path" env:"PROCESSING_PAYMENT_FRONTEND_BASE_PATH" env-description:"Base path for payment UI. Example: https://pay.site.com"`
	PaymentFrontendSubPath  string `yaml:"payment_frontend_sub_path" env:"PROCESSING_PAYMENT_FRONTEND_SUB_PATH" env-default:"/p" env-description:"Sub path for payment UI"`
	// DefaultServiceFee as float percentage. 1% is 0.01
	DefaultServiceFee float64 `yaml:"default_service_fee" env:"PROCESSING_DEFAULT_SERVICE_FEE" env-default:"0" env-description:"Internal variable"`
}

func (c *Config) PaymentFrontendPath() string {
	base := strings.TrimSuffix(c.PaymentFrontendBasePath, "/")
	sub := strings.Trim(c.PaymentFrontendSubPath, "/")

	if sub == "" {
		return base
	}

	return base + "/" + sub
}

var (
	ErrStatusInvalid         = errors.New("payment status is invalid")
	ErrPaymentOptionsMissing = errors.New("payment options are not fully fulfilled")
	ErrSignatureVerification = errors.New("unable to verify request signature")
)

func New(
	config Config,
	wallets *wallet.Service,
	merchants *merchant.Service,
	payments *payment.Service,
	transactions *transaction.Service,
	xpubService *xpub.Service,
	evmCollectorService *evmcollector.Service,
	emailService *email.Service,
	subscriptionService *subscription.Service,
	blockchainService BlockchainService,
	publisher bus.Publisher,
	locker *lock.Locker,
	logger *zerolog.Logger,
) *Service {
	log := logger.With().Str("channel", "processing_service").Logger()

	return &Service{
		config:        config,
		wallets:       wallets,
		merchants:     merchants,
		payments:      payments,
		transactions:  transactions,
		xpubService:   xpubService,
		evmCollector:  evmCollectorService,
		emailService:  emailService,
		subscriptions: subscriptionService,
		blockchain:    blockchainService,
		publisher:     publisher,
		locker:        locker,
		logger:        &log,
	}
}

type DetailedPayment struct {
	Payment       *payment.Payment
	Customer      *payment.Customer
	Merchant      *merchant.Merchant
	PaymentMethod *payment.Method
	PaymentInfo   *PaymentInfo
}

// PaymentInfo represents simplified transaction information.
type PaymentInfo struct {
	Status           payment.Status
	PaymentLink      string
	RecipientAddress string

	Amount          string
	AmountFormatted string

	// FactAmountFormatted is what the customer actually sent (set for underpaid/completed payments).
	FactAmountFormatted string

	// ReceivedAmount is the cumulative amount observed across all fills
	// (formatted, in the invoice's crypto currency). Non-empty when the
	// payment is in StatusPartial — used by the customer-facing SPA to show
	// "you've sent X, send Y more to the same address".
	ReceivedAmount string

	// RemainingAmount is tx.Amount - ReceivedAmount (formatted). Drives the
	// new QR code shown in the partial-payment UI so the customer can scan
	// to pay only the missing portion.
	RemainingAmount string

	// RemainingPaymentLink is a wallet-deeplink (bitcoin:..., ethereum:...)
	// pre-filled with RemainingAmount so the customer can scan a fresh QR
	// for the top-up portion only.
	RemainingPaymentLink string

	ExpiresAt             time.Time
	ExpirationDurationMin int64

	SuccessAction  *payment.SuccessAction
	SuccessURL     *string
	SuccessMessage *string
}

func (s *Service) GetDetailedPayment(ctx context.Context, merchantID, paymentID int64) (*DetailedPayment, error) {
	pt, err := s.payments.GetByID(ctx, merchantID, paymentID)
	if err != nil {
		return nil, err
	}

	mt, err := s.merchants.GetByID(ctx, pt.MerchantID, false)
	if err != nil {
		return nil, err
	}

	result := &DetailedPayment{
		Payment:  pt,
		Merchant: mt,
	}

	if pt.CustomerID != nil {
		person, errPerson := s.payments.GetCustomerByID(ctx, merchantID, *pt.CustomerID)
		if errPerson != nil {
			return nil, errors.Wrap(errPerson, "unable to get customer")
		}
		result.Customer = person
	}

	paymentMethod, err := s.payments.GetPaymentMethod(ctx, pt)
	switch {
	case errors.Is(err, payment.ErrPaymentMethodNotSet):
		// okay, that's fine, payment method is not set by the user yet
	case err != nil:
		return nil, errors.Wrap(err, "unable to get payment method")
	case err == nil:
		result.PaymentMethod = paymentMethod
	}

	withPaymentInfo := paymentMethod != nil && !pt.IsEditable()

	if withPaymentInfo {
		tx := paymentMethod.TX()
		if tx == nil {
			return nil, errors.Wrap(ErrTransaction, "transaction is nil")
		}

		var expiresAt time.Time
		if pt.ExpiresAt != nil {
			expiresAt = *pt.ExpiresAt
		}

		paymentLink, err := tx.PaymentLink()
		if err != nil {
			return nil, err
		}

		// Display precision: native chain decimals (18 for ETH/EVM) leak raw
		// fill amounts like "0.025213162071772454" into the SPA. Truncate every
		// display string to MaxDisplayDecimals so customers see "0.02521316".
		displayDecimals := tx.Currency.MaxDisplayDecimals()
		displayAmount := func(m money.Money) string {
			return m.TruncateDecimals(displayDecimals).String()
		}

		factAmountFormatted := ""
		if tx.FactAmount != nil {
			factAmountFormatted = displayAmount(*tx.FactAmount)
		}

		// Partial-fill awareness: also surfaced for StatusUnderpaid so the
		// post-expiry page can show the same received/remaining/QR info as
		// the live partial banner. The watcher no longer polls after the
		// 24h cap, but the recipient address remains valid for merchant-
		// approved manual reconcile.
		var receivedFormatted, remainingFormatted, remainingLink string
		if pt.Status == payment.StatusPartial || pt.Status == payment.StatusUnderpaid {
			receivedSum, sumErr := s.transactions.SumAllFills(ctx, tx)
			if sumErr != nil {
				return nil, errors.Wrap(sumErr, "unable to sum fills for partial payment")
			}
			receivedFormatted = displayAmount(receivedSum)

			if remaining, subErr := tx.Amount.SubNegative(receivedSum); subErr == nil {
				truncatedRemaining := remaining.TruncateDecimals(displayDecimals)
				remainingFormatted = truncatedRemaining.String()
				if rl, linkErr := blockchain.CreatePaymentLink(tx.RecipientAddress, tx.Currency, truncatedRemaining, tx.IsTest); linkErr == nil {
					remainingLink = rl
				}
			}
		}

		result.PaymentInfo = &PaymentInfo{
			Status:           pt.PublicStatus(),
			PaymentLink:      paymentLink,
			RecipientAddress: tx.RecipientAddress,

			Amount:              tx.Amount.StringRaw(),
			AmountFormatted:     displayAmount(tx.Amount),
			FactAmountFormatted: factAmountFormatted,

			ReceivedAmount:       receivedFormatted,
			RemainingAmount:      remainingFormatted,
			RemainingPaymentLink: remainingLink,

			ExpiresAt:             expiresAt,
			ExpirationDurationMin: pt.ExpirationDurationMin(),

			SuccessAction:  pt.PublicSuccessAction(),
			SuccessURL:     pt.PublicSuccessURL(),
			SuccessMessage: pt.PublicSuccessMessage(),
		}
	}

	return result, nil
}

// LockPaymentOptions locks payment editing.
// This method is used to finish payment setup by the end customer.
func (s *Service) LockPaymentOptions(ctx context.Context, merchantID, paymentID int64) error {
	details, err := s.GetDetailedPayment(ctx, merchantID, paymentID)
	if err != nil {
		return errors.Wrap(err, "unable to get detailed payment")
	}

	if !details.Payment.IsEditable() {
		return nil
	}

	if details.Customer == nil || details.PaymentMethod == nil {
		return ErrPaymentOptionsMissing
	}

	_, err = s.payments.Update(ctx, merchantID, paymentID, payment.UpdateProps{Status: payment.StatusLocked})
	if err != nil {
		return errors.Wrap(err, "unable to lock payment")
	}

	return nil
}

// SetPaymentMethod created/changes payment's underlying transaction.
func (s *Service) SetPaymentMethod(ctx context.Context, p *payment.Payment, ticker string) (*payment.Method, error) {
	if p == nil {
		return nil, errors.New("payment is nil")
	}

	if !p.IsEditable() {
		return nil, ErrStatusInvalid
	}

	mt, err := s.merchants.GetByID(ctx, p.MerchantID, false)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get merchant")
	}

	currency, err := s.getPaymentMethod(ctx, mt, ticker)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get payment method")
	}

	lockKey := lock.RowKey{Table: "payments", ID: p.ID}

	var (
		method    *payment.Method
		errReturn error
	)

	_ = s.locker.Do(ctx, lockKey, func() error {
		tx, err := s.transactions.GetLatestByPaymentID(ctx, p.ID)

		switch {
		case errors.Is(err, transaction.ErrNotFound):
			// case 1. no transaction yet -> create
			method, errReturn = s.createIncomingTransaction(ctx, p, mt, currency)
		case err != nil:
			// case 2. unknown error
			errReturn = errors.Wrap(err, "unable to get latest payment by id")
		case tx.Status == transaction.StatusCancelled:
			// case 1*. transaction was canceled, but BE had error while changing payment method earlier.
			// This can happen when for example currency provider returns error when fetching currency rates.
			method, errReturn = s.createIncomingTransaction(ctx, p, mt, currency)
		case tx.Currency.Ticker == currency.Ticker && tx.Currency.NetworkID == currency.NetworkID:
			// case 3. no changes, do nothing
			method = payment.MakeMethod(tx, currency)
		default:
			// case 4. ticker has changed. Change pending transaction.
			method, errReturn = s.changePaymentMethod(ctx, p, mt, tx, currency)
		}

		return nil
	})

	return method, errReturn
}

func (s *Service) getPaymentMethod(ctx context.Context, mt *merchant.Merchant, ticker string) (money.CryptoCurrency, error) {
	currency, err := s.blockchain.GetCurrencyByTicker(ticker)
	if err != nil {
		return money.CryptoCurrency{}, errors.Wrap(err, "unable to get currency by ticker")
	}

	supported, err := s.merchants.ListSupportedCurrencies(ctx, mt)
	if err != nil {
		return money.CryptoCurrency{}, errors.Wrap(err, "unable to list merchant currencies")
	}

	for i := range supported {
		if supported[i].Currency.Ticker == currency.Ticker && supported[i].Enabled {
			return currency, nil
		}
	}

	err = errors.Wrapf(blockchain.ErrCurrencyNotFound, "currency %q is disabled for merchant", currency.Ticker)

	return money.CryptoCurrency{}, err
}

// createIncomingTransaction creates transaction that represents pending payment created by merchant.
// Each time customer changes payment method (e.g. switching from ETH to ETH_USDT in payment UI) we need
// to create a new tx.
func (s *Service) createIncomingTransaction(
	ctx context.Context,
	pt *payment.Payment,
	mt *merchant.Merchant,
	currency money.CryptoCurrency,
) (*payment.Method, error) {
	// 1. Calculate service fee in crypto and USD price.
	conv, err := s.blockchain.FiatToCrypto(ctx, pt.Price, currency)
	if err != nil {
		return nil, err
	}

	cryptoAmount := conv.To

	// Apply merchant's volatility buffer (fee markup) to the crypto amount.
	// This increases the crypto amount the customer must send so the merchant
	// receives the full invoice value even with minor price swings.
	if mt != nil {
		feePercent := mt.Settings().GlobalFeePercent()
		if feePercent > 0 {
			multiplier := 1.0 + (feePercent / 100.0)
			cryptoAmount, err = cryptoAmount.MultiplyFloat64(multiplier)
			if err != nil {
				s.logger.Warn().Err(err).Float64("fee_percent", feePercent).Msg("unable to apply merchant fee markup")
				// Fall back to original amount — don't block payment
				cryptoAmount = conv.To
			}
		}
	}

	// Truncate to industry-standard display precision (max 8 decimals for native coins).
	// This rounds down so the customer never underpays when sending the displayed amount.
	cryptoAmount = cryptoAmount.TruncateDecimals(currency.MaxDisplayDecimals())

	var cryptoServiceFee money.Money
	if s.config.DefaultServiceFee > 0 {
		cryptoServiceFee, err = cryptoAmount.MultiplyFloat64(s.config.DefaultServiceFee)
		if err != nil {
			return nil, errors.Wrap(err, "unable to calculate service fee")
		}
	}

	conv, err = s.blockchain.FiatToFiat(ctx, pt.Price, money.USD)
	if err != nil {
		return nil, err
	}

	usdAmount := conv.To

	// 2. Determine recipient address.
	// Smart contract collector for EVM/TRON chains, xpub for BTC.
	// No fallback — merchant must have a wallet set up for the blockchain.
	blockchain := currency.Blockchain.String()

	// 2a. Check if merchant has a smart contract collector for this blockchain
	if s.evmCollector != nil {
		collector, collectorErr := s.evmCollector.GetByMerchantAndBlockchain(ctx, pt.MerchantID, blockchain)
		if collectorErr == nil && collector != nil {
			return s.createTransactionWithCollectorAddress(ctx, pt, currency, collector, cryptoAmount, cryptoServiceFee, usdAmount)
		}
	}

	// 2b. Check if merchant has a BTC xpub wallet
	xpubWallets, err := s.xpubService.ListByMerchantID(ctx, pt.MerchantID)
	if err != nil {
		s.logger.Warn().Err(err).Msg("unable to list xpub wallets")
	}

	for _, w := range xpubWallets {
		if w.Blockchain == blockchain && w.IsActive {
			return s.createTransactionWithXpubAddress(ctx, pt, currency, w, cryptoAmount, cryptoServiceFee, usdAmount)
		}
	}

	// No wallet configured for this blockchain — reject the payment
	return nil, errors.Errorf("no wallet configured for %s. Merchant must deploy a smart contract collector or set up an xpub wallet.", blockchain)
}

// createTransactionWithXpubAddress creates a transaction using an xpub-derived address (non-custodial flow)
func (s *Service) createTransactionWithXpubAddress(
	ctx context.Context,
	pt *payment.Payment,
	currency money.CryptoCurrency,
	xpubWallet *xpub.XpubWallet,
	cryptoAmount money.Money,
	cryptoServiceFee money.Money,
	usdAmount money.Money,
) (*payment.Method, error) {
	// Get next unused address from xpub wallet
	derivedAddr, err := s.xpubService.GetNextUnusedAddress(ctx, xpubWallet.ID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get next unused address from xpub wallet")
	}

	s.logger.Info().
		Str("address", derivedAddr.Address).
		Int("derivation_index", derivedAddr.DerivationIndex).
		Int64("xpub_wallet_id", xpubWallet.ID).
		Int64("payment_id", pt.ID).
		Msg("using xpub-derived address for payment")

	// Create transaction with the derived address (no traditional wallet)
	tx, err := s.transactions.Create(ctx, pt.MerchantID, transaction.CreateTransaction{
		Type:             transaction.TypeIncoming,
		EntityID:         pt.ID,
		RecipientAddress: derivedAddr.Address,
		Currency:         currency,
		Amount:           cryptoAmount,
		ServiceFee:       cryptoServiceFee,
		USDAmount:        usdAmount,
		IsTest:           pt.IsTest,
	})

	if err != nil {
		s.logger.Err(err).
			Str("ticker", currency.Ticker).
			Int64("payment_id", pt.ID).
			Str("address", derivedAddr.Address).
			Msg("unable to create transaction with xpub address")
		return nil, errors.Wrap(err, "unable to create transaction with xpub address")
	}

	// Mark the address as used and link to this payment
	if _, err := s.xpubService.MarkAddressAsUsed(ctx, derivedAddr.ID, pt.ID); err != nil {
		s.logger.Warn().Err(err).
			Int64("address_id", derivedAddr.ID).
			Int64("payment_id", pt.ID).
			Msg("unable to mark xpub address as used")
		// Don't fail the transaction creation, just log the warning
	}

	// Ensure the xpub-derived address is registered for payment detection
	if err := s.ensureXpubAddressSubscription(ctx, derivedAddr, currency); err != nil {
		s.logger.Warn().Err(err).
			Str("address", derivedAddr.Address).
			Int64("payment_id", pt.ID).
			Msg("unable to register xpub address for payment detection")
		// Don't fail - the scheduler will still poll for confirmation
	}

	return payment.MakeMethod(tx, currency), nil
}

// createTransactionWithCollectorAddress creates a transaction using a smart contract collector address.
// The collector's contract address is permanent — all payments for a given merchant/chain go there.
// Because all invoices share the same address, a random dust amount (1-999 base units) is added
// to make each expected amount unique. This lets the watcher match on-chain transfers by amount.
func (s *Service) createTransactionWithCollectorAddress(
	ctx context.Context,
	pt *payment.Payment,
	currency money.CryptoCurrency,
	collector *evmcollector.Collector,
	cryptoAmount money.Money,
	cryptoServiceFee money.Money,
	usdAmount money.Money,
) (*payment.Method, error) {
	// Add random dust (1-999 base units) to differentiate concurrent invoices
	// sharing the same collector address. For USDT (6 decimals) this is 0.000001–0.000999,
	// i.e. less than a tenth of a cent — negligible but unique.
	dustAmount, err := s.generateDustAmount(currency)
	if err == nil {
		withDust, addErr := cryptoAmount.Add(dustAmount)
		if addErr == nil {
			cryptoAmount = withDust
		} else {
			s.logger.Warn().Err(addErr).Msg("unable to add dust to crypto amount, using original")
		}
	} else {
		s.logger.Warn().Err(err).Msg("unable to generate dust amount, using original")
	}

	s.logger.Info().
		Str("contract_address", collector.ContractAddress).
		Str("blockchain", collector.Blockchain).
		Int64("payment_id", pt.ID).
		Str("amount", cryptoAmount.String()).
		Msg("using EVM collector contract address for payment")

	tx, err := s.transactions.Create(ctx, pt.MerchantID, transaction.CreateTransaction{
		Type:             transaction.TypeIncoming,
		EntityID:         pt.ID,
		RecipientAddress: collector.ContractAddress,
		Currency:         currency,
		Amount:           cryptoAmount,
		ServiceFee:       cryptoServiceFee,
		USDAmount:        usdAmount,
		IsTest:           pt.IsTest,
	})

	if err != nil {
		return nil, errors.Wrap(err, "unable to create transaction with collector address")
	}

	return payment.MakeMethod(tx, currency), nil
}

// AdminReconcileFill records a synthetic confirmed fill against the latest
// transaction on a payment. Used by superadmin when a customer paid the
// wrong/stale address — funds physically arrived somewhere reconcilable
// but the watcher never bound them to the invoice. The fill goes through
// the same partial-fill aggregator as a normal top-up: if it brings the
// cumulative confirmed sum to expected, the payment promotes; otherwise
// it stays partial and the customer can still top up the rest.
//
// rawAmount is the smallest-unit string (e.g. satoshis for BTC).
// notes is logged for audit purposes; not stored in the fill row itself.
func (s *Service) AdminReconcileFill(
	ctx context.Context,
	merchantID, paymentID int64,
	txHash, rawAmount, senderAddress, notes string,
) error {
	tx, err := s.transactions.GetLatestByPaymentID(ctx, paymentID)
	if err != nil {
		return errors.Wrap(err, "unable to get latest transaction for payment")
	}

	amountInt, ok := new(big.Int).SetString(rawAmount, 10)
	if !ok {
		return errors.New("invalid amount: must be smallest-unit decimal string")
	}
	amount, err := money.NewFromBigInt(money.Crypto, tx.Amount.Ticker(), amountInt, tx.Amount.Decimals())
	if err != nil {
		return errors.Wrap(err, "unable to construct amount")
	}

	networkID := tx.Currency.ChooseNetwork(tx.IsTest)
	input := Input{
		Currency:      tx.Currency,
		Amount:        amount,
		SenderAddress: senderAddress,
		TransactionID: txHash,
		NetworkID:     networkID,
	}
	if input.SenderAddress == "" {
		// ProcessInboundTransaction.Input.validate() requires non-empty sender.
		// Use a sentinel value so the flow accepts admin-attributed fills.
		input.SenderAddress = "admin-reconcile"
	}

	s.logger.Warn().
		Int64("payment_id", paymentID).
		Int64("merchant_id", merchantID).
		Str("tx_hash", txHash).
		Str("amount", amount.String()).
		Str("notes", notes).
		Msg("AUDIT: admin reconcile recording synthetic fill")

	return s.ProcessInboundTransaction(ctx, tx, nil, input)
}

// LatestFillIdempotencyKey returns "<network_id>:<tx_hash>:<vout_or_logidx>"
// for the most recently observed fill on a payment's transaction, or empty
// string when there are no fills. Used by the webhook handler so retried
// deliveries for the same fill arrive with the same dedup key.
func (s *Service) LatestFillIdempotencyKey(ctx context.Context, merchantID, paymentID int64) (string, error) {
	tx, err := s.transactions.GetLatestByPaymentID(ctx, paymentID)
	if err != nil {
		return "", err
	}
	fills, err := s.transactions.ListFills(ctx, tx)
	if err != nil || len(fills) == 0 {
		return "", err
	}
	last := fills[len(fills)-1]
	return fmt.Sprintf("%s:%s:%d", last.NetworkID, last.TransactionHash, last.VoutOrLogIdx), nil
}

// generateDustAmount returns a random amount visible at display precision for the given currency.
// This tiny amount makes each invoice's expected crypto value unique so the watcher
// can reliably match on-chain transfers when multiple invoices share a collector address.
//
// For ETH (18 native, 8 display): dust is 0.00000001–0.00000999 ETH (~$0.00002–$0.02)
// For USDT (6 native, 6 display): dust is 0.000001–0.000999 USDT (unchanged)
func (s *Service) generateDustAmount(currency money.CryptoCurrency) (money.Money, error) {
	// Random value in [1, 999]
	n, err := rand.Int(rand.Reader, big.NewInt(999))
	if err != nil {
		return money.Money{}, errors.Wrap(err, "crypto/rand failed")
	}
	dustBase := new(big.Int).Add(n, big.NewInt(1)) // shift from [0,998] to [1,999]

	// Scale dust so it's visible at display precision.
	// Without this, ETH dust (1-999 wei) would be zeroed out by truncation.
	maxDisplay := currency.MaxDisplayDecimals()
	if currency.Decimals > maxDisplay {
		scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(currency.Decimals-maxDisplay), nil)
		dustBase.Mul(dustBase, scale)
	}

	return money.NewFromBigInt(money.Crypto, currency.Ticker, dustBase, currency.Decimals)
}

func (s *Service) changePaymentMethod(
	ctx context.Context,
	p *payment.Payment,
	mt *merchant.Merchant,
	tx *transaction.Transaction,
	currency money.CryptoCurrency,
) (*payment.Method, error) {
	const cancelReason = "customer chose another payment method"

	if tx.RecipientWalletID == nil && tx.RecipientAddress == "" {
		return nil, errors.New("wallet id is nil")
	}

	if err := s.transactions.Cancel(ctx, tx, transaction.StatusCancelled, cancelReason, nil); err != nil {
		return nil, errors.Wrap(err, "unable to mark transaction as canceled")
	}

	return s.createIncomingTransaction(ctx, p, mt, currency)
}

// ensureXpubAddressSubscription is a no-op. The internal address watcher handles
// payment detection via polling.
func (s *Service) ensureXpubAddressSubscription(_ context.Context, _ *xpub.DerivedAddress, _ money.CryptoCurrency) error {
	return nil
}
