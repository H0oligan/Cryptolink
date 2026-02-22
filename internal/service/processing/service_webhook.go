package processing

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/google/uuid"
	kms "github.com/cryptolink/cryptolink/internal/kms/wallet"
	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/service/email"
	"github.com/cryptolink/cryptolink/internal/service/evmcollector"
	"github.com/cryptolink/cryptolink/internal/service/transaction"
	"github.com/cryptolink/cryptolink/internal/service/wallet"
	"github.com/cryptolink/cryptolink/internal/service/xpub"
	"github.com/pkg/errors"
)

// TatumWebhook see https://apidoc.tatum.io/tag/Notification-subscriptions#operation/createSubscription
type TatumWebhook struct {
	SubscriptionType string `json:"subscriptionType"`
	TransactionID    string `json:"txId"`
	Address          string `json:"address"`
	Sender           string `json:"counterAddress"`

	// Asset coin name or token contact address or (!) token ticker e.g. USDT_TRON
	Asset string `json:"asset"`

	// Amount "0.000123" (float) instead of "123" (wei-like)
	Amount      string `json:"amount"`
	BlockNumber int64  `json:"blockNumber"`

	// Type can be ['native', 'token', `trc20', 'fee'] or maybe any other
	Type string `json:"type"`

	// Mempool (EMV-based blockchains only) if appears and set to "true", the transaction is in the mempool;
	// if set to "false" or does not appear at all, the transaction has been added to a block
	Mempool bool `json:"mempool"`

	// Chain for ETH test is might be 'ethereum-goerli' or 'sepolia'
	Chain string `json:"chain"`
}

func (w *TatumWebhook) MarshalBinary() ([]byte, error) {
	return json.Marshal(w)
}

func (w *TatumWebhook) CurrencyType() money.CryptoCurrencyType {
	if w.Type == "native" {
		return money.Coin
	}

	return money.Token
}

// ValidateWebhookSignature performs HMAC signature validation
func (s *Service) ValidateWebhookSignature(body []byte, hash string) error {
	if valid := s.tatumProvider.ValidateHMAC(body, hash); !valid {
		return ErrSignatureVerification
	}

	return nil
}

func (s *Service) ProcessIncomingWebhook(ctx context.Context, walletID uuid.UUID, networkID string, wh TatumWebhook) error {
	// 0. Omit certain webhooks
	switch {
	case wh.Mempool:
		s.logger.Info().Str("blockchain_tx_hash_id", wh.TransactionID).Msg("skipping mempool transaction")
		return nil
	case wh.Type == "fee":
		s.logger.Info().Str("blockchain_tx_hash_id", wh.TransactionID).Msg("skipping fee webhook")
		return nil
	}

	// 1. Try to resolve as traditional wallet first
	wt, walletErr := s.wallets.GetByUUID(ctx, walletID)
	if walletErr == nil {
		// Traditional wallet flow
		return s.processTraditionalWebhook(ctx, wt, networkID, wh)
	}

	// 2. Try to resolve as xpub-derived address
	derivedAddr, xpubErr := s.xpubService.GetDerivedAddressByUUID(ctx, walletID)
	if xpubErr == nil {
		// Xpub address flow
		return s.processXpubWebhook(ctx, derivedAddr, networkID, wh)
	}

	// 3. Try to resolve as EVM collector contract address
	if s.evmCollector != nil {
		collector, collectorErr := s.evmCollector.GetByUUID(ctx, walletID)
		if collectorErr == nil {
			return s.processCollectorWebhook(ctx, collector, networkID, wh)
		}
	}

	// None of the known webhook targets matched
	s.logger.Error().
		Err(walletErr).
		Str("wallet_uuid", walletID.String()).
		Msg("unable to resolve webhook target (not a wallet, xpub address, or evm collector)")

	return errors.Wrap(walletErr, "unable to get wallet by uuid")
}

