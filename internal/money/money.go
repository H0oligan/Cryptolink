package money

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"

	"github.com/cryptolink/cryptolink/internal/util"
	"github.com/pkg/errors"
)

// FIAT ------------------
type FiatCurrency string

const (
	USD FiatCurrency = "USD"
	EUR FiatCurrency = "EUR"
	GBP FiatCurrency = "GBP"
	CAD FiatCurrency = "CAD"
	AUD FiatCurrency = "AUD"
	CHF FiatCurrency = "CHF"
	JPY FiatCurrency = "JPY"
	CNY FiatCurrency = "CNY"
	INR FiatCurrency = "INR"
	BRL FiatCurrency = "BRL"
	MXN FiatCurrency = "MXN"
	KRW FiatCurrency = "KRW"
	SGD FiatCurrency = "SGD"
	HKD FiatCurrency = "HKD"
	SEK FiatCurrency = "SEK"
	NOK FiatCurrency = "NOK"
	DKK FiatCurrency = "DKK"
	PLN FiatCurrency = "PLN"
	CZK FiatCurrency = "CZK"
	TRY FiatCurrency = "TRY"
	ZAR FiatCurrency = "ZAR"
	NZD FiatCurrency = "NZD"
	THB FiatCurrency = "THB"
	AED FiatCurrency = "AED"
	SAR FiatCurrency = "SAR"
	RUB FiatCurrency = "RUB"
)

// FiatCurrencyInfo holds display metadata for a fiat currency.
type FiatCurrencyInfo struct {
	Code   FiatCurrency `json:"code"`
	Symbol string       `json:"symbol"`
	Name   string       `json:"name"`
}

// fiatCurrencyData is the single source of truth for all supported fiat currencies.
var fiatCurrencyData = map[FiatCurrency]FiatCurrencyInfo{
	USD: {Code: USD, Symbol: "$", Name: "US Dollar"},
	EUR: {Code: EUR, Symbol: "€", Name: "Euro"},
	GBP: {Code: GBP, Symbol: "£", Name: "British Pound"},
	CAD: {Code: CAD, Symbol: "C$", Name: "Canadian Dollar"},
	AUD: {Code: AUD, Symbol: "A$", Name: "Australian Dollar"},
	CHF: {Code: CHF, Symbol: "Fr", Name: "Swiss Franc"},
	JPY: {Code: JPY, Symbol: "¥", Name: "Japanese Yen"},
	CNY: {Code: CNY, Symbol: "¥", Name: "Chinese Yuan"},
	INR: {Code: INR, Symbol: "₹", Name: "Indian Rupee"},
	BRL: {Code: BRL, Symbol: "R$", Name: "Brazilian Real"},
	MXN: {Code: MXN, Symbol: "MX$", Name: "Mexican Peso"},
	KRW: {Code: KRW, Symbol: "₩", Name: "Korean Won"},
	SGD: {Code: SGD, Symbol: "S$", Name: "Singapore Dollar"},
	HKD: {Code: HKD, Symbol: "HK$", Name: "Hong Kong Dollar"},
	SEK: {Code: SEK, Symbol: "kr", Name: "Swedish Krona"},
	NOK: {Code: NOK, Symbol: "kr", Name: "Norwegian Krone"},
	DKK: {Code: DKK, Symbol: "kr", Name: "Danish Krone"},
	PLN: {Code: PLN, Symbol: "zł", Name: "Polish Zloty"},
	CZK: {Code: CZK, Symbol: "Kč", Name: "Czech Koruna"},
	TRY: {Code: TRY, Symbol: "₺", Name: "Turkish Lira"},
	ZAR: {Code: ZAR, Symbol: "R", Name: "South African Rand"},
	NZD: {Code: NZD, Symbol: "NZ$", Name: "New Zealand Dollar"},
	THB: {Code: THB, Symbol: "฿", Name: "Thai Baht"},
	AED: {Code: AED, Symbol: "د.إ", Name: "UAE Dirham"},
	SAR: {Code: SAR, Symbol: "﷼", Name: "Saudi Riyal"},
	RUB: {Code: RUB, Symbol: "₽", Name: "Russian Ruble"},
}

// fiatCurrencies is the validation set derived from fiatCurrencyData.
var fiatCurrencies map[FiatCurrency]struct{}

