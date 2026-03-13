import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {
    Row,
    Col,
    Typography,
    Card,
    Select,
    InputNumber,
    Table,
    Switch,
    notification,
    Space,
    Button,
    Tooltip,
    Divider
} from "antd";
import {InfoCircleOutlined, SaveOutlined, CheckOutlined} from "@ant-design/icons";
import {ColumnsType} from "antd/es/table";
import {useMount} from "react-use";
import useSharedMerchant from "src/hooks/use-merchant";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import merchantProvider from "src/providers/merchant-provider";

const {Title, Text, Paragraph} = Typography;

interface CurrencyRow {
    ticker: string;
    displayName: string;
    blockchainName: string;
    enabled: boolean;
    feeOverride: string;
}

const CurrenciesPage: React.FC = () => {
    const {merchant, getMerchant} = useSharedMerchant();
    const {merchantId} = useSharedMerchantId();
    const [notificationApi, contextHolder] = notification.useNotification();

    const [preferredCurrency, setPreferredCurrency] = React.useState<string>("USD");
    const [globalFee, setGlobalFee] = React.useState<number | null>(null);
    const [perCurrencyFees, setPerCurrencyFees] = React.useState<Record<string, string>>({});
    const [feeSettings, setFeeSettings] = React.useState<{preferredCurrency: string; globalFeePercentage: string}>({
        preferredCurrency: "USD",
        globalFeePercentage: ""
    });
    const [saving, setSaving] = React.useState(false);
    const [savingMethods, setSavingMethods] = React.useState(false);

    const openSuccess = (msg: string) => {
        notificationApi.info({
            message: msg,
            placement: "bottomRight",
            icon: <CheckOutlined style={{color: "#10b981"}} />
        });
    };

    const loadAll = async () => {
        if (!merchantId) return;
        await getMerchant(merchantId);
        try {
            const fs = await merchantProvider.getFeeSettings(merchantId);
            setFeeSettings(fs);
            setPreferredCurrency(fs.preferredCurrency || "USD");
            setGlobalFee(fs.globalFeePercentage ? parseFloat(fs.globalFeePercentage) : null);
        } catch (_) {}
    };

    useMount(loadAll);

    React.useEffect(() => {
        loadAll();
    }, [merchantId]);

    const saveFeeSettings = async () => {
        if (!merchantId) return;
        setSaving(true);
        try {
            const fees: Record<string, string> = {};
            Object.entries(perCurrencyFees).forEach(([k, v]) => {
                if (v !== "" && v !== null && v !== undefined) fees[k] = v;
            });
            await merchantProvider.updateFeeSettings(merchantId, {
                preferredCurrency,
                globalFeePercentage: globalFee !== null ? String(globalFee) : "",
                perCurrencyFees: fees
            });
            openSuccess("Currencies & fees saved");
        } catch (_) {
            notificationApi.error({message: "Failed to save settings", placement: "bottomRight"});
        } finally {
            setSaving(false);
        }
    };

    const toggleCurrency = async (ticker: string, enabled: boolean) => {
        if (!merchantId || !merchant?.supportedPaymentMethods) return;
        setSavingMethods(true);
        const updated = merchant.supportedPaymentMethods.map((m) =>
            m.ticker === ticker ? {...m, enabled} : m
        );
        const enabled_ = updated.filter((m) => m.enabled).map((m) => m.ticker);
        try {
            await merchantProvider.updateSupportedMethods(merchantId, {supportedPaymentMethods: enabled_});
            await getMerchant(merchantId);
        } catch (_) {
            notificationApi.error({message: "Failed to update currency", placement: "bottomRight"});
        } finally {
            setSavingMethods(false);
        }
    };

    const currencyRows: CurrencyRow[] = (merchant?.supportedPaymentMethods ?? []).map((m) => ({
        ticker: m.ticker,
        displayName: m.displayName,
        blockchainName: m.blockchainName,
        enabled: m.enabled,
        feeOverride: perCurrencyFees[m.ticker] ?? (feeSettings as any)[`perCurrencyFees`]?.[m.ticker] ?? ""
    }));

    const columns: ColumnsType<CurrencyRow> = [
        {
            title: "Currency",
            dataIndex: "displayName",
            key: "displayName",
            render: (text, record) => (
                <Space>
                    <Text strong>{text}</Text>
                    <Text type="secondary" style={{fontSize: 12}}>
                        {record.blockchainName}
                    </Text>
                </Space>
            )
        },
        {
            title: "Accept payments",
            key: "enabled",
            width: 140,
            render: (_, record) => (
                <Switch
                    checked={record.enabled}
                    loading={savingMethods}
                    onChange={(v) => toggleCurrency(record.ticker, v)}
                />
            )
        },
        {
            title: (
                <Space>
                    Fee override (%)
                    <Tooltip title="Leave blank to use the global default fee. Set a specific percentage for this currency to override.">
                        <InfoCircleOutlined style={{color: "#94a3b8"}} />
                    </Tooltip>
                </Space>
            ),
            key: "fee",
            width: 180,
            render: (_, record) => (
                <InputNumber
                    value={
                        perCurrencyFees[record.ticker] !== undefined
                            ? parseFloat(perCurrencyFees[record.ticker]) || undefined
                            : undefined
                    }
                    min={0}
                    max={100}
                    step={0.1}
                    precision={2}
                    placeholder={globalFee !== null ? `Default: ${globalFee}%` : "Use global default"}
                    style={{width: "100%"}}
                    addonAfter="%"
                    onChange={(v) =>
                        setPerCurrencyFees((prev) => ({
                            ...prev,
                            [record.ticker]: v !== null && v !== undefined ? String(v) : ""
                        }))
                    }
                />
            )
        }
    ];

    return (
        <PageContainer header={{title: ""}}>
            {contextHolder}
            <Row align="middle" justify="space-between">
                <Title>Currencies &amp; Fees</Title>
            </Row>

            {/* Invoice Currency */}
            <Card style={{marginBottom: 24}}>
                <Title level={4}>Invoice Currency</Title>
                <Paragraph type="secondary">
                    Choose the fiat currency your invoices are created in. This is the currency your customers
                    see on the payment page. Exchange rates are fetched live when the customer selects a
                    cryptocurrency.
                </Paragraph>
                <Select
                    value={preferredCurrency}
                    onChange={setPreferredCurrency}
                    style={{width: 200}}
                    options={[
                        {label: "USD — US Dollar", value: "USD"},
                        {label: "EUR — Euro", value: "EUR"}
                    ]}
                />
            </Card>

            {/* Global Markup */}
            <Card style={{marginBottom: 24}}>
                <Title level={4}>
                    Volatility Buffer (Global)&nbsp;
                    <Tooltip title="Crypto prices fluctuate. This markup adds a buffer to the crypto amount so you receive the full invoice value even with minor price swings. The customer always sees only the fiat price.">
                        <InfoCircleOutlined style={{color: "#94a3b8", fontSize: 16}} />
                    </Tooltip>
                </Title>
                <Paragraph type="secondary">
                    Set a global markup percentage applied to all crypto payments. The customer sees{" "}
                    <Text strong>$100</Text> on the payment page, but the crypto amount calculated equals{" "}
                    <Text strong>$100 + fee%</Text>. This buffer covers crypto volatility and network conversion
                    costs.
                </Paragraph>
                <Paragraph type="secondary" style={{fontStyle: "italic"}}>
                    Important: you are responsible for disclosing this fee to your customers on your own
                    website (e.g. "Crypto payments include a 1–2% market conversion fee").
                </Paragraph>
                <InputNumber
                    value={globalFee ?? undefined}
                    min={0}
                    max={100}
                    step={0.1}
                    precision={2}
                    placeholder="0.00"
                    style={{width: 200}}
                    addonAfter="%"
                    onChange={(v) => setGlobalFee(v)}
                />
            </Card>

            <Divider />

            {/* Per-currency table */}
            <Title level={4}>Per-Currency Settings</Title>
            <Paragraph type="secondary">
                Enable or disable accepted cryptocurrencies, and optionally set a per-currency fee override.
                Blank = use global default above.
            </Paragraph>
            <Table
                columns={columns}
                dataSource={currencyRows}
                rowKey="ticker"
                pagination={false}
                size="middle"
                style={{marginBottom: 24}}
            />

            <Row>
                <Col>
                    <Button
                        type="primary"
                        icon={<SaveOutlined />}
                        loading={saving}
                        onClick={saveFeeSettings}
                        size="large"
                    >
                        Save Settings
                    </Button>
                </Col>
            </Row>
        </PageContainer>
    );
};

export default CurrenciesPage;
