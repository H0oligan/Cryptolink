import apiRequest from "src/utils/api-request";
import withApiPath from "src/utils/with-api-path";

export type HandoffReturnPath = "/balance" | "/wallet-setup";
export type HandoffMode = "qr" | "direct";
export type HandoffStatus = "pending" | "claimed" | "approved" | "denied" | "expired";

export type HandoffDone = {done: true; returnPath: string; merchantId: string};

export interface HandoffClaim {
    claimSecret: string;
    matchCode: string; // qr mode: show on the phone, typed on the PC
    mode: HandoffMode;
    account: string; // masked email of the account being signed into
}

const expired = () => new Error("This link has expired or was declined. Generate a new one on your computer.");

// api-request's interceptor resolves plain 400s with `undefined` instead of
// rejecting, so an empty response is treated as failure here.
const handoffProvider = {
    async create(
        returnPath: HandoffReturnPath,
        merchantId: string | null,
        mode: HandoffMode
    ): Promise<{id: string; token: string; expiresAt: string}> {
        const res = await apiRequest.post(withApiPath("/auth/handoff"), {returnPath, merchantId: merchantId ?? "", mode});
        if (!res?.data?.token) throw new Error("Could not create a sign-in link. Please try again.");
        return res.data;
    },

    async status(id: string): Promise<{status: HandoffStatus; device: string}> {
        const res = await apiRequest.get(withApiPath(`/auth/handoff/${encodeURIComponent(id)}`), {silent: true});
        return res?.data ?? {status: "expired", device: ""};
    },

    // Rejects with a 422 response on a wrong code.
    async decide(id: string, approve: boolean, code = ""): Promise<void> {
        const res = await apiRequest.post(
            withApiPath(`/auth/handoff/${encodeURIComponent(id)}/decision`),
            {approve, code},
            {silent: true}
        );
        if (!res) throw expired();
    },

    // Never signs in by itself; complete() does, after approval/confirmation.
    async claim(token: string, device: string): Promise<HandoffClaim> {
        const res = await apiRequest.post(withApiPath("/auth/handoff/claim"), {token, device}, {silent: true});
        if (!res?.data?.claimSecret) throw expired();
        return res.data;
    },

    // Rejects with a 409 response when this browser is signed in to another account.
    async complete(claimSecret: string): Promise<HandoffDone | {done: false}> {
        const res = await apiRequest.post(withApiPath("/auth/handoff/complete"), {claimSecret}, {silent: true});
        if (!res) throw expired();
        if (res.status === 202) return {done: false};
        return {done: true, returnPath: res.data.returnPath, merchantId: res.data.merchantId};
    }
};

export default handoffProvider;
