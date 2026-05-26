package cmd

import (
	"context"
	"fmt"

	"github.com/cryptolink/cryptolink/internal/app"
	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/service/processing"
	"github.com/cryptolink/cryptolink/internal/service/transaction"
	"github.com/cryptolink/cryptolink/internal/util"
	"github.com/spf13/cobra"
)

// recover-payment manually credits a pending invoice from an on-chain transfer
// the watcher missed (e.g. RPC outage caused the relevant block to fall outside
// the scan window). It calls ProcessInboundTransaction with the same payload
// the watcher would have produced, so balance updates, subscription usage
// tracking, customer/merchant emails and webhooks all fire normally.
var recoverPaymentCommand = &cobra.Command{
	Use:   "recover-payment",
	Short: "Manually credit a pending invoice from an on-chain tx the watcher missed",
	Run:   recoverPayment,
}

var recoverPaymentArgs = struct {
	TransactionID *int64
	TxHash        *string
	SenderAddress *string
	AmountRaw     *string
	IsTest        *bool
	YesIAmSure    *bool
}{
	TransactionID: util.Ptr(int64(0)),
	TxHash:        util.Ptr(""),
	SenderAddress: util.Ptr(""),
	AmountRaw:     util.Ptr(""),
	IsTest:        util.Ptr(false),
	YesIAmSure:    util.Ptr(false),
}

func recoverPayment(_ *cobra.Command, _ []string) {
	var (
		ctx               = context.Background()
		cfg               = resolveConfig()
		service           = app.New(ctx, cfg)
		txService         = service.Locator().TransactionService()
		processingService = service.Locator().ProcessingService()
		logger            = service.Logger()
		exit              = func(err error, message string) { logger.Fatal().Err(err).Msg(message) }
	)

	txID := *recoverPaymentArgs.TransactionID
	if txID == 0 {
		exit(nil, "tx-id is required")
	}

	tx, err := txService.GetByID(ctx, transaction.MerchantIDWildcard, txID)
	if err != nil {
		exit(err, "unable to load transaction")
	}

	if tx.Status != transaction.StatusPending {
		exit(nil, fmt.Sprintf("transaction is not pending (status=%s) — refusing to recover", tx.Status))
	}

	amount, err := money.CryptoFromRaw(tx.Currency.Ticker, *recoverPaymentArgs.AmountRaw, tx.Currency.Decimals)
	if err != nil {
		exit(err, "invalid amount")
	}

	networkID := tx.Currency.ChooseNetwork(*recoverPaymentArgs.IsTest)

	logger.Info().
		Int64("tx_id", tx.ID).
		Int64("payment_id", tx.EntityID).
		Str("currency", tx.Currency.Ticker).
		Str("amount", amount.String()).
		Str("expected", tx.Amount.String()).
		Str("sender", *recoverPaymentArgs.SenderAddress).
		Str("recipient", tx.RecipientAddress).
		Str("txhash", *recoverPaymentArgs.TxHash).
		Str("network_id", networkID).
		Msg("about to recover payment via ProcessInboundTransaction")

	if !*recoverPaymentArgs.YesIAmSure {
		if !confirm("Proceed?") {
			logger.Info().Msg("Aborting.")
			return
		}
	}

	input := processing.Input{
		Currency:      tx.Currency,
		Amount:        amount,
		SenderAddress: *recoverPaymentArgs.SenderAddress,
		TransactionID: *recoverPaymentArgs.TxHash,
		NetworkID:     networkID,
	}

	// Collector / xpub flow: wallet is nil. Managed hot wallet would require
	// the wallet object, but xpub & EVM collector payments don't have one.
	if err := processingService.ProcessInboundTransaction(ctx, tx, nil, input); err != nil {
		exit(err, "ProcessInboundTransaction failed")
	}

	logger.Info().
		Int64("tx_id", tx.ID).
		Int64("payment_id", tx.EntityID).
		Msg("recovery complete — invoice credited; balance, usage, emails and webhooks should fire normally")
}

func recoverPaymentSetup(cmd *cobra.Command) {
	f := cmd.Flags()

	f.Int64Var(recoverPaymentArgs.TransactionID, "tx-id", 0, "Internal transaction.id (transactions table) of the pending invoice")
	f.StringVar(recoverPaymentArgs.TxHash, "tx-hash", "", "On-chain transaction hash (with 0x prefix for EVM)")
	f.StringVar(recoverPaymentArgs.SenderAddress, "sender", "", "On-chain sender address")
	f.StringVar(recoverPaymentArgs.AmountRaw, "amount-raw", "", "Raw amount in smallest unit (wei for ETH, satoshis for BTC, etc.)")
	f.BoolVar(recoverPaymentArgs.IsTest, "is-test", false, "Use testnet network ID")
	f.BoolVar(recoverPaymentArgs.YesIAmSure, "yes", false, "Skip interactive confirmation")

	for _, name := range []string{"tx-id", "tx-hash", "sender", "amount-raw"} {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic(name + ": " + err.Error())
		}
	}
}
