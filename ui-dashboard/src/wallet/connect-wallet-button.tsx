import React from "react";
import {Button} from "antd";
import {WalletOutlined} from "@ant-design/icons";
import {useWallet} from "src/wallet/wallet-context";
import {shortAddr} from "src/wallet/owner";

const ConnectWalletButton: React.FC<{compact?: boolean}> = ({compact}) => {
    const wallet = useWallet();
    const conns = [wallet.evm, wallet.tron].filter(Boolean) as {wallet: {id: string; name: string; icon?: string}; address: string}[];

    if (conns.length === 0) {
        return (
            <Button icon={<WalletOutlined />} onClick={() => wallet.openModal()}>
                {compact ? null : "Connect Wallet"}
            </Button>
        );
    }

    return (
        <Button onClick={() => wallet.openModal()} title={conns.map((c) => `${c.wallet.name}: ${c.address}`).join("\n")}>
            <span style={{display: "inline-flex", alignItems: "center", gap: 6}}>
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
    );
};

export default ConnectWalletButton;
