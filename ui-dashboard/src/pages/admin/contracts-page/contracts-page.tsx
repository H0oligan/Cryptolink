import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {
    Typography, Card, Button, notification, Space, Tag, Alert, Spin, Steps, Modal, Input, Divider,
} from "antd";
import {
    ThunderboltOutlined, CheckCircleOutlined, LoadingOutlined,
    ExclamationCircleOutlined, LinkOutlined, RocketOutlined,
} from "@ant-design/icons";
import adminProvider, {CollectorFactoryConfig} from "src/providers/admin-provider";
import {TRON_CHAIN} from "src/constants/merchant-collector";
import {
    isTronLinkAvailable, connectTronWallet,
    deployTronImplementation, deployTronFactory,
} from "src/utils/tron-wallet";
import {WarningOutlined} from "@ant-design/icons";

const {Title, Text, Paragraph} = Typography;

type DeployPhase = "idle" | "connecting" | "deploying-impl" | "deploying-factory" | "saving" | "done" | "error";

const PHASE_LABELS: Record<DeployPhase, string> = {
    idle: "",
    connecting: "Connecting to TronLink...",
    "deploying-impl": "Deploying Implementation contract...",
    "deploying-factory": "Deploying Clone Factory contract...",
    saving: "Saving to CryptoLink...",
    done: "Done!",
    error: "Failed",
};

