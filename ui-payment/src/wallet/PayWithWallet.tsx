import React from "react";
import {createPortal} from "react-dom";
import type {Payment} from "src/types";
import {planWalletPayment, type PayPlan} from "src/wallet/plan";
import {payEvm, payTron} from "src/wallet/execute";
import {connectEvm, connectTron, describeError, recallSent, rememberSent} from "src/wallet/connect";
import {discoverEvmWallets} from "src/wallet/evm-discovery";
import {discoverTronWallets} from "src/wallet/tron-discovery";
import {splitWalletList} from "src/wallet/wallet-list";
import {KNOWN_WALLETS} from "src/wallet/catalog";
import {hasInjectedWallet, isMobileDevice} from "src/wallet/env";
import {EVM_CHAINS} from "src/wallet/chains";
import type {DetectedWallet} from "src/wallet/types";

const short = (a: string) => (a.length > 14 ? `${a.slice(0, 8)}…${a.slice(-6)}` : a);

type Step =
    | {kind: "closed"}
    | {kind: "pick"; wallets: DetectedWallet[] | null}
    | {kind: "connecting"; wallet: DetectedWallet}
    | {kind: "confirm"; wallet: DetectedWallet; from: string; tronWeb?: any}
    | {kind: "sending"; wallet: DetectedWallet}
    | {kind: "sent"; hash: string; url: string}
    | {kind: "error"; text: string};

const Row: React.FC<{onClick?: () => void; href?: string; icon?: string; name: string; tag?: string; disabled?: boolean}> = ({
    onClick, href, icon, name, tag, disabled
}) => {
    const body = (
        <>
            {icon ? (
                <img src={icon} alt="" className="h-7 w-7 rounded-md" />
            ) : (
                <span className="flex h-7 w-7 items-center justify-center rounded-md bg-[#1e1e2a] text-sm font-semibold text-card-desc">
                    {name.charAt(0)}
                </span>
            )}
            <span className="flex-1 text-left">{name}</span>
            {tag && <span className="text-xs text-card-desc">{tag}</span>}
        </>
    );
    const cls = "mb-2 flex w-full items-center gap-3 rounded-xl border border-[#2a2a3e] bg-[#13131a] px-3 py-2.5 text-sm text-white hover:border-main-green-1 disabled:opacity-50";
    return href ? (
        <a className={cls} href={href} target={href.startsWith("http") ? "_blank" : undefined} rel="noopener noreferrer">{body}</a>
    ) : (
        <button type="button" className={cls} onClick={onClick} disabled={disabled}>{body}</button>
    );
};

