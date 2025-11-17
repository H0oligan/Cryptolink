package blockchain

import (
	"context"
	"math/big"
	"time"

	kmswallet "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/pkg/errors"
)

type FeeCalculator interface {
	CalculateFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error)
	CalculateWithdrawalFeeUSD(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (money.Money, error)
}

// withdrawalNetworkFeeMultiplier when customer wants to withdraw his assets from the system, we already spent
// 1x network fee for INBOUND -> OUTBOUND processing. In total o2pay would pay x2 network fee in order to withdraw
// assets. So it should be kinda fair if customer pays for 0.5x INBOUND -> OUTBOUND & x1 for OUTBOUND -> EXTERNAL.
const withdrawalNetworkFeeMultiplier = 1.5

// CalculateFee calculates blockchain transaction fee for selected currency.
func (s *Service) CalculateFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	if baseCurrency.Type != money.Coin || baseCurrency.Blockchain != currency.Blockchain {
		return Fee{}, errors.New("invalid arguments")
	}

	switch kmswallet.Blockchain(currency.Blockchain) {
	case kmswallet.ETH:
		return s.ethFee(ctx, baseCurrency, currency, isTest)
	case kmswallet.MATIC:
		return s.maticFee(ctx, baseCurrency, currency, isTest)
	case kmswallet.BSC:
		return s.bscFee(ctx, baseCurrency, currency, isTest)
	case kmswallet.ARBITRUM:
		return s.arbitrumFee(ctx, baseCurrency, currency, isTest)
	case kmswallet.AVAX:
		return s.avaxFee(ctx, baseCurrency, currency, isTest)
	case kmswallet.TRON:
		return s.tronFee(ctx, baseCurrency, currency, isTest)
	case kmswallet.SOL:
		return s.solanaFee(ctx, baseCurrency, currency, isTest)
	case kmswallet.XMR:
		return s.moneroFee(ctx, baseCurrency, currency, isTest)
	}

	return Fee{}, errors.New("unsupported blockchain for fees calculations " + currency.Ticker)
}

// CalculateWithdrawalFeeUSD withdrawal fees are tied to network fee but calculated in USD
// Example: usdFee, err := CalculateWithdrawalFeeUSD(ctx, eth, ethUSD, false)
func (s *Service) CalculateWithdrawalFeeUSD(
	ctx context.Context,
	baseCurrency, currency money.CryptoCurrency,
	isTest bool,
) (money.Money, error) {
	fee, err := s.CalculateFee(ctx, baseCurrency, currency, isTest)
	if err != nil {
		return money.Money{}, err
	}

	var usdFee money.Money

	switch kmswallet.Blockchain(fee.Currency.Blockchain) {
	case kmswallet.ETH:
		f, _ := fee.ToEthFee()
		usdFee = f.totalCostUSD
	case kmswallet.MATIC:
		f, _ := fee.ToMaticFee()
		usdFee = f.totalCostUSD
	case kmswallet.BSC:
		f, _ := fee.ToBSCFee()
		usdFee = f.totalCostUSD
	case kmswallet.ARBITRUM:
		f, _ := fee.ToArbitrumFee()
		usdFee = f.totalCostUSD
	case kmswallet.AVAX:
		f, _ := fee.ToAvaxFee()
		usdFee = f.totalCostUSD
	case kmswallet.TRON:
		f, _ := fee.ToTronFee()
		usdFee = f.feeLimitUSD
	case kmswallet.SOL:
		f, _ := fee.ToSolanaFee()
		usdFee = f.totalCostUSD
	case kmswallet.XMR:
		f, _ := fee.ToMoneroFee()
		usdFee = f.totalCostUSD
	default:
		return money.Money{}, ErrCurrencyNotFound
	}

	// Sometimes crypto fee lower than 1 cent, so du to rounding error we can get usdFee = $0.0.
	// We shouldn't allow that, so let's force it to 1 cent
	if usdFee.IsZero() {
		return money.FiatFromFloat64(money.USD, 0.01)
	}

	return usdFee.MultiplyFloat64(withdrawalNetworkFeeMultiplier)
}

type Fee struct {
	CalculatedAt time.Time
	Currency     money.CryptoCurrency
	IsTest       bool
	raw          any
}

func NewFee(currency money.CryptoCurrency, at time.Time, isTest bool, fee any) Fee {
	return Fee{
		CalculatedAt: at,
		Currency:     currency,
		IsTest:       isTest,
		raw:          fee,
	}
}

