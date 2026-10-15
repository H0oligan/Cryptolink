import React from "react";
import {Button, Tooltip} from "antd";
import {WalletOutlined} from "@ant-design/icons";
import {useWallet} from "src/wallet/wallet-context";
import {shortAddr} from "src/wallet/owner";

const dot: React.CSSProperties = {
    width: 8,
    height: 8,
    borderRadius: "50%",
    background: "var(--cl-accent-success)",
    boxShadow: "0 0 6px var(--cl-accent-success)",
    flex: "none"
};

// Header corner button. Disconnected: accent call-to-action. Connected: wallet
// icon(s), live dot and short address, like Uniswap's account pill.
const ConnectWalletButton: React.FC<{compact?: boolean}> = ({compact}) => {
    const wallet = useWallet();
    const conns = [wallet.evm, wallet.tron].filter(Boolean) as {wallet: {id: string; name: string; icon?: string}; address: string}[];

    if (conns.length === 0) {
        return (
            <Button type="primary" icon={<WalletOutlined />} onClick={() => wallet.openModal()} aria-label="Connect Wallet">
                {compact ? null : "Connect Wallet"}
            </Button>
        );
    }

    return (
        <Tooltip title={conns.map((c) => `${c.wallet.name}: ${c.address}`).join(" · ")}>
            <Button onClick={() => wallet.openModal()} aria-label="Wallet">
                <span style={{display: "inline-flex", alignItems: "center", gap: 8}}>
                    <span style={dot} />
                    {conns.map((c) =>
                        c.wallet.icon ? (
                            <img key={c.wallet.id} src={c.wallet.icon} alt="" style={{width: 18, height: 18, borderRadius: 4}} />
                        ) : (
                            <WalletOutlined key={c.wallet.id} />
                        )
                    )}
                    {compact ? null : <span style={{fontFamily: "var(--cl-monospace)"}}>{shortAddr(conns[0].address)}</span>}
                </span>
            </Button>
        </Tooltip>
    );
};

export default ConnectWalletButton;