func init() {
	fiatCurrencies = make(map[FiatCurrency]struct{}, len(fiatCurrencyData))
	for code := range fiatCurrencyData {
		fiatCurrencies[code] = struct{}{}
	}
}

const (
	FiatDecimals    = int64(2)
	FiatDecimalsJPY = int64(0)
	FiatMin         = float64(0.01)
	FiatMax         = float64(10_000_000)
)

var (
	ErrInvalidFiatCurrency = errors.New("unsupported fiat currency")
	ErrIncompatibleMoney   = errors.New("incompatible money type")
	ErrNegative            = errors.New("money can't be negative")
	ErrParse               = errors.New("unable to parse value")
)

// SupportedFiatCurrencies returns all supported fiat currencies sorted by code.
func SupportedFiatCurrencies() []FiatCurrencyInfo {
	result := make([]FiatCurrencyInfo, 0, len(fiatCurrencyData))
	for _, info := range fiatCurrencyData {
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		return string(result[i].Code) < string(result[j].Code)
	})
	return result
}

// FiatInfo returns the display info for a fiat currency. Returns empty info if not found.
func FiatInfo(code FiatCurrency) (FiatCurrencyInfo, bool) {
	info, ok := fiatCurrencyData[code]
	return info, ok
}

// FiatSymbol returns the currency symbol for a fiat currency code (e.g. "$" for USD).
func FiatSymbol(code FiatCurrency) string {
	if info, ok := fiatCurrencyData[code]; ok {
		return info.Symbol
	}
	return string(code)
}

// IsFiatCurrency returns true if the given string is a supported fiat currency.
func IsFiatCurrency(s string) bool {
	_, exists := fiatCurrencies[FiatCurrency(s)]
	return exists
}

func (f FiatCurrency) String() string {
	return string(f)
}

func (f FiatCurrency) MakeAmount(raw string) (Money, error) {
	return New(Fiat, f.String(), raw, FiatDecimals)
}

func MakeFiatCurrency(s string) (FiatCurrency, error) {
	f := FiatCurrency(s)

	var err error
	if _, exists := fiatCurrencies[f]; !exists {
		err = errors.Wrap(ErrInvalidFiatCurrency, s)
	}

	return f, err
}

// CRYPTO  ------------------
type Blockchain string

func (b Blockchain) String() string {
	return string(b)
}

type CryptoCurrencyType string

const (
	Coin  CryptoCurrencyType = "coin"
	Token CryptoCurrencyType = "token"
)

// CryptoCurrency represents CRYPTO coin or token.
type CryptoCurrency struct {
	Blockchain     Blockchain
	BlockchainName string
	NetworkID      string
	TestNetworkID  string

	Type   CryptoCurrencyType
	Ticker string
	Name   string

	Decimals int64

	TokenContractAddress     string
	TestTokenContractAddress string
	Aliases                  []string
	Deprecated               bool
}

func (c CryptoCurrency) DisplayName() string {
	return fmt.Sprintf("%s (%s)", c.Name, c.BlockchainName)
}

func (c CryptoCurrency) ChooseNetwork(isTest bool) string {
	if isTest {
		return c.TestNetworkID
	}

	return c.NetworkID
}

// ChooseContractAddress returns contract address and tries to return specific contract for testnet if present
func (c CryptoCurrency) ChooseContractAddress(isTest bool) string {
	if isTest && c.TestTokenContractAddress != "" {
		return c.TestTokenContractAddress
	}

	return c.TokenContractAddress
}

func (c CryptoCurrency) MakeAmount(raw string) (Money, error) {
	return CryptoFromRaw(c.Ticker, raw, c.Decimals)
}

func (c CryptoCurrency) MakeAmountMust(raw string) Money {
	m, err := c.MakeAmount(raw)
	if err != nil {
		panic(err)
	}
	return m
}

func (c CryptoCurrency) MakeAmountFromBigInt(amount *big.Int) (Money, error) {
	return NewFromBigInt(Crypto, c.Ticker, amount, c.Decimals)
}

// MONEY  ------------------
type Type string

const (
	Fiat   Type = "fiat"
	Crypto Type = "crypto"
)

type Money struct {
	moneyType Type
	ticker    string
	value     *big.Int
	decimals  int64
}

func (m Money) Ticker() string {
	return m.ticker
}

func (m Money) Type() Type {
	return m.moneyType
}

func (m Money) Decimals() int64 {
	return m.decimals
}

