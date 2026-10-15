import type {WalletFamily} from "src/wallet/types";

// UNVERIFIED: TIP-6963 (TRON's EIP-6963). Event names per the TRON proposal; confirm in Task 12.
export const TIP6963_ANNOUNCE = "TIP6963:announceProvider";
export const TIP6963_REQUEST = "TIP6963:requestProvider";

export interface KnownWallet {
    id: string;
    name: string;
    families: WalletFamily[];
    rdns?: string; // EIP-6963 rdns, used to mark it "Installed"
    installUrl: string;
    // UNVERIFIED until Task 12. Both open pageUrl inside the wallet app's built-in browser.
    // openInApp: https universal link (passes through the vendor's server); QR mode only, where the PC approves.
    // openInAppNative: custom scheme, stays on the device; the only kind allowed for same-phone "direct" mode.
    openInApp?: (pageUrl: string) => string;
    openInAppNative?: (pageUrl: string) => string;
}

const stripScheme = (url: string) => url.replace(/^https?:\/\//, "");

export const KNOWN_WALLETS: KnownWallet[] = [
    {id: "metamask", name: "MetaMask", families: ["evm"], rdns: "io.metamask", installUrl: "https://metamask.io/download/",
        openInApp: (u) => `https://metamask.app.link/dapp/${stripScheme(u)}`,
        openInAppNative: (u) => `metamask://dapp/${stripScheme(u)}`},
    {id: "trust", name: "Trust Wallet", families: ["evm", "tron"], rdns: "com.trustwallet.app", installUrl: "https://trustwallet.com/download",
        openInApp: (u) => `https://link.trustwallet.com/open_url?coin_id=60&url=${encodeURIComponent(u)}`,
        openInAppNative: (u) => `trust://open_url?coin_id=60&url=${encodeURIComponent(u)}`},
    {id: "okx", name: "OKX Wallet", families: ["evm", "tron"], rdns: "com.okex.wallet", installUrl: "https://www.okx.com/web3",
        openInApp: (u) => `https://www.okx.com/download?deeplink=${encodeURIComponent("okx://wallet/dapp/url?dappUrl=" + encodeURIComponent(u))}`,
        openInAppNative: (u) => `okx://wallet/dapp/url?dappUrl=${encodeURIComponent(u)}`},
    {id: "coinbase", name: "Coinbase Wallet", families: ["evm"], rdns: "com.coinbase.wallet", installUrl: "https://www.coinbase.com/wallet/downloads",
        openInApp: (u) => `https://go.cb-w.com/dapp?cb_url=${encodeURIComponent(u)}`,
        openInAppNative: (u) => `cbwallet://dapp?url=${encodeURIComponent(u)}`},
    {id: "tokenpocket", name: "TokenPocket", families: ["evm", "tron"], installUrl: "https://www.tokenpocket.pro/en/download/app",
        openInApp: (u) => `tpdapp://open?params=${encodeURIComponent(JSON.stringify({url: u, chain: "ETH", source: "cryptolink"}))}`,
        openInAppNative: (u) => `tpdapp://open?params=${encodeURIComponent(JSON.stringify({url: u, chain: "ETH", source: "cryptolink"}))}`},
    {id: "tronlink", name: "TronLink", families: ["tron"], installUrl: "https://www.tronlink.org/",
        openInApp: (u) => `tronlinkoutside://pull.activity?param=${encodeURIComponent(JSON.stringify({url: u, action: "open", protocol: "tronlink", version: "1.0"}))}`,
        openInAppNative: (u) => `tronlinkoutside://pull.activity?param=${encodeURIComponent(JSON.stringify({url: u, action: "open", protocol: "tronlink", version: "1.0"}))}`},
    {id: "rabby", name: "Rabby", families: ["evm"], rdns: "io.rabby", installUrl: "https://rabby.io/"}
];

export const buildOpenInWalletUrl = (walletId: string, pageUrl: string): string | null => {
    const w = KNOWN_WALLETS.find((x) => x.id === walletId);
    return w?.openInApp ? w.openInApp(pageUrl) : null;
};
