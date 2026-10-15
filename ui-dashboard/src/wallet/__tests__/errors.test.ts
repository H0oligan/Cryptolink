import {describe, it, expect} from "vitest";
import {describeWalletError} from "src/wallet/errors";
import {NotOwnerError} from "src/wallet/owner";

describe("describeWalletError", () => {
    it("maps EIP-1193 4001 to a rejection", () => {
        expect(describeWalletError({code: 4001, message: "User rejected the request."}).rejected).toBe(true);
    });
    it("maps TRON string rejections", () => {
        expect(describeWalletError("Confirmation declined by user").rejected).toBe(true);
    });
    it("passes NotOwnerError through with its title", () => {
        const d = describeWalletError(new NotOwnerError("0x1", "0x2"));
        expect(d.title).toBe("Wrong wallet");
        expect(d.rejected).toBe(false);
    });
    it("explains insufficient funds for gas", () => {
        expect(describeWalletError({message: "insufficient funds for gas * price + value"}).title).toBe("Not enough gas");
    });
    it("never returns an empty description", () => {
        expect(describeWalletError(undefined).description.length).toBeGreaterThan(0);
    });
});
