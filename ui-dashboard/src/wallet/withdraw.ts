import type {Address, Hash} from "viem";
import type {EvmChainConfig} from "src/constants/merchant-collector";
import {switchEvmChain, readEvmOwner, withdrawAllEvm} from "src/wallet/evm-actions";
import {getReadyTronWeb, readTronOwner, withdrawAllTronWith} from "src/wallet/tron-actions";
import {assertOwner} from "src/wallet/owner";
import type {EvmConnection, TronConnection} from "src/wallet/wallet-state";

// Switch first: eth_call follows the wallet's current network, so the owner
// read must happen on the collector's chain. A non-owner never gets a
// transaction prompt (it would only revert and burn gas).
export const withdrawEvmFlow = async (
    conn: EvmConnection,
    chain: EvmChainConfig,
    contract: Address,
    tokens: Address[]
): Promise<Hash> => {
    const p = conn.wallet.provider;
    await switchEvmChain(p, chain);
    assertOwner("evm", await readEvmOwner(p, contract), conn.address);
    return withdrawAllEvm(p, conn.address, contract, tokens);
};

export const withdrawTronFlow = async (conn: TronConnection, contractBase58: string, tokens: string[]): Promise<string> => {
    const tronWeb = await getReadyTronWeb(conn.wallet);
    assertOwner("tron", await readTronOwner(tronWeb, contractBase58), conn.address);
    return withdrawAllTronWith(tronWeb, contractBase58, tokens);
};
