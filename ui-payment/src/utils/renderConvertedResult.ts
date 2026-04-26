const MAX_DISPLAY_DECIMALS = 8;

const renderConvertedResult = (amountFormatted: string | undefined, ticker: string | undefined) => {
    if (amountFormatted && ticker) {
        const sliceRes = amountFormatted.split(".");
        const amount = Number(amountFormatted);

        if (isNaN(amount)) {
            return null;
        }

        // Backend truncates to max 8 decimals, but guard against edge cases
        if (!sliceRes[1] || sliceRes[1].length <= MAX_DISPLAY_DECIMALS) {
            return amountFormatted + " " + ticker;
        }

        // Trim to 8 decimals and strip trailing zeros
        return amount.toFixed(MAX_DISPLAY_DECIMALS).replace(/0+$/, "").replace(/\.$/, "") + " " + ticker;
    }

    return null;
};

export default renderConvertedResult;
