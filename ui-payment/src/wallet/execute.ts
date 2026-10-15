import {encodeErc20Transfer, type EvmPaymentRequest} from "src/wallet/eip681";
import {EVM_CHAINS} from "src/wallet/chains";
import type {PayPlan} from "src/wallet/plan";

export interface Eip1193Provider {
    request(args: {method: string; params?: unknown}): Promise<any>;
}

export class WrongNetworkError extends Error {
    constructor(expected: number) {
        super(`Your wallet is not on ${EVM_CHAINS[expected]?.name ?? `chain ${expected}`}. Switch network in your wallet and try again.`);
        this.name = "WrongNetworkError";
    }
}

const toHex = (n: number | bigint) => "0x" + n.toString(16);

const currentChain = async (p: Eip1193Provider) => parseInt(await p.request({method: "eth_chainId"}), 16);

const switchTo = async (p: Eip1193Provider, chainId: number) => {
    try {
        await p.request({method: "wallet_switchEthereumChain", params: [{chainId: toHex(chainId)}]});
    } catch (err: any) {
        const chain = EVM_CHAINS[chainId];
        if (err?.code !== 4902 || !chain) throw err;
        await p.request({
            method: "wallet_addEthereumChain",
            params: [{chainId: toHex(chainId), chainName: chain.name, rpcUrls: [chain.rpcUrl],
                nativeCurrency: {name: chain.nativeSymbol, symbol: chain.nativeSymbol, decimals: 18}}]
        });
    }
};

// Wrong-network deposits are invisible to the watcher and nearly unrecoverable,
// so the chain is verified after switching and pinned in the transaction.
export const payEvm = async (p: Eip1193Provider, from: string, req: EvmPaymentRequest): Promise<string> => {
    await switchTo(p, req.chainId);
    if ((await currentChain(p)) !== req.chainId) throw new WrongNetworkError(req.chainId);

    const tx = req.token
        ? {from, to: req.token, data: encodeErc20Transfer(req.recipient, req.amount), value: "0x0", chainId: toHex(req.chainId)}
        : {from, to: req.recipient, value: toHex(req.amount), chainId: toHex(req.chainId)};

    return p.request({method: "eth_sendTransaction", params: [tx]});
};

const TESTNET_HOST = /nile|shasta|testnet/i;

export const payTron = async (tronWeb: any, plan: Extract<PayPlan, {family: "tron"}>): Promise<string> => {
    const host: string = tronWeb?.fullNode?.host ?? "";
    if (TESTNET_HOST.test(host)) throw new Error("Your TRON wallet is on a testnet. Switch it to TRON Mainnet and try again.");

    if (!plan.token) {
        const r = await tronWeb.trx.sendTransaction(plan.recipient, Number(plan.amount));
        if (r?.result === false) throw new Error(r?.message || "The TRX transfer was rejected.");
        return r?.txid ?? r?.transaction?.txID;
    }

    const token = await tronWeb.contract().at(plan.token);
    return token.transfer(plan.recipient, plan.amount.toString()).send({feeLimit: 100_000_000});
};
