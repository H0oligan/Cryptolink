import {describe, it, expect} from "vitest";
import {buildOpenInWalletUrl, KNOWN_WALLETS} from "src/wallet/catalog";
import {isMobileDevice} from "src/wallet/env";

const page = "https://cryptolink.cc/merchants/handoff#t=abc";

describe("buildOpenInWalletUrl", () => {
    it("MetaMask strips the scheme and keeps the fragment", () => {
        expect(buildOpenInWalletUrl("metamask", page)).toBe("https://metamask.app.link/dapp/cryptolink.cc/merchants/handoff#t=abc");
    });
    it("Trust encodes the full URL", () => {
        expect(buildOpenInWalletUrl("trust", page)).toBe(
            "https://link.trustwallet.com/open_url?coin_id=60&url=" + encodeURIComponent(page)
        );
    });
    it("native MetaMask link uses the custom scheme (token stays on the device)", () => {
        const mm = KNOWN_WALLETS.find((w) => w.id === "metamask")!;
        expect(mm.openInAppNative!(page)).toBe("metamask://dapp/cryptolink.cc/merchants/handoff#t=abc");
    });
    it("returns null for an unknown wallet", () => {
        expect(buildOpenInWalletUrl("nope", page)).toBeNull();
    });
    it("every known wallet has an install URL", () => {
        for (const w of KNOWN_WALLETS) expect(w.installUrl).toMatch(/^https:\/\//);
    });
});

describe("isMobileDevice", () => {
    it("detects iPhone and Android, not desktop Chrome", () => {
        expect(isMobileDevice("Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)")).toBe(true);
        expect(isMobileDevice("Mozilla/5.0 (Linux; Android 14; Pixel 8) Mobile")).toBe(true);
        expect(isMobileDevice("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/130.0")).toBe(false);
    });
});
