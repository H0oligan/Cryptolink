import React from "react";
import {Modal, Typography} from "antd";

export const confirmPermanentOwner = (address: string, networkLabel: string, walletName: string): Promise<boolean> =>
    new Promise((resolve) => {
        Modal.confirm({
            title: `Deploy your ${networkLabel} collector`,
            content: (
                <>
                    <p>
                        This wallet will be the <b>permanent owner</b> of the contract:
                    </p>
                    <Typography.Text code copyable>
                        {address}
                    </Typography.Text>
                    <p style={{marginTop: 12}}>
                        Only this address can ever withdraw funds, and it cannot be changed later. Make sure you
                        control its recovery phrase. You pay the network fee from {walletName}.
                    </p>
                </>
            ),
            okText: "I control this wallet, deploy",
            cancelText: "Cancel",
            onOk: () => resolve(true),
            onCancel: () => resolve(false)
        });
    });