// processTraditionalWebhook handles webhooks for traditional (hot wallet) addresses
func (s *Service) processTraditionalWebhook(ctx context.Context, wt *wallet.Wallet, networkID string, wh TatumWebhook) error {
	currency, err := s.resolveCurrencyFromWebhook(wt.Blockchain.ToMoneyBlockchain(), networkID, wh)
	if err != nil {
		return errors.Wrap(err, "unable to resolve currency from webhook")
	}

	amount, err := money.CryptoFromStringFloat(currency.Ticker, wh.Amount, currency.Decimals)
	if err != nil {
		return errors.Wrap(err, "unable to make crypto amount from webhook data")
	}

	input := Input{
		Currency:      currency,
		Amount:        amount,
		SenderAddress: wh.Sender,
		TransactionID: wh.TransactionID,
		NetworkID:     networkID,
	}

	processors := []webhookProcessor{
		s.processTronAccountActivation,
		s.processExpectedWebhook,
		s.processUnexpectedWebhook,
	}

	for _, ingest := range processors {
		err := ingest(ctx, wt, input)

		if errors.Is(err, errSkippedProcessor) {
			continue
		}

		if err != nil {
			return errors.Wrap(err, "unable to process webhook")
		}

		break
	}

	return nil
}

// processXpubWebhook handles webhooks for xpub-derived addresses (non-custodial flow)
func (s *Service) processXpubWebhook(ctx context.Context, addr *xpub.DerivedAddress, networkID string, wh TatumWebhook) error {
	bc := money.Blockchain(addr.Blockchain)

	currency, err := s.resolveCurrencyFromWebhook(bc, networkID, wh)
	if err != nil {
		return errors.Wrap(err, "unable to resolve currency from xpub webhook")
	}

	amount, err := money.CryptoFromStringFloat(currency.Ticker, wh.Amount, currency.Decimals)
	if err != nil {
		return errors.Wrap(err, "unable to make crypto amount from xpub webhook data")
	}

	input := Input{
		Currency:      currency,
		Amount:        amount,
		SenderAddress: wh.Sender,
		TransactionID: wh.TransactionID,
		NetworkID:     networkID,
	}

	// Find pending transaction by recipient address
	tx, err := s.transactions.GetByFilter(ctx, transaction.Filter{
		RecipientAddress: addr.Address,
		NetworkID:        input.NetworkID,
		Currency:         input.Currency.Ticker,
		Statuses:         []transaction.Status{transaction.StatusPending},
		Types:            []transaction.Type{transaction.TypeIncoming},
		HashIsEmpty:      true,
	})

	if err != nil {
		if errors.Is(err, transaction.ErrNotFound) {
			s.logger.Info().
				Str("address", addr.Address).
				Str("blockchain_tx_hash_id", wh.TransactionID).
				Msg("no pending xpub transaction found for webhook, might be unexpected")
			return nil
		}
		return errors.Wrap(err, "unable to find xpub transaction")
	}

	// Process the inbound transaction (nil wallet is fine for xpub flow)
	if err := s.ProcessInboundTransaction(ctx, tx, nil, input); err != nil {
		return errors.Wrap(err, "unable to process xpub incoming transaction")
	}

	s.logger.Info().
		Str("address", addr.Address).
		Int64("transaction_id", tx.ID).
		Str("blockchain_tx_hash_id", wh.TransactionID).
		Msg("processed xpub incoming transaction via webhook")

	return nil
}

