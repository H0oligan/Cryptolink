// Copied from ui-dashboard/src/wallet/tron-discovery.ts — keep in sync (separate SPA package).
import type {DetectedWallet} from "src/wallet/types";
import {TIP6963_ANNOUNCE, TIP6963_REQUEST} from "src/wallet/catalog";

// Legacy globals are the PRIMARY path (verified). TIP-6963 is additive (unverified, see catalog.ts).
const LEGACY_TRON: {id: string; name: string; get: (w: any) => any}[] = [
    {id: "legacy:tronlink", name: "TronLink", get: (w) => w.tronLink},
    {id: "legacy:okx-tron", name: "OKX Wallet", get: (w) => w.okxwallet?.tronLink},
    {id: "legacy:bitget-tron", name: "Bitget Wallet", get: (w) => w.bitkeep?.tronLink},
    {id: "legacy:trust-tron", name: "Trust Wallet", get: (w) => w.trustwallet?.tronLink}
];

export const discoverTronWallets = (win: any = window, timeoutMs = 400): Promise<DetectedWallet[]> =>
    new Promise((resolve) => {
        const found: DetectedWallet[] = [];
        const seen = new Set<any>();

        const onAnnounce = (event: Event) => {
            const d = (event as CustomEvent).detail;
            if (!d?.info?.rdns || !d.provider || seen.has(d.provider)) return;
            seen.add(d.provider);
            found.push({id: d.info.rdns, name: d.info.name, icon: d.info.icon, family: "tron", source: "tip6963", provider: d.provider});
        };

        win.addEventListener(TIP6963_ANNOUNCE, onAnnounce);
        win.dispatchEvent(new Event(TIP6963_REQUEST));

        setTimeout(() => {
            win.removeEventListener(TIP6963_ANNOUNCE, onAnnounce);
            for (const l of LEGACY_TRON) {
                const p = l.get(win);
                if (!p || seen.has(p)) continue;
                seen.add(p);
                found.push({id: l.id, name: l.name, family: "tron", source: "legacy", provider: p});
            }
            resolve(found);
        }, timeoutMs);
    });
