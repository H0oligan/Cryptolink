// EIP-681 payment URIs as built by the backend (blockchain/currencies.go):
//   native: ethereum:<recipient>@<chainId>?value=<raw>
//   token:  ethereum:<token>@<chainId>/transfer?address=<recipient>&uint256=<raw>
export interface EvmPaymentRequest {
    chainId: number;
    recipient: string;
    token?: string;
    amount: bigint;
}

const ADDR = /^0x[0-9a-fA-F]{40}$/;
const UINT = /^[0-9]+$/;

export const parseEip681 = (uri: string): EvmPaymentRequest | null => {
    const m = /^ethereum:(0x[0-9a-fA-F]{40})@([0-9]+)(\/transfer)?\?(.+)$/.exec(uri ?? "");
    if (!m) return null;

    const [, target, chain, transfer, query] = m;
    const params = new URLSearchParams(query);
    const chainId = Number(chain);

    if (transfer) {
        const recipient = params.get("address") ?? "";
        const amount = params.get("uint256") ?? "";
        if (!ADDR.test(recipient) || !UINT.test(amount)) return null;
        return {chainId, recipient, token: target, amount: BigInt(amount)};
    }

    const value = params.get("value") ?? "";
    if (!UINT.test(value)) return null;
    return {chainId, recipient: target, amount: BigInt(value)};
};

const pad32 = (hex: string) => hex.padStart(64, "0");

// transfer(address,uint256) — selector 0xa9059cbb
export const encodeErc20Transfer = (to: string, amount: bigint): string =>
    "0xa9059cbb" + pad32(to.toLowerCase().replace(/^0x/, "")) + pad32(amount.toString(16));
