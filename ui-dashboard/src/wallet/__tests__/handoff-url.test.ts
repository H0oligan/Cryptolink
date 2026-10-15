import {describe, it, expect} from "vitest";
import {buildHandoffPageUrl, readHandoffToken, describeDevice} from "src/wallet/handoff-url";

describe("handoff urls", () => {
    it("puts the token in the fragment, never the query", () => {
        expect(buildHandoffPageUrl("https://cryptolink.cc", "abc")).toBe("https://cryptolink.cc/merchants/handoff#t=abc");
    });
    it("reads the fragment first, then the query fallback", () => {
        expect(readHandoffToken("#t=frag", "?t=query")).toBe("frag");
        expect(readHandoffToken("", "?t=query")).toBe("query");
        expect(readHandoffToken("", "")).toBeNull();
    });
    it("labels the device for the PC approval prompt", () => {
        expect(describeDevice("Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)", "MetaMask")).toBe("MetaMask · iPhone");
        expect(describeDevice("Mozilla/5.0 (Linux; Android 14; Pixel 8) Mobile", null)).toBe("Browser · Android");
    });
});
