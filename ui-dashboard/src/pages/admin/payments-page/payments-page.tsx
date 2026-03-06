import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {Card, Input, Button, Space, Descriptions, Tag, Alert, Typography, Divider, Table, notification} from "antd";
import {SearchOutlined, SendOutlined, ReloadOutlined} from "@ant-design/icons";
import type {ColumnsType} from "antd/es/table";
import adminProvider, {AdminPayment} from "src/providers/admin-provider";

const {Text} = Typography;

const statusColor = (status: string): string => {
    switch (status) {
        case "success": return "success";
        case "failed": return "error";
        case "pending": return "default";
        case "locked": return "processing";
        default: return "warning";
    }
};

const AdminPaymentsPage: React.FC = () => {
    const [notificationApi, notificationElement] = notification.useNotification();

    const [searchId, setSearchId] = React.useState("");
    const [searching, setSearching] = React.useState(false);
    const [foundPayment, setFoundPayment] = React.useState<AdminPayment | null>(null);
    const [searchError, setSearchError] = React.useState("");
    const [resendResult, setResendResult] = React.useState<{success: boolean; message: string} | null>(null);

    // Per-row resend loading: track which payment IDs are in-flight
    const [resendingIds, setResendingIds] = React.useState<Set<string>>(new Set());

    const [pendingPayments, setPendingPayments] = React.useState<AdminPayment[]>([]);
    const [loadingPending, setLoadingPending] = React.useState(true);

    const loadPendingPayments = async () => {
        try {
            setLoadingPending(true);
            const data = await adminProvider.listPendingWebhookPayments();
            setPendingPayments(data.results || []);
        } catch (e) {
            console.error("Failed to load pending webhook payments", e);
        } finally {
            setLoadingPending(false);
        }
    };

    React.useEffect(() => {
        loadPendingPayments();
    }, []);

    const handleSearch = async () => {
        if (!searchId.trim()) return;
        setSearchError("");
        setFoundPayment(null);
        setResendResult(null);

        try {
            setSearching(true);
            const p = await adminProvider.getPaymentByPublicId(searchId.trim());
            setFoundPayment(p);
        } catch (e: any) {
            const status = e?.response?.status;
            setSearchError(status === 404 ? "Payment not found" : "Error: " + (e?.response?.data?.message || e.message));
        } finally {
            setSearching(false);
        }
    };

    const handleResend = async (paymentId: string) => {
        setResendingIds(prev => new Set(prev).add(paymentId));
        if (foundPayment?.public_id === paymentId) {
            setResendResult(null);
        }

        try {
            const result = await adminProvider.resendWebhook(paymentId);

            if (result.success) {
                notificationApi.success({message: "Webhook delivered successfully", description: result.message, placement: "bottomRight"});
                // Refresh the found payment and pending list
                if (foundPayment?.public_id === paymentId) {
                    try {
                        const updated = await adminProvider.getPaymentByPublicId(paymentId);
                        setFoundPayment(updated);
                    } catch (_) {}
                }
                await loadPendingPayments();
            } else {
                notificationApi.error({message: "Webhook delivery failed", description: result.message, placement: "bottomRight"});
            }

            if (foundPayment?.public_id === paymentId) {
                setResendResult(result);
            }
        } catch (e: any) {
            notificationApi.error({message: "Error sending webhook", description: e?.response?.data?.message || e.message, placement: "bottomRight"});
        } finally {
            setResendingIds(prev => {
                const next = new Set(prev);
                next.delete(paymentId);
                return next;
            });
        }
    };

    const pendingColumns: ColumnsType<AdminPayment> = [
        {
            title: "Payment UUID",
            dataIndex: "public_id",
            key: "public_id",
            render: (id: string) => <Text copyable code style={{fontSize: 11}}>{id}</Text>
        },
        {
            title: "Merchant",
            dataIndex: "merchant_id",
            key: "merchant_id",
            width: 90
        },
        {
            title: "Status",
            dataIndex: "status",
            key: "status",
            width: 100,
            render: (s: string) => <Tag color={statusColor(s)}>{s}</Tag>
        },
        {
            title: "Attempts",
            dataIndex: "webhook_attempts",
            key: "webhook_attempts",
            width: 80
        },
        {
            title: "Updated",
            dataIndex: "updated_at",
            key: "updated_at",
            render: (d: string) => new Date(d).toLocaleString()
        },
        {
            title: "Action",
            key: "action",
            width: 110,
            render: (_: any, record: AdminPayment) => (
                <Button
                    type="primary"
                    size="small"
                    icon={<SendOutlined />}
                    loading={resendingIds.has(record.public_id)}
                    onClick={() => handleResend(record.public_id)}
                >
                    Resend
                </Button>
            )
        }
    ];

    return (
        <PageContainer title="Payment Support">
            {notificationElement}
            <Space direction="vertical" size="large" style={{width: "100%"}}>

                {/* Search by payment UUID */}
                <Card title="Search Payment by UUID">
                    <Space>
                        <Input
                            placeholder="Enter payment UUID..."
                            value={searchId}
                            onChange={(e) => setSearchId(e.target.value)}
                            onPressEnter={handleSearch}
                            style={{width: 380}}
                        />
                        <Button
                            type="primary"
                            icon={<SearchOutlined />}
                            loading={searching}
                            onClick={handleSearch}
                        >
                            Search
                        </Button>
                    </Space>

                    {searchError && (
                        <Alert style={{marginTop: 16}} type="error" message={searchError} />
                    )}

                    {foundPayment && (
                        <div style={{marginTop: 16}}>
                            <Divider />
                            <Descriptions bordered column={2} size="small">
                                <Descriptions.Item label="UUID">{foundPayment.public_id}</Descriptions.Item>
                                <Descriptions.Item label="Status">
                                    <Tag color={statusColor(foundPayment.status)}>{foundPayment.status}</Tag>
                                </Descriptions.Item>
                                <Descriptions.Item label="Merchant ID">{foundPayment.merchant_id}</Descriptions.Item>
                                <Descriptions.Item label="Webhook Attempts">{foundPayment.webhook_attempts}</Descriptions.Item>
                                <Descriptions.Item label="Webhook Sent At">
                                    {foundPayment.webhook_sent_at
                                        ? new Date(foundPayment.webhook_sent_at).toLocaleString()
                                        : <Text type="warning">Not sent</Text>
                                    }
                                </Descriptions.Item>
                                <Descriptions.Item label="Updated At">
                                    {new Date(foundPayment.updated_at).toLocaleString()}
                                </Descriptions.Item>
                            </Descriptions>

                            <div style={{marginTop: 12}}>
                                <Button
                                    type="primary"
                                    icon={<SendOutlined />}
                                    loading={resendingIds.has(foundPayment.public_id)}
                                    onClick={() => handleResend(foundPayment.public_id)}
                                >
                                    Resend Webhook
                                </Button>
                            </div>

                            {resendResult && (
                                <Alert
                                    style={{marginTop: 12}}
                                    type={resendResult.success ? "success" : "error"}
                                    message={resendResult.message}
                                />
                            )}
                        </div>
                    )}
                </Card>

                {/* Pending webhook retries */}
                <Card
                    title="Payments Awaiting Webhook Delivery"
                    extra={
                        <Button
                            icon={<ReloadOutlined />}
                            loading={loadingPending}
                            onClick={loadPendingPayments}
                        >
                            Refresh
                        </Button>
                    }
                >
                    <Table
                        columns={pendingColumns}
                        dataSource={pendingPayments}
                        rowKey="id"
                        loading={loadingPending}
                        pagination={{pageSize: 20}}
                        locale={{emptyText: "No payments awaiting webhook delivery"}}
                        size="small"
                    />
                </Card>
            </Space>
        </PageContainer>
    );
};

export default AdminPaymentsPage;
