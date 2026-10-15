import {describe, it, expect} from "vitest";
import {discoverEvmWallets} from "src/wallet/evm-discovery";
import {discoverTronWallets} from "src/wallet/tron-discovery";

const announcing = (wallets: {rdns: string; name: string; provider: any}[]) => {
    const w: any = new EventTarget();
    w.addEventListener("eip6963:requestProvider", () => {
        for (const x of wallets) {
            w.dispatchEvent(new CustomEvent("eip6963:announceProvider", {
                detail: {info: {rdns: x.rdns, name: x.name, uuid: x.rdns, icon: "data:image/svg+xml,x"}, provider: x.provider}
            }));
        }
    });
    return w;
};

describe("discoverEvmWallets", () => {
    it("lists every announced wallet even when another owns window.ethereum", async () => {
        const mm = {request: async () => null};
        const rabby = {request: async () => null, isRabby: true};
        const w = announcing([{rdns: "io.metamask", name: "MetaMask", provider: mm}, {rdns: "io.rabby", name: "Rabby", provider: rabby}]);
        w.ethereum = rabby;
        const list = await discoverEvmWallets(w, 10);
        expect(list.map((x) => x.id).sort()).toEqual(["io.metamask", "io.rabby"]);
        expect(list.every((x) => x.source === "eip6963")).toBe(true);
    });

    it("dedupes duplicate announcements of the same rdns", async () => {
        const mm = {request: async () => null};
        const w = announcing([{rdns: "io.metamask", name: "MetaMask", provider: mm}, {rdns: "io.metamask", name: "MetaMask", provider: mm}]);
        expect(await discoverEvmWallets(w, 10)).toHaveLength(1);
    });

    it("falls back to legacy window.ethereum when nothing announces", async () => {
        const w: any = new EventTarget();
        w.ethereum = {request: async () => null, isMetaMask: true, _metamask: {}};
        const list = await discoverEvmWallets(w, 10);
        expect(list).toHaveLength(1);
        expect(list[0]).toMatchObject({id: "legacy:ethereum", name: "MetaMask", source: "legacy"});
    });

    it("does not list the legacy provider twice when it also announced", async () => {
        const mm = {request: async () => null, isMetaMask: true, _metamask: {}};
        const w = announcing([{rdns: "io.metamask", name: "MetaMask", provider: mm}]);
        w.ethereum = mm;
        expect(await discoverEvmWallets(w, 10)).toHaveLength(1);
    });

    it("returns [] when there is no wallet at all (phone browser)", async () => {
        expect(await discoverEvmWallets(new EventTarget(), 10)).toEqual([]);
    });
});

describe("discoverTronWallets", () => {
    it("finds TronLink and OKX via legacy globals without duplicates", async () => {
        const w: any = new EventTarget();
        const tl = {request: async () => null, tronWeb: {}};
        w.tronLink = tl;
        w.okxwallet = {tronLink: {request: async () => null, tronWeb: {}}};
        const list = await discoverTronWallets(w, 10);
        expect(list.map((x) => x.id).sort()).toEqual(["legacy:okx-tron", "legacy:tronlink"]);
    });

    it("returns [] when no TRON wallet exists", async () => {
        expect(await discoverTronWallets(new EventTarget(), 10)).toEqual([]);
    });
});
