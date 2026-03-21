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
    LinkOutlined,
    WarningOutlined,
    QuestionCircleOutlined,
    CopyOutlined,
    CloseCircleOutlined
} from "@ant-design/icons";
import bevis from "src/utils/bevis";
import xpubProvider, {XpubWallet} from "src/providers/xpub-provider";
import evmCollectorProvider, {EvmCollector} from "src/providers/evm-collector-provider";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import {EVM_CHAINS, TRON_CHAIN, type EvmChainConfig} from "src/constants/merchant-collector";
import {isMetaMaskAvailable, connectWallet, switchChain, deployCollector} from "src/utils/evm-wallet";
import {isTronLinkAvailable, connectTronWallet, deployTronCloneViaFactory} from "src/utils/tron-wallet";

const b = bevis("wallet-setup-page");

const {Title, Text, Paragraph} = Typography;

// xpub-only blockchains — BTC only (TRON uses collector address approach)
const XPUB_BLOCKCHAINS = [
    {value: "BTC",  label: "Bitcoin", path: "m/84'/0'/0'",   color: "#F7931A"},
];

// Detect key format from prefix (SLIP-0132 version bytes)
function detectKeyFormat(key: string): {format: string; label: string; path: string} | null {
    const trimmed = key.trim();
    if (trimmed.startsWith("zpub")) return {format: "p2wpkh", label: "Native SegWit (zpub) — bc1q addresses", path: "m/84'/0'/0'"};
    if (trimmed.startsWith("ypub")) return {format: "p2sh-segwit", label: "SegWit (ypub) — 3-prefix addresses", path: "m/49'/0'/0'"};
    if (trimmed.startsWith("xpub")) return {format: "p2pkh", label: "Legacy (xpub) — 1-prefix addresses", path: "m/44'/0'/0'"};
    if (trimmed.startsWith("vpub")) return {format: "p2wpkh", label: "Native SegWit Testnet (vpub)", path: "m/84'/1'/0'"};
    if (trimmed.startsWith("upub")) return {format: "p2sh-segwit", label: "SegWit Testnet (upub)", path: "m/49'/1'/0'"};
    if (trimmed.startsWith("tpub")) return {format: "p2pkh", label: "Legacy Testnet (tpub)", path: "m/44'/1'/0'"};
    return null;
}

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
                destroyOnClose
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
// TRON Collector Panel — deploy MerchantCollector via TronLink
// ============================================================

type TronDeployStep = "idle" | "connecting" | "deploying" | "registering" | "done" | "error";

const TRON_STEP_LABELS: Record<TronDeployStep, string> = {
    idle: "",
    connecting: "Connecting to TronLink...",
    deploying: "Deploying contract on TRON...",
    registering: "Registering with CryptoLink...",
    done: "Done!",
    error: "Failed",
};

