import React from "react";
import {Alert, Button, Collapse, Input, Space, Typography} from "antd";
import {MobileOutlined} from "@ant-design/icons";
import {QRCodeSVG} from "qrcode.react";
import {AxiosError} from "axios";
import handoffProvider, {type HandoffReturnPath, type HandoffStatus} from "src/providers/handoff-provider";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import {useWallet} from "src/wallet/wallet-context";
import {KNOWN_WALLETS, type KnownWallet} from "src/wallet/catalog";
import {buildHandoffPageUrl} from "src/wallet/handoff-url";
import {hasInjectedWallet, isMobileDevice} from "src/wallet/env";
import type {WalletFamily} from "src/wallet/types";

const {Text} = Typography;
const POLL_MS = 2000;

const currentReturnPath = (): HandoffReturnPath =>
    window.location.pathname.endsWith("/wallet-setup") ? "/wallet-setup" : "/balance";

const forFamily = (family: WalletFamily | null) => (k: KnownWallet) => !family || k.families.includes(family);

// Same phone: open this page inside a wallet app (native scheme; the token
// never leaves the device). The phone then asks "Continue as …".
const OpenInApp: React.FC<{family: WalletFamily | null}> = ({family}) => {
    const {merchantId} = useSharedMerchantId();
    const [busy, setBusy] = React.useState<string | null>(null);
    const [error, setError] = React.useState<string | null>(null);
    const wallets = KNOWN_WALLETS.filter((k) => k.openInAppNative).filter(forFamily(family));

    const open = async (k: KnownWallet) => {
        setBusy(k.id);
        setError(null);
        try {
            const {token} = await handoffProvider.create(currentReturnPath(), merchantId, "direct");
            window.location.href = k.openInAppNative!(buildHandoffPageUrl(window.location.origin, token));
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setBusy(null);
        }
    };

    return (
        <section className="connect-wallet-modal__section">
            <div className="connect-wallet-modal__section-title">Open in your wallet app</div>
            {error && <Alert type="error" showIcon message={error} className="connect-wallet-modal__error" />}
            {wallets.map((k) => (
                <button key={k.id} type="button" className="connect-wallet-modal__row" disabled={busy !== null} onClick={() => open(k)}>
                    <span className="connect-wallet-modal__icon connect-wallet-modal__icon_letter">{k.name.charAt(0)}</span>
                    <span className="connect-wallet-modal__grow">Open in {k.name}</span>
                </button>
            ))}
            <Text type="secondary" className="connect-wallet-modal__note">
                CryptoLink reopens inside the app, where the wallet can sign.
            </Text>
        </section>
    );
};

interface Session {
    id: string;
    qrValue: string;
    expiresAt: number;
}

