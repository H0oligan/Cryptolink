import React from "react";
import {useNavigate} from "react-router-dom";
import {Button, Result, Space, Spin, Typography} from "antd";
import {AxiosError} from "axios";
import authProvider from "src/providers/auth-provider";
import handoffProvider, {type HandoffClaim, type HandoffDone} from "src/providers/handoff-provider";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import {readHandoffToken, describeDevice} from "src/wallet/handoff-url";
import {discoverEvmWallets} from "src/wallet/evm-discovery";
import {discoverTronWallets} from "src/wallet/tron-discovery";

const SAFE_PATHS = ["/balance", "/wallet-setup"];
const POLL_MS = 2000;
const MAX_WAIT_MS = 2 * 60 * 1000;

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

type Phase =
    | {kind: "working"}
    | {kind: "confirm"; claim: HandoffClaim} // same phone: "Continue as …"
    | {kind: "approve"; claim: HandoffClaim} // QR: show the code to type on the PC
    | {kind: "failed"; message: string};

const failureMessage = (err: unknown) =>
    err instanceof AxiosError && err.response?.status === 409
        ? "This browser is signed in to another CryptoLink account. Log out first, then open the link again."
        : "Generate a new one on your computer, or sign in here.";

// Lands inside a wallet app's browser with #t=<token>. The claim alone never
// signs in: same-phone links ask "Continue as …", QR links wait until the
// code shown here is typed on the computer.
const HandoffPage: React.FC = () => {
    const navigate = useNavigate();
    const {setMerchantId} = useSharedMerchantId();
    const [phase, setPhase] = React.useState<Phase>({kind: "working"});
    const cancelled = React.useRef(false);

    const finish = (r: HandoffDone) => {
        if (r.merchantId) setMerchantId(r.merchantId); // a foreign id is reset by App's storedIdValid check
        const path = SAFE_PATHS.includes(r.returnPath) ? r.returnPath : "/balance";
        navigate(path, {replace: true, state: {realoadUserInfo: true}});
    };

    const completeLoop = async (claim: HandoffClaim) => {
        const deadline = Date.now() + MAX_WAIT_MS;
        while (!cancelled.current && Date.now() < deadline) {
            const r = await handoffProvider.complete(claim.claimSecret);
            if (r.done) return finish(r);
            await sleep(POLL_MS);
        }
        throw new Error("timed out");
    };

    const fail = (err: unknown) => {
        if (!cancelled.current) setPhase({kind: "failed", message: failureMessage(err)});
    };

    React.useEffect(() => {
        cancelled.current = false;

        (async () => {
            const token = readHandoffToken(window.location.hash, window.location.search);
            window.history.replaceState(null, "", window.location.pathname); // drop the token from history
            if (!token) throw new Error("missing token");

            await authProvider.getCookie();

            const [evm, tron] = await Promise.all([discoverEvmWallets(), discoverTronWallets()]);
            const claim = await handoffProvider.claim(token, describeDevice(navigator.userAgent, [...evm, ...tron][0]?.name ?? null));

            if (claim.mode === "direct") {
                setPhase({kind: "confirm", claim});
                return;
            }
            setPhase({kind: "approve", claim});
            await completeLoop(claim);
        })().catch(fail);

        return () => {
            cancelled.current = true;
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    if (phase.kind === "failed") {
        return (
            <Result
                status="warning"
                title="This link has expired or was declined"
                subTitle={phase.message}
                extra={
                    <Button type="primary" onClick={() => navigate("/login", {replace: true})}>
                        Go to login
                    </Button>
                }
            />
        );
    }

    if (phase.kind === "confirm") {
        return (
            <Result
                status="info"
                title="Continue to CryptoLink?"
                subTitle={`You are signing in as ${phase.claim.account}. Only continue if this is your account.`}
                extra={
                    <Space>
                        <Button onClick={() => navigate("/login", {replace: true})}>Cancel</Button>
                        <Button
                            type="primary"
                            onClick={() => {
                                setPhase({kind: "working"});
                                completeLoop(phase.claim).catch(fail);
                            }}
                        >
                            Continue as {phase.claim.account}
                        </Button>
                    </Space>
                }
            />
        );
    }

    if (phase.kind === "approve") {
        return (
            <Result
                icon={<Spin size="large" />}
                title="Type this code on your computer"
                subTitle={
                    <>
                        <Typography.Title level={2} style={{letterSpacing: "0.2em", fontFamily: "var(--cl-monospace)", margin: "12px 0"}}>
                            {phase.claim.matchCode}
                        </Typography.Title>
                        Signing in as {phase.claim.account}. Keep this page open.
                    </>
                }
            />
        );
    }

    return <Result icon={<Spin size="large" />} title="Signing you in…" />;
};

export default HandoffPage;
