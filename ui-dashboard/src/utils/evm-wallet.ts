import {
    createPublicClient,
    custom,
    encodeDeployData,
    encodeFunctionData,
    type Hash,
    type Address,
    type Chain,
} from "viem";
import {
    MERCHANT_COLLECTOR_ABI,
    MERCHANT_COLLECTOR_BYTECODE,
    type EvmChainConfig,
} from "src/constants/merchant-collector";

declare global {
    interface Window {
        ethereum?: any;
    }
}

// ---------------------------------------------------------------
// EIP-6963 — Multi Injected Provider Discovery
// Specifically targets MetaMask by RDNS, ignoring other wallets.
// ---------------------------------------------------------------

const METAMASK_RDNS = "io.metamask";

const getMetaMaskProvider = (): Promise<any> => {
    return new Promise((resolve, reject) => {
        if (typeof window === "undefined") {
            reject(new Error("No browser environment."));
            return;
        }

        const found: any[] = [];

        const handler = (event: Event) => {
            const detail = (event as CustomEvent).detail;
            if (detail?.info?.rdns === METAMASK_RDNS) {
                found.push(detail.provider);
            }
        };

        window.addEventListener("eip6963:announceProvider", handler);
        window.dispatchEvent(new Event("eip6963:requestProvider"));

        setTimeout(() => {
            window.removeEventListener("eip6963:announceProvider", handler);

            if (found.length > 0) { resolve(found[0]); return; }

            // Fallback 1: providers array (multiple wallets injected under window.ethereum.providers)
            // Prefer provider with _metamask (MetaMask-specific), falling back to isMetaMask only.
            if (window.ethereum?.providers) {
                const providers = window.ethereum.providers as any[];
                const mm = providers.find((p) => p._metamask) ?? providers.find((p) => p.isMetaMask && !p.isAtomicWallet);
                if (mm) { resolve(mm); return; }
            }

            // Fallback 2: single injected wallet — must have _metamask to avoid Atomic Wallet etc.
            if (window.ethereum?.isMetaMask && window.ethereum?._metamask) {
                resolve(window.ethereum);
                return;
            }

            reject(new Error(
                "MetaMask not found. Please install MetaMask (metamask.io) and try again, or disable conflicting wallet extensions."
            ));
        }, 500);
    });
};

// ---------------------------------------------------------------
// Build a viem Chain object from our EvmChainConfig
// ---------------------------------------------------------------

const buildChain = (config: EvmChainConfig): Chain => ({
    id: config.chainId,
    name: config.label,
    nativeCurrency: {
        name: config.nativeTicker,
        symbol: config.nativeTicker,
        decimals: 18,
    },
    rpcUrls: {
        default: { http: [config.rpcUrl] },
        public:  { http: [config.rpcUrl] },
    },
});

// ---------------------------------------------------------------
// Synchronous availability check for UI
// ---------------------------------------------------------------

export const isMetaMaskAvailable = (): boolean => {
    if (typeof window === "undefined") return false;
    if (window.ethereum?.providers?.some((p: any) => p.isMetaMask)) return true;
    if (window.ethereum?.isMetaMask) return true;
    return false;
};

// ---------------------------------------------------------------
// Public API — all functions receive the chain config explicitly
// ---------------------------------------------------------------

export const connectWallet = async (): Promise<Address> => {
    const provider = await getMetaMaskProvider();
    const accounts: string[] = await provider.request({ method: "eth_requestAccounts" });
    if (!accounts || accounts.length === 0) throw new Error("No accounts returned from MetaMask.");
    return accounts[0] as Address;
};

export const switchChain = async (config: EvmChainConfig): Promise<void> => {
    const provider = await getMetaMaskProvider();
    const hex = `0x${config.chainId.toString(16)}`;
    try {
        await provider.request({ method: "wallet_switchEthereumChain", params: [{ chainId: hex }] });
    } catch (err: any) {
        if (err.code === 4902) {
            await provider.request({
                method: "wallet_addEthereumChain",
                params: [{
                    chainId: hex,
                    chainName: config.label,
                    rpcUrls: [config.rpcUrl],
                    nativeCurrency: {
                        name: config.nativeTicker,
                        symbol: config.nativeTicker,
                        decimals: 18,
                    },
                }],
            });
        } else {
            throw err;
        }
    }
};

export const deployCollector = async (
    ownerAddress: Address,
    chainConfig: EvmChainConfig
): Promise<Address> => {
    const provider = await getMetaMaskProvider();
    const chain = buildChain(chainConfig);

    // Encode deployment data with viem, then send via provider directly.
    // This bypasses viem's JSON-RPC account chain validation which fails with custom() transport.
    const data = encodeDeployData({
        abi: MERCHANT_COLLECTOR_ABI,
        bytecode: MERCHANT_COLLECTOR_BYTECODE as `0x${string}`,
        args: [ownerAddress],
    } as any);

    const hash: Hash = await provider.request({
        method: "eth_sendTransaction",
        params: [{ from: ownerAddress, data }],
    });

    const publicClient = createPublicClient({ chain, transport: custom(provider) });
    const receipt = await publicClient.waitForTransactionReceipt({ hash });

    if (!receipt.contractAddress) {
        throw new Error("Contract deployment failed — no contract address in receipt.");
    }

    return receipt.contractAddress;
};

export const withdrawAll = async (
    contractAddress: Address,
    tokenAddresses: Address[],
    chainConfig: EvmChainConfig
): Promise<Hash> => {
    const provider = await getMetaMaskProvider();

    const accounts: string[] = await provider.request({ method: "eth_accounts" });
    if (!accounts || accounts.length === 0) throw new Error("No wallet connected.");
    const account = accounts[0] as Address;

    // Encode function call with viem, then send via provider directly.
    const data = encodeFunctionData({
        abi: MERCHANT_COLLECTOR_ABI,
        functionName: "withdrawAll",
        args: [tokenAddresses],
    } as any);

    return provider.request({
        method: "eth_sendTransaction",
        params: [{ from: account, to: contractAddress, data }],
    });
};
