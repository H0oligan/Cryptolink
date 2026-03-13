import {
    IMPLEMENTATION_BYTECODE_TRON,
    FACTORY_BYTECODE_TRON,
    FACTORY_ABI,
    MERCHANT_COLLECTOR_V2_ABI,
} from "src/constants/merchant-collector";

declare global {
    interface Window {
        tronWeb?: any;
        tronLink?: any;
    }
}

// ---------------------------------------------------------------
// TronLink detection — only TronLink, not Atomic Wallet etc.
// ---------------------------------------------------------------

const getTronLink = (): any | null => {
    if (typeof window === "undefined") return null;
    if (window.tronLink) return window.tronLink;
    return null;
};

const getTronWeb = (): any | null => {
    const tl = getTronLink();
    if (!tl) return null;
    return tl.tronWeb || window.tronWeb || null;
};

export const isTronLinkAvailable = (): boolean => {
    return getTronLink() !== null;
};

// ---------------------------------------------------------------
// Connect wallet via TronLink
// ---------------------------------------------------------------

export const connectTronWallet = async (): Promise<string> => {
    const tronLink = getTronLink();
    if (!tronLink) {
        throw new Error(
            "TronLink not found. Please install the TronLink browser extension (tronlink.org) and try again. " +
            "Other TRON wallets (Atomic, etc.) are not supported."
        );
    }

    // TronLink v4+ uses request API
    if (tronLink.request) {
        const res = await tronLink.request({method: "tron_requestAccounts"});
        if (res?.code === 4001) {
            throw new Error("You rejected the TronLink connection request.");
        }
    }

    // Wait briefly for tronWeb to be ready after connection
    for (let i = 0; i < 10; i++) {
        const tw = getTronWeb();
        if (tw?.ready && tw?.defaultAddress?.base58) {
            return tw.defaultAddress.base58;
        }
        await new Promise((r) => setTimeout(r, 500));
    }

    throw new Error(
        "TronLink connected but no account found. Please unlock TronLink and select a TRON account."
    );
};

// ---------------------------------------------------------------
// Helper: deploy a contract on TRON and wait for confirmation
// ---------------------------------------------------------------

const ensureTronWebReady = async (): Promise<any> => {
    let tw = getTronWeb();
    if (tw?.ready && tw?.defaultAddress?.base58) return tw;

    // TronLink may have auto-locked during the previous deploy wait.
    // Re-request accounts to wake it up.
    const tl = getTronLink();
    if (tl?.request) {
        await tl.request({method: "tron_requestAccounts"});
    }

    for (let i = 0; i < 15; i++) {
        tw = getTronWeb();
        if (tw?.ready && tw?.defaultAddress?.base58) return tw;
        await new Promise((r) => setTimeout(r, 1000));
    }

    throw new Error("TronLink is not ready. Please unlock TronLink and try again.");
};

const deployContractOnTron = async (
    abi: readonly any[],
    bytecode: string,
    parameters: any[],
    feeLimit: number = 1_000_000_000
): Promise<{contractAddress: string; txid: string}> => {
    const tronWeb = await ensureTronWebReady();

    const tx = await tronWeb.transactionBuilder.createSmartContract(
        {
            abi,
            bytecode,
            feeLimit,
            callValue: 0,
            parameters,
            ownerAddress: tronWeb.defaultAddress.hex,
        },
        tronWeb.defaultAddress.hex
    );

    const signedTx = await tronWeb.trx.sign(tx);

    const result = await tronWeb.trx.sendRawTransaction(signedTx);
    if (!result?.result && !result?.txid) {
        throw new Error("Failed to broadcast TRON transaction. Please try again.");
    }

    const contractHex =
        tx.contract_address ||
        tx.raw_data?.contract?.[0]?.parameter?.value?.new_contract?.contract_address;

    if (!contractHex) {
        throw new Error("Contract address not found in transaction. Check TronScan for tx: " + (result.txid || ""));
    }

    const contractBase58 = tronWeb.address.fromHex(contractHex);
    const txid = result.txid || signedTx.txID;

    // Wait for confirmation (poll up to ~90s)
    for (let i = 0; i < 30; i++) {
        await new Promise((r) => setTimeout(r, 3000));
        try {
            const info = await tronWeb.trx.getTransactionInfo(txid);
            if (info?.id) {
                if (info.receipt?.result === "SUCCESS" || info.receipt?.result === "success") {
                    return {contractAddress: contractBase58, txid};
                }
                if (info.receipt?.result && info.receipt.result !== "SUCCESS") {
                    throw new Error(`Contract deployment failed on-chain: ${info.receipt.result}. Tx: ${txid}`);
                }
            }
        } catch (e: any) {
            if (e.message?.includes("failed on-chain")) throw e;
        }
    }

    throw new Error(`Transaction confirmation timed out. Check TronScan for tx: ${txid}`);
};

