import "./connect-wallet-modal.scss";

import React from "react";
import {Modal, Button, Spin, Tag, Typography, Alert} from "antd";
import {LinkOutlined} from "@ant-design/icons";
import {useWallet} from "src/wallet/wallet-context";
import {discoverEvmWallets} from "src/wallet/evm-discovery";
import {discoverTronWallets} from "src/wallet/tron-discovery";
import {splitWalletList} from "src/wallet/wallet-list";
import {describeWalletError} from "src/wallet/errors";
import {shortAddr} from "src/wallet/owner";
import {isMobileDevice} from "src/wallet/env";
import type {DetectedWallet, WalletFamily} from "src/wallet/types";

const {Text} = Typography;

const FAMILY_LABEL: Record<WalletFamily, string> = {
    evm: "Ethereum · BNB · Polygon · Arbitrum · Avalanche",
    tron: "TRON"
};

const WalletIcon: React.FC<{src?: string; name: string}> = ({src, name}) =>
    src ? (
        <img className="connect-wallet-modal__icon" src={src} alt="" />
    ) : (
        <span className="connect-wallet-modal__icon connect-wallet-modal__icon_letter">{name.charAt(0)}</span>
    );

interface Props {
    // Extra content for the "not installed" area (phone handoff); Task 10.
    renderHandoff?: (family: WalletFamily | null) => React.ReactNode;
}

const ConnectWalletModal: React.FC<Props> = ({renderHandoff}) => {
    const wallet = useWallet();
    const [found, setFound] = React.useState<DetectedWallet[] | null>(null);
    const [connectingId, setConnectingId] = React.useState<string | null>(null);
    const [error, setError] = React.useState<{text: string; rejected: boolean} | null>(null);
    const [narrow, setNarrow] = React.useState(() => window.innerWidth < 768);
    const mobile = isMobileDevice();

    React.useEffect(() => {
        const onResize = () => setNarrow(window.innerWidth < 768);
        window.addEventListener("resize", onResize);
        return () => window.removeEventListener("resize", onResize);
    }, []);

    React.useEffect(() => {
        if (!wallet.modal.open) return;
        let cancelled = false;
        setFound(null);
        setError(null);
        Promise.all([discoverEvmWallets(), discoverTronWallets()]).then(([evm, tron]) => {
            if (!cancelled) setFound([...evm, ...tron]);
        });
        return () => {
            cancelled = true;
        };
    }, [wallet.modal.open]);

    const onConnect = async (w: DetectedWallet) => {
        setConnectingId(w.id);
        setError(null);
        try {
            await wallet.connect(w);
        } catch (err) {
            const d = describeWalletError(err);
            setError({text: d.description, rejected: d.rejected});
        } finally {
            setConnectingId(null);
        }
    };

    const families: WalletFamily[] = wallet.modal.family ? [wallet.modal.family] : ["evm", "tron"];

    const renderFamily = (family: WalletFamily) => {
        const conn = wallet[family];
        const {detected, notInstalled} = splitWalletList(family, found ?? []);

        return (
            <section key={family} className="connect-wallet-modal__section">
                <div className="connect-wallet-modal__section-title">{FAMILY_LABEL[family]}</div>

                {conn && (
                    <div className="connect-wallet-modal__connected">
                        <WalletIcon src={conn.wallet.icon} name={conn.wallet.name} />
                        <div className="connect-wallet-modal__grow">
                            <div>{conn.wallet.name}</div>
                            <Text type="secondary" className="connect-wallet-modal__addr">
                                {shortAddr(conn.address)}
                            </Text>
                        </div>
                        <Button size="small" onClick={() => wallet.disconnect(family)}>
                            Disconnect
                        </Button>
                    </div>
                )}

                {detected.map((w) => (
                    <button
                        key={w.id}
                        type="button"
                        className="connect-wallet-modal__row"
                        disabled={connectingId !== null}
                        onClick={() => onConnect(w)}
                    >
                        <WalletIcon src={w.icon} name={w.name} />
                        <span className="connect-wallet-modal__grow">
                            {connectingId === w.id ? `Approve in ${w.name}…` : w.name}
                        </span>
                        {connectingId === w.id ? <Spin size="small" /> : <Tag color="success">Detected</Tag>}
                    </button>
                ))}

                {detected.length === 0 && (
                    <Text type="secondary" className="connect-wallet-modal__empty">
                        No {family === "evm" ? "EVM" : "TRON"} wallet found in this browser.
                    </Text>
                )}

                {!mobile &&
                    notInstalled.map((k) => (
                        <a
                            key={k.id}
                            className="connect-wallet-modal__row connect-wallet-modal__row_muted"
                            href={k.installUrl}
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            <WalletIcon name={k.name} />
                            <span className="connect-wallet-modal__grow">{k.name}</span>
                            <span className="connect-wallet-modal__install">
                                Install <LinkOutlined />
                            </span>
                        </a>
                    ))}
            </section>
        );
    };

    return (
        <Modal
            open={wallet.modal.open}
            onCancel={wallet.closeModal}
            footer={null}
            title="Connect a wallet"
            width={narrow ? "100%" : 440}
            className={narrow ? "connect-wallet-modal connect-wallet-modal--sheet" : "connect-wallet-modal"}
            destroyOnHidden
        >
            {error && (
                <Alert
                    className="connect-wallet-modal__error"
                    type={error.rejected ? "info" : "error"}
                    showIcon
                    message={error.text}
                />
            )}
            {found === null ? (
                <div className="connect-wallet-modal__loading">
                    <Spin />
                </div>
            ) : (
                <>
                    {families.map(renderFamily)}
                    {renderHandoff?.(wallet.modal.family)}
                </>
            )}
            <Text type="secondary" className="connect-wallet-modal__note">
                Your keys never leave your wallet. CryptoLink only asks it to sign the transactions you approve.
            </Text>
        </Modal>
    );
};

export default ConnectWalletModal;