// processCollectorWebhook handles webhooks for EVM smart contract collector addresses.
func (s *Service) processCollectorWebhook(ctx context.Context, collector *evmcollector.Collector, networkID string, wh TatumWebhook) error {
	bc := money.Blockchain(collector.Blockchain)

	// The webhook URL may carry the blockchain name (e.g. "ETH") instead of the
	// numeric chain ID (e.g. "1") that resolveCurrencyFromWebhook and transaction
	// filters require. Use the chain ID stored on the collector record instead.
	effectiveNetworkID := networkID
	if collector.ChainID != 0 {
		effectiveNetworkID = strconv.Itoa(collector.ChainID)
	}

	currency, err := s.resolveCurrencyFromWebhook(bc, effectiveNetworkID, wh)
	if err != nil {
		return errors.Wrap(err, "unable to resolve currency from collector webhook")
	}

	amount, err := money.CryptoFromStringFloat(currency.Ticker, wh.Amount, currency.Decimals)
	if err != nil {
		return errors.Wrap(err, "unable to make crypto amount from collector webhook data")
	}

	input := Input{
		Currency:      currency,
		Amount:        amount,
		SenderAddress: wh.Sender,
		TransactionID: wh.TransactionID,
		NetworkID:     effectiveNetworkID,
	}

	// Find pending transaction for this collector's contract address
	tx, err := s.transactions.GetByFilter(ctx, transaction.Filter{
		RecipientAddress: collector.ContractAddress,
		NetworkID:        input.NetworkID,
		Currency:         input.Currency.Ticker,
		Statuses:         []transaction.Status{transaction.StatusPending},
		Types:            []transaction.Type{transaction.TypeIncoming},
		HashIsEmpty:      true,
	})

	if err != nil {
		if errors.Is(err, transaction.ErrNotFound) {
			s.logger.Info().
				Str("contract_address", collector.ContractAddress).
				Str("blockchain_tx_hash_id", wh.TransactionID).
				Str("amount", wh.Amount).
				Str("ticker", currency.Ticker).
				Msg("no pending collector transaction found for webhook (unexpected payment to collector)")
			return nil
		}
		return errors.Wrap(err, "unable to find collector transaction")
	}

	// Process the inbound transaction
	if err := s.ProcessInboundTransaction(ctx, tx, nil, input); err != nil {
		return errors.Wrap(err, "unable to process collector incoming transaction")
	}

	s.logger.Info().
		Str("contract_address", collector.ContractAddress).
		Int64("transaction_id", tx.ID).
		Str("blockchain_tx_hash_id", wh.TransactionID).
		Msg("processed collector incoming transaction via webhook")

	// Send payment received email notification (best-effort, non-blocking)
	if s.emailService != nil {
		go s.sendPaymentReceivedEmail(context.Background(), collector.MerchantID, tx, currency, wh)
	}

	return nil
}

// sendPaymentReceivedEmail looks up the merchant email and sends the payment notification.
func (s *Service) sendPaymentReceivedEmail(ctx context.Context, merchantID int64, tx *transaction.Transaction, currency money.CryptoCurrency, wh TatumWebhook) {
	mt, err := s.merchants.GetByID(ctx, merchantID, false)
	if err != nil {
		s.logger.Warn().Err(err).Int64("merchant_id", merchantID).Msg("unable to get merchant for payment email")
		return
	}

	merchantEmail, err := s.evmCollector.GetMerchantEmail(ctx, merchantID)
	if err != nil || merchantEmail == "" {
		s.logger.Warn().Err(err).Int64("merchant_id", merchantID).Msg("no merchant email found for payment notification")
		return
	}

	explorerLink := ""
	if link, linkErr := tx.ExplorerLink(); linkErr == nil {
		explorerLink = link
	}

	networkName := string(currency.Blockchain)

	params := email.PaymentReceivedParams{
		MerchantEmail:    merchantEmail,
		MerchantName:     mt.Name,
		TxHash:           wh.TransactionID,
		Amount:           wh.Amount,
		Ticker:           currency.Ticker,
		USDAmount:        tx.USDAmount.String(),
		SenderAddress:    wh.Sender,
		RecipientAddress: wh.Address,
		ExplorerLink:     explorerLink,
		Network:          networkName,
		ReceivedAt:       tx.CreatedAt,
	}

	s.emailService.SendPaymentReceived(ctx, params)
}

var errSkippedProcessor = errors.New("processor is skipped")

type webhookProcessor func(ctx context.Context, wt *wallet.Wallet, input Input) error

