import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {
    Table, Typography, Tag, Button, Space, Modal, Form, Input,
    InputNumber, Switch, Select, notification, Popconfirm
} from "antd";
import {PlusOutlined, EditOutlined, CheckOutlined} from "@ant-design/icons";
import type {ColumnsType} from "antd/es/table";
import adminProvider, {AdminPlan, CreatePlanParams} from "src/providers/admin-provider";

const AdminPlansPage: React.FC = () => {
    const [notificationApi, contextHolder] = notification.useNotification();
    const [plans, setPlans] = React.useState<AdminPlan[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [modalOpen, setModalOpen] = React.useState(false);
    const [editingPlan, setEditingPlan] = React.useState<AdminPlan | null>(null);
    const [submitting, setSubmitting] = React.useState(false);
    const [form] = Form.useForm();

    const loadPlans = async () => {
        try {
            setLoading(true);
            const data = await adminProvider.listPlans();
            setPlans(data || []);
        } catch (e) {
            console.error("Failed to load plans", e);
        } finally {
            setLoading(false);
        }
    };

    React.useEffect(() => {
        loadPlans();
    }, []);

    const openCreate = () => {
        setEditingPlan(null);
        form.resetFields();
        form.setFieldsValue({
            billing_period: "monthly",
            max_merchants: 1,
            is_active: true
        });
        setModalOpen(true);
    };

    const openEdit = (plan: AdminPlan) => {
        setEditingPlan(plan);
        form.setFieldsValue({
            id: plan.id,
            name: plan.name,
            description: plan.description,
            price_usd: parseFloat(plan.price_usd),
            billing_period: plan.billing_period,
            max_payments_monthly: plan.max_payments_monthly,
            max_merchants: plan.max_merchants,
            max_api_calls_monthly: plan.max_api_calls_monthly,
            max_volume_monthly_usd: plan.max_volume_monthly_usd ? parseFloat(plan.max_volume_monthly_usd) : null,
            is_active: plan.is_active
        });
        setModalOpen(true);
    };

    const handleSubmit = async (values: any) => {
        try {
            setSubmitting(true);
            const params: CreatePlanParams = {
                id: values.id,
                name: values.name,
                description: values.description || "",
                price_usd: values.price_usd || 0,
                billing_period: values.billing_period || "monthly",
                max_payments_monthly: values.max_payments_monthly ?? null,
                max_merchants: values.max_merchants || 1,
                max_api_calls_monthly: values.max_api_calls_monthly ?? null,
                max_volume_monthly_usd: values.max_volume_monthly_usd ?? null,
                features: {},
                is_active: values.is_active ?? true
            };

            if (editingPlan) {
                await adminProvider.updatePlan(editingPlan.id, params);
                notificationApi.success({
                    message: "Plan updated",
                    description: `Plan "${params.name}" has been updated.`,
                    placement: "bottomRight",
                    icon: <CheckOutlined style={{color: "#49D1AC"}} />
                });
            } else {
                await adminProvider.createPlan(params);
                notificationApi.success({
                    message: "Plan created",
                    description: `Plan "${params.name}" has been created.`,
                    placement: "bottomRight",
                    icon: <CheckOutlined style={{color: "#49D1AC"}} />
                });
            }

            setModalOpen(false);
            await loadPlans();
        } catch (e: any) {
            notificationApi.error({
                message: "Error",
                description: e?.response?.data?.message || "Failed to save plan.",
                placement: "bottomRight"
            });
        } finally {
            setSubmitting(false);
        }
    };

    const formatLimit = (val: number | null): string => {
        if (val === null || val === undefined) return "Unlimited";
        if (val === -1) return "Unlimited";
        return val.toLocaleString();
    };

    const columns: ColumnsType<AdminPlan> = [
        {
            title: "ID",
            dataIndex: "id",
            key: "id",
            width: 100
        },
        {
            title: "Name",
            dataIndex: "name",
            key: "name",
            render: (name: string) => <strong>{name}</strong>
        },
        {
            title: "Price (USD)",
            dataIndex: "price_usd",
            key: "price",
            render: (price: string) => `$${parseFloat(price).toFixed(2)}/mo`
        },
        {
            title: "Volume Limit",
            dataIndex: "max_volume_monthly_usd",
            key: "volume",
            render: (vol: string | null) => vol ? `$${parseFloat(vol).toLocaleString()}/mo` : "Unlimited"
        },
        {
            title: "Payments/mo",
            dataIndex: "max_payments_monthly",
            key: "payments",
            render: formatLimit
        },
        {
            title: "Max Merchants",
            dataIndex: "max_merchants",
            key: "merchants",
            render: formatLimit
        },
        {
            title: "API Calls/mo",
            dataIndex: "max_api_calls_monthly",
            key: "api",
            render: formatLimit
        },
        {
            title: "Status",
            dataIndex: "is_active",
            key: "status",
            render: (active: boolean) => (
                <Tag color={active ? "green" : "red"}>{active ? "Active" : "Inactive"}</Tag>
            )
        },
        {
            title: "Actions",
            key: "actions",
            width: 100,
            render: (_: any, record: AdminPlan) => (
                <Button
                    type="link"
                    icon={<EditOutlined />}
                    onClick={() => openEdit(record)}
                >
                    Edit
                </Button>
            )
        }
    ];

    return (
        <PageContainer header={{title: ""}}>
            {contextHolder}
            <Row justify="space-between" align="middle" style={{marginBottom: 16}}>
                <Typography.Title level={3} style={{margin: 0}}>Subscription Plans</Typography.Title>
                <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                    Create Plan
                </Button>
            </Row>

            <Table
                columns={columns}
                dataSource={plans}
                rowKey="id"
                loading={loading}
                pagination={false}
            />

            <Modal
                title={editingPlan ? `Edit Plan: ${editingPlan.name}` : "Create New Plan"}
                open={modalOpen}
                onCancel={() => setModalOpen(false)}
                footer={null}
                width={600}
            >
                <Form
                    form={form}
                    layout="vertical"
                    onFinish={handleSubmit}
                    style={{marginTop: 16}}
                >
                    <Form.Item
                        name="id"
                        label="Plan ID (slug)"
                        rules={[{required: true, message: "Plan ID is required"}]}
                    >
                        <Input
                            placeholder="e.g. starter, growth"
                            disabled={!!editingPlan}
                        />
                    </Form.Item>

                    <Form.Item
                        name="name"
                        label="Display Name"
                        rules={[{required: true, message: "Name is required"}]}
                    >
                        <Input placeholder="e.g. Starter Plan" />
                    </Form.Item>

                    <Form.Item name="description" label="Description">
                        <Input.TextArea rows={2} placeholder="Plan description" />
                    </Form.Item>

                    <Space size="large" style={{display: "flex"}}>
                        <Form.Item
                            name="price_usd"
                            label="Price (USD/month)"
                            rules={[{required: true, message: "Price is required"}]}
                        >
                            <InputNumber min={0} precision={2} prefix="$" style={{width: 150}} />
                        </Form.Item>

                        <Form.Item name="billing_period" label="Billing Period">
                            <Select style={{width: 150}}>
                                <Select.Option value="monthly">Monthly</Select.Option>
                                <Select.Option value="yearly">Yearly</Select.Option>
                            </Select>
                        </Form.Item>
                    </Space>

                    <Form.Item
                        name="max_volume_monthly_usd"
                        label="Monthly Volume Limit (USD)"
                        tooltip="Leave empty for unlimited"
                    >
                        <InputNumber
                            min={0}
                            precision={2}
                            prefix="$"
                            placeholder="Unlimited"
                            style={{width: "100%"}}
                        />
                    </Form.Item>

                    <Space size="large" style={{display: "flex"}}>
                        <Form.Item
                            name="max_payments_monthly"
                            label="Max Payments/Month"
                            tooltip="-1 or empty for unlimited"
                        >
                            <InputNumber min={-1} placeholder="Unlimited" style={{width: 150}} />
                        </Form.Item>

                        <Form.Item
                            name="max_merchants"
                            label="Max Merchants"
                            tooltip="-1 for unlimited"
                        >
                            <InputNumber min={-1} style={{width: 150}} />
                        </Form.Item>

                        <Form.Item
                            name="max_api_calls_monthly"
                            label="Max API Calls/Month"
                            tooltip="-1 or empty for unlimited"
                        >
                            <InputNumber min={-1} placeholder="Unlimited" style={{width: 150}} />
                        </Form.Item>
                    </Space>

                    <Form.Item name="is_active" label="Active" valuePropName="checked">
                        <Switch />
                    </Form.Item>

                    <Form.Item>
                        <Space>
                            <Button type="primary" htmlType="submit" loading={submitting}>
                                {editingPlan ? "Update Plan" : "Create Plan"}
                            </Button>
                            <Button onClick={() => setModalOpen(false)}>Cancel</Button>
                        </Space>
                    </Form.Item>
                </Form>
            </Modal>
        </PageContainer>
    );
};

const Row: React.FC<{justify?: string; align?: string; style?: React.CSSProperties; children: React.ReactNode}> =
    ({justify, align, style, children}) => (
        <div style={{display: "flex", justifyContent: justify === "space-between" ? "space-between" : undefined, alignItems: align === "middle" ? "center" : undefined, ...style}}>
            {children}
        </div>
    );

export default AdminPlansPage;
