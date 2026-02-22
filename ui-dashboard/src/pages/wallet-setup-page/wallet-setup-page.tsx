import "./wallet-setup-page.scss";

import * as React from "react";
import {
    Button,
    Typography,
    Card,
    Select,
    Alert,
    Input,
    notification,
    Space,
    Tag,
    Row,
    Col,
    Tabs,
    Divider,
    Spin,
    Modal,
    Descriptions,
    Tooltip,
    Steps
} from "antd";
import {
    CheckCircleOutlined,
    WalletOutlined,
    ImportOutlined,
    SafetyCertificateOutlined,
    LockOutlined,
    DeleteOutlined,
    ArrowLeftOutlined,
    ExclamationCircleOutlined,
    ThunderboltOutlined,
    LoadingOutlined,
    LinkOutlined
} from "@ant-design/icons";
import bevis from "src/utils/bevis";
import xpubProvider, {XpubWallet} from "src/providers/xpub-provider";
import evmCollectorProvider, {EvmCollector} from "src/providers/evm-collector-provider";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import {EVM_CHAINS, type EvmChainConfig} from "src/constants/merchant-collector";
import {isMetaMaskAvailable, connectWallet, switchChain, deployCollector} from "src/utils/evm-wallet";

const b = bevis("wallet-setup-page");

const {Title, Text, Paragraph} = Typography;

// xpub-only blockchains — BTC and TRON only
const XPUB_BLOCKCHAINS = [
    {value: "BTC",  label: "Bitcoin", path: "m/44'/0'/0'/0",   color: "#F7931A"},
    {value: "TRON", label: "TRON",    path: "m/44'/195'/0'/0", color: "#FF0013"},
];

// ============================================================
// EVM Collector Panel — automated MetaMask deploy
// ============================================================

type DeployStep = "idle" | "connecting" | "switching" | "deploying" | "registering" | "done" | "error";

interface DeployState {
    step: DeployStep;
    error?: string;
    contractAddress?: string;
    ownerAddress?: string;
}

const STEP_LABELS: Record<DeployStep, string> = {
    idle:        "Ready",
    connecting:  "Connecting wallet...",
    switching:   "Switching network...",
    deploying:   "Deploying contract...",
    registering: "Registering with CryptoLink...",
    done:        "Done!",
    error:       "Error",
};

