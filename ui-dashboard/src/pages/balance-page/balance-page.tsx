import "./balance-page.scss";

import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {
    Result,
    Space,
    Table,
    Tag,
    Typography,
    Row,
    Col,
    Card,
    Button,
    notification,
    Spin,
    Alert,
    Divider,
    Tooltip,
} from "antd";
import bevis from "src/utils/bevis";
import {ColumnsType} from "antd/es/table";
import {MerchantBalance, CURRENCY_SYMBOL} from "src/types";
import CollapseString from "src/components/collapse-string/collapse-string";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import balancesQueries from "src/queries/balances-queries";
import Icon from "src/components/icon/icon";
import evmCollectorProvider, {EvmCollector, CollectorBalance} from "src/providers/evm-collector-provider";
import {EVM_CHAINS, KNOWN_TOKENS, TRON_CHAIN, TRON_KNOWN_TOKENS} from "src/constants/merchant-collector";
import {isMetaMaskAvailable, connectWallet, switchChain, withdrawAll} from "src/utils/evm-wallet";
import {isTronLinkAvailable, connectTronWallet, withdrawAllTron} from "src/utils/tron-wallet";
import {ThunderboltOutlined, WalletOutlined, LinkOutlined} from "@ant-design/icons";

const b = bevis("balance-page");
const {Title, Text} = Typography;

// ============================================================
// EVM Collector Balances & Withdraw
// ============================================================

