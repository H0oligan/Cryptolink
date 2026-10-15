import React from "react";
import {useWallet} from "src/wallet/wallet-context";

// While a phone session started from this page is active (deploy/withdraw
// happening on the phone), keep re-fetching so the result shows up here.
export const useHandoffRefresh = (refetch: () => void, everyMs = 10000): void => {
    const {handoffActiveUntil} = useWallet();
    const refetchRef = React.useRef(refetch);
    refetchRef.current = refetch;

    React.useEffect(() => {
        if (!handoffActiveUntil || Date.now() >= handoffActiveUntil) return;
        const timer = setInterval(() => {
            if (Date.now() >= handoffActiveUntil) {
                clearInterval(timer);
                return;
            }
            refetchRef.current();
        }, everyMs);
        return () => clearInterval(timer);
    }, [handoffActiveUntil, everyMs]);
};