const AdminContractsPage: React.FC = () => {
    const [api, contextHolder] = notification.useNotification();
    const [factories, setFactories] = React.useState<CollectorFactoryConfig[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [deploying, setDeploying] = React.useState(false);
    const [phase, setPhase] = React.useState<DeployPhase>("idle");
    const [error, setError] = React.useState("");
    const [implAddress, setImplAddress] = React.useState("");
    const [factoryAddress, setFactoryAddress] = React.useState("");
    const [resumeImplAddress, setResumeImplAddress] = React.useState("");

    const loadFactories = () => {
        setLoading(true);
        adminProvider.listCollectorFactories()
            .then(setFactories)
            .catch(() => {})
            .finally(() => setLoading(false));
    };

    React.useEffect(loadFactories, []);

    const tronFactory = factories.find((f) => f.blockchain === "TRON");

    const handleDeployTron = async (existingImplAddress?: string) => {
        setDeploying(true);
        setPhase("connecting");
        setError("");
        setImplAddress(existingImplAddress || "");
        setFactoryAddress("");

        try {
            // 1. Connect TronLink
            await connectTronWallet();

            let impl = existingImplAddress || "";

            if (!impl) {
                // 2. Deploy Implementation
                setPhase("deploying-impl");
                impl = await deployTronImplementation();
                setImplAddress(impl);
            }

            // 3. Deploy Factory
            setPhase("deploying-factory");
            const factory = await deployTronFactory(impl);
            setFactoryAddress(factory);

            // 4. Save to backend
            setPhase("saving");
            await adminProvider.upsertCollectorFactory({
                blockchain: "TRON",
                implementationAddress: impl,
                factoryAddress: factory,
            });

            setPhase("done");
            setResumeImplAddress("");
            loadFactories();

            setTimeout(() => {
                setDeploying(false);
                setPhase("idle");
            }, 3000);
        } catch (err: any) {
            setPhase("error");
            setError(err?.message || "Deployment failed");
        }
    };

    const stepItems = [
        {title: "Connect TronLink"},
        {title: "Deploy Implementation"},
        {title: "Deploy Factory"},
        {title: "Save Config"},
    ];

    const stepIndex: Record<DeployPhase, number> = {
        idle: -1, connecting: 0, "deploying-impl": 1, "deploying-factory": 2, saving: 3, done: 3, error: -1,
    };

    return (
        <PageContainer header={{title: "", breadcrumb: {}}}>
            {contextHolder}
            <Title>Smart Contract Factories</Title>
            <Paragraph type="secondary">
                Deploy clone factory contracts per blockchain. Merchants then deploy cheap proxy clones
                (~$0.50) instead of full contracts (~$16). Each factory only needs to be deployed once.
            </Paragraph>

            {loading ? (
                <Spin />
            ) : (
                <>
                    {/* TRON */}
                    <Card
                        title={
                            <Space>
                                <span style={{
                                    display: "inline-flex", alignItems: "center", justifyContent: "center",
                                    width: 28, height: 28, borderRadius: "50%", background: TRON_CHAIN.color,
                                }}>
                                    <ThunderboltOutlined style={{color: "#fff", fontSize: 12}} />
                                </span>
                                <Text strong style={{fontSize: 16}}>TRON</Text>
                                {tronFactory ? (
                                    <Tag color="green"><CheckCircleOutlined /> Deployed</Tag>
                                ) : (
                                    <Tag color="orange">Not deployed</Tag>
                                )}
                            </Space>
                        }
                        style={{maxWidth: 700, marginBottom: 24}}
                    >
                        {tronFactory ? (
                            <Space direction="vertical" size={12} style={{width: "100%"}}>
                                <div>
                                    <Text type="secondary" style={{fontSize: 12}}>Implementation Contract</Text>
                                    <div>
                                        <Text code copyable={{text: tronFactory.implementationAddress}} style={{fontSize: 12, wordBreak: "break-all"}}>
                                            {tronFactory.implementationAddress}
                                        </Text>
                                    </div>
                                </div>
                                <div>
                                    <Text type="secondary" style={{fontSize: 12}}>Clone Factory Contract</Text>
                                    <div>
                                        <Text code copyable={{text: tronFactory.factoryAddress}} style={{fontSize: 12, wordBreak: "break-all"}}>
                                            {tronFactory.factoryAddress}
                                        </Text>
                                    </div>
                                </div>
                                <div>
                                    <Text type="secondary" style={{fontSize: 12}}>
                                        Deployed: {new Date(tronFactory.createdAt).toLocaleString()}
                                    </Text>
                                </div>
                                <Alert
                                    message="Factory is active"
                                    description="Merchants can now deploy cheap proxy clones via TronLink from the Wallet Setup page."
                                    type="success"
                                    showIcon
                                />
                                <Alert
                                    message="Disable other TRON wallets"
                                    description="Disable Atomic Wallet and other TRON browser extensions before deploying. Only TronLink should be active."
                                    type="warning"
                                    showIcon
                                    icon={<WarningOutlined />}
                                    style={{marginTop: 4}}
                                />
                                <Button
                                    icon={<RocketOutlined />}
                                    disabled={!isTronLinkAvailable()}
                                    onClick={() => handleDeployTron()}
                                >
                                    Redeploy (replace existing)
                                </Button>
                            </Space>
                        ) : (
                            <Space direction="vertical" size={16} style={{width: "100%"}}>
                                <Alert
                                    message="Factory not deployed yet"
                                    description={
                                        <span>
                                            Deploy the Implementation + Factory contracts on TRON.
                                            This is a one-time setup that costs ~$16-20 total (paid by admin).
                                            After this, each merchant deploys a clone for ~$0.50-1.
                                        </span>
                                    }
                                    type="info"
                                    showIcon
                                />

                                {!isTronLinkAvailable() && (
                                    <Alert
                                        message="TronLink not detected"
                                        description="Install the TronLink browser extension to deploy contracts."
                                        type="warning"
                                        showIcon
                                        action={
                                            <Button size="small" href="https://www.tronlink.org/" target="_blank" icon={<LinkOutlined />}>
                                                Install TronLink
                                            </Button>
                                        }
                                    />
                                )}

                                <Alert
                                    message="Disable other TRON wallets before deploying"
                                    description={
                                        <span>
                                            If you have <strong>Atomic Wallet</strong> or any other TRON-compatible browser extension installed,
                                            you must <strong>disable it</strong> in Chrome (chrome://extensions) before deploying.
                                            Multiple TRON wallets conflict with each other and the wrong wallet will be used.
                                            Only TronLink should be active.
                                        </span>
                                    }
                                    type="warning"
                                    showIcon
                                    icon={<WarningOutlined />}
                                />

                                <Button
                                    type="primary"
                                    size="large"
                                    icon={<RocketOutlined />}
                                    disabled={!isTronLinkAvailable()}
                                    onClick={() => handleDeployTron()}
                                >
                                    Deploy TRON Factory
                                </Button>

                                <Divider plain style={{margin: "8px 0", fontSize: 12}}>
                                    Or resume a failed deployment
                                </Divider>
                                <Space.Compact style={{width: "100%", maxWidth: 500}}>
                                    <Input
                                        placeholder="Existing Implementation address (e.g. THnS5h...)"
                                        value={resumeImplAddress}
                                        onChange={(e) => setResumeImplAddress(e.target.value.trim())}
                                        style={{fontFamily: "monospace", fontSize: 12}}
                                    />
                                    <Button
                                        icon={<RocketOutlined />}
                                        disabled={!isTronLinkAvailable() || !resumeImplAddress}
                                        onClick={() => handleDeployTron(resumeImplAddress)}
                                    >
                                        Deploy Factory Only
                                    </Button>
                                </Space.Compact>
                            </Space>
                        )}
                    </Card>
                </>
            )}

            {/* Deployment progress modal */}
            <Modal
                title="Deploying TRON Factory Contracts"
                open={deploying}
                footer={
                    phase === "error" ? (
                        <Space>
                            {implAddress && !factoryAddress && (
                                <Button type="primary" icon={<RocketOutlined />} onClick={() => {
                                    setDeploying(false);
                                    setPhase("idle");
                                    handleDeployTron(implAddress);
                                }}>
                                    Resume — Deploy Factory Only
                                </Button>
                            )}
                            <Button onClick={() => {setDeploying(false); setPhase("idle");}}>
                                Close
                            </Button>
                        </Space>
                    ) : phase === "done" ? (
                        <Button onClick={() => {setDeploying(false); setPhase("idle");}}>
                            Close
                        </Button>
                    ) : null
                }
                closable={phase === "error"}
                onCancel={() => {setDeploying(false); setPhase("idle");}}
                width={520}
                maskClosable={false}
            >
                {phase !== "error" && phase !== "idle" && (
                    <Steps
                        size="small"
                        current={stepIndex[phase]}
                        status={phase === "done" ? "finish" : "process"}
                        items={stepItems}
                        style={{marginBottom: 24}}
                    />
                )}

                <div style={{textAlign: "center", padding: "16px 0"}}>
                    {phase === "done" ? (
                        <Space direction="vertical" size={8}>
                            <CheckCircleOutlined style={{fontSize: 40, color: "#52c41a"}} />
                            <Text strong style={{fontSize: 16}}>Factory deployed successfully!</Text>
                            {implAddress && (
                                <div>
                                    <Text type="secondary" style={{fontSize: 11}}>Implementation:</Text>
                                    <br />
                                    <Text code style={{fontSize: 11}}>{implAddress}</Text>
                                </div>
                            )}
                            {factoryAddress && (
                                <div>
                                    <Text type="secondary" style={{fontSize: 11}}>Factory:</Text>
                                    <br />
                                    <Text code style={{fontSize: 11}}>{factoryAddress}</Text>
                                </div>
                            )}
                        </Space>
                    ) : phase === "error" ? (
                        <Space direction="vertical" size={8}>
                            <ExclamationCircleOutlined style={{fontSize: 40, color: "#ff4d4f"}} />
                            <Text strong style={{fontSize: 16}}>Deployment failed</Text>
                            <Text type="secondary">{error}</Text>
                            {implAddress && (
                                <div>
                                    <Text type="secondary" style={{fontSize: 11}}>
                                        Implementation was deployed at: {implAddress}
                                    </Text>
                                </div>
                            )}
                        </Space>
                    ) : (
                        <Space direction="vertical" size={8}>
                            <Spin indicator={<LoadingOutlined style={{fontSize: 32}} spin />} />
                            <Text type="secondary">{PHASE_LABELS[phase]}</Text>
                            {(phase === "deploying-impl" || phase === "deploying-factory") && (
                                <Text type="secondary" style={{fontSize: 12}}>
                                    Confirm the transaction in TronLink and wait for on-chain confirmation (up to 90s)...
                                </Text>
                            )}
                        </Space>
                    )}
                </div>
            </Modal>
        </PageContainer>
    );
};

export default AdminContractsPage;
