import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {
    Button, Table, Typography, Row, Tag, Modal, Form, Input, Select, Card,
    Drawer, Progress, Statistic, Space, Tabs, notification, Tooltip, Radio
} from "antd";
import {
    SendOutlined, PlusOutlined, EyeOutlined, ReloadOutlined,
    MailOutlined, TeamOutlined, UserOutlined
} from "@ant-design/icons";
import adminProvider from "src/providers/admin-provider";

const {TextArea} = Input;
const {Text} = Typography;

interface EmailTemplate {
    id: string;
    name: string;
    description: string;
    subject: string;
    body_html: string;
}

interface Campaign {
    id: number;
    uuid: string;
    name: string;
    subject: string;
    body_html: string;
    template_id: string | null;
    audience: string;
    status: string;
    total_recipients: number;
    sent_count: number;
    failed_count: number;
    pending_count: number;
    created_at: string;
    started_at: string | null;
    completed_at: string | null;
}

interface Recipient {
    id: number;
    email: string;
    status: string;
    sent_at: string | null;
    error_message: string | null;
}

interface Quota {
    sent: number;
    limit: number;
    remaining: number;
    reset_at: string;
}

const audienceLabels: Record<string, string> = {
    merchants: "All Merchants",
    contacts_opted_in: "Opted-in Contacts",
    all: "Everyone (Merchants + Contacts)"
};

const statusColors: Record<string, string> = {
    draft: "default",
    sending: "processing",
    paused: "warning",
    completed: "success",
    cancelled: "error"
};

