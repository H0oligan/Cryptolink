import React from "react";
import type {Address} from "viem";
import localStorage from "src/utils/local-storage";
import {discoverEvmWallets} from "src/wallet/evm-discovery";
import {discoverTronWallets} from "src/wallet/tron-discovery";
import {requestEvmAccount} from "src/wallet/evm-actions";
import {connectTron} from "src/wallet/tron-actions";
import type {DetectedWallet, WalletFamily} from "src/wallet/types";
import {
    walletReducer,
    initialWalletState,
    type EvmConnection,
    type TronConnection,
    type WalletState
} from "src/wallet/wallet-state";

export {walletReducer, initialWalletState};
export type {EvmConnection, TronConnection};

const HANDOFF_ACTIVE_MS = 10 * 60 * 1000;
const lastWalletKey = (family: WalletFamily) => `wallet:last:${family}`;

interface Pending {
    resolve: (conn: any) => void;
    reject: (err: unknown) => void;
}

export interface WalletContextValue extends WalletState {
    modal: {open: boolean; family: WalletFamily | null};
    handoffEnabled: boolean;
    openModal(family?: WalletFamily): void;
    closeModal(): void;
    connect(wallet: DetectedWallet): Promise<void>;
    disconnect(family: WalletFamily): void;
    // Resolve the connected wallet, opening the picker first if needed.
    // Rejects with {code: 4001} when the user closes the picker.
    ensureEvm(): Promise<EvmConnection>;
    ensureTron(): Promise<TronConnection>;
    handoffActiveUntil: number | null;
    markHandoffActive(): void;
}

const WalletContext = React.createContext<WalletContextValue | null>(null);

export const WalletProvider: React.FC<{handoffEnabled: boolean; children: React.ReactNode}> = ({
    handoffEnabled,
    children
}) => {
    const [state, dispatch] = React.useReducer(walletReducer, initialWalletState);
    const [modal, setModal] = React.useState<{open: boolean; family: WalletFamily | null}>({open: false, family: null});
    const [handoffActiveUntil, setHandoffActiveUntil] = React.useState<number | null>(null);

    const stateRef = React.useRef(state);
    stateRef.current = state;
    const pending = React.useRef<Record<WalletFamily, Pending[]>>({evm: [], tron: []});
    const unsubscribe = React.useRef<Partial<Record<WalletFamily, () => void>>>({});

    const settle = (family: WalletFamily, ok: boolean, value: unknown) => {
        const waiters = pending.current[family];
        pending.current[family] = [];
        waiters.forEach((p) => (ok ? p.resolve(value) : p.reject(value)));
    };

    const watchEvm = (wallet: DetectedWallet) => {
        unsubscribe.current.evm?.();
        const p = wallet.provider;
        if (!p?.on) return;
        const onAccounts = (accounts: string[]) =>
            dispatch({type: "accountsChanged", family: "evm", address: accounts?.[0] ?? null});
        const onDisconnect = () => dispatch({type: "disconnect", family: "evm"});
        p.on("accountsChanged", onAccounts);
        p.on("disconnect", onDisconnect);
        unsubscribe.current.evm = () => {
            p.removeListener?.("accountsChanged", onAccounts);
            p.removeListener?.("disconnect", onDisconnect);
        };
    };

    const finishConnect = (wallet: DetectedWallet, address: string) => {
        const conn = {wallet, address};
        dispatch({type: "connected", family: wallet.family, conn});
        localStorage.set(lastWalletKey(wallet.family), wallet.id);
        settle(wallet.family, true, conn);
    };

    const connect = async (wallet: DetectedWallet) => {
        if (wallet.family === "evm") {
            const address = await requestEvmAccount(wallet.provider);
            watchEvm(wallet);
            finishConnect(wallet, address);
        } else {
            finishConnect(wallet, await connectTron(wallet));
        }
        setModal({open: false, family: null});
    };

    const disconnect = (family: WalletFamily) => {
        unsubscribe.current[family]?.();
        delete unsubscribe.current[family];
        dispatch({type: "disconnect", family});
        localStorage.remove(lastWalletKey(family));
    };

    const openModal = (family?: WalletFamily) => setModal({open: true, family: family ?? null});

    const closeModal = () => {
        setModal({open: false, family: null});
        const cancelled = {code: 4001, message: "Wallet connection cancelled."};
        settle("evm", false, cancelled);
        settle("tron", false, cancelled);
    };

    const ensure = <T,>(family: WalletFamily): Promise<T> => {
        const current = stateRef.current[family];
        if (current) return Promise.resolve(current as unknown as T);
        return new Promise<T>((resolve, reject) => {
            pending.current[family].push({resolve, reject});
            openModal(family);
        });
    };

    // Silent reconnect: never prompts, only restores a wallet that already
    // granted this site access (eth_accounts / an unlocked tronWeb).
    React.useEffect(() => {
        let cancelled = false;

        (async () => {
            const lastEvm = localStorage.get<string>(lastWalletKey("evm"));
            if (lastEvm) {
                const wallet = (await discoverEvmWallets()).find((w) => w.id === lastEvm);
                try {
                    const accounts: string[] = wallet ? await wallet.provider.request({method: "eth_accounts"}) : [];
                    if (!cancelled && wallet && accounts?.[0]) {
                        watchEvm(wallet);
                        dispatch({type: "connected", family: "evm", conn: {wallet, address: accounts[0] as Address}});
                    }
                } catch {
                    // wallet refused a silent query: stay disconnected
                }
            }

            const lastTron = localStorage.get<string>(lastWalletKey("tron"));
            if (lastTron) {
                const wallet = (await discoverTronWallets()).find((w) => w.id === lastTron);
                const tw = wallet?.provider?.tronWeb || (wallet?.id === "legacy:tronlink" ? (window as any).tronWeb : null);
                const address = tw?.ready ? tw?.defaultAddress?.base58 : null;
                if (!cancelled && wallet && address) {
                    dispatch({type: "connected", family: "tron", conn: {wallet, address}});
                }
            }
        })();

        return () => {
            cancelled = true;
            Object.values(unsubscribe.current).forEach((fn) => fn?.());
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const value: WalletContextValue = {
        ...state,
        modal,
        handoffEnabled,
        openModal,
        closeModal,
        connect,
        disconnect,
        ensureEvm: () => ensure<EvmConnection>("evm"),
        ensureTron: () => ensure<TronConnection>("tron"),
        handoffActiveUntil,
        markHandoffActive: () => setHandoffActiveUntil(Date.now() + HANDOFF_ACTIVE_MS)
    };

    return <WalletContext.Provider value={value}>{children}</WalletContext.Provider>;
};

export const useWallet = (): WalletContextValue => {
    const ctx = React.useContext(WalletContext);
    if (!ctx) throw new Error("useWallet must be used inside <WalletProvider>");
    return ctx;
};
