import type {DetectedWallet} from "src/wallet/types";

const legacyName = (p: any): string => {
    if (p.isRabby) return "Rabby";
    if (p.isOkxWallet || p.isOKExWallet) return "OKX Wallet";
    if (p.isCoinbaseWallet) return "Coinbase Wallet";
    if (p.isTrust || p.isTrustWallet) return "Trust Wallet";
    if (p.isTokenPocket) return "TokenPocket";
    if (p.isBraveWallet) return "Brave Wallet";
    if (p.isMetaMask && p._metamask) return "MetaMask";
    return "Browser wallet";
};

// EIP-6963: every wallet announces itself, so none can hide another by
// overwriting window.ethereum. Legacy globals are only a fallback.
export const discoverEvmWallets = (win: any = window, timeoutMs = 400): Promise<DetectedWallet[]> =>
    new Promise((resolve) => {
        const byId = new Map<string, DetectedWallet>();

        const onAnnounce = (event: Event) => {
            const d = (event as CustomEvent).detail;
            if (!d?.info?.rdns || !d.provider || byId.has(d.info.rdns)) return;
            byId.set(d.info.rdns, {id: d.info.rdns, name: d.info.name, icon: d.info.icon, family: "evm", source: "eip6963", provider: d.provider});
        };

        win.addEventListener("eip6963:announceProvider", onAnnounce);
        win.dispatchEvent(new Event("eip6963:requestProvider"));

        setTimeout(() => {
            win.removeEventListener("eip6963:announceProvider", onAnnounce);

            const announced = new Set([...byId.values()].map((w) => w.provider));
            const legacy: any[] = win.ethereum?.providers?.length ? win.ethereum.providers : win.ethereum ? [win.ethereum] : [];
            legacy.forEach((p, i) => {
                if (announced.has(p)) return;
                const id = legacy.length === 1 ? "legacy:ethereum" : `legacy:ethereum-${i}`;
                byId.set(id, {id, name: legacyName(p), family: "evm", source: "legacy", provider: p});
            });

            resolve([...byId.values()]);
        }, timeoutMs);
    });