const MarketingPage: React.FC = () => {
    const [api, contextHolder] = notification.useNotification();
    const [campaigns, setCampaigns] = React.useState<Campaign[]>([]);
    const [templates, setTemplates] = React.useState<EmailTemplate[]>([]);
    const [quota, setQuota] = React.useState<Quota | null>(null);
    const [loading, setLoading] = React.useState(true);
    const [total, setTotal] = React.useState(0);

    // Create campaign
    const [createOpen, setCreateOpen] = React.useState(false);
    const [form] = Form.useForm();
    const [selectedTemplate, setSelectedTemplate] = React.useState<EmailTemplate | null>(null);
    const [previewOpen, setPreviewOpen] = React.useState(false);
    const [previewHtml, setPreviewHtml] = React.useState("");
    const [creating, setCreating] = React.useState(false);
    const bodyHtml = Form.useWatch("body_html", form);

    // Campaign detail
    const [detailCampaign, setDetailCampaign] = React.useState<Campaign | null>(null);
    const [recipients, setRecipients] = React.useState<Recipient[]>([]);
    const [recipientsTotal, setRecipientsTotal] = React.useState(0);
    const [detailOpen, setDetailOpen] = React.useState(false);

    const loadData = async () => {
        setLoading(true);
        try {
            const [campRes, tmplRes, quotaRes] = await Promise.all([
                adminProvider.listCampaigns(),
                adminProvider.listMarketingTemplates(),
                adminProvider.getMarketingQuota()
            ]);
            setCampaigns(campRes.results || []);
            setTotal(campRes.total || 0);
            setTemplates(tmplRes);
            setQuota(quotaRes);
        } catch (e) {
            console.error(e);
        }
        setLoading(false);
    };

    React.useEffect(() => { loadData(); }, []);

    const handleTemplateSelect = (templateId: string) => {
        const t = templates.find(t => t.id === templateId);
        setSelectedTemplate(t || null);
        if (t) {
            form.setFieldsValue({subject: t.subject, body_html: t.body_html, template_id: t.id});
        }
    };

    const handlePreview = () => {
        const html = form.getFieldValue("body_html") || selectedTemplate?.body_html || "";
        setPreviewHtml(html);
        setPreviewOpen(true);
    };

    const handleCreate = async () => {
        try {
            const values = await form.validateFields();
            setCreating(true);
            await adminProvider.createCampaign(values);
            api.success({message: "Campaign created", placement: "bottomRight"});
            setCreateOpen(false);
            form.resetFields();
            setSelectedTemplate(null);
            loadData();
        } catch (e: any) {
            if (e?.response?.data?.error) {
                api.error({message: e.response.data.error, placement: "bottomRight"});
            }
        } finally {
            setCreating(false);
        }
    };

    const handleSend = async (campaign: Campaign) => {
        Modal.confirm({
            title: `Send campaign "${campaign.name}"?`,
            content: `This will queue emails to the ${audienceLabels[campaign.audience] || campaign.audience} audience. Emails are sent at max 200/day.`,
            okText: "Send",
            okType: "primary",
            onOk: async () => {
                try {
                    await adminProvider.sendCampaign(campaign.uuid);
                    api.success({message: "Campaign queued for sending!", placement: "bottomRight"});
                    loadData();
                } catch (e: any) {
                    api.error({message: e?.response?.data?.error || "Failed to send", placement: "bottomRight"});
                }
            }
        });
    };

    const openDetail = async (campaign: Campaign) => {
        setDetailCampaign(campaign);
        setDetailOpen(true);
        try {
            const res = await adminProvider.getCampaignRecipients(campaign.uuid);
            setRecipients(res.results || []);
            setRecipientsTotal(res.total || 0);
        } catch { setRecipients([]); }
    };

    const columns = [
        {
            title: "Name", dataIndex: "name", key: "name",
            render: (_: any, r: Campaign) => (
                <a onClick={() => openDetail(r)} style={{color: "#10b981", cursor: "pointer"}}>{r.name}</a>
            )
        },
        {title: "Subject", dataIndex: "subject", key: "subject", ellipsis: true},
        {
            title: "Audience", dataIndex: "audience", key: "audience",
            render: (v: string) => <Tag>{audienceLabels[v] || v}</Tag>
        },
        {
            title: "Status", dataIndex: "status", key: "status",
            render: (v: string) => <Tag color={statusColors[v] || "default"}>{v.toUpperCase()}</Tag>
        },
        {
            title: "Progress", key: "progress",
            render: (_: any, r: Campaign) => r.total_recipients > 0 ? (
                <Tooltip title={`${r.sent_count} sent / ${r.failed_count} failed / ${r.pending_count} pending`}>
                    <Progress
                        percent={Math.round(((r.sent_count + r.failed_count) / r.total_recipients) * 100)}
                        success={{percent: Math.round((r.sent_count / r.total_recipients) * 100)}}
                        size="small"
                        style={{width: 120}}
                    />
                </Tooltip>
            ) : <Text type="secondary">—</Text>
        },
        {
            title: "Created", dataIndex: "created_at", key: "created_at",
            render: (v: string) => new Date(v).toLocaleDateString()
        },
        {
            title: "Actions", key: "actions",
            render: (_: any, r: Campaign) => (
                <Space>
                    <Tooltip title="View details">
                        <Button size="small" icon={<EyeOutlined />} onClick={() => openDetail(r)} />
                    </Tooltip>
                    {(r.status === "draft" || r.status === "paused") && (
                        <Tooltip title="Send campaign">
                            <Button size="small" type="primary" icon={<SendOutlined />} onClick={() => handleSend(r)} />
                        </Tooltip>
                    )}
                </Space>
            )
        }
    ];

    const recipientColumns = [
        {title: "Email", dataIndex: "email", key: "email"},
        {
            title: "Status", dataIndex: "status", key: "status",
            render: (v: string) => (
                <Tag color={v === "sent" ? "success" : v === "failed" ? "error" : v === "pending" ? "default" : "warning"}>
                    {v.toUpperCase()}
                </Tag>
            )
        },
        {title: "Sent At", dataIndex: "sent_at", key: "sent_at", render: (v: string | null) => v ? new Date(v).toLocaleString() : "—"},
        {title: "Error", dataIndex: "error_message", key: "error_message", ellipsis: true, render: (v: string | null) => v || "—"}
    ];

    return (
        <PageContainer header={{title: "", breadcrumb: {}}}>
            {contextHolder}
            <Row align="middle" justify="space-between" style={{marginBottom: 16}}>
                <Typography.Title level={3} style={{margin: 0}}>Marketing Campaigns</Typography.Title>
                <Space>
                    <Button icon={<ReloadOutlined />} onClick={loadData}>Refresh</Button>
                    <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>New Campaign</Button>
                </Space>
            </Row>

            {quota && (
                <Card size="small" style={{marginBottom: 16}}>
                    <Row justify="space-around">
                        <Statistic title="Emails Sent Today" value={quota.sent} suffix={`/ ${quota.limit}`} valueStyle={{color: quota.remaining > 0 ? "#10b981" : "#ef4444"}} />
                        <Statistic title="Remaining" value={quota.remaining} valueStyle={{color: quota.remaining > 50 ? "#10b981" : quota.remaining > 0 ? "#f59e0b" : "#ef4444"}} />
                        <Statistic title="Quota Resets" value={new Date(quota.reset_at).toLocaleString()} valueStyle={{fontSize: 14, color: "#94a3b8"}} />
                    </Row>
                </Card>
            )}

            <Table
                columns={columns}
                dataSource={campaigns}
                rowKey="uuid"
                loading={loading}
                pagination={{total, pageSize: 20}}
                size="middle"
            />

            {/* Create Campaign Drawer */}
            <Drawer
                title="Create Campaign"
                width={720}
                open={createOpen}
                onClose={() => { setCreateOpen(false); form.resetFields(); setSelectedTemplate(null); }}
                extra={
                    <Space>
                        <Button onClick={handlePreview} icon={<EyeOutlined />}>Preview</Button>
                        <Button type="primary" onClick={handleCreate} loading={creating} icon={<SendOutlined />}>Create</Button>
                    </Space>
                }
            >
                <Form form={form} layout="vertical">
                    <Form.Item label="Campaign Name" name="name" rules={[{required: true}]}>
                        <Input placeholder="e.g. March 2026 Newsletter" />
                    </Form.Item>

                    <Form.Item label="Email Template" name="template_id">
                        <Select
                            placeholder="Choose a predefined template or write custom..."
                            allowClear
                            onChange={handleTemplateSelect}
                            options={templates.map(t => ({
                                value: t.id,
                                label: (
                                    <Space>
                                        <MailOutlined />
                                        <span>{t.name}</span>
                                        <Text type="secondary" style={{fontSize: 12}}>— {t.description}</Text>
                                    </Space>
                                )
                            }))}
                        />
                    </Form.Item>

                    {selectedTemplate && (
                        <Card size="small" style={{marginBottom: 16, background: "#0a0a0a", border: "1px solid #1e1e1e"}}>
                            <Space direction="vertical" style={{width: "100%"}}>
                                <Text strong style={{color: "#10b981"}}>{selectedTemplate.name}</Text>
                                <Text type="secondary">{selectedTemplate.description}</Text>
                                <Button size="small" onClick={handlePreview} icon={<EyeOutlined />}>Preview Template</Button>
                            </Space>
                        </Card>
                    )}

                    <Form.Item label="Subject" name="subject" rules={[{required: true}]}>
                        <Input placeholder="Email subject line" />
                    </Form.Item>

                    <Form.Item label="Audience" name="audience" rules={[{required: true}]} initialValue="contacts_opted_in">
                        <Radio.Group>
                            <Radio.Button value="contacts_opted_in"><TeamOutlined /> Opted-in Contacts</Radio.Button>
                            <Radio.Button value="merchants"><UserOutlined /> All Merchants</Radio.Button>
                            <Radio.Button value="all"><MailOutlined /> Everyone</Radio.Button>
                        </Radio.Group>
                    </Form.Item>

                    <Form.Item label="Email Body (HTML)" name="body_html" rules={[{required: true}]}>
                        <TextArea rows={12} placeholder="Paste HTML email content here..." style={{fontFamily: "monospace", fontSize: 12}} />
                    </Form.Item>
                </Form>
            </Drawer>

            {/* Email Preview Modal */}
            <Modal
                title="Email Preview"
                open={previewOpen}
                onCancel={() => setPreviewOpen(false)}
                footer={null}
                width={680}
                styles={{body: {padding: 0}}}
            >
                <div
                    style={{maxHeight: 600, overflow: "auto", background: "#050505", borderRadius: 8}}
                    dangerouslySetInnerHTML={{__html: previewHtml}}
                />
            </Modal>

            {/* Campaign Detail Drawer */}
            <Drawer
                title={detailCampaign?.name || "Campaign Details"}
                width={700}
                open={detailOpen}
                onClose={() => { setDetailOpen(false); setDetailCampaign(null); }}
            >
                {detailCampaign && (
                    <>
                        <Card size="small" style={{marginBottom: 16}}>
                            <Row justify="space-around">
                                <Statistic title="Total" value={detailCampaign.total_recipients} />
                                <Statistic title="Sent" value={detailCampaign.sent_count} valueStyle={{color: "#10b981"}} />
                                <Statistic title="Failed" value={detailCampaign.failed_count} valueStyle={{color: "#ef4444"}} />
                                <Statistic title="Pending" value={detailCampaign.pending_count} valueStyle={{color: "#f59e0b"}} />
                            </Row>
                            {detailCampaign.total_recipients > 0 && (
                                <Progress
                                    percent={Math.round(((detailCampaign.sent_count + detailCampaign.failed_count) / detailCampaign.total_recipients) * 100)}
                                    success={{percent: Math.round((detailCampaign.sent_count / detailCampaign.total_recipients) * 100)}}
                                    style={{marginTop: 12}}
                                />
                            )}
                        </Card>

                        <Tabs items={[
                            {
                                key: "recipients",
                                label: `Recipients (${recipientsTotal})`,
                                children: (
                                    <Table
                                        columns={recipientColumns}
                                        dataSource={recipients}
                                        rowKey="id"
                                        size="small"
                                        pagination={{pageSize: 20, total: recipientsTotal}}
                                    />
                                )
                            },
                            {
                                key: "preview",
                                label: "Email Preview",
                                children: (
                                    <div
                                        style={{maxHeight: 500, overflow: "auto", background: "#050505", borderRadius: 8, padding: 8}}
                                        dangerouslySetInnerHTML={{__html: detailCampaign.body_html}}
                                    />
                                )
                            }
                        ]} />

                        {(detailCampaign.status === "draft" || detailCampaign.status === "paused") && (
                            <div style={{marginTop: 16, textAlign: "right"}}>
                                <Button type="primary" icon={<SendOutlined />} onClick={() => handleSend(detailCampaign)}>
                                    Send Campaign
                                </Button>
                            </div>
                        )}
                    </>
                )}
            </Drawer>
        </PageContainer>
    );
};

export default MarketingPage;
