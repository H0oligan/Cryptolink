import type {WalletFamily} from "src/wallet/types";

export const shortAddr = (a: string) => (a.length > 12 ? `${a.slice(0, 6)}…${a.slice(-4)}` : a);

export class NotOwnerError extends Error {
    constructor(public connected: string, public owner: string) {
        super(
            `Connected wallet ${shortAddr(connected)} is not the owner of this merchant's contract (owner ${shortAddr(owner)}). ` +
            "Switch to the owner account in your wallet and try again."
        );
        this.name = "NotOwnerError";
    }
}

// The contract's onlyOwner is the real guard; this only avoids sending a
// transaction that would revert and waste gas.
export const assertOwner = (family: WalletFamily, onChainOwner: string, connected: string): void => {
    const same = family === "evm"
        ? onChainOwner.toLowerCase() === connected.toLowerCase()
        : onChainOwner.trim() === connected.trim(); // TRON base58 is case-sensitive
    if (!same) throw new NotOwnerError(connected, onChainOwner);
};