// One-click checkout with the customer's own wallet. Every value the wallet
// signs (network, token, recipient, amount) comes from the server's payment
// URI; when anything doesn't line up, only the QR/manual path is shown.
const PayWithWallet: React.FC<{payment: Payment}> = ({payment}) => {
    const [step, setStep] = React.useState<Step>({kind: "closed"});
    const info = payment.paymentInfo!;
    const method = payment.paymentMethod!;
    const result = planWalletPayment(payment, Date.now());
    const plan: PayPlan | null = "plan" in result ? result.plan : null;
    const alreadySent = recallSent(payment.id, info.status);
    const mobile = isMobileDevice();
    const uri = info.status === "partial" && info.remainingPaymentLink ? info.remainingPaymentLink : info.paymentLink;
    const amountText = `${info.status === "partial" ? info.remainingAmount : info.amountFormatted} ${method.displayName}`;
    const network = plan?.family === "evm" ? EVM_CHAINS[plan.request.chainId].name : method.blockchainName;

    const openPicker = async () => {
        setStep({kind: "pick", wallets: null});
        const found = plan!.family === "evm" ? await discoverEvmWallets() : await discoverTronWallets();
        setStep({kind: "pick", wallets: found});
    };

    const connect = async (w: DetectedWallet) => {
        setStep({kind: "connecting", wallet: w});
        try {
            if (plan!.family === "evm") {
                setStep({kind: "confirm", wallet: w, from: await connectEvm(w)});
            } else {
                const {address, tronWeb} = await connectTron(w);
                setStep({kind: "confirm", wallet: w, from: address, tronWeb});
            }
        } catch (err) {
            const d = describeError(err);
            setStep(d.rejected ? {kind: "pick", wallets: null} : {kind: "error", text: d.text});
            if (d.rejected) openPicker();
        }
    };

    const send = async (s: Extract<Step, {kind: "confirm"}>) => {
        // Re-plan at click time: status/expiry may have changed while the sheet was open.
        const fresh = planWalletPayment(payment, Date.now());
        if (!("plan" in fresh)) return setStep({kind: "error", text: `${fresh.reason}. Please use the QR code or address below.`});
        setStep({kind: "sending", wallet: s.wallet});
        try {
            const p = fresh.plan;
            const hash = p.family === "evm" ? await payEvm(s.wallet.provider, s.from, p.request) : await payTron(s.tronWeb, p);
            const url = p.explorerTx(hash);
            rememberSent(payment.id, info.status, hash, url);
            setStep({kind: "sent", hash, url});
        } catch (err) {
            const d = describeError(err);
            setStep(d.rejected ? s : {kind: "error", text: d.text});
        }
    };

    // Phone browser without a wallet: reopen this checkout inside a wallet app,
    // or hand the standard payment URI to whatever wallet app is installed.
    const appLinks = mobile && !hasInjectedWallet();
    if (info.status !== "pending" && info.status !== "partial") return null;
    if (!plan && !(appLinks && uri)) return null;

    if (alreadySent) {
        return (
            <div className="mx-auto mb-5 max-w-sm rounded-md border border-emerald-500/40 bg-emerald-500/10 p-3 text-center">
                <p className="text-sm font-semibold text-emerald-300">Payment sent from your wallet</p>
                <p className="mt-1 text-xs text-emerald-100/90">
                    Waiting for the network to confirm it. Don&apos;t send again.{" "}
                    <a className="underline" href={alreadySent.url} target="_blank" rel="noopener noreferrer">View transaction</a>
                </p>
            </div>
        );
    }

    const family = plan?.family ?? (method.blockchain === "TRON" ? "tron" : "evm");
    const openInApp = KNOWN_WALLETS.filter((k) => k.openInApp && k.families.includes(family));

    return (
        <div className="mx-auto mb-5 max-w-sm">
            {plan && (
                <button
                    type="button"
                    onClick={openPicker}
                    className="flex h-12 w-full items-center justify-center gap-2 rounded-3xl border border-main-green-1 bg-main-green-1 font-medium text-white"
                >
                    <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2">
                        <rect x="3" y="6" width="18" height="13" rx="2" />
                        <path d="M16 12h2M3 9h18" />
                    </svg>
                    Pay with wallet
                </button>
            )}
            {appLinks && uri && !plan && (
                <a href={uri} className="flex h-12 w-full items-center justify-center rounded-3xl border border-main-green-1 bg-main-green-1 font-medium text-white">
                    Open in wallet app
                </a>
            )}

            {step.kind !== "closed" && createPortal(
                <div className="fixed inset-0 z-50 flex items-end justify-center bg-black/60 lg:items-center" onClick={() => step.kind !== "sending" && setStep({kind: "closed"})}>
                    <div
                        className="max-h-[85vh] w-full max-w-md overflow-y-auto rounded-t-2xl border border-[#2a2a3e] bg-[#0f0f16] p-5 text-white lg:rounded-2xl"
                        onClick={(e) => e.stopPropagation()}
                    >
                        <div className="mb-4 flex items-start justify-between">
                            <div>
                                <p className="text-lg font-semibold">Pay {amountText}</p>
                                <p className="text-xs text-card-desc">on {network} · to {payment.merchantName}</p>
                            </div>
                            {step.kind !== "sending" && (
                                <button type="button" aria-label="Close" className="text-card-desc hover:text-white" onClick={() => setStep({kind: "closed"})}>✕</button>
                            )}
                        </div>

                        {step.kind === "pick" && step.wallets === null && <p className="py-6 text-center text-sm text-card-desc">Looking for wallets…</p>}

                        {step.kind === "pick" && step.wallets !== null && (() => {
                            const {detected, notInstalled} = splitWalletList(family, step.wallets);
                            return (
                                <>
                                    {detected.map((w) => <Row key={w.id} icon={w.icon} name={w.name} tag="Detected" onClick={() => connect(w)} />)}
                                    {detected.length === 0 && appLinks && (
                                        <>
                                            <p className="mb-2 text-xs text-card-desc">Open this checkout in your wallet app:</p>
                                            {openInApp.map((k) => <Row key={k.id} name={`Open in ${k.name}`} href={k.openInApp!(window.location.href)} />)}
                                            {uri && <Row name="Other wallet app" href={uri} />}
                                        </>
                                    )}
                                    {detected.length === 0 && !appLinks && (
                                        <p className="mb-3 text-sm text-card-desc">
                                            No {family === "evm" ? "EVM" : "TRON"} wallet found in this browser. Install one below, or scan the QR code with your phone wallet.
                                        </p>
                                    )}
                                    {!mobile && notInstalled.map((k) => <Row key={k.id} name={k.name} tag="Install ↗" href={k.installUrl} />)}
                                </>
                            );
                        })()}

                        {step.kind === "connecting" && <p className="py-6 text-center text-sm text-card-desc">Approve the connection in {step.wallet.name}…</p>}

                        {step.kind === "confirm" && (
                            <>
                                <dl className="mb-4 space-y-2 rounded-xl border border-[#2a2a3e] bg-[#13131a] p-3 text-sm">
                                    <div className="flex justify-between"><dt className="text-card-desc">Amount</dt><dd className="font-medium">{amountText}</dd></div>
                                    <div className="flex justify-between"><dt className="text-card-desc">Network</dt><dd>{network}</dd></div>
                                    <div className="flex justify-between"><dt className="text-card-desc">To</dt><dd className="font-mono">{short(info.recipientAddress)}</dd></div>
                                    <div className="flex justify-between"><dt className="text-card-desc">From</dt><dd className="font-mono">{short(step.from)} · {step.wallet.name}</dd></div>
                                </dl>
                                <p className="mb-3 text-xs text-card-desc">Your wallet adds a small network fee. Check the amount and network in the wallet before approving.</p>
                                <button type="button" onClick={() => send(step)} className="h-12 w-full rounded-3xl bg-main-green-1 font-medium text-white">
                                    Confirm in {step.wallet.name}
                                </button>
                            </>
                        )}

                        {step.kind === "sending" && <p className="py-6 text-center text-sm text-card-desc">Approve the payment in {step.wallet.name}…</p>}

                        {step.kind === "sent" && (
                            <div className="py-4 text-center">
                                <p className="text-lg font-semibold text-emerald-300">Payment sent</p>
                                <p className="mt-2 text-sm text-card-desc">
                                    This page updates automatically once the network confirms it. Don&apos;t send again.
                                </p>
                                <a className="mt-3 inline-block text-sm text-main-green-1 underline" href={step.url} target="_blank" rel="noopener noreferrer">
                                    View transaction ({short(step.hash)})
                                </a>
                            </div>
                        )}

                        {step.kind === "error" && (
                            <>
                                <p className="mb-4 rounded-md border border-rose-500/50 bg-rose-500/10 p-3 text-sm text-rose-200">{step.text}</p>
                                <button type="button" onClick={openPicker} className="h-11 w-full rounded-3xl border border-[#2a2a3e] text-sm">Try again</button>
                            </>
                        )}
                    </div>
                </div>,
                document.body
            )}
        </div>
    );
};

export default PayWithWallet;
