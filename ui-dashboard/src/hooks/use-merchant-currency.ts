import useSharedMerchant from "src/hooks/use-merchant";
import {formatFiat, fiatSymbol, FIAT_CURRENCIES} from "src/utils/format-fiat";

/**
 * Returns the merchant's chosen fiat currency code, symbol, and a bound format function.
 * All dashboard components should use this instead of hardcoding "$" or "USD".
 */
const useMerchantCurrency = () => {
    const {merchant} = useSharedMerchant();

    const currencyCode = merchant?.fiatCurrency || "USD";
    const currencySymbol = merchant?.fiatCurrencySymbol || fiatSymbol(currencyCode);
    const currencyName = FIAT_CURRENCIES[currencyCode]?.name || currencyCode;

    /** Format an amount in the merchant's chosen currency. */
    const fmt = (amount: number | string | undefined | null, opts?: {useLocale?: boolean}) =>
        formatFiat(amount, currencyCode, opts);

    return {
        currencyCode,
        currencySymbol,
        currencyName,
        formatFiat: fmt
    };
};

export default useMerchantCurrency;
