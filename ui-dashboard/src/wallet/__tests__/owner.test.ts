import {describe, it, expect} from "vitest";
import {assertOwner, NotOwnerError} from "src/wallet/owner";

describe("assertOwner", () => {
    it("accepts EVM addresses case-insensitively", () => {
        expect(() => assertOwner("evm", "0xABCdef0000000000000000000000000000000001", "0xabcdef0000000000000000000000000000000001")).not.toThrow();
    });
    it("rejects a different EVM wallet with both addresses in the error", () => {
        try {
            assertOwner("evm", "0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222");
            throw new Error("should have thrown");
        } catch (e) {
            expect(e).toBeInstanceOf(NotOwnerError);
            expect((e as NotOwnerError).message).toContain("0x2222…2222");
            expect((e as NotOwnerError).message).toContain("0x1111…1111");
        }
    });
    it("compares TRON base58 exactly (base58 is case-sensitive)", () => {
        expect(() => assertOwner("tron", "TXyz111", "TXyz111")).not.toThrow();
        expect(() => assertOwner("tron", "TXyz111", "txyz111")).toThrow(NotOwnerError);
    });
});
