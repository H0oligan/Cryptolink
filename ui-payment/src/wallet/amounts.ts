// Exact decimal -> raw conversion (no floats: money).
export const decimalToRaw = (value: string, decimals: number): bigint => {
    const m = /^([0-9]+)(?:\.([0-9]+))?$/.exec(value.trim());
    if (!m) throw new Error(`Invalid amount "${value}"`);
    const frac = m[2] ?? "";
    if (frac.length > decimals) throw new Error(`Amount "${value}" has more than ${decimals} decimals`);
    return BigInt(m[1] + frac.padEnd(decimals, "0"));
};

// The backend truncates displayed amounts to 8 decimals (MaxDisplayDecimals).
// A wallet top-up built from a truncated "remaining" could fall short by dust,
// so add one unit of the last displayed decimal. A tiny overpayment is handled.
const DISPLAY_DECIMALS = 8;

export const roundUpTruncated = (raw: bigint, decimals: number): bigint =>
    decimals > DISPLAY_DECIMALS ? raw + 10n ** BigInt(decimals - DISPLAY_DECIMALS) : raw;
