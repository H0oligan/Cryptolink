interface CurrencyConvertResult {
    cryptoAmount: string;
    cryptoCurrency: string;
    displayName: string;
    exchangeRate: number;
    fiatAmount: number;
    fiatCurrency: string;
    network: string;
}

interface PaymentMethod {
    blockchain: string;
    blockchainName: string;
    displayName: string;
    name: string;
    ticker: string;
}

interface Customer {
    email: string;
    id: string;
}

const CURRENCY = [
    "USD", "EUR", "GBP", "CAD", "AUD", "CHF", "JPY", "CNY", "INR", "BRL",
    "MXN", "KRW", "SGD", "HKD", "SEK", "NOK", "DKK", "PLN", "CZK", "TRY",
    "ZAR", "NZD", "THB", "AED", "SAR", "RUB"
] as const;
type Currency = typeof CURRENCY[number];

type PaymentStatus = "pending" | "inProgress" | "success" | "failed" | "underpaid";
type PaymentAction = "redirect" | "showMessage";

interface PaymentInfo {
    amount: string;
    amountFormatted: string;
    recipientAddress: string;
    status: PaymentStatus;
    successUrl?: string;
    expiresAt: string;
    expirationDurationMin: number;
    successAction?: PaymentAction;
    successMessage?: string;
    paymentLink: string;
}

interface Payment {
    currency: Currency;
    customer?: Customer;
    description?: string;
    id: string;
    isLocked: boolean;
    merchantName: string;
    paymentInfo?: PaymentInfo;
    paymentMethod?: PaymentMethod;
    price: number;
    feePercent?: number;
}

interface PaymentLink {
    currency: Currency;
    description?: string;
    merchantName: string;
    price: number;
}

export type {CurrencyConvertResult, PaymentMethod, Payment, Customer, PaymentLink};
