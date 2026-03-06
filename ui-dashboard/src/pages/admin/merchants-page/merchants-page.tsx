import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {Table, Typography, Tag, Button, Modal, Select, message, Space} from "antd";
import type {ColumnsType} from "antd/es/table";
import adminProvider, {AdminMerchant, AdminPlan} from "src/providers/admin-provider";

const AdminMerchantsPage: React.FC = () => {
    const [merchants, setMerchants] = React.useState<AdminMerchant[]>([]);
    const [plans, setPlans] = React.useState<AdminPlan[]>([]);
    const [total, setTotal] = React.useState(0);
    const [loading, setLoading] = React.useState(true);
    const [page, setPage] = React.useState(1);
    const pageSize = 20;

    // Plan assignment modal state
    const [assignModalOpen, setAssignModalOpen] = React.useState(false);
    const [selectedMerchant, setSelectedMerchant] = React.useState<AdminMerchant | null>(null);
    const [selectedPlanId, setSelectedPlanId] = React.useState<string>("");
    const [assigning, setAssigning] = React.useState(false);

    const loadMerchants = async (p: number) => {
        try {
            setLoading(true);
            const data = await adminProvider.listMerchants(pageSize, (p - 1) * pageSize);
            setMerchants(data.results || []);
            setTotal(data.total);
        } catch (e) {
            console.error("Failed to load merchants", e);
        } finally {
            setLoading(false);
        }
    };

    const loadPlans = async () => {
        try {
            const data = await adminProvider.listPlans();
            setPlans(data.filter((p) => p.is_active));
        } catch (e) {
            console.error("Failed to load plans", e);
        }
    };

    React.useEffect(() => {
        loadMerchants(page);
        loadPlans();
    }, [page]);

    const openAssignModal = (merchant: AdminMerchant) => {
        setSelectedMerchant(merchant);
        setSelectedPlanId(merchant.active_plan_id || "");
        setAssignModalOpen(true);
    };

    const handleAssignPlan = async () => {
        if (!selectedMerchant || !selectedPlanId) return;

        try {
            setAssigning(true);
            await adminProvider.assignMerchantPlan(selectedMerchant.id, selectedPlanId);
            message.success(`Plan assigned to ${selectedMerchant.name}`);
            setAssignModalOpen(false);
            loadMerchants(page);
        } catch (e: any) {
            message.error("Failed to assign plan: " + (e?.response?.data?.message || e.message));
        } finally {
            setAssigning(false);
        }
    };

    const columns: ColumnsType<AdminMerchant> = [
        {
            title: "Name",
            dataIndex: "name",
            key: "name",
            render: (name: string) => <strong>{name}</strong>
        },
        {
            title: "Owner",
            dataIndex: "creator_email",
            key: "creator_email"
        },
        {
            title: "Plan",
            dataIndex: "active_plan_name",
            key: "plan",
            render: (name: string | null, record: AdminMerchant) => {
                if (!name) return <Tag color="default">No Plan</Tag>;
                const colors: Record<string, string> = {
                    free: "default",
                    basic: "blue",
                    pro: "green",
                    enterprise: "purple",
                    unlimited: "gold"
                };
                return <Tag color={colors[record.active_plan_id || ""] || "blue"}>{name}</Tag>;
            }
        },
        {
            title: "Monthly Volume",
            dataIndex: "monthly_volume_usd",
            key: "volume",
            render: (vol: string) => `$${parseFloat(vol || "0").toLocaleString()}`
        },
        {
            title: "Payments",
            dataIndex: "payment_count",
            key: "payments"
        },
        {
            title: "Website",
            dataIndex: "website",
            key: "website",
            render: (url: string) =>
                url ? (
                    <a href={url} target="_blank" rel="noopener noreferrer">
                        {url}
                    </a>
                ) : (
                    "-"
                )
        },
        {
            title: "Created",
            dataIndex: "created_at",
            key: "created_at",
            render: (date: string) => new Date(date).toLocaleDateString()
        },
        {
            title: "Actions",
            key: "actions",
            render: (_: any, record: AdminMerchant) => (
                <Button type="link" size="small" onClick={() => openAssignModal(record)}>
                    Assign Plan
                </Button>
            )
        }
    ];

    return (
        <PageContainer header={{title: ""}}>
            <Typography.Title level={3}>All Merchants</Typography.Title>
            <Table
                columns={columns}
                dataSource={merchants}
                rowKey="id"
                loading={loading}
                pagination={{
                    current: page,
                    pageSize,
                    total,
                    onChange: setPage,
                    showTotal: (t) => `${t} merchants`
                }}
            />

            <Modal
                title={`Assign Plan — ${selectedMerchant?.name || ""}`}
                open={assignModalOpen}
                onCancel={() => setAssignModalOpen(false)}
                onOk={handleAssignPlan}
                confirmLoading={assigning}
                okText="Assign Plan"
                okButtonProps={{disabled: !selectedPlanId}}
            >
                <div style={{marginBottom: 12}}>
                    <Typography.Text type="secondary">
                        Current plan:{" "}
                        <strong>{selectedMerchant?.active_plan_name || "None"}</strong>
                    </Typography.Text>
                </div>
                <Select
                    style={{width: "100%"}}
                    placeholder="Select a plan"
                    value={selectedPlanId || undefined}
                    onChange={setSelectedPlanId}
                    options={plans.map((p) => ({
                        label: `${p.name} — $${p.price_usd}/mo`,
                        value: p.id
                    }))}
                />
            </Modal>
        </PageContainer>
    );
};

export default AdminMerchantsPage;