type EthFee struct {
	GasUnits     uint   `json:"gasUnits"`
	GasPrice     string `json:"gasPrice"`
	PriorityFee  string `json:"priorityFee"`
	TotalCostWEI string `json:"totalCostWei"`
	TotalCostETH string `json:"totalCostEth"`
	TotalCostUSD string `json:"totalCostUsd"`

	totalCostUSD money.Money
}

func (f *Fee) ToEthFee() (EthFee, error) {
	if fee, ok := f.raw.(EthFee); ok {
		return fee, nil
	}

	return EthFee{}, errors.New("invalid fee type assertion for ETH")
}

func (s *Service) ethFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	const (
		gasUnitsForCoin  = 21_000
		gasUnitsForToken = 65_000

		gasConfidentRate = 1.15
	)

	bigIntToETH := func(i *big.Int) (money.Money, error) {
		return money.NewFromBigInt(money.Crypto, baseCurrency.Ticker, i, baseCurrency.Decimals)
	}

	// 1. Connect to ETH node
	client, err := s.providers.Tatum.EthereumRPC(ctx, isTest)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to setup RPC")
	}

	// 2. Calculate gasPrice
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest gas price")
	}

	gasPriceETH, err := bigIntToETH(gasPrice)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make ETH from gas price")
	}

	// In order to be confident that tx will be processed, let's multiply price by gasConfidentRate
	gasPriceETHConfident, err := gasPriceETH.MultiplyFloat64(gasConfidentRate)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to multiply ETH gas price")
	}

	// 3. Calculate priorityFee
	priorityFee, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest ETH gas tip cap")
	}

	priorityFeeETH, err := bigIntToETH(priorityFee)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest make ETH from priorityFee")
	}

	// 4. Calculate gasUnits and total cost in WEI
	totalFeePerGas, err := gasPriceETHConfident.Add(priorityFeeETH)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total fee per gas")
	}

	gasUnits := gasUnitsForCoin
	if currency.Type == money.Token {
		gasUnits = gasUnitsForToken
	}

	totalCost, err := totalFeePerGas.MultiplyFloat64(float64(gasUnits))
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total tx cost")
	}

	conv, err := s.CryptoToFiat(ctx, totalCost, money.USD)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total cost in USD")
	}

	return NewFee(currency, time.Now().UTC(), isTest, EthFee{
		GasUnits:     uint(gasUnits),
		GasPrice:     gasPriceETHConfident.StringRaw(),
		PriorityFee:  priorityFeeETH.StringRaw(),
		TotalCostWEI: totalCost.StringRaw(),
		TotalCostETH: totalCost.String(),
		TotalCostUSD: conv.To.String(),

		totalCostUSD: conv.To,
	}), nil
}

type MaticFee struct {
	GasUnits       uint   `json:"gasUnits"`
	GasPrice       string `json:"gasPrice"`
	PriorityFee    string `json:"priorityFee"`
	TotalCostWEI   string `json:"totalCostWei"`
	TotalCostMATIC string `json:"totalCostMatic"`
	TotalCostUSD   string `json:"totalCostUsd"`

	totalCostUSD money.Money
}

func (f *Fee) ToMaticFee() (MaticFee, error) {
	if fee, ok := f.raw.(MaticFee); ok {
		return fee, nil
	}

	return MaticFee{}, errors.New("invalid fee type assertion for MATIC")
}

func (s *Service) maticFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	const (
		gasUnitsForCoin  = 21_000
		gasUnitsForToken = 65_000

		gasConfidentRate = 1.10
	)

	bigIntToMATIC := func(i *big.Int) (money.Money, error) {
		return money.NewFromBigInt(money.Crypto, baseCurrency.Ticker, i, baseCurrency.Decimals)
	}

	// 1. Connect to MATIC node
	client, err := s.providers.Tatum.MaticRPC(ctx, isTest)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to setup RPC")
	}

	// 2. Calculate gasPrice
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest gas price")
	}

	gasPriceMATIC, err := bigIntToMATIC(gasPrice)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make MATIC from gas price")
	}

	// In order to be confident that tx will be processed, let's multiply price by gasConfidentRate
	gasPriceMATICConfident, err := gasPriceMATIC.MultiplyFloat64(gasConfidentRate)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to multiply MATIC gas price")
	}

	// 3. Calculate priorityFee
	priorityFee, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest MATIC gas tip cap")
	}

	priorityFeeMATIC, err := bigIntToMATIC(priorityFee)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest make MATIC from priorityFee")
	}

	// 4. Calculate gasUnits and total cost in WEI
	totalFeePerGas, err := gasPriceMATICConfident.Add(priorityFeeMATIC)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total fee per gas")
	}

	gasUnits := gasUnitsForCoin
	if currency.Type == money.Token {
		gasUnits = gasUnitsForToken
	}

	totalCost, err := totalFeePerGas.MultiplyFloat64(float64(gasUnits))
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total tx cost")
	}

	conv, err := s.CryptoToFiat(ctx, totalCost, money.USD)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total cost in USD")
	}

	return NewFee(currency, time.Now().UTC(), isTest, MaticFee{
		GasUnits:       uint(gasUnits),
		GasPrice:       gasPriceMATICConfident.StringRaw(),
		PriorityFee:    priorityFeeMATIC.StringRaw(),
		TotalCostWEI:   totalCost.StringRaw(),
		TotalCostMATIC: totalCost.String(),
		TotalCostUSD:   conv.To.String(),

		totalCostUSD: conv.To,
	}), nil
}

