export const isMobileDevice = (ua: string = typeof navigator !== "undefined" ? navigator.userAgent : ""): boolean =>
    /Android|iPhone|iPad|iPod|Mobile/i.test(ua);

// True inside a wallet's in-app browser or with any extension present.
export const hasInjectedWallet = (win: any = typeof window !== "undefined" ? window : {}): boolean =>
    Boolean(win.ethereum || win.tronLink || win.okxwallet || win.bitkeep || win.trustwallet);
