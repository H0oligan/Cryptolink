import "./wallet-setup-page.scss";

import * as React from "react";
import {useNavigate} from "react-router-dom";
import {
    Button,
    Typography,
    Steps,
    Card,
    Select,
    Alert,
    Input,
    Checkbox,
    notification,
    Space,
    Tag
} from "antd";
import {CopyOutlined, CheckCircleOutlined, WarningOutlined} from "@ant-design/icons";
import * as bip39 from "bip39";
import {HDKey} from "@scure/bip32";
import bevis from "src/utils/bevis";
import xpubProvider from "src/providers/xpub-provider";
import useSharedMerchantId from "src/hooks/use-merchant-id";

const b = bevis("wallet-setup-page");

const {Title, Text, Paragraph} = Typography;

// Supported blockchains with their derivation paths
const BLOCKCHAINS = [
    {value: "ETH", label: "Ethereum", path: "m/44'/60'/0'/0"},
    {value: "MATIC", label: "Polygon", path: "m/44'/60'/0'/0"},
    {value: "BSC", label: "BNB Smart Chain", path: "m/44'/60'/0'/0"},
    {value: "ARBITRUM", label: "Arbitrum", path: "m/44'/60'/0'/0"},
    {value: "AVAX", label: "Avalanche", path: "m/44'/60'/0'/0"},
    {value: "BTC", label: "Bitcoin", path: "m/44'/0'/0'/0"},
    {value: "TRON", label: "TRON", path: "m/44'/195'/0'/0"}
];

