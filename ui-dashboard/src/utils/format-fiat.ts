/**
 * Single source of truth for fiat currency display data.
 * This map mirrors the backend's money.fiatCurrencyData in internal/money/money.go.
 * When adding currencies, update BOTH this file and money.go.
 */
export interface FiatCurrencyInfo {
    code: string;
    symbol: string;
    name: string;
}

export const FIAT_CURRENCIES: Record<string, FiatCurrencyInfo> = {
    USD: {code: "USD", symbol: "$", name: "US Dollar"},
    EUR: {code: "EUR", symbol: "€", name: "Euro"},
    GBP: {code: "GBP", symbol: "£", name: "British Pound"},
    CAD: {code: "CAD", symbol: "C$", name: "Canadian Dollar"},
    AUD: {code: "AUD", symbol: "A$", name: "Australian Dollar"},
    CHF: {code: "CHF", symbol: "Fr", name: "Swiss Franc"},
    JPY: {code: "JPY", symbol: "¥", name: "Japanese Yen"},
    CNY: {code: "CNY", symbol: "¥", name: "Chinese Yuan"},
    INR: {code: "INR", symbol: "₹", name: "Indian Rupee"},
    BRL: {code: "BRL", symbol: "R$", name: "Brazilian Real"},
    MXN: {code: "MXN", symbol: "MX$", name: "Mexican Peso"},
    KRW: {code: "KRW", symbol: "₩", name: "Korean Won"},
    SGD: {code: "SGD", symbol: "S$", name: "Singapore Dollar"},
    HKD: {code: "HKD", symbol: "HK$", name: "Hong Kong Dollar"},
    SEK: {code: "SEK", symbol: "kr", name: "Swedish Krona"},
    NOK: {code: "NOK", symbol: "kr", name: "Norwegian Krone"},
    DKK: {code: "DKK", symbol: "kr", name: "Danish Krone"},
    PLN: {code: "PLN", symbol: "zł", name: "Polish Zloty"},
    CZK: {code: "CZK", symbol: "Kč", name: "Czech Koruna"},
    TRY: {code: "TRY", symbol: "₺", name: "Turkish Lira"},
    ZAR: {code: "ZAR", symbol: "R", name: "South African Rand"},
    NZD: {code: "NZD", symbol: "NZ$", name: "New Zealand Dollar"},
    THB: {code: "THB", symbol: "฿", name: "Thai Baht"},
    AED: {code: "AED", symbol: "د.إ", name: "UAE Dirham"},
    SAR: {code: "SAR", symbol: "﷼", name: "Saudi Riyal"},
    RUB: {code: "RUB", symbol: "₽", name: "Russian Ruble"}
};

/** All fiat currency codes supported by the system. */
export const FIAT_CURRENCY_CODES = Object.keys(FIAT_CURRENCIES);

/** Sorted list of currency options for dropdowns. */
export const FIAT_CURRENCY_OPTIONS = Object.values(FIAT_CURRENCIES)
    .sort((a, b) => a.code.localeCompare(b.code))
    .map((c) => ({label: `${c.code} — ${c.name}`, value: c.code}));

/**
 * Format a numeric amount with the correct fiat currency symbol.
 * e.g. formatFiat(100, "EUR") => "€100.00"
 *      formatFiat(1500, "GBP") => "£1,500.00"
 *
 * @param amount - numeric value (number or string)
 * @param currencyCode - fiat currency code (defaults to "USD")
 * @param opts.useLocale - if true, use toLocaleString for thousands separators (default: false)
 */
export function formatFiat(
    amount: number | string | undefined | null,
    currencyCode?: string,
    opts?: {useLocale?: boolean}
): string {
    const code = currencyCode || "USD";
    const info = FIAT_CURRENCIES[code];
    const symbol = info?.symbol || code;

    if (amount === undefined || amount === null || amount === "") {
        return `${symbol}0.00`;
    }

    const num = typeof amount === "string" ? parseFloat(amount) : amount;
    if (isNaN(num)) {
        return `${symbol}0.00`;
    }

    const formatted = opts?.useLocale ? num.toLocaleString(undefined, {minimumFractionDigits: 2, maximumFractionDigits: 2}) : num.toFixed(2);
    return `${symbol}${formatted}`;
}

/**
 * Get the currency symbol for a given code. Falls back to the code itself.
 */
export function fiatSymbol(currencyCode?: string): string {
    if (!currencyCode) return "$";
    return FIAT_CURRENCIES[currencyCode]?.symbol || currencyCode;
}
