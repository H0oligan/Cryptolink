import {describe, it, expect, vi, beforeEach} from "vitest";

const fakeWindow = () => {
    const w: any = new EventTarget();
    w.ethereum = {isMetaMask: false}; // another wallet owns window.ethereum
    w.addEventListener("eip6963:requestProvider", () => {
        w.dispatchEvent(new CustomEvent("eip6963:announceProvider", {
            detail: {info: {rdns: "io.metamask", name: "MetaMask", uuid: "1", icon: ""}, provider: {request: vi.fn()}}
        }));
    });
    return w;
};

describe("isMetaMaskAvailable (hotfix)", () => {
    beforeEach(() => vi.resetModules());

    it("is true when MetaMask announces via EIP-6963 but does not own window.ethereum", async () => {
        vi.stubGlobal("window", fakeWindow());
        const {isMetaMaskAvailable} = await import("src/utils/evm-wallet");
        expect(isMetaMaskAvailable()).toBe(true);
    });
});
