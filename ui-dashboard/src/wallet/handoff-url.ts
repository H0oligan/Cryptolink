// The token travels in the URL fragment: browsers never send it to the
// server, so it never lands in nginx access logs.
export const buildHandoffPageUrl = (origin: string, token: string): string => `${origin}/merchants/handoff#t=${token}`;

// Fragment first; the query fallback is for wallets whose deep links drop "#…".
export const readHandoffToken = (hash: string, search: string): string | null =>
    new URLSearchParams(hash.replace(/^#/, "")).get("t") || new URLSearchParams(search).get("t") || null;

export const describeDevice = (ua: string, walletName: string | null): string => {
    const platform = /iPhone/i.test(ua) ? "iPhone" : /iPad/i.test(ua) ? "iPad" : /Android/i.test(ua) ? "Android" : "Desktop";
    return `${walletName ?? "Browser"} · ${platform}`;
};