func (m Money) String() string {
	stringRaw := m.StringRaw()

	isNegative := m.IsNegative()
	if isNegative {
		stringRaw = stringRaw[1:]
	}

	l, d := len(stringRaw), int(m.decimals)

	var result string

	switch {
	case l > d:
		index := l - d
		result = stringRaw[:index] + "." + stringRaw[index:]
	case l == d:
		result = "0." + stringRaw
	case l < d:
		result = "0." + strings.Repeat("0", d-l) + stringRaw
	}

	if m.moneyType == Fiat {
		result = strings.TrimSuffix(result, ".00")
	} else {
		result = strings.TrimRight(strings.TrimRight(result, "0"), ".")
	}

	if isNegative {
		result = "-" + result
	}

	return result
}

func (m Money) StringRaw() string {
	return m.val().String()
}

// nolint:gocritic
func (m Money) BigInt() (*big.Int, int64) {
	return m.val(), m.decimals
}

func (m Money) FiatToFloat64() (float64, error) {
	if m.Type() != Fiat {
		return 0, errors.New("money should be fiat")
	}

	return toFloat64(m.val(), m.decimals), nil
}

func (m Money) CompatibleTo(b Money) bool {
	return m.ticker == b.ticker && m.decimals == b.decimals
}

// Add adds money of the same type.
func (m Money) Add(amount Money) (Money, error) {
	if !m.CompatibleTo(amount) {
		return Money{}, errors.Wrapf(
			ErrIncompatibleMoney,
			"a: %q (%d dec.), b: %q (%d dec.)",
			m.ticker, m.decimals, amount.ticker, amount.decimals,
		)
	}

	a, _ := m.BigInt()
	b, _ := amount.BigInt()

	return NewFromBigInt(m.moneyType, m.ticker, a.Add(a, b), m.decimals)
}

// Sub subtracts money of the same type. Restricts having negative values.
func (m Money) Sub(amount Money) (Money, error) {
	out, err := m.SubNegative(amount)
	if err != nil {
		return Money{}, err
	}

	if out.IsNegative() {
		return Money{}, ErrNegative
	}

	return out, nil
}

// SubNegative subtracts money allowing negative outcome
func (m Money) SubNegative(amount Money) (Money, error) {
	if !m.CompatibleTo(amount) {
		return Money{}, errors.Wrapf(
			ErrIncompatibleMoney,
			"a: %q (%d dec.), b: %q (%d dec.)",
			m.ticker, m.decimals, amount.ticker, amount.decimals,
		)
	}

	a, _ := m.BigInt()
	b, _ := amount.BigInt()

	m, err := NewFromBigInt(m.moneyType, m.ticker, a.Sub(a, b), m.decimals)
	if err != nil {
		return Money{}, nil
	}

	return m, nil
}

func (m Money) MultiplyFloat64(by float64) (Money, error) {
	if by <= 0 {
		return Money{}, errors.New("multiplier should be greater than zero")
	}

	a := new(big.Float).SetInt(m.val())
	b := big.NewFloat(by)

	result := new(big.Float).Mul(a, b)
	bigInt, _ := result.Int(nil)

	return NewFromBigInt(m.moneyType, m.ticker, bigInt, m.decimals)
}

// MultiplyInt64 has less error than MultiplyFloat64
func (m Money) MultiplyInt64(by int64) (Money, error) {
	if by <= 0 {
		return Money{}, errors.New("multiplier should be greater than zero")
	}

	bigInt := new(big.Int).Mul(m.val(), big.NewInt(by))

	return NewFromBigInt(m.moneyType, m.ticker, bigInt, m.decimals)
}

func (m Money) Equals(b Money) bool {
	return m.CompatibleTo(b) && m.val().Cmp(b.val()) == 0
}

func (m Money) GreaterThan(b Money) bool {
	return m.CompatibleTo(b) && m.val().Cmp(b.val()) == +1
}

func (m Money) GreaterThanOrEqual(b Money) bool {
	return m.Equals(b) || m.GreaterThan(b)
}

func (m Money) LessThan(b Money) bool {
	return m.CompatibleTo(b) && m.val().Cmp(b.val()) == -1
}

func (m Money) LessThanOrEqual(b Money) bool {
	return m.Equals(b) || m.LessThan(b)
}

func (m Money) IsZero() bool {
	zero := new(big.Int)

	return m.val().Cmp(zero) == 0
}

