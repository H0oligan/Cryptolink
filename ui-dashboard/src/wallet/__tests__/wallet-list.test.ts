import {describe, it, expect} from "vitest";
import {splitWalletList} from "src/wallet/wallet-list";

const det = (id: string, name: string, family: "evm" | "tron" = "evm"): any => ({id, name, family, source: "eip6963", provider: {}});

describe("splitWalletList", () => {
    it("does not list a detected wallet again under 'install'", () => {
        const {detected, notInstalled} = splitWalletList("evm", [det("io.metamask", "MetaMask")]);
        expect(detected.map((w) => w.id)).toEqual(["io.metamask"]);
        expect(notInstalled.map((w) => w.id)).not.toContain("metamask");
    });
    it("matches legacy wallets by name when there is no rdns", () => {
        const {notInstalled} = splitWalletList("tron", [det("legacy:tronlink", "TronLink", "tron")]);
        expect(notInstalled.map((w) => w.id)).not.toContain("tronlink");
    });
    it("only offers wallets of the requested family", () => {
        const {notInstalled} = splitWalletList("tron", []);
        expect(notInstalled.map((w) => w.id)).not.toContain("metamask");
        expect(notInstalled.map((w) => w.id)).toContain("tronlink");
    });
    it("filters detected wallets by family", () => {
        const {detected} = splitWalletList("evm", [det("io.metamask", "MetaMask"), det("legacy:tronlink", "TronLink", "tron")]);
        expect(detected).toHaveLength(1);
    });
});
