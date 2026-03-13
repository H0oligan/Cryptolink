import "./subscription-page.scss";

import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {
    Row, Col, Typography, Progress, Button, Space,
    notification, Spin, Table, Divider
} from "antd";
import {
    CheckOutlined, CrownOutlined, RocketOutlined,
    ThunderboltOutlined, StarOutlined
} from "@ant-design/icons";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import subscriptionProvider, {
    SubscriptionPlan, CurrentSubscription, UsageInfo
} from "src/providers/subscription-provider";

const PLAN_ICONS: Record<string, React.ReactNode> = {
    free: <ThunderboltOutlined />,
    starter: <StarOutlined />,
    growth: <RocketOutlined />,
    business: <CrownOutlined />,
    enterprise: <CrownOutlined />
};

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
                    icon: <CheckOutlined style={{color: "#10b981"}} />
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
                icon: <CheckOutlined style={{color: "#10b981"}} />
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
    const volumeColor = volumePercent > 90 ? "#ef4444" : volumePercent > 70 ? "#f59e0b" : "#10b981";

    // Payment count usage
    const paymentCount = usage?.payment_count || 0;
    const paymentLimit = currentPlan?.max_payments_monthly;
    const paymentPercent = paymentLimit ? Math.min((paymentCount / paymentLimit) * 100, 100) : 0;

    return (
        <PageContainer header={{title: ""}}>
            {contextHolder}

            <div className="sub-page">
                <Typography.Title level={3} className="sub-page__title">
                    Subscription
                </Typography.Title>

                {/* ── Current Plan Status ── */}
                {current?.subscription && (
                    <div className="sub-page__status-card">
                        <div className="sub-page__status-header">
                            <div className="sub-page__status-plan">
                                <span className="sub-page__status-label">CURRENT PLAN</span>
                                <span className="sub-page__status-name">
                                    {currentPlan?.name || currentPlanId}
                                </span>
                                <span className={`sub-page__status-badge sub-page__status-badge--${current.subscription.status}`}>
                                    {current.subscription.status}
                                </span>
                            </div>
                            {current.subscription.auto_renew && current.subscription.status === "active" && (
                                <div className="sub-page__status-actions">
                                    <span className="sub-page__status-renew">
                                        Renews {new Date(current.subscription.current_period_end).toLocaleDateString()}
                                    </span>
                                    <Button type="text" danger size="small" onClick={handleCancel}>
                                        Cancel renewal
                                    </Button>
                                </div>
                            )}
                        </div>
                        <div className="sub-page__status-meters">
                            <div className="sub-page__meter">
                                <div className="sub-page__meter-label">
                                    <span>Volume</span>
                                    <span className="sub-page__meter-value">
                                        {volumeLimit
                                            ? `$${volumeUsed.toLocaleString()} / $${volumeLimit.toLocaleString()}`
                                            : `$${volumeUsed.toLocaleString()}`
                                        }
                                    </span>
                                </div>
                                <Progress
                                    percent={Math.round(volumePercent)}
                                    strokeColor={volumeColor}
                                    showInfo={false}
                                    size="small"
                                />
                            </div>
                            <div className="sub-page__meter">
                                <div className="sub-page__meter-label">
                                    <span>Payments</span>
                                    <span className="sub-page__meter-value">
                                        {paymentLimit
                                            ? `${paymentCount} / ${paymentLimit.toLocaleString()}`
                                            : `${paymentCount}`
                                        }
                                    </span>
                                </div>
                                <Progress
                                    percent={paymentLimit ? Math.round(paymentPercent) : 0}
                                    strokeColor={paymentPercent > 90 ? "#ef4444" : "#10b981"}
                                    showInfo={false}
                                    size="small"
                                />
                            </div>
                        </div>
                    </div>
                )}

                {/* ── Plan Grid ── */}
                <div className="sub-page__section-label">AVAILABLE PLANS</div>
                <div className="sub-page__plans-grid">
                    {plans.map((plan) => {
                        const isCurrent = plan.id === currentPlanId;
                        const isHigher = !currentPlanId || parseFloat(plan.price_usd) > parseFloat(currentPlan?.price_usd || "0");
                        const isFree = parseFloat(plan.price_usd) === 0;
                        const planKey = plan.name.toLowerCase();
                        const isPopular = planKey === "growth";

                        return (
                            <div
                                className={`sub-page__plan-card ${isCurrent ? "sub-page__plan-card--current" : ""} ${isPopular ? "sub-page__plan-card--popular" : ""}`}
                                key={plan.id}
                            >
                                {isPopular && <div className="sub-page__plan-badge">POPULAR</div>}
                                {isCurrent && <div className="sub-page__plan-badge sub-page__plan-badge--current">ACTIVE</div>}

                                <div className="sub-page__plan-icon">
                                    {PLAN_ICONS[planKey] || <ThunderboltOutlined />}
                                </div>

                                <div className="sub-page__plan-name">{plan.name}</div>
                                <div className="sub-page__plan-desc">{plan.description}</div>

                                <div className="sub-page__plan-price">
                                    {isFree ? (
                                        <span className="sub-page__plan-amount">Free</span>
                                    ) : (
                                        <>
                                            <span className="sub-page__plan-currency">$</span>
                                            <span className="sub-page__plan-amount">
                                                {parseFloat(plan.price_usd).toFixed(2)}
                                            </span>
                                            <span className="sub-page__plan-period">/mo</span>
                                        </>
                                    )}
                                </div>

                                <Divider className="sub-page__plan-divider" />

                                <div className="sub-page__plan-features">
                                    <div className="sub-page__plan-feature">
                                        <CheckOutlined className="sub-page__plan-check" />
                                        <span>
                                            {plan.max_volume_monthly_usd
                                                ? `$${parseFloat(plan.max_volume_monthly_usd).toLocaleString()}/mo volume`
                                                : "Unlimited volume"}
                                        </span>
                                    </div>
                                    <div className="sub-page__plan-feature">
                                        <CheckOutlined className="sub-page__plan-check" />
                                        <span>
                                            {plan.max_payments_monthly === null
                                                ? "Unlimited payments"
                                                : `${plan.max_payments_monthly}/mo payments`}
                                        </span>
                                    </div>
                                    <div className="sub-page__plan-feature">
                                        <CheckOutlined className="sub-page__plan-check" />
                                        <span>
                                            {plan.max_merchants === -1
                                                ? "Unlimited merchants"
                                                : `${plan.max_merchants} merchant${plan.max_merchants > 1 ? "s" : ""}`}
                                        </span>
                                    </div>
                                    <div className="sub-page__plan-feature">
                                        <CheckOutlined className="sub-page__plan-check" />
                                        <span>
                                            {plan.max_api_calls_monthly === null
                                                ? "Unlimited API calls"
                                                : `${plan.max_api_calls_monthly.toLocaleString()}/mo API calls`}
                                        </span>
                                    </div>
                                </div>

                                <div className="sub-page__plan-action">
                                    {isCurrent ? (
                                        <Button block disabled className="sub-page__plan-btn">
                                            Current Plan
                                        </Button>
                                    ) : (
                                        <Button
                                            type={isHigher ? "primary" : "default"}
                                            block
                                            loading={upgrading === plan.id}
                                            onClick={() => handleUpgrade(plan.id)}
                                            className="sub-page__plan-btn"
                                        >
                                            {isHigher ? "Upgrade" : "Switch"}
                                        </Button>
                                    )}
                                </div>
                            </div>
                        );
                    })}
                </div>

                {/* ── Usage History ── */}
                {usageHistory.length > 0 && (
                    <>
                        <div className="sub-page__section-label" style={{marginTop: 40}}>
                            USAGE HISTORY
                        </div>
                        <Table
                            dataSource={usageHistory}
                            rowKey="period_start"
                            pagination={false}
                            className="sub-page__history-table"
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
            </div>
        </PageContainer>
    );
};

export default SubscriptionPage;
