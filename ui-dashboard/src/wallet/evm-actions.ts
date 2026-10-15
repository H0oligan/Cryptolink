import {createPublicClient, custom, decodeFunctionResult, encodeDeployData, encodeFunctionData, type Address, type Chain, type Hash} from "viem";
import {MERCHANT_COLLECTOR_ABI, MERCHANT_COLLECTOR_BYTECODE, type EvmChainConfig} from "src/constants/merchant-collector";
import type {Eip1193Provider} from "src/wallet/types";

const buildChain = (c: EvmChainConfig): Chain => ({
    id: c.chainId,
    name: c.label,
    nativeCurrency: {name: c.nativeTicker, symbol: c.nativeTicker, decimals: 18},
    rpcUrls: {default: {http: [c.rpcUrl]}, public: {http: [c.rpcUrl]}}
});

export const requestEvmAccount = async (p: Eip1193Provider): Promise<Address> => {
    const accounts: string[] = await p.request({method: "eth_requestAccounts"});
    if (!accounts?.length) throw new Error("The wallet returned no account. Unlock it and try again.");
    return accounts[0] as Address;
};

export const switchEvmChain = async (p: Eip1193Provider, c: EvmChainConfig): Promise<void> => {
    const chainId = `0x${c.chainId.toString(16)}`;
    try {
        await p.request({method: "wallet_switchEthereumChain", params: [{chainId}]});
    } catch (err: any) {
        if (err?.code !== 4902) throw err;
        await p.request({
            method: "wallet_addEthereumChain",
            params: [{chainId, chainName: c.label, rpcUrls: [c.rpcUrl], nativeCurrency: {name: c.nativeTicker, symbol: c.nativeTicker, decimals: 18}}]
        });
    }
};

// Call after switchEvmChain: eth_call goes to the wallet's current network.
export const readEvmOwner = async (p: Eip1193Provider, contract: Address): Promise<Address> => {
    const data = encodeFunctionData({abi: MERCHANT_COLLECTOR_ABI, functionName: "owner"} as any);
    const raw = await p.request({method: "eth_call", params: [{to: contract, data}, "latest"]});
    return decodeFunctionResult({abi: MERCHANT_COLLECTOR_ABI, functionName: "owner", data: raw} as any) as Address;
};

export const deployEvmCollector = async (p: Eip1193Provider, owner: Address, c: EvmChainConfig): Promise<Address> => {
    const data = encodeDeployData({abi: MERCHANT_COLLECTOR_ABI, bytecode: MERCHANT_COLLECTOR_BYTECODE as `0x${string}`, args: [owner]} as any);
    const hash: Hash = await p.request({method: "eth_sendTransaction", params: [{from: owner, data}]});
    const receipt = await createPublicClient({chain: buildChain(c), transport: custom(p)}).waitForTransactionReceipt({hash});
    if (!receipt.contractAddress) throw new Error("Contract deployment failed: no contract address in receipt.");
    return receipt.contractAddress;
};

export const withdrawAllEvm = (p: Eip1193Provider, from: Address, contract: Address, tokens: Address[]): Promise<Hash> => {
    const data = encodeFunctionData({abi: MERCHANT_COLLECTOR_ABI, functionName: "withdrawAll", args: [tokens]} as any);
    return p.request({method: "eth_sendTransaction", params: [{from, to: contract, data}]});
};