func (m Money) IsNegative() bool {
	zero := new(big.Int)

	return m.val().Cmp(zero) == -1
}

func (m Money) IsPositive() bool {
	zero := new(big.Int)

	return m.val().Cmp(zero) == +1
}

func (m Money) val() *big.Int {
	if m.value == nil {
		return new(big.Int)
	}

	return new(big.Int).Set(m.value)
}

func New(moneyType Type, ticker, value string, decimals int64) (Money, error) {
	bigInt, set := new(big.Int).SetString(clear(value), 10)
	if !set {
		return Money{}, ErrParse
	}

	m := Money{
		moneyType: moneyType,
		ticker:    ticker,
		value:     bigInt,
		decimals:  decimals,
	}

	return m, nil
}

func NewFromBigInt(moneyType Type, ticker string, bigInt *big.Int, decimals int64) (Money, error) {
	if bigInt == nil {
		return Money{}, errors.Wrap(ErrParse, "bigInt is nil")
	}

	m := Money{
		moneyType: moneyType,
		ticker:    ticker,
		value:     new(big.Int).Set(bigInt),
		decimals:  decimals,
	}

	return m, nil
}

func FiatFromFloat64(ticker FiatCurrency, f float64) (Money, error) {
	if f < FiatMin || f > FiatMax {
		return Money{}, errors.Wrapf(ErrParse, "fiat value should be between %.2f and %.0f", FiatMin, FiatMax)
	}

	value := fmt.Sprintf("%.f", math.Floor(f*pow(FiatDecimals)))

	return New(Fiat, string(ticker), value, FiatDecimals)
}

func CryptoFromFloat64(ticker string, f float64, decimals int64) (Money, error) {
	if f <= 0 {
		return Money{}, errors.Wrap(ErrParse, "crypto value should be more than zero")
	}

	bigInt, _ := new(big.Float).
		Mul(big.NewFloat(f), bigPow(decimals)).
		Int(nil)

	return New(Crypto, ticker, bigInt.String(), decimals)
}

// CryptoFromStringFloat constructs crypto from floats string e.g. "0.042"
func CryptoFromStringFloat(ticker, floatString string, decimals int64) (Money, error) {
	parts := strings.Split(floatString, ".")

	if len(parts) > 2 {
		return Money{}, errors.Wrapf(ErrParse, "invalid floatString provided")
	}

	raw := ""

	// no dot: "42", "123" -> "42 000", "123 000"
	if len(parts) == 1 {
		raw = floatString + strings.Repeat("0", int(decimals))
	} else {
		// example: 42.123 (6 digits)
		// "42" + "123" + "0"*3 -> "42 123 000"
		raw = parts[0] + parts[1] + strings.Repeat("0", int(decimals)-len(parts[1]))
	}

	return CryptoFromRaw(ticker, raw, decimals)
}

func CryptoFromRaw(ticker, value string, decimals int64) (Money, error) {
	return New(Crypto, ticker, value, decimals)
}

func MustCryptoFromRaw(ticker, value string, decimals int64) Money {
	m, err := New(Crypto, ticker, value, decimals)
	if err != nil {
		panic(err)
	}

	return m
}

func CryptoToFiat(crypto Money, fiat FiatCurrency, exchangeRate float64) (Money, error) {
	if crypto.Type() != Crypto {
		return Money{}, ErrIncompatibleMoney
	}

	multiplied, err := crypto.MultiplyFloat64(exchangeRate * float64(util.Pow64(10, FiatDecimals)))
	if err != nil {
		return Money{}, errors.Wrap(err, "unable to multiply crypto")
	}

	// 1234.123123123 (price in "cents")
	floatString := multiplied.String()
	if dotIndex := strings.Index(floatString, "."); dotIndex != -1 {
		floatString = floatString[:dotIndex]
	}

	return New(Fiat, fiat.String(), floatString, FiatDecimals)
}

func pow(i int64) float64 {
	return math.Pow10(int(i))
}

func bigPow(i int64) *big.Float {
	return big.NewFloat(pow(i))
}

func toFloat64(i *big.Int, decimals int64) float64 {
	bigF := new(big.Float).SetInt(i)
	bigF = big.NewFloat(0).Quo(bigF, bigPow(decimals))

	f, _ := bigF.Float64()

	return f
}

func clear(s string) string {
	return strings.ReplaceAll(s, "_", "")
}