func (s *Service) processExpectedWebhook(ctx context.Context, wt *wallet.Wallet, input Input) error {
	tx, err := s.transactions.GetByFilter(ctx, transaction.Filter{
		RecipientWalletID: wt.ID,
		NetworkID:         input.NetworkID,
		Currency:          input.Currency.Ticker,
		Statuses:          []transaction.Status{transaction.StatusPending},
		Types:             []transaction.Type{transaction.TypeIncoming},
		HashIsEmpty:       true,
	})

	switch {
	case errors.Is(err, transaction.ErrNotFound):
		return errSkippedProcessor
	case err != nil:
		s.logger.Warn().Err(err).
			Int64("wallet_id", wt.ID).
			Str("blockchain_tx_hash_id", input.TransactionID).
			Msg("unable to find transaction")

		return errors.Wrap(err, "unable to find transaction")
	}

	if err := s.ProcessInboundTransaction(ctx, tx, wt, input); err != nil {
		return errors.Wrap(err, "unable to process incoming transaction")
	}

	s.logger.Info().
		Str("transaction_type", string(tx.Type)).
		Int64("wallet_id", wt.ID).
		Int64("transaction_id", tx.ID).
		Str("blockchain_tx_hash_id", input.TransactionID).
		Msg("Processed incoming transaction")

	return nil
}

func (s *Service) processUnexpectedWebhook(ctx context.Context, wt *wallet.Wallet, input Input) error {
	tx, err := s.transactions.GetByHash(ctx, input.NetworkID, input.TransactionID)

	switch {
	case errors.Is(err, transaction.ErrNotFound):
		if errCreate := s.createUnexpectedTransaction(ctx, wt, input); errCreate != nil {
			return errors.Wrap(errCreate, "unable to create unexpected transaction")
		}
		return nil
	case err != nil:
		return errors.Wrap(err, "unable to get transaction by hash")
	}

	s.logger.Info().
		Int64("wallet_id", wt.ID).
		Int64("transaction_id", tx.ID).
		Str("currency", input.Amount.Ticker()).
		Str("network_id", input.NetworkID).
		Msg("Skipping unexpected webhook")

	return nil
}

// https://developers.tron.network/docs/account#account-activation
func (s *Service) processTronAccountActivation(ctx context.Context, wt *wallet.Wallet, input Input) error {
	isTronCoin := wt.Blockchain == kms.TRON && input.Currency.Type == money.Coin
	isOneTrx := input.Amount.StringRaw() == "1"

	if !isTronCoin || !isOneTrx {
		return errSkippedProcessor
	}

	s.logger.Info().
		Int64("wallet_id", wt.ID).
		Str("blockchain_tx_hash_id", input.TransactionID).
		Str("currency", input.Amount.Ticker()).
		Str("network_id", input.NetworkID).
		Msg("received address activation transaction")

	return s.processUnexpectedWebhook(ctx, wt, input)
}

func (s *Service) resolveCurrencyFromWebhook(bc money.Blockchain, networkID string, wh TatumWebhook) (money.CryptoCurrency, error) {
	var (
		currency money.CryptoCurrency
		err      error
		isCoin   = wh.CurrencyType() == money.Coin
	)

	if isCoin {
		currency, err = s.blockchain.GetNativeCoin(bc)
	} else {
		currency, err = s.blockchain.GetCurrencyByBlockchainAndContract(bc, networkID, wh.Asset)
	}

	if err != nil {
		if !isCoin {
			s.logger.Warn().Err(err).
				Str("blockchain", bc.String()).
				Str("contract_address", wh.Asset).
				Str("transaction_hash", wh.TransactionID).
				Msg("unknown asset occurred")
		}

		return money.CryptoCurrency{}, err
	}

	// guard unknown network ids
	if currency.NetworkID != networkID && currency.TestNetworkID != networkID {
		return money.CryptoCurrency{}, errors.Errorf(
			"unknown %s network id %q, expected one of [%s, %s]",
			currency.Blockchain.String(), networkID, currency.NetworkID, currency.TestNetworkID,
		)
	}

	return currency, nil
}