const WalletSetupPage: React.FC = () => {
    const navigate = useNavigate();
    const [api, contextHolder] = notification.useNotification();
    const {merchantId} = useSharedMerchantId();

    const [currentStep, setCurrentStep] = React.useState(0);
    const [selectedBlockchain, setSelectedBlockchain] = React.useState<string>("");
    const [mnemonic, setMnemonic] = React.useState<string>("");
    const [xpub, setXpub] = React.useState<string>("");
    const [hasBackedUp, setHasBackedUp] = React.useState(false);
    const [isSubmitting, setIsSubmitting] = React.useState(false);
    const [verificationWord, setVerificationWord] = React.useState("");
    const [verificationIndex, setVerificationIndex] = React.useState(0);

    // Generate mnemonic when blockchain is selected
    const handleBlockchainSelect = (value: string) => {
        setSelectedBlockchain(value);
        // Generate 24-word mnemonic
        const newMnemonic = bip39.generateMnemonic(256);
        setMnemonic(newMnemonic);

        // Derive xpub from mnemonic
        const seed = bip39.mnemonicToSeedSync(newMnemonic);
        const blockchain = BLOCKCHAINS.find(b => b.value === value);
        if (blockchain) {
            const hdkey = HDKey.fromMasterSeed(seed);
            const derived = hdkey.derive(blockchain.path);
            setXpub(derived.publicExtendedKey);
        }

        // Set random word for verification
        const words = newMnemonic.split(" ");
        const randomIndex = Math.floor(Math.random() * words.length);
        setVerificationIndex(randomIndex);

        setCurrentStep(1);
    };

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text);
        api.success({
            message: "Copied to clipboard",
            placement: "bottomRight"
        });
    };

    const handleVerifyBackup = () => {
        const words = mnemonic.split(" ");
        if (verificationWord.toLowerCase().trim() === words[verificationIndex].toLowerCase()) {
            setCurrentStep(3);
        } else {
            api.error({
                message: "Incorrect word",
                description: `Please enter word #${verificationIndex + 1} from your recovery phrase`,
                placement: "bottomRight"
            });
        }
    };

    const handleSubmit = async () => {
        if (!merchantId || !selectedBlockchain || !xpub) return;

        setIsSubmitting(true);
        try {
            const blockchain = BLOCKCHAINS.find(b => b.value === selectedBlockchain);
            await xpubProvider.createXpubWallet(merchantId, {
                blockchain: selectedBlockchain,
                xpub: xpub,
                derivationPath: blockchain?.path || ""
            });

            api.success({
                message: "Wallet setup complete!",
                description: "Your non-custodial wallet has been configured",
                placement: "bottomRight"
            });

            // Navigate to settings or dashboard
            setTimeout(() => navigate("/settings"), 1500);
        } catch (error: any) {
            api.error({
                message: "Failed to setup wallet",
                description: error.response?.data?.error || "Please try again",
                placement: "bottomRight"
            });
        } finally {
            setIsSubmitting(false);
        }
    };

    const renderStepContent = () => {
        switch (currentStep) {
            case 0:
                return (
                    <Card className={b("step-card")}>
                        <Title level={4}>Select Blockchain</Title>
                        <Paragraph>
                            Choose which blockchain you want to accept payments on.
                            You can setup multiple blockchains later.
                        </Paragraph>
                        <Select
                            style={{width: "100%"}}
                            placeholder="Select blockchain"
                            options={BLOCKCHAINS}
                            onChange={handleBlockchainSelect}
                            size="large"
                        />
                    </Card>
                );

            case 1:
                return (
                    <Card className={b("step-card")}>
                        <Alert
                            message="IMPORTANT: Backup Your Recovery Phrase"
                            description="Write down these 24 words in order. This is the ONLY way to recover your wallet. Never share it with anyone!"
                            type="warning"
                            icon={<WarningOutlined />}
                            showIcon
                            style={{marginBottom: 24}}
                        />

                        <div className={b("mnemonic-grid")}>
                            {mnemonic.split(" ").map((word, index) => (
                                <div key={index} className={b("mnemonic-word")}>
                                    <span className={b("word-number")}>{index + 1}</span>
                                    <span className={b("word-text")}>{word}</span>
                                </div>
                            ))}
                        </div>

                        <Space style={{marginTop: 24}}>
                            <Button
                                icon={<CopyOutlined />}
                                onClick={() => copyToClipboard(mnemonic)}
                            >
                                Copy to Clipboard
                            </Button>
                        </Space>

                        <div style={{marginTop: 24}}>
                            <Checkbox
                                checked={hasBackedUp}
                                onChange={(e) => setHasBackedUp(e.target.checked)}
                            >
                                I have written down my recovery phrase and stored it safely
                            </Checkbox>
                        </div>

                        <Button
                            type="primary"
                            disabled={!hasBackedUp}
                            onClick={() => setCurrentStep(2)}
                            style={{marginTop: 16}}
                        >
                            Continue
                        </Button>
                    </Card>
                );

            case 2:
                return (
                    <Card className={b("step-card")}>
                        <Title level={4}>Verify Your Backup</Title>
                        <Paragraph>
                            To confirm you've backed up your recovery phrase, please enter word #{verificationIndex + 1}
                        </Paragraph>

                        <Input
                            placeholder={`Enter word #${verificationIndex + 1}`}
                            value={verificationWord}
                            onChange={(e) => setVerificationWord(e.target.value)}
                            onPressEnter={handleVerifyBackup}
                            size="large"
                            style={{marginBottom: 16}}
                        />

                        <Button type="primary" onClick={handleVerifyBackup}>
                            Verify
                        </Button>
                    </Card>
                );

            case 3:
                return (
                    <Card className={b("step-card")}>
                        <div style={{textAlign: "center", marginBottom: 24}}>
                            <CheckCircleOutlined style={{fontSize: 48, color: "#52c41a"}} />
                        </div>

                        <Title level={4} style={{textAlign: "center"}}>Ready to Complete Setup</Title>

                        <Alert
                            message="Non-Custodial Security"
                            description="Your private keys never leave your device. Only the public key (xpub) will be sent to our server to generate payment addresses."
                            type="info"
                            showIcon
                            style={{marginBottom: 24}}
                        />

                        <div style={{marginBottom: 24}}>
                            <Text strong>Blockchain: </Text>
                            <Tag color="blue">{BLOCKCHAINS.find(b => b.value === selectedBlockchain)?.label}</Tag>
                        </div>

                        <div style={{marginBottom: 24}}>
                            <Text strong>Extended Public Key (xpub):</Text>
                            <div className={b("xpub-display")}>
                                <Text code style={{wordBreak: "break-all", fontSize: 12}}>
                                    {xpub}
                                </Text>
                            </div>
                        </div>

                        <Button
                            type="primary"
                            size="large"
                            block
                            loading={isSubmitting}
                            onClick={handleSubmit}
                        >
                            Complete Setup
                        </Button>
                    </Card>
                );

            default:
                return null;
        }
    };

    return (
        <>
            {contextHolder}
            <div className={b()}>
                <div className={b("header")}>
                    <Title level={2}>Setup Non-Custodial Wallet</Title>
                    <Paragraph>
                        Generate a new wallet to receive crypto payments. Your private keys stay on your device.
                    </Paragraph>
                </div>

                <Steps
                    current={currentStep}
                    items={[
                        {title: "Select Blockchain"},
                        {title: "Backup Phrase"},
                        {title: "Verify Backup"},
                        {title: "Complete"}
                    ]}
                    style={{marginBottom: 32}}
                />

                {renderStepContent()}
            </div>
        </>
    );
};

export default WalletSetupPage;