const EvmCollectorBalances: React.FC<{merchantId: string}> = ({merchantId}) => {
    const [api, contextHolder] = notification.useNotification();
    const [collectors, setCollectors] = React.useState<EvmCollector[]>([]);
    const [balances, setBalances] = React.useState<Record<string, CollectorBalance>>({});
    const [loadingCollectors, setLoadingCollectors] = React.useState(true);
    const [loadingBalances, setLoadingBalances] = React.useState<Record<string, boolean>>({});
    const [withdrawing, setWithdrawing] = React.useState<Record<string, boolean>>({});

    React.useEffect(() => {
        evmCollectorProvider
            .listCollectors(merchantId)
            .then((cols) => {
                const active = (cols || []).filter((c) => c.isActive);
                setCollectors(active);
                // Load balances for each active collector
                active.forEach((col) => {
                    setLoadingBalances((prev) => ({...prev, [col.blockchain]: true}));
                    evmCollectorProvider
                        .getBalance(merchantId, col.blockchain)
                        .then((bal) => {
                            setBalances((prev) => ({...prev, [col.blockchain]: bal}));
                        })
                        .catch(() => {})
                        .finally(() => {
                            setLoadingBalances((prev) => ({...prev, [col.blockchain]: false}));
                        });
                });
            })
            .catch(() => {})
            .finally(() => setLoadingCollectors(false));
    }, [merchantId]);

    const handleWithdraw = async (collector: EvmCollector) => {
        const chain = EVM_CHAINS.find((c) => c.value === collector.blockchain);
        if (!chain) return;

        setWithdrawing((prev) => ({...prev, [collector.blockchain]: true}));
        try {
            const account = await connectWallet();
            await switchChain(chain);
            const tokens = KNOWN_TOKENS[collector.blockchain] || [];
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
        } catch (err: any) {
            const msg = err?.message || "Withdrawal failed";
            const isUserRejected = msg.includes("rejected") || msg.includes("denied") || err?.code === 4001;
            api.error({
                message: isUserRejected ? "Transaction rejected" : "Withdrawal failed",
                description: isUserRejected ? "You rejected the transaction in your wallet." : msg,
                placement: "bottomRight",
            });
        } finally {
            setWithdrawing((prev) => ({...prev, [collector.blockchain]: false}));
        }
    };

    if (loadingCollectors) {
        return <Spin size="small" />;
    }

    if (collectors.length === 0) {
        return null;
    }

    return (
        <>
            {contextHolder}
            <Divider />
            <div style={{marginBottom: 8}}>
                <Title level={4} style={{marginBottom: 4}}>
                    <ThunderboltOutlined style={{marginRight: 8, color: "#6366f1"}} />
                    Smart Contract Wallets
                </Title>
                <Typography.Text type="secondary">
                    Funds accumulated in your deployed MerchantCollector contracts. Connect MetaMask to withdraw.
                </Typography.Text>
            </div>

            {!isMetaMaskAvailable() && (
                <Alert
                    message="MetaMask not detected"
                    description="Install MetaMask or another EIP-1193 compatible wallet to withdraw funds."
                    type="warning"
                    showIcon
                    style={{marginBottom: 12}}
                    action={
                        <Button size="small" href="https://metamask.io/download/" target="_blank" icon={<LinkOutlined />}>
                            Install
                        </Button>
                    }
                />
            )}

            <Row gutter={[16, 16]} style={{marginTop: 16}}>
                {collectors.map((collector) => {
                    const chain = EVM_CHAINS.find((c) => c.value === collector.blockchain);
                    const bal = balances[collector.blockchain];
                    const isLoadingBal = loadingBalances[collector.blockchain];
                    const isWithdrawing = withdrawing[collector.blockchain];

                    return (
                        <Col xs={24} sm={12} md={8} key={collector.blockchain}>
                            <Card
                                size="small"
                                style={{borderColor: "var(--cl-border)", height: "100%"}}
                                extra={
                                    <Button
                                        type="primary"
                                        size="small"
                                        icon={<WalletOutlined />}
                                        loading={isWithdrawing}
                                        disabled={!isMetaMaskAvailable()}
                                        onClick={() => handleWithdraw(collector)}
                                    >
                                        Withdraw
                                    </Button>
                                }
                            >
                                <Space direction="vertical" size={4} style={{width: "100%"}}>
                                    <Space>
                                        <div style={{
                                            width: 24, height: 24, borderRadius: "50%",
                                            background: chain?.color || "#666",
                                            display: "inline-flex", alignItems: "center", justifyContent: "center",
                                            flexShrink: 0,
                                        }}>
                                            <ThunderboltOutlined style={{color: "#fff", fontSize: 10}} />
                                        </div>
                                        <Text strong>{chain?.label || collector.blockchain}</Text>
                                    </Space>

                                    <Tooltip title={collector.contractAddress}>
                                        <Text code style={{fontSize: 11}}>
                                            {collector.contractAddress.slice(0, 10)}...{collector.contractAddress.slice(-8)}
                                        </Text>
                                    </Tooltip>

                                    {isLoadingBal ? (
                                        <Spin size="small" />
                                    ) : bal ? (
                                        <Space direction="vertical" size={2} style={{width: "100%"}}>
                                            <Space>
                                                <Text style={{fontSize: 12}}>{chain?.nativeTicker}:</Text>
                                                <Text strong style={{fontSize: 12}}>{bal.native.amount}</Text>
                                                {bal.native.usdAmount !== "0" && (
                                                    <Text type="secondary" style={{fontSize: 11}}>
                                                        ≈ ${bal.native.usdAmount}
                                                    </Text>
                                                )}
                                            </Space>
                                            {bal.tokens.map((t) => (
                                                <Space key={t.contract}>
                                                    <Text style={{fontSize: 12}}>{t.ticker}:</Text>
                                                    <Text strong style={{fontSize: 12}}>{t.amount}</Text>
                                                    {t.usdAmount !== "0" && (
                                                        <Text type="secondary" style={{fontSize: 11}}>
                                                            ≈ ${t.usdAmount}
                                                        </Text>
                                                    )}
                                                </Space>
                                            ))}
                                        </Space>
                                    ) : (
                                        <Text type="secondary" style={{fontSize: 12}}>Balance unavailable</Text>
                                    )}
                                </Space>
                            </Card>
                        </Col>
                    );
                })}
            </Row>
        </>
    );
};

// ============================================================
// TRON Collector Balance & Withdraw
// ============================================================

