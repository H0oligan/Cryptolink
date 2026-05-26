import "./balance-page.scss";

import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {
    Space,
    Typography,
    Button,
    notification,
    Spin,
    Alert,
    Card,
    Tooltip,
    Empty,
} from "antd";
import {WalletOutlined, LinkOutlined, CopyOutlined, ReloadOutlined, DollarOutlined} from "@ant-design/icons";
import evmCollectorProvider, {EvmCollector, CollectorBalance} from "src/providers/evm-collector-provider";
import balancesProvider from "src/providers/balance-provider";
import {EVM_CHAINS, KNOWN_TOKENS, TRON_CHAIN, TRON_KNOWN_TOKENS} from "src/constants/merchant-collector";
import {isMetaMaskAvailable, connectWallet, switchChain, withdrawAll} from "src/utils/evm-wallet";
import {isTronLinkAvailable, connectTronWallet, withdrawAllTron} from "src/utils/tron-wallet";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import useSharedMerchant from "src/hooks/use-merchant";
import useMerchantCurrency from "src/hooks/use-merchant-currency";
import Icon from "src/components/icon/icon";
import {PaymentMethod} from "src/types";

const {Title, Text} = Typography;

// ============================================================
// Helpers
// ============================================================

/** Map blockchain ticker prefix to a display-friendly network name */
const NETWORK_META: Record<string, {label: string; color: string; icon: string; nativeTicker: string; explorerUrl: string}> = {
    ETH:      {label: "Ethereum",        color: "#627EEA", icon: "eth",   nativeTicker: "ETH",   explorerUrl: "https://etherscan.io"},
    MATIC:    {label: "Polygon",         color: "#8247E5", icon: "matic", nativeTicker: "MATIC",  explorerUrl: "https://polygonscan.com"},
    BSC:      {label: "BNB Smart Chain", color: "#F0B90B", icon: "bnb",   nativeTicker: "BNB",    explorerUrl: "https://bscscan.com"},
    ARBITRUM: {label: "Arbitrum",        color: "#28A0F0", icon: "arb",   nativeTicker: "ETH",    explorerUrl: "https://arbiscan.io"},
    AVAX:     {label: "Avalanche",       color: "#E84142", icon: "avax",  nativeTicker: "AVAX",   explorerUrl: "https://snowtrace.io"},
    TRON:     {label: "TRON",            color: "#FF0013", icon: "tron",  nativeTicker: "TRX",    explorerUrl: "https://tronscan.org"},
};

/** Resolve a ticker like "ETH_USDT" → icon name "usdt", or "ETH" → "eth" */
const tickerToIcon = (ticker: string): string => {
    const parts = ticker.toLowerCase().split("_");
    return parts.length > 1 ? parts[1] : parts[0];
};

/** Resolve a ticker like "ETH_USDT" → display label "USDT", "TRON" → "TRX" */
const tickerToDisplayName = (ticker: string, blockchain: string): string => {
    const parts = ticker.split("_");
    if (parts.length > 1) return parts[1]; // e.g. "ETH_USDT" → "USDT"
    // native ticker
    return NETWORK_META[blockchain]?.nativeTicker || ticker;
};

/** Group enabled payment methods by blockchain */
const groupByBlockchain = (methods: PaymentMethod[]): Record<string, PaymentMethod[]> => {
    const groups: Record<string, PaymentMethod[]> = {};
    for (const m of methods) {
        if (!m.enabled) continue;
        if (!groups[m.blockchain]) groups[m.blockchain] = [];
        groups[m.blockchain].push(m);
    }
    return groups;
};

// ============================================================
// Token Balance Row
// ============================================================

interface TokenRowProps {
    ticker: string;
    blockchain: string;
    displayName: string;
    amount: string | null;
    fiatEquivalent: string | null;
    fiatLoading: boolean;
    loading: boolean;
}

