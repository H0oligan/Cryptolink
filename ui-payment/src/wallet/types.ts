// Copied from ui-dashboard/src/wallet/types.ts — keep in sync (separate SPA package).
export type WalletFamily = "evm" | "tron";

export interface Eip1193Provider {
    request(args: {method: string; params?: unknown}): Promise<any>;
    on?(event: string, cb: (...args: any[]) => void): void;
    removeListener?(event: string, cb: (...args: any[]) => void): void;
}

export interface DetectedWallet {
    id: string; // EIP-6963 rdns, or "legacy:<name>"
    name: string;
    icon?: string; // data: URI supplied by the wallet itself
    family: WalletFamily;
    source: "eip6963" | "tip6963" | "legacy";
    provider: any; // Eip1193Provider for evm, TronLink-like object for tron
}
