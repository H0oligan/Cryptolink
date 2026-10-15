import {NotOwnerError} from "src/wallet/owner";

const REJECT_RE = /reject|denied|declined|cancel/i;

export const describeWalletError = (err: unknown): {title: string; description: string; rejected: boolean} => {
    if (err instanceof NotOwnerError) return {title: "Wrong wallet", description: err.message, rejected: false};

    const e: any = err;
    const msg: string = typeof e === "string" ? e : e?.shortMessage || e?.message || "";

    if (e?.code === 4001 || REJECT_RE.test(msg)) {
        return {title: "Request rejected", description: "You rejected the request in your wallet.", rejected: true};
    }
    if (/insufficient funds/i.test(msg)) {
        return {title: "Not enough gas", description: "The connected wallet doesn't have enough native coin (ETH/BNB/TRX…) to pay the network fee.", rejected: false};
    }
    if (e?.code === 4902) {
        return {title: "Network not added", description: "Your wallet doesn't know this network yet. Approve adding it, then try again.", rejected: false};
    }
    return {title: "Wallet error", description: msg || "Something went wrong with the wallet. Please try again.", rejected: false};
};
