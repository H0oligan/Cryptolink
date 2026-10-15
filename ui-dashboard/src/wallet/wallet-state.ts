import type {Address} from "viem";
import type {DetectedWallet, WalletFamily} from "src/wallet/types";

export interface EvmConnection {
    wallet: DetectedWallet;
    address: Address;
}

export interface TronConnection {
    wallet: DetectedWallet;
    address: string;
}

export interface WalletState {
    evm: EvmConnection | null;
    tron: TronConnection | null;
}

export type WalletAction =
    | {type: "connected"; family: WalletFamily; conn: {wallet: DetectedWallet; address: string}}
    | {type: "accountsChanged"; family: WalletFamily; address: string | null}
    | {type: "disconnect"; family: WalletFamily};

export const initialWalletState: WalletState = {evm: null, tron: null};

export const walletReducer = (state: WalletState, action: WalletAction): WalletState => {
    switch (action.type) {
        case "connected":
            return {...state, [action.family]: action.conn};
        case "accountsChanged": {
            const current = state[action.family];
            if (!current) return state;
            // An empty account list means the wallet locked or revoked access.
            return {...state, [action.family]: action.address ? {...current, address: action.address} : null};
        }
        case "disconnect":
            return {...state, [action.family]: null};
        default:
            return state;
    }
};