const TronCollectorBalance: React.FC<{merchantId: string}> = ({merchantId}) => {
    const [api, contextHolder] = notification.useNotification();
    const [collector, setCollector] = React.useState<EvmCollector | null>(null);
    const [balance, setBalance] = React.useState<CollectorBalance | null>(null);
    const [loadingCollector, setLoadingCollector] = React.useState(true);
    const [loadingBalance, setLoadingBalance] = React.useState(false);
    const [withdrawing, setWithdrawing] = React.useState(false);

    React.useEffect(() => {
        evmCollectorProvider
            .listCollectors(merchantId)
            .then((cols) => {
                const tron = (cols || []).find((c) => c.blockchain === "TRON" && c.isActive);
                setCollector(tron || null);
                if (tron) {
                    setLoadingBalance(true);
                    evmCollectorProvider
                        .getBalance(merchantId, "TRON")
                        .then((bal) => setBalance(bal))
                        .catch(() => {})
                        .finally(() => setLoadingBalance(false));
                }
            })
            .catch(() => {})
            .finally(() => setLoadingCollector(false));
    }, [merchantId]);

    const handleWithdraw = async () => {
        if (!collector) return;
        setWithdrawing(true);
        try {
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
        } catch (err: any) {
            const msg = err?.message || "Withdrawal failed";
            const isUserRejected = msg.includes("rejected") || msg.includes("denied") || msg.includes("Confirmation");
            api.error({
                message: isUserRejected ? "Transaction rejected" : "Withdrawal failed",
                description: isUserRejected ? "You rejected the transaction in TronLink." : msg,
                placement: "bottomRight",
            });
        } finally {
            setWithdrawing(false);
        }
    };

    if (loadingCollector) return <Spin size="small" />;
    if (!collector) return null;

    return (
        <>
            {contextHolder}
            <Divider />
            <div style={{marginBottom: 8}}>
                <Title level={4} style={{marginBottom: 4}}>
                    <span style={{
                        display: "inline-flex", alignItems: "center", justifyContent: "center",
                        width: 24, height: 24, borderRadius: "50%", background: TRON_CHAIN.color,
                        marginRight: 8, verticalAlign: "middle",
                    }}>
                        <ThunderboltOutlined style={{color: "#fff", fontSize: 10}} />
                    </span>
                    TRON Smart Contract Wallet
                </Title>
                <Text type="secondary">
                    Funds accumulated in your TRON MerchantCollector contract. Connect TronLink to withdraw.
                </Text>
            </div>

            {!isTronLinkAvailable() && (
                <Alert
                    message="TronLink not detected"
                    description="Install the TronLink browser extension to withdraw funds."
                    type="warning"
                    showIcon
                    style={{marginBottom: 12}}
                    action={
                        <Button size="small" href="https://www.tronlink.org/" target="_blank" icon={<LinkOutlined />}>
                            Install
                        </Button>
                    }
                />
            )}

            <Row gutter={[16, 16]} style={{marginTop: 16}}>
                <Col xs={24} sm={12} md={8}>
                    <Card
                        size="small"
                        style={{borderColor: "var(--cl-border)", height: "100%"}}
                        extra={
                            <Button
                                type="primary"
                                size="small"
                                icon={<WalletOutlined />}
                                loading={withdrawing}
                                disabled={!isTronLinkAvailable()}
                                onClick={handleWithdraw}
                            >
                                Withdraw
                            </Button>
                        }
                    >
                        <Space direction="vertical" size={4} style={{width: "100%"}}>
                            <Space>
                                <div style={{
                                    width: 24, height: 24, borderRadius: "50%",
                                    background: TRON_CHAIN.color,
                                    display: "inline-flex", alignItems: "center", justifyContent: "center",
                                    flexShrink: 0,
                                }}>
                                    <ThunderboltOutlined style={{color: "#fff", fontSize: 10}} />
                                </div>
                                <Text strong>TRON</Text>
                            </Space>

                            <Tooltip title={collector.contractAddress}>
                                <Text code style={{fontSize: 11}}>
                                    {collector.contractAddress.slice(0, 10)}...{collector.contractAddress.slice(-8)}
                                </Text>
                            </Tooltip>

                            {loadingBalance ? (
                                <Spin size="small" />
                            ) : balance ? (
                                <Space direction="vertical" size={2} style={{width: "100%"}}>
                                    <Space>
                                        <Text style={{fontSize: 12}}>TRX:</Text>
                                        <Text strong style={{fontSize: 12}}>{balance.native.amount}</Text>
                                        {balance.native.usdAmount !== "0" && (
                                            <Text type="secondary" style={{fontSize: 11}}>
                                                ≈ ${balance.native.usdAmount}
                                            </Text>
                                        )}
                                    </Space>
                                    {balance.tokens.map((t) => (
                                        <Space key={t.contract}>
                                            <Text style={{fontSize: 12}}>{t.ticker}:</Text>
                                            <Text strong style={{fontSize: 12}}>{t.amount}</Text>
                                            {t.usdAmount !== "0" && (
                                                <Text type="secondary" style={{fontSize: 11}}>
                                                    ≈ ${t.usdAmount}
                                                </Text>
                                            )}
                                        </Space>
                                    ))}
                                </Space>
                            ) : (
                                <Text type="secondary" style={{fontSize: 12}}>Balance unavailable</Text>
                            )}
                        </Space>
                    </Card>
                </Col>
            </Row>
        </>
    );
};

