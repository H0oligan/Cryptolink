import {KNOWN_WALLETS, type KnownWallet} from "src/wallet/catalog";
import type {DetectedWallet, WalletFamily} from "src/wallet/types";

// Detected wallets of a family, plus known wallets of that family that were
// not detected (offered as "Install" or "Open in app").
export const splitWalletList = (
    family: WalletFamily,
    all: DetectedWallet[]
): {detected: DetectedWallet[]; notInstalled: KnownWallet[]} => {
    const detected = all.filter((w) => w.family === family);
    const isDetected = (k: KnownWallet) =>
        detected.some((d) => (k.rdns && d.id === k.rdns) || d.name.toLowerCase() === k.name.toLowerCase());
    const notInstalled = KNOWN_WALLETS.filter((k) => k.families.includes(family) && !isDetected(k));

    return {detected, notInstalled};
};