const TronCollectorPanel: React.FC<{
    merchantId: string;
    collectors: EvmCollector[];
    onRefresh: () => void;
}> = ({merchantId, collectors, onRefresh}) => {
    const [api, contextHolder] = notification.useNotification();
    const [isDeleting, setIsDeleting] = React.useState(false);
    const [deploying, setDeploying] = React.useState(false);
    const [deployState, setDeployState] = React.useState<{step: TronDeployStep; error?: string; contractAddress?: string}>({step: "idle"});

    const tronCollector = collectors.find((c) => c.blockchain === "TRON");

    const handleDeploy = async () => {
        setDeploying(true);
        setDeployState({step: "connecting"});
        try {
            // 1. Connect TronLink
            const ownerAddress = await connectTronWallet();

            // 2. Fetch factory address from backend
            const factoryConfig = await evmCollectorProvider.getCollectorFactory(merchantId, "TRON");

            // 3. Deploy clone via factory (cheap!)
            setDeployState({step: "deploying"});
            const contractAddress = await deployTronCloneViaFactory(factoryConfig.factoryAddress, ownerAddress);

            // 4. Register with backend
            setDeployState({step: "registering"});
            await evmCollectorProvider.setupCollector(merchantId, {
                blockchain: "TRON",
                ownerAddress: ownerAddress,
                contractAddress: contractAddress,
                chainId: 728126428,
                factoryAddress: factoryConfig.factoryAddress,
            });

            setDeployState({step: "done", contractAddress});
            onRefresh();

            setTimeout(() => {
                setDeploying(false);
                setDeployState({step: "idle"});
            }, 2000);
        } catch (err: any) {
            const msg = err?.message || "Deployment failed. Please try again.";
            if (msg.includes("not found") || msg.includes("404")) {
                setDeployState({step: "error", error: "TRON factory not deployed yet. Please contact the administrator."});
            } else {
                setDeployState({step: "error", error: msg});
            }
        }
    };

    const handleDelete = () => {
        Modal.confirm({
            title: "Remove TRON Collector Contract",
            icon: <ExclamationCircleOutlined />,
            content: (
                <div>
                    <p>Remove the TRON MerchantCollector registration?</p>
                    <p style={{color: "#ff4d4f", marginTop: 8}}>
                        This only removes the registration from CryptoLink. The contract on-chain is not
                        affected — your funds remain accessible via TronLink at any time.
                    </p>
                </div>
            ),
            okText: "Remove",
            okType: "danger",
            cancelText: "Cancel",
            onOk: async () => {
                setIsDeleting(true);
                try {
                    await evmCollectorProvider.deleteCollector(merchantId, "TRON");
                    api.success({message: "TRON collector removed", placement: "bottomRight"});
                    onRefresh();
                } catch (err: any) {
                    api.error({message: "Failed to remove", description: err.response?.data?.message || "Please try again", placement: "bottomRight"});
                } finally {
                    setIsDeleting(false);
                }
            },
        });
    };

    const deployStepItems = [
        {title: "Connect TronLink"},
        {title: "Deploy Contract"},
        {title: "Register"},
    ];

    const deployStepIndex: Record<TronDeployStep, number> = {
        idle: -1, connecting: 0, deploying: 1, registering: 2, done: 2, error: -1,
    };

    return (
        <>
            {contextHolder}
            <Divider />
            <Title level={4} style={{marginBottom: 16}}>
                <span style={{
                    display: "inline-flex", alignItems: "center", justifyContent: "center",
                    width: 28, height: 28, borderRadius: "50%", background: TRON_CHAIN.color,
                    marginRight: 8, verticalAlign: "middle",
                }}>
                    <ThunderboltOutlined style={{color: "#fff", fontSize: 12}} />
                </span>
                TRON (TRX & TRC-20 Tokens)
            </Title>

            <Alert
                message="Non-custodial TRON Smart Contract"
                description={
                    <ul style={{margin: "8px 0 0 0", paddingLeft: 20}}>
                        <li>Deploys the same <strong>MerchantCollector</strong> contract on TRON via TronLink — customers pay to the contract address.</li>
                        <li>You call <strong>withdrawAll()</strong> from the Balances page to sweep all TRX & TRC-20 tokens to your wallet.</li>
                        <li>CryptoLink has <strong>zero admin access</strong> — only your TronLink wallet (the owner) can withdraw.</li>
                        <li>Click <strong>"Deploy with TronLink"</strong> and confirm the transaction in the TronLink popup.</li>
                    </ul>
                }
                type="info"
                showIcon
                icon={<SafetyCertificateOutlined />}
                style={{marginBottom: 16}}
            />

            {!isTronLinkAvailable() && (
                <Alert
                    message="TronLink not detected"
                    description="Install the TronLink browser extension to deploy a TRON smart contract wallet."
                    type="warning"
                    showIcon
                    style={{marginBottom: 16}}
                    action={
                        <Button size="small" href="https://www.tronlink.org/" target="_blank" icon={<LinkOutlined />}>
                            Install TronLink
                        </Button>
                    }
                />
            )}

            <Alert
                message="Disable other TRON wallets"
                description="If you have Atomic Wallet or other TRON browser extensions, disable them in chrome://extensions before deploying. Only TronLink should be active."
                type="warning"
                showIcon
                icon={<WarningOutlined />}
                style={{marginBottom: 16}}
            />

            {tronCollector ? (
                <Card
                    size="small"
                    style={{borderColor: "#10b981", background: "rgba(16,185,129,0.08)", maxWidth: 500}}
                >
                    <Space direction="vertical" size={8} style={{width: "100%"}}>
                        <Space>
                            <Tag color="green"><CheckCircleOutlined /> Active</Tag>
                            <Text strong>TRON</Text>
                        </Space>
                        <div>
                            <Text type="secondary" style={{fontSize: 11}}>Contract</Text>
                            <div>
                                <Tooltip title={tronCollector.contractAddress}>
                                    <Text
                                        code
                                        copyable={{text: tronCollector.contractAddress}}
                                        style={{fontSize: 11}}
                                    >
                                        {tronCollector.contractAddress.slice(0, 10)}...{tronCollector.contractAddress.slice(-8)}
                                    </Text>
                                </Tooltip>
                            </div>
                        </div>
                        <div>
                            <Text type="secondary" style={{fontSize: 11}}>Owner</Text>
                            <div>
                                <Tooltip title={tronCollector.ownerAddress}>
                                    <Text code style={{fontSize: 11}}>
                                        {tronCollector.ownerAddress.slice(0, 10)}...{tronCollector.ownerAddress.slice(-8)}
                                    </Text>
                                </Tooltip>
                            </div>
                        </div>
                        <Button
                            danger
                            size="small"
                            icon={<DeleteOutlined />}
                            loading={isDeleting}
                            onClick={handleDelete}
                            style={{marginTop: 4}}
                        >
                            Remove
                        </Button>
                    </Space>
                </Card>
            ) : (
                <Button
                    type="primary"
                    icon={<ThunderboltOutlined />}
                    disabled={!isTronLinkAvailable()}
                    onClick={handleDeploy}
                    size="large"
                >
                    Deploy with TronLink
                </Button>
            )}

            {/* Deployment progress modal */}
            <Modal
                title="Deploying TRON Collector Contract"
                open={deploying}
                destroyOnClose
                footer={
                    deployState.step === "error" || deployState.step === "done" ? (
                        <Button onClick={() => {setDeploying(false); setDeployState({step: "idle"});}}>
                            Close
                        </Button>
                    ) : null
                }
                closable={deployState.step === "error"}
                onCancel={() => {setDeploying(false); setDeployState({step: "idle"});}}
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
                            <Text type="secondary">{TRON_STEP_LABELS[deployState.step]}</Text>
                            {deployState.step === "deploying" && (
                                <Text type="secondary" style={{fontSize: 12}}>
                                    Please confirm the transaction in TronLink and wait for it to be confirmed on-chain...
                                    This may take up to 90 seconds.
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

    const [mode, setMode] = React.useState<"overview" | "import" | "verify" | "detail">("overview");
    const [selectedBlockchain, setSelectedBlockchain] = React.useState<string>("");
    const [importXpub, setImportXpub] = React.useState<string>("");
    const [importPath, setImportPath] = React.useState<string>("");
    const [detectedFormat, setDetectedFormat] = React.useState<{format: string; label: string; path: string} | null>(null);
    const [isSubmitting, setIsSubmitting] = React.useState(false);
    const [existingWallets, setExistingWallets] = React.useState<XpubWallet[]>([]);
    const [collectors, setCollectors] = React.useState<EvmCollector[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [selectedWallet, setSelectedWallet] = React.useState<XpubWallet | null>(null);
    const [isDeleting, setIsDeleting] = React.useState(false);
    const [firstAddress, setFirstAddress] = React.useState<string>("");
    const [loadingAddress, setLoadingAddress] = React.useState(false);

    // Verify mode state
    const [verifyWallet, setVerifyWallet] = React.useState<XpubWallet | null>(null);
    const [verifyAddress, setVerifyAddress] = React.useState<string>("");
    const [verifyLoading, setVerifyLoading] = React.useState(false);
    const [showMismatchHelp, setShowMismatchHelp] = React.useState(false);

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
                xpub: importXpub.trim(),
                ...(importPath ? {derivationPath: importPath} : {}),
            });

            // Refresh wallet list and find the new wallet
            const wallets = await xpubProvider.listXpubWallets(merchantId);
            setExistingWallets(wallets || []);
            const newWallet = (wallets || []).find((w) => w.blockchain === selectedBlockchain);

            if (newWallet) {
                // Transition to verify mode
                setVerifyWallet(newWallet);
                setMode("verify");
                setVerifyLoading(true);
                setShowMismatchHelp(false);
                try {
                    const derived = await xpubProvider.deriveAddress(merchantId, newWallet.uuid);
                    setVerifyAddress(derived.address);
                } catch {
                    setVerifyAddress("");
                } finally {
                    setVerifyLoading(false);
                }
            } else {
                api.success({
                    message: "Wallet imported!",
                    description: `${blockchain?.label} wallet has been configured`,
                    placement: "bottomRight",
                });
                resetState();
            }
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
        setDetectedFormat(null);
        setSelectedWallet(null);
        setFirstAddress("");
        setVerifyWallet(null);
        setVerifyAddress("");
        setVerifyLoading(false);
        setShowMismatchHelp(false);
    };

    const renderOverview = () => (
        <>
            <Card style={{marginBottom: 24, borderLeft: "4px solid #10b981"}}>
                <Space direction="vertical" size={8}>
                    <Space>
                        <SafetyCertificateOutlined style={{fontSize: 20, color: "#10b981"}} />
                        <Title level={5} style={{margin: 0}}>HD Wallets — How it Works</Title>
                    </Space>
                    <Paragraph type="secondary" style={{margin: 0}}>
                        CryptoLink uses <strong>HD wallets (BIP32/BIP84)</strong> so customers pay directly to
                        your wallet. We store your <strong>extended public key (xpub/zpub)</strong> to derive
                        unique payment addresses. Your private keys never leave your device.
                    </Paragraph>
                    <Space size={16} wrap>
                        <Tag icon={<LockOutlined />} color="green">Private keys stay local</Tag>
                        <Tag icon={<WalletOutlined />} color="blue">Direct-to-wallet payments</Tag>
                        <Tag color="default">Import xpub/zpub — no key generation</Tag>
                    </Space>
                </Space>
            </Card>

            <Title level={4}>Supported Networks</Title>
            <Paragraph type="secondary" style={{marginBottom: 16}}>
                xpub HD wallets are supported for Bitcoin. For EVM networks and TRON,
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
                Import Extended Public Key
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
                <Title level={4} style={{margin: 0}}>Import Extended Public Key</Title>
            </Space>
            <Paragraph type="secondary">
                Import your HD wallet's extended public key (xpub, ypub, or zpub). Works with hardware
                wallets (Ledger, Trezor), Exodus, Electrum, and any BIP32/BIP84 compatible wallet.
                Your private keys never leave your device.
            </Paragraph>

            <Alert
                message="Need help extracting your key?"
                description={
                    <span>
                        Download our{" "}
                        <a href="/xpub-extractor.html" download="xpub-extractor.html" style={{fontWeight: "bold"}}>
                            Offline Key Extractor Tool
                        </a>{" "}
                        — a single HTML file you run locally on your computer. It converts your seed phrase into
                        an extended public key entirely offline. Your seed phrase never leaves your machine.
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
                        if (!detectedFormat) {
                            setImportPath(bc?.path || "");
                        }
                    }}
                    size="large"
                />
            </div>

            <div style={{marginBottom: 16}}>
                <Text strong>Extended Public Key (xpub / ypub / zpub)</Text>
                <Input.TextArea
                    rows={3}
                    placeholder="zpub6rFR7y4Q2A... or xpub6CatW... or ypub6X..."
                    value={importXpub}
                    onChange={(e) => {
                        const val = e.target.value;
                        setImportXpub(val);
                        const detected = detectKeyFormat(val);
                        setDetectedFormat(detected);
                        if (detected) {
                            setImportPath(detected.path);
                        }
                    }}
                    style={{marginTop: 4, fontFamily: "monospace"}}
                />
                {detectedFormat && (
                    <div style={{marginTop: 8}}>
                        <Tag color="green" style={{fontSize: 13, padding: "2px 10px"}}>
                            Detected: {detectedFormat.label}
                        </Tag>
                    </div>
                )}
                <Text type="secondary" style={{fontSize: 12, display: "block", marginTop: 4}}>
                    Paste the key exported from your wallet. The address format (SegWit, Legacy) is auto-detected
                    from the key prefix. zpub = Native SegWit (recommended), ypub = SegWit, xpub = Legacy.
                </Text>
            </div>

            <div style={{marginBottom: 16}}>
                <Text strong>Derivation Path <Text type="secondary" style={{fontWeight: 400}}>(auto-detected)</Text></Text>
                <Input
                    value={importPath}
                    onChange={(e) => setImportPath(e.target.value)}
                    placeholder="m/84'/0'/0'"
                    disabled={!!detectedFormat}
                    style={{marginTop: 4, fontFamily: "monospace"}}
                />
                {detectedFormat && (
                    <Text type="secondary" style={{fontSize: 12, display: "block", marginTop: 4}}>
                        Path auto-detected from key format.
                    </Text>
                )}
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

            <Divider />

            {/* Educational section — Hardware Wallet Guide */}
            <details style={{cursor: "pointer"}}>
                <summary style={{fontWeight: 600, color: "var(--cl-text-primary)", marginBottom: 12}}>
                    <QuestionCircleOutlined style={{marginRight: 8}} />
                    How to export your extended public key (per wallet)
                </summary>
                <div style={{padding: "12px 0 0 8px"}}>
                    <Space direction="vertical" size={12} style={{width: "100%"}}>
                        <Card size="small" style={{borderLeft: "3px solid #F7931A"}}>
                            <Text strong>Exodus</Text>
                            <br />
                            <Text type="secondary">
                                Settings &rarr; Developer Menu &rarr; Export xpub for Bitcoin.
                                Exodus exports all three formats (xpub, ypub, zpub). Use the <strong>zpub</strong> for
                                Native SegWit (bc1q addresses, recommended).
                            </Text>
                        </Card>
                        <Card size="small" style={{borderLeft: "3px solid #3B99FC"}}>
                            <Text strong>Electrum</Text>
                            <br />
                            <Text type="secondary">
                                Wallet &rarr; Information &rarr; Master Public Key.
                                Electrum exports <strong>zpub</strong> for Native SegWit wallets (default).
                            </Text>
                        </Card>
                        <Card size="small" style={{borderLeft: "3px solid #41B883"}}>
                            <Text strong>Ledger (via Ledger Live)</Text>
                            <br />
                            <Text type="secondary">
                                Account &rarr; Wrench icon &rarr; Advanced &rarr; Extended Public Key.
                                Ledger Live exports as <strong>xpub</strong> regardless of address format.
                                CryptoLink will detect it as Legacy (1-prefix). If you use Native SegWit (bc1q),
                                export via Electrum or Sparrow instead to get a <strong>zpub</strong>.
                            </Text>
                        </Card>
                        <Card size="small" style={{borderLeft: "3px solid #1A9B2D"}}>
                            <Text strong>Trezor (via Trezor Suite)</Text>
                            <br />
                            <Text type="secondary">
                                Account &rarr; Show Public Key. Trezor Suite correctly exports <strong>zpub</strong> for
                                Native SegWit accounts.
                            </Text>
                        </Card>
                        <Card size="small" style={{borderLeft: "3px solid #E74C3C"}}>
                            <Text strong>Coldcard</Text>
                            <br />
                            <Text type="secondary">
                                Settings &rarr; Advanced &rarr; Export &rarr; XPUB.
                                Coldcard uses correct SLIP-0132 prefixes (zpub for BIP84).
                            </Text>
                        </Card>
                        <Card size="small" style={{borderLeft: "3px solid #10b981"}}>
                            <Text strong>CryptoLink Offline Extractor</Text>
                            <br />
                            <Text type="secondary">
                                Download <a href="/xpub-extractor.html" target="_blank" rel="noopener noreferrer">xpub-extractor.html</a>,
                                disconnect from internet, enter your seed phrase. Outputs zpub (BIP84) or ypub (BIP49).
                                Your seed phrase never leaves your computer.
                            </Text>
                        </Card>
                    </Space>

                    <Alert
                        message="Which format should I use?"
                        description={
                            <>
                                <strong>zpub</strong> (Native SegWit / bc1q addresses) is recommended. It has the lowest
                                transaction fees and is the default in modern wallets. If your wallet only exports <strong>xpub</strong>,
                                CryptoLink will treat it as Legacy (1-prefix addresses). After import, verify that the generated
                                address matches your wallet.
                            </>
                        }
                        type="info"
                        showIcon
                        style={{marginTop: 16}}
                    />
                </div>
            </details>
        </Card>
    );

    const renderVerifyMode = () => {
        if (!verifyWallet) return null;
        const chain = XPUB_BLOCKCHAINS.find((c) => c.value === verifyWallet.blockchain);

        const handleVerifyDelete = async () => {
            if (!merchantId || !verifyWallet) return;
            try {
                await xpubProvider.deleteXpubWallet(merchantId, verifyWallet.uuid);
                const wallets = await xpubProvider.listXpubWallets(merchantId);
                setExistingWallets(wallets || []);
                setVerifyWallet(null);
                setVerifyAddress("");
                setShowMismatchHelp(false);
                setMode("import");
                api.info({message: "Wallet removed. Please re-import with the correct key format.", placement: "bottomRight"});
            } catch {
                api.error({message: "Failed to delete wallet", placement: "bottomRight"});
            }
        };

        return (
            <Card>
                <Space style={{marginBottom: 24}} align="center">
                    <SafetyCertificateOutlined style={{fontSize: 24, color: "#10b981"}} />
                    <Title level={4} style={{margin: 0}}>Verify Your Wallet</Title>
                </Space>

                <Alert
                    message="Wallet imported successfully"
                    description={`Your ${chain?.label || "Bitcoin"} extended public key has been saved. Now verify the address below matches your wallet software.`}
                    type="success"
                    showIcon
                    style={{marginBottom: 24}}
                />

                {verifyLoading ? (
                    <div style={{textAlign: "center", padding: "40px 0"}}>
                        <Spin size="large" />
                        <div style={{marginTop: 16}}>
                            <Text type="secondary">Deriving your first receive address...</Text>
                        </div>
                    </div>
                ) : verifyAddress ? (
                    <>
                        <div style={{
                            background: "var(--cl-bg-container, #0a0a0a)",
                            border: "2px solid #10b981",
                            borderRadius: 8,
                            padding: "20px 24px",
                            textAlign: "center",
                            marginBottom: 24,
                            boxShadow: "0 0 20px rgba(16,185,129,0.15)",
                        }}>
                            <Text type="secondary" style={{display: "block", marginBottom: 8, fontSize: 12, textTransform: "uppercase", letterSpacing: 1}}>
                                First Receive Address (index 0)
                            </Text>
                            <Text
                                copyable
                                style={{
                                    fontFamily: "monospace",
                                    fontSize: 15,
                                    wordBreak: "break-all",
                                    lineHeight: 1.6,
                                }}
                            >
                                {verifyAddress}
                            </Text>
                        </div>

                        <Alert
                            message="How to verify"
                            description={
                                <>
                                    Open your wallet software (Exodus, Electrum, Ledger Live, Trezor Suite, etc.)
                                    and navigate to the <strong>first receive address</strong>.
                                    It should <strong>exactly match</strong> the address shown above.
                                    If it matches, your wallet is correctly configured and payments will go
                                    directly to your wallet.
                                </>
                            }
                            type="info"
                            showIcon
                            style={{marginBottom: 24}}
                        />

                        <Space direction="vertical" size={12} style={{width: "100%"}}>
                            <Button
                                type="primary"
                                size="large"
                                block
                                icon={<CheckCircleOutlined />}
                                onClick={() => {
                                    api.success({message: "Wallet verified!", description: `${chain?.label} wallet is ready to receive payments.`, placement: "bottomRight"});
                                    resetState();
                                }}
                            >
                                Addresses Match — Done
                            </Button>
                            <Button
                                size="large"
                                block
                                danger
                                icon={<CloseCircleOutlined />}
                                onClick={() => setShowMismatchHelp(true)}
                            >
                                Addresses Don&apos;t Match
                            </Button>
                        </Space>

                        {showMismatchHelp && (
                            <div style={{marginTop: 24}}>
                                <Alert
                                    message="Address mismatch — likely a key format issue"
                                    description={
                                        <Space direction="vertical" size={8}>
                                            <Text>
                                                This usually happens when your wallet exports an <strong>xpub</strong>-prefixed key
                                                but your wallet actually uses <strong>Native SegWit (bc1q)</strong> addresses.
                                                Some hardware wallets (notably <strong>Ledger Live</strong>) export all keys as <code>xpub</code>,
                                                even for SegWit accounts.
                                            </Text>
                                            <Text>
                                                <strong>Solution:</strong> Re-export your key as a <strong>zpub</strong> (for Native SegWit / bc1q)
                                                or <strong>ypub</strong> (for SegWit / 3-prefix addresses). You can do this by:
                                            </Text>
                                            <ul style={{margin: "4px 0", paddingLeft: 20}}>
                                                <li>Opening your Bitcoin wallet in <strong>Electrum</strong> or <strong>Sparrow Wallet</strong> (both export zpub correctly)</li>
                                                <li>Using CryptoLink's <a href="/xpub-extractor.html" target="_blank" rel="noopener noreferrer">Offline Key Extractor</a> tool</li>
                                                <li>Checking if your wallet software has a "show zpub" option in advanced settings</li>
                                            </ul>
                                            <Text type="secondary" style={{fontSize: 12}}>
                                                Technical detail: CryptoLink uses the key's 4-byte version prefix (SLIP-0132) to determine the address
                                                format. <code>zpub</code> = BIP84 (bc1q), <code>ypub</code> = BIP49 (3-prefix), <code>xpub</code> = BIP44 (1-prefix).
                                            </Text>
                                        </Space>
                                    }
                                    type="warning"
                                    showIcon
                                    style={{marginBottom: 16}}
                                />
                                <Space>
                                    <Button
                                        danger
                                        icon={<DeleteOutlined />}
                                        onClick={handleVerifyDelete}
                                    >
                                        Delete &amp; Re-import
                                    </Button>
                                    <Button onClick={() => {
                                        api.info({message: "Wallet kept as-is", placement: "bottomRight"});
                                        resetState();
                                    }}>
                                        Keep Anyway
                                    </Button>
                                </Space>
                            </div>
                        )}
                    </>
                ) : (
                    <Alert
                        message="Could not derive verification address"
                        description="The wallet was saved successfully but we couldn't derive a test address. You can verify later in the wallet details view."
                        type="warning"
                        showIcon
                        action={<Button onClick={resetState}>Continue</Button>}
                        style={{marginBottom: 16}}
                    />
                )}
            </Card>
        );
    };

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
                    {mode === "verify" && renderVerifyMode()}
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
                        <>
                            <EvmCollectorPanel
                                merchantId={merchantId}
                                collectors={collectors}
                                onRefresh={loadData}
                            />
                            <TronCollectorPanel
                                merchantId={merchantId}
                                collectors={collectors}
                                onRefresh={loadData}
                            />
                        </>
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
                        HD wallets (xpub) for Bitcoin — Smart Contracts for EVM chains & TRON (deployed via MetaMask/TronLink).
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