// ============================================================
// Balance Page
// ============================================================

const BalancePage: React.FC = () => {
    const listBalances = balancesQueries.listBalances();
    const [balances, setBalances] = React.useState<MerchantBalance[]>(listBalances.data?.pages[0] || []);
    const {merchantId} = useSharedMerchantId();

    const renderIconName = (name: string) => {
        const lowered = name.toLowerCase();
        return lowered.includes("_") ? lowered.split("_")[1] : lowered;
    };

    const balancesColumns: ColumnsType<MerchantBalance> = [
        {
            title: "Network",
            dataIndex: "network",
            key: "network",
            render: (_, record) => <span style={{whiteSpace: "nowrap"}}>{record.blockchainName}</span>,
        },
        {
            title: "Currency",
            dataIndex: "currency",
            key: "currency",
            width: "min-content",
            render: (_, record) => (
                <Space align="center">
                    <Icon name={renderIconName(record.ticker.toLowerCase())} dir="crypto" className={b("icon")} />
                    <span style={{whiteSpace: "nowrap"}}> {record.currency} </span>
                </Space>
            ),
        },
        {
            title: "Balance",
            dataIndex: "balance",
            key: "balance",
            render: (_, record) => (
                <Row align="middle" justify="space-between">
                    <Space>
                        <CollapseString
                            style={{marginRight: "10px"}}
                            text={`${record.amount} ${record.ticker}`}
                            collapseAt={12}
                            withPopover
                        />
                    </Space>
                </Row>
            ),
        },
        {
            title: "USD Balance",
            dataIndex: "usdBalance",
            key: "usdBalance",
            render: (_, record) => (
                <Row align="middle" justify="space-between">
                    <Space>
                        <CollapseString
                            style={{marginRight: "10px"}}
                            text={`${CURRENCY_SYMBOL["USD"]}${record.usdAmount}`}
                            collapseAt={12}
                            withPopover
                        />
                        {record.isTest ? <Tag color="yellow">Test Balance</Tag> : null}
                    </Space>
                </Row>
            ),
        },
    ];

    const isLoadingBalance = listBalances.isLoading || listBalances.isFetching;

    React.useEffect(() => {
        setBalances(listBalances.data?.pages[0] || []);
    }, [listBalances.data]);

    React.useEffect(() => {
        if (merchantId) {
            listBalances.refetch();
        }
    }, [merchantId]);

    return (
        <PageContainer header={{title: "", breadcrumb: {}}}>
            <Typography.Title>Balances</Typography.Title>
            <Typography.Text type="secondary" style={{marginBottom: 16, display: "block"}}>
                Incoming payment balances tracked by CryptoLink. xpub-based wallets (BTC) are shown
                below. Smart contract wallet balances and withdrawals (EVM chains + TRON) are shown further down.
            </Typography.Text>
            <Table
                columns={balancesColumns}
                dataSource={balances}
                rowKey={(record) => record.id}
                className={b("row")}
                loading={isLoadingBalance}
                pagination={false}
                size="middle"
                locale={{
                    emptyText: (
                        <Result
                            icon={null}
                            title="Your balances will appear here after you receive any payment from a customer"
                        />
                    ),
                }}
            />

            {merchantId && <EvmCollectorBalances merchantId={merchantId} />}
            {merchantId && <TronCollectorBalance merchantId={merchantId} />}
        </PageContainer>
    );
};

export default BalancePage;
