import type {Payment} from "src/types";
import {parseEip681, type EvmPaymentRequest} from "src/wallet/eip681";
import {decimalToRaw, roundUpTruncated} from "src/wallet/amounts";
import {CURRENCY_DECIMALS, EVM_CHAINS, TRON_EXPLORER, TRON_USDT_CONTRACT} from "src/wallet/chains";

export type PayPlan =
    | {family: "evm"; request: EvmPaymentRequest; explorerTx: (hash: string) => string}
    | {family: "tron"; recipient: string; amount: bigint; token?: string; explorerTx: (hash: string) => string};

const MIN_TIME_LEFT_MS = 2 * 60 * 1000;

// Everything the wallet will sign comes from the server's payment data; any
// inconsistency means "no button, use the QR" rather than a guess.
export const planWalletPayment = (payment: Payment, now: number): {plan: PayPlan} | {reason: string} => {
    try {
        return plan(payment, now);
    } catch {
        return {reason: "Invalid payment data"};
    }
};

const plan = (payment: Payment, now: number): {plan: PayPlan} | {reason: string} => {
    const info = payment.paymentInfo;
    const method = payment.paymentMethod;
    if (!payment.isLocked || !info || !method) return {reason: "Payment not ready"};

    const partial = info.status === "partial";
    if (info.status !== "pending" && !partial) return {reason: "Not awaiting payment"};
    if (Date.parse(info.expiresAt) - now < MIN_TIME_LEFT_MS) return {reason: "Payment is about to expire"};

    if (method.blockchain === "TRON") {
        const amount = partial ? decimalToRaw(info.remainingAmount ?? "", 6) : BigInt(info.amount);
        if (amount <= 0n) return {reason: "Nothing to pay"};
        if (method.ticker === "TRON") return {plan: {family: "tron", recipient: info.recipientAddress, amount, explorerTx: (h) => TRON_EXPLORER + h}};
        if (method.ticker === "TRON_USDT") {
            return {plan: {family: "tron", recipient: info.recipientAddress, amount, token: TRON_USDT_CONTRACT, explorerTx: (h) => TRON_EXPLORER + h}};
        }
        return {reason: "Unsupported TRON token"};
    }

    const uri = partial ? info.remainingPaymentLink : info.paymentLink;
    const request = uri ? parseEip681(uri) : null;
    if (!request) return {reason: "No wallet payment for this network"};

    const chain = EVM_CHAINS[request.chainId];
    const decimals = CURRENCY_DECIMALS[method.ticker];
    if (!chain || decimals === undefined) return {reason: "Unsupported network"};
    if (request.recipient.toLowerCase() !== info.recipientAddress.toLowerCase()) return {reason: "Address mismatch"};

    if (partial) {
        request.amount = roundUpTruncated(request.amount, decimals);
    } else if (request.amount !== BigInt(info.amount)) {
        return {reason: "Amount mismatch"};
    }
    if (request.amount <= 0n) return {reason: "Nothing to pay"};

    return {plan: {family: "evm", request, explorerTx: (h) => `${chain.explorer}/tx/${h}`}};
};
