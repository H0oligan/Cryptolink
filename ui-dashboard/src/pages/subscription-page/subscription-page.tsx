import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {
    Row, Col, Card, Typography, Progress, Tag, Button, Space,
    notification, Spin, Table, Statistic, Badge, Divider
} from "antd";
import {
    CheckOutlined, CrownOutlined, RocketOutlined,
    ThunderboltOutlined, WarningOutlined
} from "@ant-design/icons";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import subscriptionProvider, {
    SubscriptionPlan, CurrentSubscription, UsageInfo
} from "src/providers/subscription-provider";

const SubscriptionPage: React.FC = () => {
    const [notificationApi, contextHolder] = notification.useNotification();
    const {merchantId} = useSharedMerchantId();
    const [plans, setPlans] = React.useState<SubscriptionPlan[]>([]);
    const [current, setCurrent] = React.useState<CurrentSubscription | null>(null);
    const [usageHistory, setUsageHistory] = React.useState<UsageInfo[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [upgrading, setUpgrading] = React.useState<string | null>(null);

    const loadData = async () => {
        if (!merchantId) return;
        try {
            setLoading(true);
            const [plansData, currentData, historyData] = await Promise.all([
                subscriptionProvider.listPlans(),
                subscriptionProvider.getCurrentSubscription(merchantId).catch(() => null),
                subscriptionProvider.getUsageHistory(merchantId).catch(() => [])
            ]);
            setPlans(plansData || []);
            setCurrent(currentData);
            setUsageHistory(historyData || []);
        } catch (e) {
            console.error("Failed to load subscription data", e);
        } finally {
            setLoading(false);
        }
    };

    React.useEffect(() => {
        loadData();
    }, [merchantId]);

    const handleUpgrade = async (planId: string) => {
        if (!merchantId) return;
        try {
            setUpgrading(planId);
            const result = await subscriptionProvider.upgradePlan(
                merchantId,
                planId,
                window.location.origin + "/dashboard/subscription"
            );
            if (result.payment_url) {
                window.location.href = result.payment_url;
            } else {
                notificationApi.success({
                    message: "Plan activated",
                    description: "Your subscription has been activated.",
                    placement: "bottomRight",
                    icon: <CheckOutlined style={{color: "#49D1AC"}} />
                });
                await loadData();
            }
        } catch (e: any) {
            notificationApi.error({
                message: "Upgrade failed",
                description: e?.response?.data?.message || "Failed to upgrade plan.",
                placement: "bottomRight"
            });
        } finally {
            setUpgrading(null);
        }
    };

    const handleCancel = async () => {
        if (!merchantId) return;
        try {
            await subscriptionProvider.cancelSubscription(merchantId);
            notificationApi.success({
                message: "Auto-renew cancelled",
                description: "Your subscription will not renew at the end of the current period.",
                placement: "bottomRight",
                icon: <CheckOutlined style={{color: "#49D1AC"}} />
            });
            await loadData();
        } catch (e: any) {
            notificationApi.error({
                message: "Cancel failed",
                description: e?.response?.data?.message || "Failed to cancel subscription.",
                placement: "bottomRight"
            });
        }
    };

    if (loading) {
        return (
            <PageContainer header={{title: ""}}>
                <Row justify="center" style={{padding: 100}}>
                    <Spin size="large" />
                </Row>
            </PageContainer>
        );
    }

    const currentPlanId = current?.subscription?.plan_id;
    const usage = current?.usage;
    const currentPlan = plans.find((p) => p.id === currentPlanId);

    // Volume usage calculation
    const volumeUsed = parseFloat(usage?.payment_volume_usd || "0");
    const volumeLimit = currentPlan?.max_volume_monthly_usd
        ? parseFloat(currentPlan.max_volume_monthly_usd)
        : null;
    const volumePercent = volumeLimit ? Math.min((volumeUsed / volumeLimit) * 100, 100) : 0;
    const volumeColor = volumePercent > 90 ? "#ff4d4f" : volumePercent > 70 ? "#faad14" : "#10b981";

    // Payment count usage
    const paymentCount = usage?.payment_count || 0;
    const paymentLimit = currentPlan?.max_payments_monthly;
    const paymentPercent = paymentLimit ? Math.min((paymentCount / paymentLimit) * 100, 100) : 0;

    return (
        <PageContainer header={{title: ""}}>
            {contextHolder}
            <Typography.Title level={3}>Subscription</Typography.Title>

            {/* Current Plan */}
            {current?.subscription && (
                <Card title="Current Plan" style={{marginBottom: 24}}>
                    <Row gutter={24}>
                        <Col xs={24} md={8}>
                            <Space direction="vertical" size={4}>
                                <Typography.Text type="secondary">Plan</Typography.Text>
                                <Typography.Title level={4} style={{margin: 0}}>
                                    {currentPlan?.name || currentPlanId}
                                </Typography.Title>
                                <Tag color={
                                    current.subscription.status === "active" ? "green" :
                                    current.subscription.status === "pending_payment" ? "orange" : "red"
                                }>
                                    {current.subscription.status}
                                </Tag>
                                {current.subscription.auto_renew && (
                                    <Typography.Text type="secondary" style={{fontSize: 12}}>
                                        Renews: {new Date(current.subscription.current_period_end).toLocaleDateString()}
                                    </Typography.Text>
                                )}
                            </Space>
                        </Col>
                        <Col xs={24} md={8}>
                            <Typography.Text type="secondary">Monthly Volume</Typography.Text>
                            <Progress
                                percent={Math.round(volumePercent)}
                                strokeColor={volumeColor}
                                format={() => volumeLimit
                                    ? `$${volumeUsed.toLocaleString()} / $${volumeLimit.toLocaleString()}`
                                    : `$${volumeUsed.toLocaleString()} (Unlimited)`
                                }
                            />
                        </Col>
                        <Col xs={24} md={8}>
                            <Typography.Text type="secondary">Payments This Month</Typography.Text>
                            <Progress
                                percent={paymentLimit ? Math.round(paymentPercent) : 0}
                                strokeColor={paymentPercent > 90 ? "#ff4d4f" : "#6366f1"}
                                format={() => paymentLimit
                                    ? `${paymentCount} / ${paymentLimit.toLocaleString()}`
                                    : `${paymentCount} (Unlimited)`
                                }
                            />
                        </Col>
                    </Row>
                    {current.subscription.auto_renew && current.subscription.status === "active" && (
                        <div style={{marginTop: 16}}>
                            <Button type="link" danger onClick={handleCancel}>
                                Cancel auto-renewal
                            </Button>
                        </div>
                    )}
                </Card>
            )}

            {/* Plan Comparison */}
            <Typography.Title level={4}>Available Plans</Typography.Title>
            <Row gutter={[16, 16]}>
                {plans.map((plan) => {
                    const isCurrent = plan.id === currentPlanId;
                    const isHigher = !currentPlanId || parseFloat(plan.price_usd) > parseFloat(currentPlan?.price_usd || "0");

                    return (
                        <Col xs={24} sm={12} lg={8} xl={Math.floor(24 / Math.min(plans.length, 5))} key={plan.id}>
                            <Badge.Ribbon
                                text={isCurrent ? "Current" : ""}
                                color={isCurrent ? "#6366f1" : "transparent"}
                                style={isCurrent ? {} : {display: "none"}}
                            >
                                <Card
                                    style={{
                                        height: "100%",
                                        borderColor: isCurrent ? "#6366f1" : undefined,
                                        borderWidth: isCurrent ? 2 : 1
                                    }}
                                >
                                    <Space direction="vertical" size={12} style={{width: "100%"}}>
                                        <Typography.Title level={4} style={{margin: 0}}>
                                            {plan.name}
                                        </Typography.Title>
                                        <Typography.Text type="secondary">{plan.description}</Typography.Text>

                                        <Typography.Title level={2} style={{margin: 0, color: "#6366f1"}}>
                                            ${parseFloat(plan.price_usd).toFixed(2)}
                                            <Typography.Text type="secondary" style={{fontSize: 14}}>/mo</Typography.Text>
                                        </Typography.Title>

                                        <Divider style={{margin: "8px 0"}} />

                                        <div>
                                            <div style={{marginBottom: 4}}>
                                                <CheckOutlined style={{color: "#10b981", marginRight: 8}} />
                                                Volume: {plan.max_volume_monthly_usd
                                                    ? `$${parseFloat(plan.max_volume_monthly_usd).toLocaleString()}/mo`
                                                    : "Unlimited"}
                                            </div>
                                            <div style={{marginBottom: 4}}>
                                                <CheckOutlined style={{color: "#10b981", marginRight: 8}} />
                                                Payments: {plan.max_payments_monthly === null ? "Unlimited" : `${plan.max_payments_monthly}/mo`}
                                            </div>
                                            <div style={{marginBottom: 4}}>
                                                <CheckOutlined style={{color: "#10b981", marginRight: 8}} />
                                                Merchants: {plan.max_merchants === -1 ? "Unlimited" : plan.max_merchants}
                                            </div>
                                            <div style={{marginBottom: 4}}>
                                                <CheckOutlined style={{color: "#10b981", marginRight: 8}} />
                                                API Calls: {plan.max_api_calls_monthly === null ? "Unlimited" : `${plan.max_api_calls_monthly.toLocaleString()}/mo`}
                                            </div>
                                        </div>

                                        {!isCurrent && (
                                            <Button
                                                type={isHigher ? "primary" : "default"}
                                                block
                                                loading={upgrading === plan.id}
                                                onClick={() => handleUpgrade(plan.id)}
                                                icon={isHigher ? <RocketOutlined /> : undefined}
                                            >
                                                {isHigher ? "Upgrade" : "Switch"}
                                            </Button>
                                        )}
                                        {isCurrent && (
                                            <Button type="primary" block disabled>
                                                Current Plan
                                            </Button>
                                        )}
                                    </Space>
                                </Card>
                            </Badge.Ribbon>
                        </Col>
                    );
                })}
            </Row>

            {/* Usage History */}
            {usageHistory.length > 0 && (
                <>
                    <Typography.Title level={4} style={{marginTop: 32}}>Usage History</Typography.Title>
                    <Table
                        dataSource={usageHistory}
                        rowKey="period_start"
                        pagination={false}
                        columns={[
                            {
                                title: "Period",
                                key: "period",
                                render: (_: any, record: UsageInfo) => {
                                    const start = new Date(record.period_start);
                                    return start.toLocaleDateString("en-US", {month: "long", year: "numeric"});
                                }
                            },
                            {
                                title: "Volume (USD)",
                                dataIndex: "payment_volume_usd",
                                key: "volume",
                                render: (vol: string) => `$${parseFloat(vol).toLocaleString()}`
                            },
                            {
                                title: "Payments",
                                dataIndex: "payment_count",
                                key: "payments"
                            },
                            {
                                title: "API Calls",
                                dataIndex: "api_calls_count",
                                key: "api"
                            }
                        ]}
                    />
                </>
            )}
        </PageContainer>
    );
};

export default SubscriptionPage;
