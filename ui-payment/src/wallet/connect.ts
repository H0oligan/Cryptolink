import type {DetectedWallet} from "src/wallet/types";

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

export const connectEvm = async (w: DetectedWallet): Promise<string> => {
    const accounts: string[] = await w.provider.request({method: "eth_requestAccounts"});
    if (!accounts?.length) throw new Error(`${w.name} returned no account. Unlock it and try again.`);
    return accounts[0];
};

// TronLink keeps tronWeb on window.tronWeb; OKX/Bitget/Trust hang it off their own object.
const tronWebOf = (w: DetectedWallet): any =>
    w.provider?.tronWeb || (w.id === "legacy:tronlink" ? (window as any).tronWeb : null);

export const connectTron = async (w: DetectedWallet): Promise<{address: string; tronWeb: any}> => {
    if (w.provider?.request) {
        const res = await w.provider.request({method: "tron_requestAccounts"});
        if (res?.code === 4001) throw {code: 4001, message: "You rejected the connection request."};
    }
    for (let i = 0; i < 15; i++) {
        const tw = tronWebOf(w);
        if (tw?.ready && tw?.defaultAddress?.base58) return {address: tw.defaultAddress.base58, tronWeb: tw};
        await sleep(500);
    }
    throw new Error(`${w.name} is not ready. Unlock it, select a TRON account and try again.`);
};

const REJECT_RE = /reject|denied|declined|cancel/i;

export const describeError = (err: unknown): {text: string; rejected: boolean} => {
    const e: any = err;
    const msg: string = typeof e === "string" ? e : e?.message || "";
    if (e?.code === 4001 || REJECT_RE.test(msg)) return {text: "You cancelled the request in your wallet.", rejected: true};
    if (/insufficient funds|balance is not sufficient|exceeds balance/i.test(msg)) {
        return {text: "Not enough funds in this wallet for the amount plus the network fee.", rejected: false};
    }
    return {text: msg || "Something went wrong with the wallet. Please try again.", rejected: false};
};

// Remember a broadcast per payment + status so a reload doesn't invite a second send.
const sentKey = (paymentId: string) => `cl-pay-sent:${paymentId}`;

export const rememberSent = (paymentId: string, status: string, hash: string, url: string) => {
    try {
        sessionStorage.setItem(sentKey(paymentId), JSON.stringify({status, hash, url}));
    } catch {
        // storage unavailable: the page's own inProgress status still guards
    }
};

export const recallSent = (paymentId: string, status: string): {hash: string; url: string} | null => {
    try {
        const v = JSON.parse(sessionStorage.getItem(sentKey(paymentId)) || "null");
        return v && v.status === status ? v : null;
    } catch {
        return null;
    }
};
