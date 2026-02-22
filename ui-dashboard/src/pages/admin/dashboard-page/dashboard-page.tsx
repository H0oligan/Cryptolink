import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {Row, Col, Card, Statistic, Typography, Spin} from "antd";
import {DollarOutlined, ShopOutlined, UserOutlined, CrownOutlined, TeamOutlined} from "@ant-design/icons";
import adminProvider, {SystemStats} from "src/providers/admin-provider";

const AdminDashboardPage: React.FC = () => {
    const [stats, setStats] = React.useState<SystemStats | null>(null);
    const [loading, setLoading] = React.useState(true);

    React.useEffect(() => {
        const load = async () => {
            try {
                const data = await adminProvider.getStats();
                setStats(data);
            } catch (e) {
                console.error("Failed to load stats", e);
            } finally {
                setLoading(false);
            }
        };
        load();
    }, []);

    if (loading) {
        return (
            <PageContainer header={{title: ""}}>
                <Row justify="center" style={{padding: 100}}>
                    <Spin size="large" />
                </Row>
            </PageContainer>
        );
    }

    return (
        <PageContainer header={{title: ""}}>
            <Typography.Title level={3}>Admin Dashboard</Typography.Title>

            <Row gutter={[16, 16]}>
                <Col xs={24} sm={12} lg={6}>
                    <Card>
                        <Statistic
                            title="Total Merchants"
                            value={stats?.total_merchants ?? 0}
                            prefix={<ShopOutlined />}
                        />
                    </Card>
                </Col>
                <Col xs={24} sm={12} lg={6}>
                    <Card>
                        <Statistic
                            title="Total Users"
                            value={stats?.total_users ?? 0}
                            prefix={<TeamOutlined />}
                        />
                    </Card>
                </Col>
                <Col xs={24} sm={12} lg={6}>
                    <Card>
                        <Statistic
                            title="Paying Merchants"
                            value={stats?.paying_merchants ?? 0}
                            prefix={<CrownOutlined />}
                            valueStyle={{color: "#6366f1"}}
                        />
                    </Card>
                </Col>
                <Col xs={24} sm={12} lg={6}>
                    <Card>
                        <Statistic
                            title="Monthly Revenue"
                            value={stats?.monthly_revenue ?? "0"}
                            prefix={<DollarOutlined />}
                            precision={2}
                            valueStyle={{color: "#10b981"}}
                        />
                    </Card>
                </Col>
            </Row>

            <Row gutter={[16, 16]} style={{marginTop: 16}}>
                <Col xs={24} sm={8} lg={4}>
                    <Card>
                        <Statistic title="Free Tier" value={stats?.free_tier_count ?? 0} prefix={<UserOutlined />} />
                    </Card>
                </Col>
                <Col xs={24} sm={8} lg={4}>
                    <Card>
                        <Statistic title="Starter" value={stats?.basic_tier_count ?? 0} />
                    </Card>
                </Col>
                <Col xs={24} sm={8} lg={4}>
                    <Card>
                        <Statistic title="Growth" value={stats?.pro_tier_count ?? 0} />
                    </Card>
                </Col>
                <Col xs={24} sm={8} lg={4}>
                    <Card>
                        <Statistic title="Business/Enterprise" value={stats?.enterprise_tier_count ?? 0} />
                    </Card>
                </Col>
                <Col xs={24} sm={8} lg={4}>
                    <Card>
                        <Statistic
                            title="No Subscription"
                            value={stats?.no_subscription_count ?? 0}
                            valueStyle={{color: "#f59e0b"}}
                        />
                    </Card>
                </Col>
            </Row>
        </PageContainer>
    );
};

export default AdminDashboardPage;
