/**
 * Fiat currency symbol map — single source of truth for the payment UI.
 * Mirrors the backend's money.fiatCurrencyData (internal/money/money.go).
 */
const FIAT_SYMBOLS: Record<string, string> = {
    USD: "$", EUR: "€", GBP: "£", CAD: "C$", AUD: "A$", CHF: "Fr",
    JPY: "¥", CNY: "¥", INR: "₹", BRL: "R$", MXN: "MX$", KRW: "₩",
    SGD: "S$", HKD: "HK$", SEK: "kr", NOK: "kr", DKK: "kr", PLN: "zł",
    CZK: "Kč", TRY: "₺", ZAR: "R", NZD: "NZ$", THB: "฿",
    AED: "د.إ", SAR: "﷼", RUB: "₽"
};

const renderCurrency = (currency?: string, price?: number) => {
    if (currency === undefined || price === undefined) {
        return;
    }

    const symbol = FIAT_SYMBOLS[currency];
    if (symbol) {
        return `${symbol}${price.toFixed(2)}`;
    }

    return `${price.toFixed(2)} ${currency}`;
};

export default renderCurrency;
