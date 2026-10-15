import {describe, it, expect, vi} from "vitest";
import {encodeAbiParameters} from "viem";
import {withdrawEvmFlow} from "src/wallet/withdraw";
import {NotOwnerError} from "src/wallet/owner";

const OWNER = "0x1111111111111111111111111111111111111111";
const OTHER = "0x2222222222222222222222222222222222222222";
const CONTRACT = "0x3333333333333333333333333333333333333333";
const BSC: any = {value: "BSC", label: "BNB Smart Chain", chainId: 56, rpcUrl: "https://bsc-dataseed.binance.org", nativeTicker: "BNB"};

const fakeProvider = () => {
    const calls: string[] = [];
    const request = vi.fn(async ({method}: any) => {
        calls.push(method);
        if (method === "eth_call") return encodeAbiParameters([{type: "address"}], [OWNER]);
        if (method === "eth_sendTransaction") return "0xhash";
        return null;
    });
    return {calls, provider: {request}};
};

describe("withdrawEvmFlow", () => {
    it("switches network before reading owner, then sends from the connected account", async () => {
        const {calls, provider} = fakeProvider();
        const hash = await withdrawEvmFlow({wallet: {provider} as any, address: OWNER}, BSC, CONTRACT, []);
        expect(hash).toBe("0xhash");
        expect(calls).toEqual(["wallet_switchEthereumChain", "eth_call", "eth_sendTransaction"]);
    });

    it("never sends a transaction when the connected wallet is not the owner", async () => {
        const {calls, provider} = fakeProvider();
        await expect(withdrawEvmFlow({wallet: {provider} as any, address: OTHER}, BSC, CONTRACT, [])).rejects.toBeInstanceOf(NotOwnerError);
        expect(calls).not.toContain("eth_sendTransaction");
    });
});
