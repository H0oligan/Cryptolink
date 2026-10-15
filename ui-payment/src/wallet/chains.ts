// Mainnet chains the checkout can pay on with a browser/app wallet.
// Keep in sync with internal/service/blockchain/currencies.json. A chain ID
// missing here (e.g. a testnet) means "QR only", never a guessed network.
export interface EvmChain {
    name: string;
    nativeSymbol: string;
    rpcUrl: string;
    explorer: string;
}

export const EVM_CHAINS: Record<number, EvmChain> = {
    1: {name: "Ethereum", nativeSymbol: "ETH", rpcUrl: "https://eth.llamarpc.com", explorer: "https://etherscan.io"},
    56: {name: "BNB Smart Chain", nativeSymbol: "BNB", rpcUrl: "https://bsc-dataseed.binance.org", explorer: "https://bscscan.com"},
    137: {name: "Polygon", nativeSymbol: "MATIC", rpcUrl: "https://polygon-rpc.com", explorer: "https://polygonscan.com"},
    42161: {name: "Arbitrum One", nativeSymbol: "ETH", rpcUrl: "https://arb1.arbitrum.io/rpc", explorer: "https://arbiscan.io"},
    43114: {name: "Avalanche", nativeSymbol: "AVAX", rpcUrl: "https://api.avax.network/ext/bc/C/rpc", explorer: "https://snowtrace.io"}
};

// ticker -> on-chain decimals (currencies.json)
export const CURRENCY_DECIMALS: Record<string, number> = {
    ETH: 18, ETH_USDT: 6, ETH_USDC: 6,
    MATIC: 18, MATIC_USDT: 6, MATIC_USDC: 6,
    BNB: 18, BSC_USDT: 18,
    ARB: 18, ARBITRUM_USDT: 6, ARBITRUM_USDC: 6,
    AVAX: 18, AVAX_USDT: 6, AVAX_USDC: 6,
    TRON: 6, TRON_USDT: 6
};

export const TRON_USDT_CONTRACT = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t";
export const TRON_EXPLORER = "https://tronscan.org/#/transaction/";
