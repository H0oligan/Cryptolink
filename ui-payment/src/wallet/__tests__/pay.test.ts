import {describe, it, expect, vi} from "vitest";
import {parseEip681, encodeErc20Transfer} from "src/wallet/eip681";
import {decimalToRaw, roundUpTruncated} from "src/wallet/amounts";
import {planWalletPayment} from "src/wallet/plan";
import {payEvm, payTron, WrongNetworkError} from "src/wallet/execute";

const COLLECTOR = "0x1111111111111111111111111111111111111111";
const USDT_BSC = "0x55d398326f99059fF775485246999027B3197955";
const NOW = Date.parse("2026-10-16T12:00:00Z");
const LATER = "2026-10-16T12:30:00Z";

const payment = (over: any = {}, info: any = {}): any => ({
    id: "p1",
    isLocked: true,
    currency: "EUR",
    merchantName: "m",
    price: 10,
    paymentMethod: {blockchain: "BSC", blockchainName: "BNB Chain", displayName: "USDT", name: "BSC_USDT", ticker: "BSC_USDT", ...over.method},
    paymentInfo: {
        status: "pending",
        expiresAt: LATER,
        recipientAddress: COLLECTOR,
        amount: "25000000000000000000",
        amountFormatted: "25",
        paymentLink: `ethereum:${USDT_BSC}@56/transfer?address=${COLLECTOR}&uint256=25000000000000000000`,
        expirationDurationMin: 30,
        ...info
    },
    ...over.payment
});

describe("parseEip681", () => {
    it("parses a native payment", () => {
        expect(parseEip681(`ethereum:${COLLECTOR}@1?value=1000`)).toEqual({chainId: 1, recipient: COLLECTOR, amount: 1000n});
    });
    it("parses an ERC-20 transfer", () => {
        expect(parseEip681(`ethereum:${USDT_BSC}@56/transfer?address=${COLLECTOR}&uint256=25`)).toEqual({
            chainId: 56,
            recipient: COLLECTOR,
            token: USDT_BSC,
            amount: 25n
        });
    });
    it("rejects anything malformed", () => {
        for (const bad of ["", "bitcoin:abc?amount=1", "ethereum:0x12@1?value=1", `ethereum:${COLLECTOR}?value=1`, `ethereum:${COLLECTOR}@1?value=-5`, `ethereum:${COLLECTOR}@1/approve?address=${COLLECTOR}&uint256=1`]) {
            expect(parseEip681(bad)).toBeNull();
        }
    });
});

describe("encodeErc20Transfer", () => {
    it("matches the ABI encoding of transfer(address,uint256)", () => {
        expect(encodeErc20Transfer(COLLECTOR, 1000000n)).toBe(
            "0xa9059cbb" +
                "0000000000000000000000001111111111111111111111111111111111111111" +
                "00000000000000000000000000000000000000000000000000000000000f4240"
        );
    });
});

describe("amounts", () => {
    it("converts decimals to raw with string math", () => {
        expect(decimalToRaw("12.5", 6)).toBe(12500000n);
        expect(decimalToRaw("0.000001", 6)).toBe(1n);
        expect(decimalToRaw("3", 18)).toBe(3000000000000000000n);
        expect(() => decimalToRaw("0.0000001", 6)).toThrow();
        expect(() => decimalToRaw("1e5", 6)).toThrow();
    });
    it("rounds a display-truncated amount up by one display unit only above 8 decimals", () => {
        expect(roundUpTruncated(1234500000000000000n, 18)).toBe(1234500010000000000n);
        expect(roundUpTruncated(1500000n, 6)).toBe(1500000n);
        expect(roundUpTruncated(150000000n, 8)).toBe(150000000n);
    });
});