type BSCFee struct {
	GasUnits     uint   `json:"gasUnits"`
	GasPrice     string `json:"gasPrice"`
	PriorityFee  string `json:"priorityFee"`
	TotalCostWEI string `json:"totalCostWei"`
	TotalCostBNB string `json:"totalCostBNB"`
	TotalCostUSD string `json:"totalCostUsd"`

	totalCostUSD money.Money
}

func (f *Fee) ToBSCFee() (BSCFee, error) {
	if fee, ok := f.raw.(BSCFee); ok {
		return fee, nil
	}

	return BSCFee{}, errors.New("invalid fee type assertion for BSC")
}

func (s *Service) bscFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	const (
		gasUnitsForCoin  = 21_000
		gasUnitsForToken = 65_000

		gasConfidentRate = 1.10
	)

	// 1. Connect to BSC node
	client, err := s.providers.Tatum.BinanceSmartChainRPC(ctx, isTest)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to setup RPC")
	}

	// 2. Calculate gasPrice
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest gas price")
	}

	gasPriceMATIC, err := baseCurrency.MakeAmountFromBigInt(gasPrice)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make BSC from gas price")
	}

	// In order to be confident that tx will be processed, let's multiply price by gasConfidentRate
	gasPriceMATICConfident, err := gasPriceMATIC.MultiplyFloat64(gasConfidentRate)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to multiply BSC gas price")
	}

	// 3. Calculate priorityFee
	priorityFee, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest BSC gas tip cap")
	}

	priorityFeeBSC, err := baseCurrency.MakeAmountFromBigInt(priorityFee)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest make BSC from priorityFee")
	}

	// 4. Calculate gasUnits and total cost in WEI
	totalFeePerGas, err := gasPriceMATICConfident.Add(priorityFeeBSC)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total fee per gas")
	}

	gasUnits := gasUnitsForCoin
	if currency.Type == money.Token {
		gasUnits = gasUnitsForToken
	}

	totalCost, err := totalFeePerGas.MultiplyFloat64(float64(gasUnits))
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total tx cost")
	}

	conv, err := s.CryptoToFiat(ctx, totalCost, money.USD)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total cost in USD")
	}

	return NewFee(currency, time.Now().UTC(), isTest, BSCFee{
		GasUnits:     uint(gasUnits),
		GasPrice:     gasPriceMATICConfident.StringRaw(),
		PriorityFee:  priorityFeeBSC.StringRaw(),
		TotalCostWEI: totalCost.StringRaw(),
		TotalCostBNB: totalCost.String(),
		TotalCostUSD: conv.To.String(),

		totalCostUSD: conv.To,
	}), nil
}

type TronFee struct {
	FeeLimitSun uint64 `json:"feeLimit"`
	FeeLimitTRX string `json:"feeLimitTrx"`
	FeeLimitUSD string `json:"feeLimitUsd"`

	feeLimitUSD money.Money
}

func (f *Fee) ToTronFee() (TronFee, error) {
	if fee, ok := f.raw.(TronFee); ok {
		return fee, nil
	}

	return TronFee{}, errors.New("invalid fee type assertion for TRON")
}