const EvmCollectorPanel: React.FC<{
    merchantId: string;
    collectors: EvmCollector[];
    onRefresh: () => void;
}> = ({merchantId, collectors, onRefresh}) => {
    const [api, contextHolder] = notification.useNotification();
    const [deployingChain, setDeployingChain] = React.useState<string | null>(null);
    const [deployState, setDeployState] = React.useState<DeployState>({step: "idle"});
    const [isDeleting, setIsDeleting] = React.useState<string | null>(null);

    const getCollector = (chain: string): EvmCollector | undefined =>
        collectors.find((c) => c.blockchain === chain);

    const handleDeploy = async (chain: EvmChainConfig) => {
        setDeployingChain(chain.value);
        setDeployState({step: "connecting"});

        try {
            // Step 1: Connect wallet
            const owner = await connectWallet();
            setDeployState({step: "switching", ownerAddress: owner});

            // Step 2: Switch to correct chain
            await switchChain(chain);
            setDeployState((s) => ({...s, step: "deploying"}));

            // Step 3: Deploy MerchantCollector
            const contractAddr = await deployCollector(owner, chain);
            setDeployState((s) => ({...s, step: "registering", contractAddress: contractAddr}));

            // Step 4: Register with CryptoLink API
            await evmCollectorProvider.setupCollector(merchantId, {
                blockchain: chain.value,
                ownerAddress: owner,
                contractAddress: contractAddr,
                chainId: chain.chainId,
                factoryAddress: "",
            });

            setDeployState({step: "done", contractAddress: contractAddr, ownerAddress: owner});
            api.success({
                message: `${chain.label} wallet deployed!`,
                description: `Contract: ${contractAddr.slice(0, 10)}...${contractAddr.slice(-8)}`,
                placement: "bottomRight",
            });
            onRefresh();

            // Auto-close modal after 2s
            setTimeout(() => {
                setDeployingChain(null);
                setDeployState({step: "idle"});
            }, 2000);
        } catch (err: any) {
            const msg = err?.message || "Deployment failed. Please try again.";
            setDeployState({step: "error", error: msg});
        }
    };

    const handleDelete = (chain: string) => {
        Modal.confirm({
            title: "Remove Smart Contract Wallet",
            icon: <ExclamationCircleOutlined />,
            content: (
                <div>
                    <p>
                        Remove the <strong>{EVM_CHAINS.find((c) => c.value === chain)?.label}</strong> smart
                        contract wallet registration?
                    </p>
                    <p style={{color: "#ff4d4f", marginTop: 8}}>
                        This only removes the registration from CryptoLink. The contract on-chain is not
                        affected — your funds remain accessible via MetaMask at any time.
                    </p>
                </div>
            ),
            okText: "Remove",
            okType: "danger",
            cancelText: "Cancel",
            onOk: async () => {
                setIsDeleting(chain);
                try {
                    await evmCollectorProvider.deleteCollector(merchantId, chain);
                    api.success({message: "Wallet removed", placement: "bottomRight"});
                    onRefresh();
                } catch (error: any) {
                    api.error({
                        message: "Failed to remove",
                        description: error.response?.data?.message || "Please try again",
                        placement: "bottomRight",
                    });
                } finally {
                    setIsDeleting(null);
                }
            },
        });
    };

    const currentChainConfig = EVM_CHAINS.find((c) => c.value === deployingChain);

    const deployStepItems = [
        {title: "Connect Wallet"},
        {title: "Switch Network"},
        {title: "Deploy Contract"},
        {title: "Register"},
    ];

    const deployStepIndex: Record<DeployStep, number> = {
        idle: -1, connecting: 0, switching: 1, deploying: 2, registering: 3, done: 3, error: -1,
    };

    return (
        <>
            {contextHolder}
            <Alert
                message="Non-custodial Smart Contract Wallets"
                description={
                    <ul style={{margin: "8px 0 0 0", paddingLeft: 20}}>
                        <li>Each EVM chain gets its own <strong>MerchantCollector</strong> contract deployed directly from your MetaMask — no factory required.</li>
                        <li>Customers pay to the contract address. You call <strong>withdrawAll()</strong> to sweep all funds to your wallet.</li>
                        <li>CryptoLink has <strong>zero admin access</strong> — only your wallet (the owner) can withdraw.</li>
                        <li>Click <strong>"Deploy with MetaMask"</strong> and follow the prompts — the entire process is automated.</li>
                    </ul>
                }
                type="info"
                showIcon
                icon={<ThunderboltOutlined />}
                style={{marginBottom: 24}}
            />

            {!isMetaMaskAvailable() && (
                <Alert
                    message="MetaMask not detected"
                    description="Install MetaMask or another EIP-1193 compatible browser wallet to deploy smart contract wallets."
                    type="warning"
                    showIcon
                    style={{marginBottom: 16}}
                    action={
                        <Button size="small" href="https://metamask.io/download/" target="_blank" icon={<LinkOutlined />}>
                            Install MetaMask
                        </Button>
                    }
                />
            )}

            <Row gutter={[16, 16]}>
                {EVM_CHAINS.map((chain) => {
                    const collector = getCollector(chain.value);
                    return (
                        <Col xs={24} sm={12} md={8} key={chain.value}>
                            <Card
                                size="small"
                                style={{
                                    borderColor: collector ? "#10b981" : "var(--cl-border)",
                                    background: collector ? "rgba(16,185,129,0.08)" : undefined,
                                    height: "100%",
                                }}
                            >
                                <Space direction="vertical" size={8} style={{width: "100%"}}>
                                    <Space>
                                        <div style={{
                                            width: 28, height: 28, borderRadius: "50%",
                                            background: chain.color,
                                            display: "inline-flex", alignItems: "center", justifyContent: "center",
                                            flexShrink: 0,
                                        }}>
                                            <ThunderboltOutlined style={{color: "#fff", fontSize: 12}} />
                                        </div>
                                        <Text strong>{chain.label}</Text>
                                        {collector ? (
                                            <Tag color="green"><CheckCircleOutlined /> Active</Tag>
                                        ) : (
                                            <Tag>Not deployed</Tag>
                                        )}
                                    </Space>

                                    {collector ? (
                                        <>
                                            <div>
                                                <Text type="secondary" style={{fontSize: 11}}>Contract</Text>
                                                <div>
                                                    <Tooltip title={collector.contractAddress}>
                                                        <Text
                                                            code
                                                            copyable={{text: collector.contractAddress}}
                                                            style={{fontSize: 11}}
                                                        >
                                                            {collector.contractAddress.slice(0, 10)}...{collector.contractAddress.slice(-8)}
                                                        </Text>
                                                    </Tooltip>
                                                </div>
                                            </div>
                                            <div>
                                                <Text type="secondary" style={{fontSize: 11}}>Owner</Text>
                                                <div>
                                                    <Tooltip title={collector.ownerAddress}>
                                                        <Text code style={{fontSize: 11}}>
                                                            {collector.ownerAddress.slice(0, 10)}...{collector.ownerAddress.slice(-8)}
                                                        </Text>
                                                    </Tooltip>
                                                </div>
                                            </div>
                                            <Button
                                                danger
                                                size="small"
                                                icon={<DeleteOutlined />}
                                                loading={isDeleting === chain.value}
                                                onClick={() => handleDelete(chain.value)}
                                                style={{marginTop: 4}}
                                            >
                                                Remove
                                            </Button>
                                        </>
                                    ) : (
                                        <Button
                                            type="primary"
                                            size="small"
                                            icon={<ThunderboltOutlined />}
                                            disabled={!isMetaMaskAvailable()}
                                            onClick={() => handleDeploy(chain)}
                                            style={{marginTop: 4}}
                                        >
                                            Deploy with MetaMask
                                        </Button>
                                    )}
                                </Space>
                            </Card>
                        </Col>
                    );
                })}
            </Row>

            {/* Deployment progress modal */}
            <Modal
                title={`Deploying ${currentChainConfig?.label} Contract`}
                open={Boolean(deployingChain)}
                footer={
                    deployState.step === "error" || deployState.step === "done" ? (
                        <Button onClick={() => {setDeployingChain(null); setDeployState({step: "idle"});}}>
                            Close
                        </Button>
                    ) : null
                }
                closable={deployState.step === "error"}
                onCancel={() => {setDeployingChain(null); setDeployState({step: "idle"});}}
                width={480}
                maskClosable={false}
            >
                {deployState.step !== "error" && deployState.step !== "idle" && (
                    <Steps
                        size="small"
                        current={deployStepIndex[deployState.step]}
                        status={deployState.step === "done" ? "finish" : "process"}
                        items={deployStepItems}
                        style={{marginBottom: 24}}
                    />
                )}

                <div style={{textAlign: "center", padding: "16px 0"}}>
                    {deployState.step === "done" ? (
                        <Space direction="vertical" size={8}>
                            <CheckCircleOutlined style={{fontSize: 40, color: "#52c41a"}} />
                            <Text strong style={{fontSize: 16}}>Deployed successfully!</Text>
                            {deployState.contractAddress && (
                                <Text code style={{fontSize: 12, wordBreak: "break-all"}}>
                                    {deployState.contractAddress}
                                </Text>
                            )}
                        </Space>
                    ) : deployState.step === "error" ? (
                        <Space direction="vertical" size={8}>
                            <ExclamationCircleOutlined style={{fontSize: 40, color: "#ff4d4f"}} />
                            <Text strong style={{fontSize: 16}}>Deployment failed</Text>
                            <Text type="secondary">{deployState.error}</Text>
                        </Space>
                    ) : (
                        <Space direction="vertical" size={8}>
                            <Spin indicator={<LoadingOutlined style={{fontSize: 32}} spin />} />
                            <Text type="secondary">{STEP_LABELS[deployState.step]}</Text>
                            {deployState.step === "deploying" && (
                                <Text type="secondary" style={{fontSize: 12}}>
                                    Please confirm the transaction in MetaMask and wait for it to be mined...
                                </Text>
                            )}
                        </Space>
                    )}
                </div>
            </Modal>
        </>
    );
};

