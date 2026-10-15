import type {Address} from "viem";
import type {EvmChainConfig} from "src/constants/merchant-collector";
import {switchEvmChain, deployEvmCollector} from "src/wallet/evm-actions";
import {getReadyTronWeb, deployTronClone} from "src/wallet/tron-actions";
import type {EvmConnection, TronConnection} from "src/wallet/wallet-state";

// The signing address becomes the contract's immutable owner (constructor arg
// on EVM, factory.deploy(owner) on TRON), so the merchant must confirm it
// before anything is sent. Returns null when they decline.
export type ConfirmOwner = (address: string, networkLabel: string, walletName: string) => Promise<boolean>;

export const deployEvmFlow = async (
    conn: EvmConnection,
    chain: EvmChainConfig,
    confirm: ConfirmOwner,
    onStep: (step: "switching" | "deploying") => void
): Promise<Address | null> => {
    if (!(await confirm(conn.address, chain.label, conn.wallet.name))) return null;

    const p = conn.wallet.provider;
    onStep("switching");
    await switchEvmChain(p, chain);
    onStep("deploying");
    return deployEvmCollector(p, conn.address, chain);
};

export const deployTronFlow = async (
    conn: TronConnection,
    getFactoryAddress: () => Promise<string>,
    confirm: ConfirmOwner
): Promise<string | null> => {
    if (!(await confirm(conn.address, "TRON", conn.wallet.name))) return null;

    const factoryAddress = await getFactoryAddress();
    const tronWeb = await getReadyTronWeb(conn.wallet);
    return deployTronClone(tronWeb, factoryAddress, conn.address);
};