describe("planWalletPayment", () => {
    it("plans an EVM token payment straight from the server URI", () => {
        const r: any = planWalletPayment(payment(), NOW);
        expect(r.plan).toMatchObject({family: "evm", request: {chainId: 56, token: USDT_BSC, recipient: COLLECTOR, amount: 25000000000000000000n}});
    });
    it("only offers the button while waiting for payment", () => {
        for (const status of ["inProgress", "success", "failed", "underpaid"]) {
            expect(planWalletPayment(payment({}, {status}), NOW)).toHaveProperty("reason");
        }
    });
    it("refuses when less than two minutes remain", () => {
        expect(planWalletPayment(payment({}, {expiresAt: "2026-10-16T12:01:30Z"}), NOW)).toHaveProperty("reason");
    });
    it("refuses unknown (e.g. testnet) chains", () => {
        expect(planWalletPayment(payment({}, {paymentLink: `ethereum:${USDT_BSC}@97/transfer?address=${COLLECTOR}&uint256=25000000000000000000`}), NOW)).toHaveProperty("reason");
    });
    it("refuses when the URI recipient differs from the payment address", () => {
        const other = "0x2222222222222222222222222222222222222222";
        expect(planWalletPayment(payment({}, {paymentLink: `ethereum:${USDT_BSC}@56/transfer?address=${other}&uint256=25000000000000000000`}), NOW)).toHaveProperty("reason");
    });
    it("refuses when the URI amount differs from the locked amount", () => {
        expect(planWalletPayment(payment({}, {amount: "26000000000000000000"}), NOW)).toHaveProperty("reason");
    });
    it("tops up a partial payment with the remaining amount, rounded up", () => {
        const r: any = planWalletPayment(
            payment({}, {status: "partial", remainingAmount: "5.1234", remainingPaymentLink: `ethereum:${USDT_BSC}@56/transfer?address=${COLLECTOR}&uint256=5123400000000000000`}),
            NOW
        );
        expect(r.plan.request.amount).toBe(5123400010000000000n);
    });
    it("plans TRON USDT with the known contract and raw amount", () => {
        const r: any = planWalletPayment(
            payment({method: {blockchain: "TRON", blockchainName: "Tron", ticker: "TRON_USDT", name: "TRON_USDT"}}, {recipientAddress: "TXcollector", amount: "25000000", paymentLink: "tron:TXcollector?amount=25"}),
            NOW
        );
        expect(r.plan).toEqual({family: "tron", recipient: "TXcollector", amount: 25000000n, token: "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", explorerTx: expect.any(Function)});
    });
    it("falls back to the QR (no throw) when a TRON partial amount is malformed", () => {
        const p = payment({method: {blockchain: "TRON", ticker: "TRON_USDT", name: "TRON_USDT"}}, {status: "partial", remainingAmount: undefined});
        expect(planWalletPayment(p, NOW)).toHaveProperty("reason");
    });
    it("has no browser-wallet path for BTC", () => {
        expect(planWalletPayment(payment({method: {blockchain: "BTC", ticker: "BTC", name: "BTC"}}, {paymentLink: "bitcoin:bc1q?amount=0.1"}), NOW)).toHaveProperty("reason");
    });
});

const fakeProvider = (chainAfterSwitch: string) => {
    const calls: any[] = [];
    const request = vi.fn(async (args: any) => {
        calls.push(args);
        if (args.method === "eth_chainId") return chainAfterSwitch;
        if (args.method === "eth_sendTransaction") return "0xhash";
        return null;
    });
    return {calls, provider: {request}};
};

describe("payEvm", () => {
    const req = {chainId: 56, recipient: COLLECTOR, token: USDT_BSC, amount: 25n};

    it("never sends when the wallet is still on another network", async () => {
        const {calls, provider} = fakeProvider("0x1");
        await expect(payEvm(provider, "0xabc", req)).rejects.toBeInstanceOf(WrongNetworkError);
        expect(calls.map((c) => c.method)).not.toContain("eth_sendTransaction");
    });

    it("sends the token transfer on the right network, pinned with chainId", async () => {
        const {calls, provider} = fakeProvider("0x38");
        expect(await payEvm(provider, "0xabc", req)).toBe("0xhash");
        const send = calls.find((c) => c.method === "eth_sendTransaction").params[0];
        expect(send).toEqual({from: "0xabc", to: USDT_BSC, data: encodeErc20Transfer(COLLECTOR, 25n), value: "0x0", chainId: "0x38"});
    });

    it("sends native coin as value to the recipient", async () => {
        const {calls, provider} = fakeProvider("0x1");
        await payEvm(provider, "0xabc", {chainId: 1, recipient: COLLECTOR, amount: 255n});
        const send = calls.find((c) => c.method === "eth_sendTransaction").params[0];
        expect(send).toEqual({from: "0xabc", to: COLLECTOR, value: "0xff", chainId: "0x1"});
    });
});

describe("payTron", () => {
    it("refuses a wallet pointed at a TRON testnet", async () => {
        const tronWeb: any = {fullNode: {host: "https://api.nileex.io"}, trx: {sendTransaction: vi.fn()}};
        await expect(payTron(tronWeb, {family: "tron", recipient: "TX", amount: 1n} as any)).rejects.toThrow(/testnet/i);
        expect(tronWeb.trx.sendTransaction).not.toHaveBeenCalled();
    });
    it("sends TRX in sun on mainnet", async () => {
        const tronWeb: any = {fullNode: {host: "https://api.trongrid.io"}, trx: {sendTransaction: vi.fn(async () => ({result: true, txid: "abc"}))}};
        expect(await payTron(tronWeb, {family: "tron", recipient: "TX", amount: 1500000n} as any)).toBe("abc");
        expect(tronWeb.trx.sendTransaction).toHaveBeenCalledWith("TX", 1500000);
    });
});
