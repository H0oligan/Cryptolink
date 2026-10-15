import {describe, it, expect, vi} from "vitest";
import {deployEvmFlow} from "src/wallet/deploy";

const OWNER = "0x1111111111111111111111111111111111111111";
const BSC: any = {value: "BSC", label: "BNB Smart Chain", chainId: 56, rpcUrl: "https://bsc-dataseed.binance.org", nativeTicker: "BNB"};

const fakeProvider = () => {
    const calls: {method: string; params?: any}[] = [];
    const request = vi.fn(async (args: any) => {
        calls.push(args);
        return null;
    });
    return {calls, provider: {request}};
};

describe("deployEvmFlow", () => {
    it("sends nothing when the permanent-owner confirmation is declined", async () => {
        const {calls, provider} = fakeProvider();
        const confirm = vi.fn(async () => false);
        const res = await deployEvmFlow({wallet: {provider, name: "MetaMask"} as any, address: OWNER}, BSC, confirm, () => undefined);
        expect(res).toBeNull();
        expect(confirm).toHaveBeenCalledWith(OWNER, "BNB Smart Chain", "MetaMask");
        expect(calls.map((c) => c.method)).not.toContain("eth_sendTransaction");
        expect(calls.map((c) => c.method)).not.toContain("wallet_switchEthereumChain");
    });

    it("deploys from the confirmed (connected) address after switching network", async () => {
        const {calls, provider} = fakeProvider();
        provider.request.mockImplementation(async (args: any) => {
            calls.push(args);
            if (args.method === "eth_sendTransaction") throw new Error("stop-after-send");
            return null;
        });
        await expect(
            deployEvmFlow({wallet: {provider, name: "MetaMask"} as any, address: OWNER}, BSC, async () => true, () => undefined)
        ).rejects.toThrow("stop-after-send");
        const methods = calls.map((c) => c.method);
        expect(methods.indexOf("wallet_switchEthereumChain")).toBeLessThan(methods.indexOf("eth_sendTransaction"));
        expect(calls.find((c) => c.method === "eth_sendTransaction")!.params[0].from).toBe(OWNER);
    });
});