// ============================================================
// Main Wallet Setup Page
// ============================================================

const WalletSetupPage: React.FC = () => {
    const [api, contextHolder] = notification.useNotification();
    const {merchantId} = useSharedMerchantId();

    const [mode, setMode] = React.useState<"overview" | "import" | "detail">("overview");
    const [selectedBlockchain, setSelectedBlockchain] = React.useState<string>("");
    const [importXpub, setImportXpub] = React.useState<string>("");
    const [importPath, setImportPath] = React.useState<string>("");
    const [isSubmitting, setIsSubmitting] = React.useState(false);
    const [existingWallets, setExistingWallets] = React.useState<XpubWallet[]>([]);
    const [collectors, setCollectors] = React.useState<EvmCollector[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [selectedWallet, setSelectedWallet] = React.useState<XpubWallet | null>(null);
    const [isDeleting, setIsDeleting] = React.useState(false);
    const [firstAddress, setFirstAddress] = React.useState<string>("");
    const [loadingAddress, setLoadingAddress] = React.useState(false);

    const loadData = async () => {
        if (!merchantId) return;
        try {
            const [wallets, cols] = await Promise.all([
                xpubProvider.listXpubWallets(merchantId),
                evmCollectorProvider.listCollectors(merchantId).catch(() => []),
            ]);
            setExistingWallets(wallets || []);
            setCollectors(cols || []);
        } catch (e) {
            console.error("Failed to load wallet data", e);
        } finally {
            setLoading(false);
        }
    };

    React.useEffect(() => {
        loadData();
    }, [merchantId]);

    const getWalletForChain = (chain: string) => existingWallets.find((w) => w.blockchain === chain);
    const isChainConfigured = (chain: string) => existingWallets.some((w) => w.blockchain === chain);

    const loadFirstAddress = async (wallet: XpubWallet) => {
        if (!merchantId) return;
        setLoadingAddress(true);
        setFirstAddress("");
        try {
            const addresses = await xpubProvider.listDerivedAddresses(merchantId, wallet.uuid);
            if (addresses && addresses.length > 0) {
                const sorted = [...addresses].sort((a, b) => a.derivationIndex - b.derivationIndex);
                setFirstAddress(sorted[0].address);
            }
        } catch (e) {
            console.error("Failed to load addresses", e);
        } finally {
            setLoadingAddress(false);
        }
    };

    const handleCardClick = (chainValue: string) => {
        const wallet = getWalletForChain(chainValue);
        if (wallet) {
            setSelectedWallet(wallet);
            setSelectedBlockchain(chainValue);
            setMode("detail");
            loadFirstAddress(wallet);
        } else {
            setSelectedBlockchain(chainValue);
            const bc = XPUB_BLOCKCHAINS.find((c) => c.value === chainValue);
            setImportPath(bc?.path || "");
            setMode("import");
        }
    };

    const handleDelete = () => {
        if (!selectedWallet || !merchantId) return;
        Modal.confirm({
            title: "Delete Wallet Configuration",
            icon: <ExclamationCircleOutlined />,
            content: (
                <div>
                    <p>
                        Remove the{" "}
                        <strong>{XPUB_BLOCKCHAINS.find((c) => c.value === selectedWallet.blockchain)?.label}</strong>{" "}
                        wallet configuration?
                    </p>
                    <p style={{color: "#ff4d4f", marginTop: 8}}>
                        This removes the xpub from CryptoLink. Your funds remain safe — we never hold your
                        private keys.
                    </p>
                    <p style={{marginTop: 8}}>You can re-configure it later by importing your xpub again.</p>
                </div>
            ),
            okText: "Delete",
            okType: "danger",
            cancelText: "Cancel",
            onOk: async () => {
                setIsDeleting(true);
                try {
                    await xpubProvider.deleteXpubWallet(merchantId, selectedWallet.uuid);
                    api.success({message: "Wallet removed", placement: "bottomRight"});
                    const wallets = await xpubProvider.listXpubWallets(merchantId);
                    setExistingWallets(wallets || []);
                    resetState();
                } catch (error: any) {
                    api.error({
                        message: "Failed to delete wallet",
                        description: error.response?.data?.message || "Please try again",
                        placement: "bottomRight",
                    });
                } finally {
                    setIsDeleting(false);
                }
            },
        });
    };

    const handleSubmitImport = async () => {
        if (!merchantId || !selectedBlockchain || !importXpub) return;
        const blockchain = XPUB_BLOCKCHAINS.find((bc) => bc.value === selectedBlockchain);
        setIsSubmitting(true);
        try {
            await xpubProvider.createXpubWallet(merchantId, {
                blockchain: selectedBlockchain,
                xpub: importXpub,
                derivationPath: importPath || blockchain?.path || "",
            });
            api.success({
                message: "Wallet imported!",
                description: `${blockchain?.label} wallet has been configured`,
                placement: "bottomRight",
            });
            const wallets = await xpubProvider.listXpubWallets(merchantId);
            setExistingWallets(wallets || []);
            resetState();
        } catch (error: any) {
            api.error({
                message: "Failed to import wallet",
                description: error.response?.data?.message || error.response?.data?.error || "Please try again",
                placement: "bottomRight",
            });
        } finally {
            setIsSubmitting(false);
        }
    };

    const resetState = () => {
        setMode("overview");
        setSelectedBlockchain("");
        setImportXpub("");
        setImportPath("");
        setSelectedWallet(null);
        setFirstAddress("");
    };

    const renderOverview = () => (
        <>
            <Card style={{marginBottom: 24, borderLeft: "4px solid #6366f1"}}>
                <Space direction="vertical" size={8}>
                    <Space>
                        <SafetyCertificateOutlined style={{fontSize: 20, color: "#6366f1"}} />
                        <Title level={5} style={{margin: 0}}>HD Wallets — How it Works</Title>
                    </Space>
                    <Paragraph type="secondary" style={{margin: 0}}>
                        CryptoLink uses <strong>HD wallets (BIP32/BIP44)</strong> so customers pay directly to
                        your wallet. We only store your <strong>extended public key (xpub)</strong> to derive
                        unique payment addresses. Your private keys never leave your device.
                    </Paragraph>
                    <Space size={16} wrap>
                        <Tag icon={<LockOutlined />} color="green">Private keys stay local</Tag>
                        <Tag icon={<WalletOutlined />} color="blue">Direct-to-wallet payments</Tag>
                        <Tag color="default">Import xpub only — no key generation</Tag>
                    </Space>
                </Space>
            </Card>

            <Title level={4}>Supported Networks</Title>
            <Paragraph type="secondary" style={{marginBottom: 16}}>
                xpub HD wallets are supported for Bitcoin and TRON. For EVM networks (Ethereum, Polygon, etc.)
                use the Smart Contract Wallets tab.
            </Paragraph>

            <Row gutter={[16, 16]} style={{marginBottom: 24}}>
                {XPUB_BLOCKCHAINS.map((chain) => {
                    const configured = isChainConfigured(chain.value);
                    return (
                        <Col xs={12} sm={8} md={6} key={chain.value}>
                            <Card
                                size="small"
                                hoverable
                                onClick={() => handleCardClick(chain.value)}
                                style={{
                                    textAlign: "center",
                                    borderColor: configured ? "#10b981" : "var(--cl-border)",
                                    background: configured ? "rgba(16, 185, 129, 0.08)" : undefined,
                                    cursor: "pointer",
                                    transition: "all 0.2s",
                                }}
                            >
                                <div style={{
                                    width: 32, height: 32, borderRadius: "50%",
                                    background: chain.color, margin: "0 auto 8px",
                                    display: "flex", alignItems: "center", justifyContent: "center",
                                }}>
                                    <WalletOutlined style={{color: "#fff", fontSize: 16}} />
                                </div>
                                <Text strong style={{display: "block"}}>{chain.label}</Text>
                                {configured ? (
                                    <Tag color="green" style={{marginTop: 4}}>
                                        <CheckCircleOutlined /> Active
                                    </Tag>
                                ) : (
                                    <Tag style={{marginTop: 4}}>Not configured</Tag>
                                )}
                            </Card>
                        </Col>
                    );
                })}
            </Row>

            <Divider />
            <Button
                size="large"
                icon={<ImportOutlined />}
                onClick={() => setMode("import")}
            >
                Import Existing xpub
            </Button>
        </>
    );

    const renderDetailMode = () => {
        if (!selectedWallet) return null;
        const chain = XPUB_BLOCKCHAINS.find((c) => c.value === selectedWallet.blockchain);
        return (
            <Card>
                <Space style={{marginBottom: 24}}>
                    <Button icon={<ArrowLeftOutlined />} onClick={resetState}>Back</Button>
                    <div style={{
                        width: 32, height: 32, borderRadius: "50%",
                        background: chain?.color || "#666",
                        display: "inline-flex", alignItems: "center", justifyContent: "center",
                        verticalAlign: "middle",
                    }}>
                        <WalletOutlined style={{color: "#fff", fontSize: 16}} />
                    </div>
                    <Title level={4} style={{margin: 0}}>{chain?.label} Wallet</Title>
                    <Tag color="green"><CheckCircleOutlined /> Active</Tag>
                </Space>

                <Descriptions bordered column={1} size="middle">
                    <Descriptions.Item label="Blockchain">
                        {chain?.label} ({selectedWallet.blockchain})
                    </Descriptions.Item>
                    <Descriptions.Item label="Derivation Path">
                        <Text code>{selectedWallet.derivationPath}</Text>
                    </Descriptions.Item>
                    <Descriptions.Item label="Addresses Derived">
                        {selectedWallet.lastDerivedIndex}
                    </Descriptions.Item>
                    <Descriptions.Item label="Created">
                        {new Date(selectedWallet.createdAt).toLocaleDateString()}
                    </Descriptions.Item>
                    <Descriptions.Item label="First Derived Address">
                        {loadingAddress ? (
                            <Spin size="small" />
                        ) : firstAddress ? (
                            <Text code copyable style={{fontSize: 12, wordBreak: "break-all"}}>
                                {firstAddress}
                            </Text>
                        ) : (
                            <Text type="secondary">No addresses derived yet</Text>
                        )}
                    </Descriptions.Item>
                </Descriptions>

                {firstAddress && (
                    <Alert
                        message="Verify your wallet"
                        description={`Compare the address above with the first receive address in your ${chain?.label} wallet software. If they match, your xpub is correctly configured.`}
                        type="info"
                        showIcon
                        style={{marginTop: 16}}
                    />
                )}

                <Divider />

                <Space direction="vertical" style={{width: "100%"}} size={12}>
                    <Title level={5} style={{margin: 0}}>Actions</Title>
                    <Button
                        danger
                        icon={<DeleteOutlined />}
                        loading={isDeleting}
                        onClick={handleDelete}
                        block
                        size="large"
                    >
                        Remove Wallet Configuration
                    </Button>
                    <Text type="secondary" style={{fontSize: 12}}>
                        Removing the wallet only deletes the xpub from CryptoLink. Your funds remain safe —
                        we never hold your private keys. You can re-import the same or a different xpub afterward.
                    </Text>
                </Space>
            </Card>
        );
    };

    const renderImportMode = () => (
        <Card className={b("step-card")}>
            <Space style={{marginBottom: 16}}>
                <Button icon={<ArrowLeftOutlined />} onClick={resetState}>Back</Button>
                <Title level={4} style={{margin: 0}}>Import Extended Public Key (xpub)</Title>
            </Space>
            <Paragraph type="secondary">
                Import your HD wallet's xpub key. This is useful if you manage your keys with a hardware
                wallet (Ledger, Trezor) or another wallet software. Your private keys never leave your device.
            </Paragraph>

            <Alert
                message="Need help extracting your xpub?"
                description={
                    <span>
                        Download our{" "}
                        <a href="/xpub-extractor.html" download="xpub-extractor.html" style={{fontWeight: "bold"}}>
                            Offline Xpub Extractor Tool
                        </a>{" "}
                        — a single HTML file you run locally on your computer. It converts your seed phrase into
                        an xpub entirely offline. Your seed phrase never leaves your machine.
                    </span>
                }
                type="info"
                showIcon
                style={{marginBottom: 16}}
            />

            <div style={{marginBottom: 16}}>
                <Text strong>Blockchain</Text>
                <Select
                    style={{width: "100%", marginTop: 4}}
                    placeholder="Select blockchain"
                    value={selectedBlockchain || undefined}
                    options={XPUB_BLOCKCHAINS.filter((c) => !isChainConfigured(c.value))}
                    onChange={(v) => {
                        setSelectedBlockchain(v);
                        const bc = XPUB_BLOCKCHAINS.find((c) => c.value === v);
                        setImportPath(bc?.path || "");
                    }}
                    size="large"
                />
            </div>

            <div style={{marginBottom: 16}}>
                <Text strong>Extended Public Key (xpub)</Text>
                <Input.TextArea
                    rows={3}
                    placeholder="xpub6CatW..."
                    value={importXpub}
                    onChange={(e) => setImportXpub(e.target.value)}
                    style={{marginTop: 4, fontFamily: "monospace"}}
                />
            </div>

            <div style={{marginBottom: 16}}>
                <Text strong>Derivation Path</Text>
                <Input
                    value={importPath}
                    onChange={(e) => setImportPath(e.target.value)}
                    placeholder="m/44'/0'/0'/0"
                    style={{marginTop: 4, fontFamily: "monospace"}}
                />
            </div>

            <Space>
                <Button
                    type="primary"
                    loading={isSubmitting}
                    disabled={!selectedBlockchain || !importXpub}
                    onClick={handleSubmitImport}
                >
                    Import Wallet
                </Button>
                <Button onClick={resetState}>Cancel</Button>
            </Space>
        </Card>
    );

    if (loading) {
        return (
            <div className={b()}>
                <Row justify="center" style={{padding: 100}}>
                    <Spin size="large" />
                </Row>
            </div>
        );
    }

    const tabItems = [
        {
            key: "xpub",
            label: (
                <span>
                    <WalletOutlined /> HD Wallets (xpub)
                </span>
            ),
            children: (
                <div style={{paddingTop: 16}}>
                    {mode === "overview" && renderOverview()}
                    {mode === "detail" && renderDetailMode()}
                    {mode === "import" && renderImportMode()}
                </div>
            ),
        },
        {
            key: "evm",
            label: (
                <span>
                    <ThunderboltOutlined /> Smart Contract Wallets
                </span>
            ),
            children: (
                <div style={{paddingTop: 16}}>
                    {merchantId && (
                        <EvmCollectorPanel
                            merchantId={merchantId}
                            collectors={collectors}
                            onRefresh={loadData}
                        />
                    )}
                </div>
            ),
        },
    ];

    return (
        <>
            {contextHolder}
            <div className={b()}>
                <div className={b("header")}>
                    <Title level={2}>
                        <WalletOutlined style={{marginRight: 12}} />
                        Wallet Setup
                    </Title>
                    <Paragraph type="secondary">
                        Configure your non-custodial wallets to receive crypto payments directly.
                        HD wallets (xpub) for Bitcoin & TRON — Smart Contract wallets for EVM chains.
                    </Paragraph>
                </div>

                <Tabs
                    defaultActiveKey="xpub"
                    items={tabItems}
                    onChange={() => resetState()}
                    size="large"
                />
            </div>
        </>
    );
};

export default WalletSetupPage;
