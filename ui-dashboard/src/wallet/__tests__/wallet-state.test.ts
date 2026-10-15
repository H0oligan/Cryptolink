import {describe, it, expect} from "vitest";
import {walletReducer, initialWalletState} from "src/wallet/wallet-state";

const w: any = {id: "io.metamask", name: "MetaMask", family: "evm", source: "eip6963", provider: {}};

describe("walletReducer", () => {
    it("tracks account switches inside the wallet", () => {
        let s = walletReducer(initialWalletState, {type: "connected", family: "evm", conn: {wallet: w, address: "0xA"}});
        s = walletReducer(s, {type: "accountsChanged", family: "evm", address: "0xB"});
        expect(s.evm?.address).toBe("0xB");
    });
    it("drops the connection when the wallet reports no accounts (locked)", () => {
        let s = walletReducer(initialWalletState, {type: "connected", family: "evm", conn: {wallet: w, address: "0xA"}});
        s = walletReducer(s, {type: "accountsChanged", family: "evm", address: null});
        expect(s.evm).toBeNull();
    });
    it("disconnect only affects one family", () => {
        let s = walletReducer(initialWalletState, {type: "connected", family: "evm", conn: {wallet: w, address: "0xA"}});
        s = walletReducer(s, {type: "connected", family: "tron", conn: {wallet: {...w, family: "tron"}, address: "T1"}});
        s = walletReducer(s, {type: "disconnect", family: "evm"});
        expect(s.evm).toBeNull();
        expect(s.tron?.address).toBe("T1");
    });
    it("ignores accountsChanged for a family that is not connected", () => {
        const s = walletReducer(initialWalletState, {type: "accountsChanged", family: "evm", address: "0xB"});
        expect(s.evm).toBeNull();
    });
});