const TokenRow: React.FC<TokenRowProps> = ({
    ticker,
    blockchain,
    displayName,
    amount,
    fiatEquivalent,
    fiatLoading,
    loading,
}) => {
    const iconName = tickerToIcon(ticker);
    const amountIsZero = amount !== null && parseFloat(amount) === 0;

    return (
        <div className="balance-page__token-row">
            <div className="balance-page__token-info">
                <Icon name={iconName} dir="crypto" className="balance-page__token-icon" />
                <div>
                    <Text strong style={{fontSize: 14}}>{displayName}</Text>
                    <br />
                    <Text type="secondary" style={{fontSize: 11}}>{ticker}</Text>
                </div>
            </div>
            <div className="balance-page__token-amount">
                {loading ? (
                    <Spin size="small" />
                ) : amount !== null ? (
                    <>
                        <Text strong style={{fontSize: 16}}>{amount}</Text>
                        <div className="balance-page__token-fiat">
                            {amountIsZero ? (
                                <Text type="secondary" style={{fontSize: 12}}>—</Text>
                            ) : fiatLoading ? (
                                <Text type="secondary" style={{fontSize: 12}}>…</Text>
                            ) : fiatEquivalent ? (
                                <Text type="secondary" style={{fontSize: 12}}>≈ {fiatEquivalent}</Text>
                            ) : null}
                        </div>
                    </>
                ) : (
                    <Text type="secondary">—</Text>
                )}
            </div>
        </div>
    );
};

// ============================================================
// Network Card
// ============================================================

interface NetworkCardProps {
    blockchain: string;
    enabledMethods: PaymentMethod[];
    collector: EvmCollector | null;
    balance: CollectorBalance | null;
    loadingBalance: boolean;
    onWithdraw: () => void;
    withdrawing: boolean;
    onRefresh: () => void;
    fiatValues: Record<string, string>;
    fiatLoading: boolean;
}

const NetworkCard: React.FC<NetworkCardProps> = ({
    blockchain,
    enabledMethods,
    collector,
    balance,
    loadingBalance,
    onWithdraw,
    withdrawing,
    onRefresh,
    fiatValues,
    fiatLoading,
}) => {
    const meta = NETWORK_META[blockchain];
    if (!meta) return null;

    const isTron = blockchain === "TRON";
    const walletAvailable = isTron ? isTronLinkAvailable() : isMetaMaskAvailable();
    const walletName = isTron ? "TronLink" : "MetaMask";

    // Build token list from enabled methods
    const tokenRows: {ticker: string; displayName: string; amount: string | null}[] = [];

    for (const method of enabledMethods) {
        const displayName = tickerToDisplayName(method.ticker, blockchain);
        const parts = method.ticker.split("_");
        const isNative = parts.length === 1;

        let amount: string | null = null;
        if (balance) {
            if (isNative) {
                amount = balance.native.amount;
            } else {
                // Find matching token balance by ticker
                const tokenTicker = parts[1]; // e.g. "USDT"
                const found = balance.tokens.find(
                    (t) => t.ticker.toUpperCase() === tokenTicker.toUpperCase()
                );
                amount = found ? found.amount : "0";
            }
        }

        tokenRows.push({ticker: method.ticker, displayName, amount});
    }

    const hasCollector = collector !== null;
    const explorerBase = isTron ? `${TRON_CHAIN.explorerUrl}/#/address` : `${meta.explorerUrl}/address`;

    return (
        <Card
            className="balance-page__network-card"
            bodyStyle={{padding: 0}}
        >
            {/* Network header */}
            <div className="balance-page__network-header" style={{borderLeftColor: meta.color}}>
                <div className="balance-page__network-title">
                    <Icon name={meta.icon} dir="crypto" className="balance-page__network-icon" />
                    <div>
                        <Title level={5} style={{margin: 0}}>{meta.label}</Title>
                        {hasCollector && (
                            <Tooltip title={collector!.contractAddress}>
                                <Text
                                    type="secondary"
                                    style={{fontSize: 11, cursor: "pointer"}}
                                    onClick={() => {
                                        navigator.clipboard.writeText(collector!.contractAddress);
                                    }}
                                >
                                    {collector!.contractAddress.slice(0, 8)}...{collector!.contractAddress.slice(-6)}
                                    <CopyOutlined style={{marginLeft: 4, fontSize: 10}} />
                                </Text>
                            </Tooltip>
                        )}
                    </div>
                </div>
                <Button
                    type="text"
                    size="small"
                    icon={<ReloadOutlined />}
                    onClick={onRefresh}
                    loading={loadingBalance}
                    disabled={!hasCollector}
                />
            </div>

            {/* Token rows */}
            <div className="balance-page__token-list">
                {tokenRows.length === 0 ? (
                    <div style={{padding: "16px 20px"}}>
                        <Text type="secondary">No tokens enabled for this network.</Text>
                    </div>
                ) : (
                    tokenRows.map((row) => (
                        <TokenRow
                            key={row.ticker}
                            ticker={row.ticker}
                            blockchain={blockchain}
                            displayName={row.displayName}
                            amount={hasCollector ? row.amount : null}
                            fiatEquivalent={fiatValues[row.ticker] || null}
                            fiatLoading={fiatLoading}
                            loading={loadingBalance}
                        />
                    ))
                )}
            </div>

            {/* Footer with status + withdraw */}
            <div className="balance-page__network-footer">
                {!hasCollector ? (
                    <Text type="secondary" style={{fontSize: 12}}>
                        No smart contract deployed.{" "}
                        <a href={`${window.location.origin}/merchants/wallet-setup`}>Deploy now</a>
                    </Text>
                ) : !walletAvailable ? (
                    <Alert
                        message={`${walletName} not detected`}
                        type="warning"
                        showIcon
                        style={{flex: 1}}
                        action={
                            <Button
                                size="small"
                                href={isTron ? "https://www.tronlink.org/" : "https://metamask.io/download/"}
                                target="_blank"
                                icon={<LinkOutlined />}
                            >
                                Install
                            </Button>
                        }
                    />
                ) : (
                    <Space style={{width: "100%", justifyContent: "space-between", alignItems: "center"}}>
                        {collector && (
                            <a
                                href={`${explorerBase}/${collector.contractAddress}`}
                                target="_blank"
                                rel="noopener noreferrer"
                                style={{fontSize: 12}}
                            >
                                <LinkOutlined /> View on Explorer
                            </a>
                        )}
                        <Button
                            type="primary"
                            icon={<WalletOutlined />}
                            loading={withdrawing}
                            onClick={onWithdraw}
                            style={{background: meta.color, borderColor: meta.color}}
                        >
                            Withdraw All
                        </Button>
                    </Space>
                )}
            </div>
        </Card>
    );
};