func (s *Service) tronFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	const (
		bandwidthSunCost      = int64(1000)
		coinTransferBandwidth = int64(350)

		// 30.01.23: based on avg tronscan data ~ 15 trx
		// 14.06.23: https://support.ledger.com/hc/en-us/articles/8085235615133-Tether-USDT-transaction-on-Tron-failed-and-ran-out-of-energy
		tokenTransactionSun = int64(30 * 1_000_000)
	)

	intToTRON := func(i int64) (money.Money, error) {
		return money.NewFromBigInt(money.Crypto, baseCurrency.Ticker, big.NewInt(i), baseCurrency.Decimals)
	}

	feeLimit := bandwidthSunCost * coinTransferBandwidth
	if currency.Type == money.Token {
		feeLimit = tokenTransactionSun
	}

	feeLimitTRON, err := intToTRON(feeLimit)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make TRON from int")
	}

	conv, err := s.CryptoToFiat(ctx, feeLimitTRON, money.USD)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total cost in USD")
	}

	return NewFee(currency, time.Now().UTC(), isTest, TronFee{
		FeeLimitSun: uint64(feeLimit),
		FeeLimitTRX: feeLimitTRON.String(),
		FeeLimitUSD: conv.To.String(),

		feeLimitUSD: conv.To,
	}), nil
}

// Arbitrum Fee structures and methods
type ArbitrumFee struct {
	GasUnits     uint   `json:"gasUnits"`
	GasPrice     string `json:"gasPrice"`
	PriorityFee  string `json:"priorityFee"`
	TotalCostWEI string `json:"totalCostWei"`
	TotalCostETH string `json:"totalCostEth"`
	TotalCostUSD string `json:"totalCostUsd"`

	totalCostUSD money.Money
}

func (f *Fee) ToArbitrumFee() (ArbitrumFee, error) {
	if fee, ok := f.raw.(ArbitrumFee); ok {
		return fee, nil
	}
	return ArbitrumFee{}, errors.New("invalid fee type assertion for ARBITRUM")
}

func (s *Service) arbitrumFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	const (
		gasUnitsForCoin  = 21_000
		gasUnitsForToken = 65_000
		gasConfidentRate = 1.10 // Arbitrum has lower and more stable fees than ETH mainnet
	)

	bigIntToETH := func(i *big.Int) (money.Money, error) {
		return money.NewFromBigInt(money.Crypto, baseCurrency.Ticker, i, baseCurrency.Decimals)
	}

	client, err := s.providers.Tatum.ArbitrumRPC(ctx, isTest)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to setup Arbitrum RPC")
	}
	defer client.Close()

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest gas price")
	}

	gasPriceETH, err := bigIntToETH(gasPrice)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make ETH from gas price")
	}

	gasPriceConfident, err := gasPriceETH.MultiplyFloat64(gasConfidentRate)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to multiply gas price")
	}

	priorityFee, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest gas tip cap")
	}

	priorityFeeETH, err := bigIntToETH(priorityFee)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make ETH from priorityFee")
	}

	totalFeePerGas, err := gasPriceConfident.Add(priorityFeeETH)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total fee per gas")
	}

	gasUnits := gasUnitsForCoin
	if currency.Type == money.Token {
		gasUnits = gasUnitsForToken
	}

	totalCost, err := totalFeePerGas.MultiplyFloat64(float64(gasUnits))
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total tx cost")
	}

	conv, err := s.CryptoToFiat(ctx, totalCost, money.USD)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total cost in USD")
	}

	return NewFee(currency, time.Now().UTC(), isTest, ArbitrumFee{
		GasUnits:     uint(gasUnits),
		GasPrice:     gasPriceConfident.StringRaw(),
		PriorityFee:  priorityFeeETH.StringRaw(),
		TotalCostWEI: totalCost.StringRaw(),
		TotalCostETH: totalCost.String(),
		TotalCostUSD: conv.To.String(),
		totalCostUSD: conv.To,
	}), nil
}

// Avalanche Fee structures and methods
type AvaxFee struct {
	GasUnits      uint   `json:"gasUnits"`
	GasPrice      string `json:"gasPrice"`
	PriorityFee   string `json:"priorityFee"`
	TotalCostWEI  string `json:"totalCostWei"`
	TotalCostAVAX string `json:"totalCostAvax"`
	TotalCostUSD  string `json:"totalCostUsd"`

	totalCostUSD money.Money
}

func (f *Fee) ToAvaxFee() (AvaxFee, error) {
	if fee, ok := f.raw.(AvaxFee); ok {
		return fee, nil
	}
	return AvaxFee{}, errors.New("invalid fee type assertion for AVAX")
}

