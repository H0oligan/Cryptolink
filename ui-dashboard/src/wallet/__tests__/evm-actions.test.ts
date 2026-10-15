import {describe, it, expect, vi} from "vitest";
import {encodeAbiParameters} from "viem";
import {readEvmOwner, withdrawAllEvm, switchEvmChain} from "src/wallet/evm-actions";

const OWNER = "0x1111111111111111111111111111111111111111";

describe("evm-actions", () => {
    it("readEvmOwner decodes owner() via eth_call on the given provider", async () => {
        const request = vi.fn(async ({method}: any) =>
            method === "eth_call" ? encodeAbiParameters([{type: "address"}], [OWNER]) : null);
        expect((await readEvmOwner({request}, "0x2222222222222222222222222222222222222222")).toLowerCase()).toBe(OWNER);
        expect(request.mock.calls[0][0].method).toBe("eth_call");
    });

    it("withdrawAllEvm sends from the explicit account, never eth_accounts[0]", async () => {
        const request = vi.fn(async (_args: any) => "0xhash");
        await withdrawAllEvm({request}, OWNER, "0x2222222222222222222222222222222222222222", []);
        const call: any = request.mock.calls[0][0];
        expect(call.method).toBe("eth_sendTransaction");
        expect(call.params[0].from).toBe(OWNER);
    });

    it("switchEvmChain adds the chain on 4902 and retries nothing else", async () => {
        const request = vi.fn()
            .mockRejectedValueOnce({code: 4902})
            .mockResolvedValueOnce(null);
        await switchEvmChain({request}, {value: "BSC", label: "BNB Smart Chain", chainId: 56, color: "", rpcUrl: "https://bsc-dataseed.binance.org", nativeTicker: "BNB", explorerUrl: ""} as any);
        expect(request.mock.calls[1][0].method).toBe("wallet_addEthereumChain");
    });
});