// ============================================================
// Balance Page
// ============================================================

const BalancePage: React.FC = () => {
    const {merchantId} = useSharedMerchantId();
    const {merchant} = useSharedMerchant();
    const {currencyCode, currencyName, formatFiat} = useMerchantCurrency();
    const [api, contextHolder] = notification.useNotification();

    const [collectors, setCollectors] = React.useState<EvmCollector[]>([]);
    const [balances, setBalances] = React.useState<Record<string, CollectorBalance>>({});
    const [loadingCollectors, setLoadingCollectors] = React.useState(true);
    const [loadingBalances, setLoadingBalances] = React.useState<Record<string, boolean>>({});
    const [withdrawing, setWithdrawing] = React.useState<Record<string, boolean>>({});

    // Per-blockchain map of ticker -> formatted fiat string (e.g. "$123.45").
    // Outer key = blockchain (matches the per-network card), inner key = full ticker like "ETH_USDT".
    const [fiatByNetwork, setFiatByNetwork] = React.useState<Record<string, Record<string, string>>>({});
    // Raw numeric values used for the portfolio total, keyed the same way.
    const [fiatNumericByNetwork, setFiatNumericByNetwork] = React.useState<Record<string, Record<string, number>>>({});
    const [loadingFiat, setLoadingFiat] = React.useState(false);

    // Fetch collectors on mount
    React.useEffect(() => {
        if (!merchantId) return;
        setLoadingCollectors(true);
        evmCollectorProvider
            .listCollectors(merchantId)
            .then((cols) => {
                setCollectors(cols || []);
                // Fetch balance for each active collector
                for (const col of (cols || []).filter((c) => c.isActive)) {
                    fetchBalance(col.blockchain);
                }
            })
            .catch(() => {})
            .finally(() => setLoadingCollectors(false));
    }, [merchantId]);

    const fetchBalance = (blockchain: string) => {
        if (!merchantId) return;
        setLoadingBalances((prev) => ({...prev, [blockchain]: true}));
        evmCollectorProvider
            .getBalance(merchantId, blockchain)
            .then((bal) => setBalances((prev) => ({...prev, [blockchain]: bal})))
            .catch(() => {})
            .finally(() => setLoadingBalances((prev) => ({...prev, [blockchain]: false})));
    };

    // Fan out crypto -> fiat conversion requests whenever balances or fiat currency change.
    // Backend rejects amount <= 0, so we filter zero-balance rows before firing requests.
    // We iterate over enabled payment methods (chain-prefixed tickers like "ETH_USDT") rather
    // than balance.tokens (which uses short form "USDT" that backend Convert can't disambiguate).
    React.useEffect(() => {
        if (!merchantId || !merchant?.supportedPaymentMethods) return;
        const blockchainsWithBalances = Object.keys(balances);
        if (blockchainsWithBalances.length === 0) return;

        type Job = {blockchain: string; ticker: string; amount: string};
        const jobs: Job[] = [];

        for (const method of merchant.supportedPaymentMethods) {
            if (!method.enabled) continue;
            const bal = balances[method.blockchain];
            if (!bal) continue;

            const parts = method.ticker.split("_");
            const isNative = parts.length === 1;
            let amount: string;
            if (isNative) {
                amount = bal.native.amount;
            } else {
                const shortTicker = parts[1];
                const found = bal.tokens.find((t) => t.ticker.toUpperCase() === shortTicker.toUpperCase());
                if (!found) continue;
                amount = found.amount;
            }
            if (parseFloat(amount) > 0) {
                jobs.push({blockchain: method.blockchain, ticker: method.ticker, amount});
            }
        }

        if (jobs.length === 0) {
            setFiatByNetwork({});
            setFiatNumericByNetwork({});
            return;
        }

        setLoadingFiat(true);
        // silent:true — this is a best-effort fan-out (one convert per enabled
        // token, often 5-10 in parallel). A single upstream pricefeed hiccup
        // would otherwise spam the user with one "Something went wrong" toast
        // per failed token even though the page itself handles the rejection
        // (the token simply shows no fiat equivalent).
        Promise.allSettled(
            jobs.map((j) =>
                balancesProvider
                    .getCurrencyExchangeRate(
                        merchantId,
                        {
                            from: j.ticker as any,
                            to: currencyCode as any,
                            amount: j.amount,
                        },
                        {silent: true}
                    )
                    .then((res) => ({job: j, res}))
            )
        )
            .then((results) => {
                const formatted: Record<string, Record<string, string>> = {};
                const numeric: Record<string, Record<string, number>> = {};
                for (const r of results) {
                    if (r.status !== "fulfilled") continue;
                    const {job, res} = r.value;
                    const num = parseFloat(res.convertedAmount);
                    if (!isFinite(num)) continue;
                    if (!formatted[job.blockchain]) formatted[job.blockchain] = {};
                    if (!numeric[job.blockchain]) numeric[job.blockchain] = {};
                    formatted[job.blockchain][job.ticker] = formatFiat(num, {useLocale: true});
                    numeric[job.blockchain][job.ticker] = num;
                }
                setFiatByNetwork(formatted);
                setFiatNumericByNetwork(numeric);
            })
            .finally(() => setLoadingFiat(false));
    }, [balances, currencyCode, merchantId, merchant?.supportedPaymentMethods]);

    // Withdraw handler
    const handleWithdraw = async (blockchain: string) => {
        const collector = collectors.find((c) => c.blockchain === blockchain);
        if (!collector) return;

        setWithdrawing((prev) => ({...prev, [blockchain]: true}));
        try {
            const isTron = blockchain === "TRON";

            if (isTron) {
                await connectTronWallet();
                const txid = await withdrawAllTron(collector.contractAddress, TRON_KNOWN_TOKENS);
                api.success({
                    message: "TRON withdrawal submitted",
                    description: (
                        <span>
                            Tx:{" "}
                            <a
                                href={`${TRON_CHAIN.explorerUrl}/#/transaction/${txid}`}
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                {String(txid).slice(0, 18)}...
                            </a>
                        </span>
                    ),
                    duration: 10,
                    placement: "bottomRight",
                });
            } else {
                const chain = EVM_CHAINS.find((c) => c.value === blockchain);
                if (!chain) return;
                await connectWallet();
                await switchChain(chain);
                const tokens = KNOWN_TOKENS[blockchain] || [];
                const txHash = await withdrawAll(
                    collector.contractAddress as `0x${string}`,
                    tokens,
                    chain
                );
                api.success({
                    message: `${chain.label} withdrawal submitted`,
                    description: (
                        <span>
                            Tx:{" "}
                            <a
                                href={`${chain.explorerUrl}/tx/${txHash}`}
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                {txHash.slice(0, 18)}...
                            </a>
                        </span>
                    ),
                    duration: 10,
                    placement: "bottomRight",
                });
            }

            // Refresh balance after withdrawal
            setTimeout(() => fetchBalance(blockchain), 5000);
        } catch (err: any) {
            const msg = err?.message || "Withdrawal failed";
            const isUserRejected =
                msg.includes("rejected") || msg.includes("denied") || msg.includes("Confirmation") || err?.code === 4001;
            api.error({
                message: isUserRejected ? "Transaction rejected" : "Withdrawal failed",
                description: isUserRejected ? "You rejected the transaction in your wallet." : msg,
                placement: "bottomRight",
            });
        } finally {
            setWithdrawing((prev) => ({...prev, [blockchain]: false}));
        }
    };

    // Group enabled payment methods by blockchain
    const enabledGroups = merchant?.supportedPaymentMethods
        ? groupByBlockchain(merchant.supportedPaymentMethods)
        : {};

    // Order: show networks that have collectors first, then the rest
    const blockchainOrder = Object.keys(enabledGroups).sort((a, b) => {
        const aHas = collectors.some((c) => c.blockchain === a && c.isActive) ? 0 : 1;
        const bHas = collectors.some((c) => c.blockchain === b && c.isActive) ? 0 : 1;
        return aHas - bHas;
    });

    // Sum all fiat values across all networks for the portfolio total.
    const portfolioTotal = React.useMemo(() => {
        let sum = 0;
        for (const network of Object.values(fiatNumericByNetwork)) {
            for (const v of Object.values(network)) sum += v;
        }
        return sum;
    }, [fiatNumericByNetwork]);

    const anyBalancesLoading =
        loadingCollectors || Object.values(loadingBalances).some(Boolean) || loadingFiat;
    const hasAnyCollector = collectors.some((c) => c.isActive);

    return (
        <PageContainer header={{title: "", breadcrumb: {}}}>
            {contextHolder}
            <div className="balance-page__header">
                <div>
                    <Title level={2} style={{margin: 0}}>Balances & Withdrawals</Title>
                    <Text type="secondary">
                        On-chain balances for each network and token you have enabled.
                        Connect your wallet to withdraw funds.
                    </Text>
                </div>
            </div>

            {hasAnyCollector && (
                <Card className="balance-page__summary" bodyStyle={{padding: 24}}>
                    <div className="balance-page__summary-inner">
                        <div className="balance-page__summary-icon">
                            <DollarOutlined />
                        </div>
                        <div className="balance-page__summary-text">
                            <Text type="secondary" style={{fontSize: 13, letterSpacing: 1, textTransform: "uppercase"}}>
                                Total Portfolio Value
                            </Text>
                            <div className="balance-page__summary-value">
                                {anyBalancesLoading ? (
                                    <Spin />
                                ) : (
                                    <Title level={1} style={{margin: 0}}>
                                        {formatFiat(portfolioTotal, {useLocale: true})}
                                    </Title>
                                )}
                            </div>
                            <Text type="secondary" style={{fontSize: 12}}>
                                Valued in {currencyCode} ({currencyName}).
                                {" "}Live conversion across all enabled networks and tokens.
                            </Text>
                        </div>
                    </div>
                </Card>
            )}

            {loadingCollectors ? (
                <div style={{textAlign: "center", padding: 60}}>
                    <Spin size="large" />
                </div>
            ) : blockchainOrder.length === 0 ? (
                <Empty
                    description={
                        <span>
                            No payment methods enabled.{" "}
                            <a href={`${window.location.origin}/merchants/settings`}>
                                Enable currencies in Settings
                            </a>
                        </span>
                    }
                    style={{padding: 60}}
                />
            ) : (
                <div className="balance-page__grid">
                    {blockchainOrder.map((blockchain) => {
                        const collector = collectors.find(
                            (c) => c.blockchain === blockchain && c.isActive
                        ) || null;
                        return (
                            <NetworkCard
                                key={blockchain}
                                blockchain={blockchain}
                                enabledMethods={enabledGroups[blockchain]}
                                collector={collector}
                                balance={balances[blockchain] || null}
                                loadingBalance={!!loadingBalances[blockchain]}
                                onWithdraw={() => handleWithdraw(blockchain)}
                                withdrawing={!!withdrawing[blockchain]}
                                onRefresh={() => fetchBalance(blockchain)}
                                fiatValues={fiatByNetwork[blockchain] || {}}
                                fiatLoading={loadingFiat}
                            />
                        );
                    })}
                </div>
            )}
        </PageContainer>
    );
};

export default BalancePage;