// ---------------------------------------------------------------
// Admin: Deploy Implementation contract (MerchantCollectorV2)
// ---------------------------------------------------------------

export const deployTronImplementation = async (): Promise<string> => {
    // Implementation has no constructor args (initialize() is called per-clone)
    const {contractAddress} = await deployContractOnTron(
        MERCHANT_COLLECTOR_V2_ABI,
        IMPLEMENTATION_BYTECODE_TRON,
        []
    );
    return contractAddress;
};

// ---------------------------------------------------------------
// Admin: Deploy Factory contract (CryptoLinkCloneFactory)
// ---------------------------------------------------------------

export const deployTronFactory = async (implementationAddress: string): Promise<string> => {
    const {contractAddress} = await deployContractOnTron(
        FACTORY_ABI,
        FACTORY_BYTECODE_TRON,
        [implementationAddress]
    );
    return contractAddress;
};

// ---------------------------------------------------------------
// Merchant: Deploy clone via Factory (cheap ~$0.50-1)
// ---------------------------------------------------------------

export const deployTronCloneViaFactory = async (
    factoryAddress: string,
    ownerAddress: string
): Promise<string> => {
    const tronWeb = await ensureTronWebReady();

    const factory = await tronWeb.contract(FACTORY_ABI, factoryAddress);
    const txResult = await factory.deploy(ownerAddress).send({
        feeLimit: 300_000_000, // 300 TRX max (clones are cheap)
    });

    // txResult is the txid string
    const txid = typeof txResult === "string" ? txResult : txResult?.txid || txResult;

    // Wait for confirmation and extract clone address from event log
    for (let i = 0; i < 30; i++) {
        await new Promise((r) => setTimeout(r, 3000));
        try {
            const info = await tronWeb.trx.getTransactionInfo(txid);
            if (info?.id) {
                if (info.receipt?.result === "SUCCESS" || info.receipt?.result === "success") {
                    // Extract clone address from CloneCreated event log
                    if (info.log && info.log.length > 0) {
                        // CloneCreated(address indexed owner, address clone)
                        // The clone address is in the data field (non-indexed param)
                        const logData = info.log[0].data;
                        if (logData) {
                            // data is hex-encoded address (32 bytes, last 20 bytes = address)
                            const cloneHex = "41" + logData.slice(24); // 41-prefix for TRON mainnet
                            return tronWeb.address.fromHex(cloneHex);
                        }
                    }
                    // Fallback: check contract_address in info
                    if (info.contract_address) {
                        return tronWeb.address.fromHex(info.contract_address);
                    }
                    throw new Error("Clone deployed but address not found in event logs. Tx: " + txid);
                }
                if (info.receipt?.result && info.receipt.result !== "SUCCESS") {
                    throw new Error(`Clone deployment failed on-chain: ${info.receipt.result}. Tx: ${txid}`);
                }
            }
        } catch (e: any) {
            if (e.message?.includes("failed on-chain") || e.message?.includes("not found in event")) throw e;
        }
    }

    throw new Error(`Transaction confirmation timed out. Check TronScan for tx: ${txid}`);
};

// ---------------------------------------------------------------
// Call withdrawAll() on a deployed TRON MerchantCollectorV2 clone
// ---------------------------------------------------------------

export const withdrawAllTron = async (
    contractBase58: string,
    tokenAddresses: string[]
): Promise<string> => {
    const tronWeb = await ensureTronWebReady();

    // Convert base58 token addresses to hex — TronWeb's ABI encoder for
    // address[] parameters requires the 41-prefixed hex form, not base58.
    // Passing base58 directly produces garbled calldata and zero balances.
    const hexAddresses = tokenAddresses.map((addr) => tronWeb.address.toHex(addr));

    const contract = await tronWeb.contract(MERCHANT_COLLECTOR_V2_ABI, contractBase58);
    const txid = await contract.withdrawAll(hexAddresses).send({
        feeLimit: 500_000_000, // 500 TRX
    });

    return txid;
};