// PC -> phone: QR of the wallet's universal link. The phone shows a 6-digit
// code; the merchant types it here to approve. Nothing is signed in before.
const UseMyPhone: React.FC<{family: WalletFamily | null}> = ({family}) => {
    const {merchantId} = useSharedMerchantId();
    const {markHandoffActive} = useWallet();
    const [session, setSession] = React.useState<Session | null>(null);
    const [status, setStatus] = React.useState<HandoffStatus>("pending");
    const [device, setDevice] = React.useState("");
    const [code, setCode] = React.useState("");
    const [error, setError] = React.useState<string | null>(null);
    const [now, setNow] = React.useState(Date.now());
    const wallets = KNOWN_WALLETS.filter((k) => k.openInApp).filter(forFamily(family));

    const start = async (k: KnownWallet | null) => {
        setError(null);
        setCode("");
        setStatus("pending");
        try {
            const r = await handoffProvider.create(currentReturnPath(), merchantId, "qr");
            const page = buildHandoffPageUrl(window.location.origin, r.token);
            setSession({id: r.id, qrValue: k ? k.openInApp!(page) : page, expiresAt: Date.parse(r.expiresAt)});
        } catch (e) {
            setError((e as Error).message);
        }
    };

    React.useEffect(() => {
        if (!session || status === "approved" || status === "denied" || status === "expired") return;
        const timer = setInterval(async () => {
            setNow(Date.now());
            const s = await handoffProvider.status(session.id);
            setStatus(s.status);
            setDevice(s.device);
            if (s.status === "approved") markHandoffActive();
        }, POLL_MS);
        return () => clearInterval(timer);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [session, status]);

    const decide = async (approve: boolean) => {
        if (!session) return;
        setError(null);
        try {
            await handoffProvider.decide(session.id, approve, code);
            const s = await handoffProvider.status(session.id);
            setStatus(s.status);
            if (s.status === "approved") markHandoffActive();
        } catch (e) {
            setError(
                e instanceof AxiosError && e.response?.status === 422
                    ? "Wrong code. Check the code shown on your phone (3 tries)."
                    : "This request expired. Generate a new code."
            );
            const s = await handoffProvider.status(session.id);
            setStatus(s.status);
        }
    };

    if (!session) {
        return (
            <>
                {error && <Alert type="error" showIcon message={error} className="connect-wallet-modal__error" />}
                <Text type="secondary" className="connect-wallet-modal__empty">
                    Pick the wallet app on your phone, then scan the code with the phone camera.
                </Text>
                {wallets.map((k) => (
                    <button key={k.id} type="button" className="connect-wallet-modal__row" onClick={() => start(k)}>
                        <span className="connect-wallet-modal__icon connect-wallet-modal__icon_letter">{k.name.charAt(0)}</span>
                        <span className="connect-wallet-modal__grow">{k.name}</span>
                    </button>
                ))}
                <button type="button" className="connect-wallet-modal__row connect-wallet-modal__row_muted" onClick={() => start(null)}>
                    <MobileOutlined className="connect-wallet-modal__icon" />
                    <span className="connect-wallet-modal__grow">Phone browser (no wallet app)</span>
                </button>
            </>
        );
    }

    const secondsLeft = Math.max(0, Math.round((session.expiresAt - now) / 1000));

    if (status === "claimed") {
        return (
            <Space direction="vertical" style={{width: "100%"}}>
                <Alert
                    type="warning"
                    showIcon
                    message={`"${device || "A phone"}" wants to sign in to your account.`}
                    description="Only approve if this is your phone. Type the 6-digit code shown on it."
                />
                {error && <Alert type="error" showIcon message={error} />}
                <Input
                    inputMode="numeric"
                    maxLength={6}
                    placeholder="123456"
                    value={code}
                    onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
                    style={{fontFamily: "var(--cl-monospace)", letterSpacing: "0.2em"}}
                />
                <Space>
                    <Button type="primary" disabled={code.length !== 6} onClick={() => decide(true)}>
                        Approve
                    </Button>
                    <Button danger onClick={() => decide(false)}>
                        Deny
                    </Button>
                </Space>
            </Space>
        );
    }

    if (status === "approved") {
        return (
            <Alert
                type="success"
                showIcon
                message="Phone connected"
                description="Finish on your phone. This page refreshes automatically."
            />
        );
    }

    if (status === "denied" || status === "expired" || secondsLeft === 0) {
        return (
            <Space direction="vertical">
                <Alert type="info" showIcon message={status === "denied" ? "Sign-in denied." : "Code expired."} />
                <Button onClick={() => setSession(null)}>Generate a new code</Button>
            </Space>
        );
    }

    return (
        <Space direction="vertical" align="center" style={{width: "100%"}}>
            <div style={{background: "#fff", padding: 8, borderRadius: 8, lineHeight: 0}}>
                <QRCodeSVG value={session.qrValue} size={220} level="M" includeMargin />
            </div>
            <Text type="secondary">Scan with your phone camera · expires in {secondsLeft}s</Text>
            <Button size="small" onClick={() => setSession(null)}>
                Choose another app
            </Button>
        </Space>
    );
};

const HandoffPanel: React.FC<{family: WalletFamily | null}> = ({family}) => {
    if (isMobileDevice()) {
        // Inside a wallet's own browser the wallet is already detected above.
        return hasInjectedWallet() ? null : <OpenInApp family={family} />;
    }

    return (
        <Collapse
            className="connect-wallet-modal__section"
            items={[{key: "phone", label: "Use my phone instead", children: <UseMyPhone family={family} />}]}
        />
    );
};

export default HandoffPanel;
