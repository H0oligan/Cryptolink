import {
    IMPLEMENTATION_BYTECODE_TRON,
    FACTORY_BYTECODE_TRON,
    FACTORY_ABI,
    MERCHANT_COLLECTOR_V2_ABI,
} from "src/constants/merchant-collector";
import type {DetectedWallet} from "src/wallet/types";

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

// TronLink keeps tronWeb on window.tronWeb; OKX/Bitget/Trust hang it off their own object.
const tronWebOf = (w: DetectedWallet): any =>
    w.provider?.tronWeb || (w.id === "legacy:tronlink" ? (window as any).tronWeb : null);

export const getReadyTronWeb = async (w: DetectedWallet): Promise<any> => {
    let tw = tronWebOf(w);
    if (tw?.ready && tw?.defaultAddress?.base58) return tw;
    // Wallets auto-lock during long deploy waits; re-request to wake them.
    if (w.provider?.request) await w.provider.request({method: "tron_requestAccounts"});
    for (let i = 0; i < 15; i++) {
        tw = tronWebOf(w);
        if (tw?.ready && tw?.defaultAddress?.base58) return tw;
        await sleep(1000);
    }
    throw new Error(`${w.name} is not ready. Unlock it, select a TRON account and try again.`);
};

export const connectTron = async (w: DetectedWallet): Promise<string> => {
    if (w.provider?.request) {
        const res = await w.provider.request({method: "tron_requestAccounts"});
        if (res?.code === 4001) throw {code: 4001, message: `You rejected the ${w.name} connection request.`};
    }
    return (await getReadyTronWeb(w)).defaultAddress.base58;
};

export const readTronOwner = async (tronWeb: any, contractBase58: string): Promise<string> => {
    const c = await tronWeb.contract(MERCHANT_COLLECTOR_V2_ABI, contractBase58);
    const hex = await c.owner().call();
    return tronWeb.address.fromHex(hex);
};

// ---------------------------------------------------------------
// Helper: deploy a contract on TRON and wait for confirmation
// ---------------------------------------------------------------

const deployContractOnTron = async (
    tronWeb: any,
    abi: readonly any[],
    bytecode: string,
    parameters: any[],
    feeLimit: number = 1_000_000_000
): Promise<{contractAddress: string; txid: string}> => {
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

export const deployTronImplementation = async (tronWeb: any): Promise<string> => {
    // Implementation has no constructor args (initialize() is called per-clone)
    const {contractAddress} = await deployContractOnTron(
        tronWeb,
        MERCHANT_COLLECTOR_V2_ABI,
        IMPLEMENTATION_BYTECODE_TRON,
        []
    );
    return contractAddress;
};

// ---------------------------------------------------------------
// Admin: Deploy Factory contract (CryptoLinkCloneFactory)
// ---------------------------------------------------------------

export const deployTronFactory = async (tronWeb: any, implementationAddress: string): Promise<string> => {
    const {contractAddress} = await deployContractOnTron(
        tronWeb,
        FACTORY_ABI,
        FACTORY_BYTECODE_TRON,
        [implementationAddress]
    );
    return contractAddress;
};

// ---------------------------------------------------------------
// Merchant: Deploy clone via Factory (cheap ~$0.50-1)
// ---------------------------------------------------------------

export const deployTronClone = async (
    tronWeb: any,
    factoryAddress: string,
    ownerAddress: string
): Promise<string> => {
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

export const withdrawAllTronWith = async (
    tronWeb: any,
    contractBase58: string,
    tokenAddresses: string[]
): Promise<string> => {
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