func (s *Service) avaxFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	const (
		gasUnitsForCoin  = 21_000
		gasUnitsForToken = 65_000
		gasConfidentRate = 1.10 // Avalanche C-Chain uses similar gas model to Ethereum
	)

	bigIntToAVAX := func(i *big.Int) (money.Money, error) {
		return money.NewFromBigInt(money.Crypto, baseCurrency.Ticker, i, baseCurrency.Decimals)
	}

	client, err := s.providers.Tatum.AvalancheRPC(ctx, isTest)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to setup Avalanche RPC")
	}
	defer client.Close()

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest gas price")
	}

	gasPriceAVAX, err := bigIntToAVAX(gasPrice)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make AVAX from gas price")
	}

	gasPriceConfident, err := gasPriceAVAX.MultiplyFloat64(gasConfidentRate)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to multiply gas price")
	}

	priorityFee, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest gas tip cap")
	}

	priorityFeeAVAX, err := bigIntToAVAX(priorityFee)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make AVAX from priorityFee")
	}

	totalFeePerGas, err := gasPriceConfident.Add(priorityFeeAVAX)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total fee per gas")
	}

	gasUnits := gasUnitsForCoin
	if currency.Type == money.Token {
		gasUnits = gasUnitsForToken
	}

	totalCost, err := totalFeePerGas.MultiplyFloat64(float64(gasUnits))
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total tx cost")
	}

	conv, err := s.CryptoToFiat(ctx, totalCost, money.USD)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total cost in USD")
	}

	return NewFee(currency, time.Now().UTC(), isTest, AvaxFee{
		GasUnits:      uint(gasUnits),
		GasPrice:      gasPriceConfident.StringRaw(),
		PriorityFee:   priorityFeeAVAX.StringRaw(),
		TotalCostWEI:  totalCost.StringRaw(),
		TotalCostAVAX: totalCost.String(),
		TotalCostUSD:  conv.To.String(),
		totalCostUSD:  conv.To,
	}), nil
}

// Solana Fee structures and methods
type SolanaFee struct {
	FeePerSignature uint64 `json:"feePerSignature"`
	TotalCostSOL    string `json:"totalCostSol"`
	TotalCostUSD    string `json:"totalCostUsd"`

	totalCostUSD money.Money
}

func (f *Fee) ToSolanaFee() (SolanaFee, error) {
	if fee, ok := f.raw.(SolanaFee); ok {
		return fee, nil
	}
	return SolanaFee{}, errors.New("invalid fee type assertion for SOL")
}

func (s *Service) solanaFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	// Solana has fixed fees per signature (currently 5000 lamports = 0.000005 SOL)
	// This is much simpler than EVM chains
	const (
		feePerSignatureLamports = uint64(5000) // Standard Solana fee
	)

	// Convert lamports to SOL
	feeInSOL, err := baseCurrency.MakeAmountFromBigInt(big.NewInt(int64(feePerSignatureLamports)))
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make SOL from lamports")
	}

	conv, err := s.CryptoToFiat(ctx, feeInSOL, money.USD)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total cost in USD")
	}

	return NewFee(currency, time.Now().UTC(), isTest, SolanaFee{
		FeePerSignature: feePerSignatureLamports,
		TotalCostSOL:    feeInSOL.String(),
		TotalCostUSD:    conv.To.String(),
		totalCostUSD:    conv.To,
	}), nil
}

// Monero Fee structures and methods
type MoneroFee struct {
	FeePerKB     uint64 `json:"feePerKb"`
	TotalCostXMR string `json:"totalCostXmr"`
	TotalCostUSD string `json:"totalCostUsd"`

	totalCostUSD money.Money
}

func (f *Fee) ToMoneroFee() (MoneroFee, error) {
	if fee, ok := f.raw.(MoneroFee); ok {
		return fee, nil
	}
	return MoneroFee{}, errors.New("invalid fee type assertion for XMR")
}

func (s *Service) moneroFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	// Monero has dynamic fees based on network congestion
	// For simplicity, we'll use a conservative estimate
	// Typical Monero transaction is ~2KB, fees are ~0.00001-0.0001 XMR
	const (
		estimatedTxSizeKB = 2
		feePerKBPiconeros = uint64(20000000) // ~0.00002 XMR per KB (conservative estimate)
	)

	totalFeePiconeros := feePerKBPiconeros * estimatedTxSizeKB

	// Convert piconeros to XMR (1 XMR = 1e12 piconeros)
	feeInXMR, err := baseCurrency.MakeAmountFromBigInt(big.NewInt(int64(totalFeePiconeros)))
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make XMR from piconeros")
	}

	conv, err := s.CryptoToFiat(ctx, feeInXMR, money.USD)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total cost in USD")
	}

	return NewFee(currency, time.Now().UTC(), isTest, MoneroFee{
		FeePerKB:     feePerKBPiconeros,
		TotalCostXMR: feeInXMR.String(),
		TotalCostUSD: conv.To.String(),
		totalCostUSD: conv.To,
	}), nil
}
