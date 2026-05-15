import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {
    Button, Table, Typography, Row, Tag, Modal, Form, Input, Select, Card,
    Drawer, Progress, Statistic, Space, Tabs, notification, Tooltip, Radio,
    Checkbox, Col
} from "antd";
import {
    SendOutlined, PlusOutlined, EyeOutlined, ReloadOutlined,
    MailOutlined, TeamOutlined, UserOutlined, FileTextOutlined, SettingOutlined
} from "@ant-design/icons";
import adminProvider, {Sequence, SequenceStats, MarketingTemplate, MarketingSettings} from "src/providers/admin-provider";

const {TextArea} = Input;
const {Text} = Typography;

const SequencesTab: React.FC = () => {
    const [api, contextHolder] = notification.useNotification();
    const [sequences, setSequences] = React.useState<Sequence[]>([]);
    const [templates, setTemplates] = React.useState<MarketingTemplate[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [builderOpen, setBuilderOpen] = React.useState(false);
    const [creating, setCreating] = React.useState(false);
    const [form] = Form.useForm();
    const [steps, setSteps] = React.useState<{template_id: string; offset_hours: number}[]>([
        {template_id: "dev_drip_hook", offset_hours: 0},
        {template_id: "dev_drip_proof", offset_hours: 72},
        {template_id: "dev_drip_math", offset_hours: 168}
    ]);
    const [detailUUID, setDetailUUID] = React.useState<string | null>(null);
    const [detailStats, setDetailStats] = React.useState<SequenceStats | null>(null);

    const loadData = async () => {
        setLoading(true);
        try {
            const [seqRes, tmplRes] = await Promise.all([
                adminProvider.listSequences(),
                adminProvider.listMarketingTemplates()
            ]);
            setSequences(seqRes.results || []);
            setTemplates(tmplRes);
        } catch (e) { console.error(e); }
        setLoading(false);
    };

    React.useEffect(() => { loadData(); }, []);

    const nextTuesday10UTC = (): string => {
        const now = new Date();
        const day = now.getUTCDay();
        const daysAhead = ((2 - day + 7) % 7) || 7;
        const tue = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate() + daysAhead, 10, 0, 0));
        return tue.toISOString();
    };

    const handleCreate = async () => {
        try {
            const values = await form.validateFields();
            setCreating(true);
            await adminProvider.createSequence({
                name: values.name,
                audience: values.audience,
                skip_if_converted: values.skip_if_converted !== false,
                start_at: values.start_at ? new Date(values.start_at).toISOString() : nextTuesday10UTC(),
                steps: steps
            });
            api.success({message: "Sequence created (draft)", placement: "bottomRight"});
            setBuilderOpen(false);
            form.resetFields();
            loadData();
        } catch (e: any) {
            if (e?.response?.data?.error) {
                api.error({message: e.response.data.error, placement: "bottomRight"});
            }
        } finally {
            setCreating(false);
        }
    };

    const handleLaunch = (seq: Sequence) => {
        Modal.confirm({
            title: `Launch sequence "${seq.name}"?`,
            content: `This will enroll all matching ${audienceLabels[seq.audience]} and start sending. Skip-if-converted: ${seq.skip_if_converted ? "ON" : "OFF"}.`,
            okText: "Launch",
            onOk: async () => {
                try {
                    await adminProvider.launchSequence(seq.uuid);
                    api.success({message: "Sequence launched", placement: "bottomRight"});
                    loadData();
                } catch (e: any) {
                    api.error({message: e?.response?.data?.error || "Failed to launch", placement: "bottomRight"});
                }
            }
        });
    };

    const handlePause = async (seq: Sequence) => {
        try { await adminProvider.pauseSequence(seq.uuid); loadData(); api.info({message: "Paused", placement: "bottomRight"}); }
        catch (e: any) { api.error({message: e?.response?.data?.error || "Failed", placement: "bottomRight"}); }
    };
    const handleResume = async (seq: Sequence) => {
        try { await adminProvider.resumeSequence(seq.uuid); loadData(); api.info({message: "Resumed", placement: "bottomRight"}); }
        catch (e: any) { api.error({message: e?.response?.data?.error || "Failed", placement: "bottomRight"}); }
    };
    const handleCancel = async (seq: Sequence) => {
        Modal.confirm({
            title: `Cancel sequence "${seq.name}"?`,
            content: "Active enrollments will stop receiving further emails. This cannot be undone.",
            okText: "Cancel sequence",
            okType: "danger",
            onOk: async () => {
                try { await adminProvider.cancelSequence(seq.uuid); loadData(); api.info({message: "Cancelled", placement: "bottomRight"}); }
                catch (e: any) { api.error({message: e?.response?.data?.error || "Failed", placement: "bottomRight"}); }
            }
        });
    };

    const openDetail = async (seq: Sequence) => {
        setDetailUUID(seq.uuid);
        try { setDetailStats(await adminProvider.getSequenceStats(seq.uuid)); }
        catch { setDetailStats(null); }
    };

    const sequenceColumns = [
        {title: "Name", dataIndex: "name", key: "name",
            render: (_: any, r: Sequence) => <a onClick={() => openDetail(r)} style={{color: "#10b981", cursor: "pointer"}}>{r.name}</a>},
        {title: "Steps", key: "steps", render: (_: any, r: Sequence) => r.steps.length},
        {title: "Audience", dataIndex: "audience", render: (v: string) => <Tag>{audienceLabels[v] || v}</Tag>},
        {title: "Status", dataIndex: "status", render: (v: string) => <Tag color={statusColors[v] || "default"}>{v.toUpperCase()}</Tag>},
        {title: "Started", dataIndex: "started_at", render: (v: string | null) => v ? new Date(v).toLocaleString() : "—"},
        {title: "Actions", key: "actions", render: (_: any, r: Sequence) => (
            <Space>
                <Button size="small" icon={<EyeOutlined />} onClick={() => openDetail(r)} />
                {r.status === "draft" && <Button size="small" type="primary" icon={<SendOutlined />} onClick={() => handleLaunch(r)}>Launch</Button>}
                {r.status === "running" && <Button size="small" onClick={() => handlePause(r)}>Pause</Button>}
                {r.status === "paused" && <Button size="small" type="primary" onClick={() => handleResume(r)}>Resume</Button>}
                {(r.status === "draft" || r.status === "running" || r.status === "paused") &&
                    <Button size="small" danger onClick={() => handleCancel(r)}>Cancel</Button>}
            </Space>
        )}
    ];

    return (
        <>
            {contextHolder}
            <Row align="middle" justify="end" style={{marginBottom: 12}}>
                <Space>
                    <Button icon={<ReloadOutlined />} onClick={loadData}>Refresh</Button>
                    <Button type="primary" icon={<PlusOutlined />} onClick={() => {
                        form.setFieldsValue({
                            name: "",
                            audience: "contacts_opted_in",
                            skip_if_converted: true,
                            start_at: nextTuesday10UTC()
                        });
                        setSteps([
                            {template_id: "dev_drip_hook", offset_hours: 0},
                            {template_id: "dev_drip_proof", offset_hours: 72},
                            {template_id: "dev_drip_math", offset_hours: 168}
                        ]);
                        setBuilderOpen(true);
                    }}>New Sequence</Button>
                </Space>
            </Row>

            <Table columns={sequenceColumns} dataSource={sequences} rowKey="uuid" loading={loading} size="middle" />

            <Drawer
                title="New Sequence"
                width={720}
                open={builderOpen}
                onClose={() => setBuilderOpen(false)}
                extra={
                    <Space>
                        <Button onClick={() => setBuilderOpen(false)}>Cancel</Button>
                        <Button type="primary" loading={creating} icon={<PlusOutlined />} onClick={handleCreate}>Create Draft</Button>
                    </Space>
                }
            >
                <Form form={form} layout="vertical">
                    <Form.Item label="Sequence Name" name="name" rules={[{required: true}]}>
                        <Input placeholder="e.g. Developer Onboarding Drip — May 2026" />
                    </Form.Item>
                    <Form.Item label="Audience" name="audience" rules={[{required: true}]} initialValue="contacts_opted_in">
                        <Radio.Group>
                            <Radio.Button value="contacts_opted_in"><TeamOutlined /> Opted-in Contacts</Radio.Button>
                            <Radio.Button value="merchants"><UserOutlined /> Merchants</Radio.Button>
                            <Radio.Button value="all"><MailOutlined /> Everyone</Radio.Button>
                        </Radio.Group>
                    </Form.Item>
                    <Form.Item label="Start time (ISO-8601 UTC)" name="start_at" tooltip="Recommended: next Tuesday 10:00 UTC. Leave blank to use default.">
                        <Input placeholder={nextTuesday10UTC()} />
                    </Form.Item>
                    <Form.Item name="skip_if_converted" valuePropName="checked" initialValue={true}>
                        <Checkbox>Skip if recipient becomes a merchant mid-sequence</Checkbox>
                    </Form.Item>

                    <div style={{marginTop: 16}}>
                        <Typography.Text strong>Steps</Typography.Text>
                        {steps.map((step, idx) => (
                            <Card key={idx} size="small" style={{marginTop: 8, background: "#0a0a0a", border: "1px solid #1e1e1e"}}>
                                <Row gutter={8} align="middle">
                                    <Col span={1}><Typography.Text>{idx + 1}</Typography.Text></Col>
                                    <Col span={14}>
                                        <Select
                                            value={step.template_id}
                                            style={{width: "100%"}}
                                            onChange={(v) => {
                                                const copy = [...steps];
                                                copy[idx].template_id = v;
                                                setSteps(copy);
                                            }}
                                            options={templates.map(t => ({value: t.id, label: t.name}))}
                                        />
                                    </Col>
                                    <Col span={6}>
                                        <Input
                                            type="number"
                                            min={0}
                                            addonAfter="h offset"
                                            value={step.offset_hours}
                                            onChange={(e) => {
                                                const copy = [...steps];
                                                copy[idx].offset_hours = parseInt(e.target.value || "0", 10);
                                                setSteps(copy);
                                            }}
                                        />
                                    </Col>
                                    <Col span={3}>
                                        <Button size="small" danger onClick={() => setSteps(steps.filter((_, i) => i !== idx))}>Remove</Button>
                                    </Col>
                                </Row>
                            </Card>
                        ))}
                        <Button block style={{marginTop: 8}} icon={<PlusOutlined />}
                            onClick={() => setSteps([...steps, {template_id: templates[0]?.id || "", offset_hours: (steps[steps.length - 1]?.offset_hours || 0) + 72}])}>
                            Add Step
                        </Button>
                    </div>
                </Form>
            </Drawer>

            <Drawer
                title={detailStats?.sequence?.name || "Sequence"}
                width={720}
                open={detailUUID !== null}
                onClose={() => { setDetailUUID(null); setDetailStats(null); }}
            >
                {detailStats && (
                    <>
                        <Card size="small" style={{marginBottom: 16}}>
                            <Row justify="space-around">
                                <Statistic title="Enrolled" value={detailStats.total_enrolled} />
                                <Statistic title="Active" value={detailStats.active} valueStyle={{color: "#f59e0b"}} />
                                <Statistic title="Converted" value={detailStats.converted} valueStyle={{color: "#10b981"}} />
                                <Statistic title="Unsub'd" value={detailStats.unsubscribed} valueStyle={{color: "#94a3b8"}} />
                                <Statistic title="Completed" value={detailStats.completed} valueStyle={{color: "#10b981"}} />
                                <Statistic title="Failed" value={detailStats.failed} valueStyle={{color: "#ef4444"}} />
                            </Row>
                        </Card>
                        <Typography.Title level={5}>Per-step funnel</Typography.Title>
                        <Table
                            size="small"
                            pagination={false}
                            dataSource={detailStats.step_breakdown}
                            rowKey="step_index"
                            columns={[
                                {title: "Step", dataIndex: "step_index"},
                                {title: "Sent", dataIndex: "sent"},
                                {title: "Pending", dataIndex: "pending"}
                            ]}
                        />
                    </>
                )}
            </Drawer>
        </>
    );
};

const TemplatesTab: React.FC<{templates: MarketingTemplate[]}> = ({templates}) => {
    const [activeTag, setActiveTag] = React.useState<string | null>(null);
    const [previewTemplate, setPreviewTemplate] = React.useState<MarketingTemplate | null>(null);

    const allTags = React.useMemo(() => {
        const set = new Set<string>();
        templates.forEach(t => (t.tags || []).forEach(tag => set.add(tag)));
        return Array.from(set).sort();
    }, [templates]);

    const visible = activeTag ? templates.filter(t => (t.tags || []).includes(activeTag)) : templates;

    return (
        <>
            <div style={{background: "#0a0a0a", border: "1px solid #1e1e1e", borderRadius: 8, padding: 12, marginBottom: 16, color: "#94a3b8", fontSize: 13}}>
                Templates live in <code style={{color: "#10b981"}}>internal/service/marketing/templates.go</code>. New templates require a PR + redeploy — keeping them code-defined prevents typos from breaking a send.
            </div>

            <Space wrap style={{marginBottom: 16}}>
                <Tag
                    color={activeTag === null ? "green" : "default"}
                    style={{cursor: "pointer"}}
                    onClick={() => setActiveTag(null)}
                >All ({templates.length})</Tag>
                {allTags.map(tag => (
                    <Tag
                        key={tag}
                        color={activeTag === tag ? "green" : "default"}
                        style={{cursor: "pointer"}}
                        onClick={() => setActiveTag(tag)}
                    >{tag}</Tag>
                ))}
            </Space>

            <Row gutter={[16, 16]}>
                {visible.map(t => (
                    <Col key={t.id} xs={24} sm={12} lg={8}>
                        <Card
                            size="small"
                            title={t.name}
                            extra={<Button size="small" icon={<EyeOutlined />} onClick={() => setPreviewTemplate(t)}>Preview</Button>}
                            style={{background: "#0a0a0a", border: "1px solid #1e1e1e"}}
                        >
                            <Space direction="vertical" size={4} style={{width: "100%"}}>
                                <Typography.Text type="secondary" style={{fontSize: 12}}>Subject:</Typography.Text>
                                <Typography.Text style={{color: "#e2e8f0", fontSize: 13}}>{t.subject}</Typography.Text>
                                <Typography.Text type="secondary" style={{fontSize: 12, marginTop: 6}}>{t.description}</Typography.Text>
                                <Space size={4} style={{marginTop: 6}}>
                                    {(t.tags || []).map(tag => <Tag key={tag} color="green">{tag}</Tag>)}
                                </Space>
                            </Space>
                        </Card>
                    </Col>
                ))}
            </Row>

            <Modal
                title={previewTemplate?.name || "Preview"}
                open={previewTemplate !== null}
                onCancel={() => setPreviewTemplate(null)}
                footer={null}
                width={680}
                styles={{body: {padding: 0}}}
            >
                <iframe
                    title="email preview"
                    sandbox=""
                    srcDoc={previewTemplate?.body_html || ""}
                    style={{width: "100%", height: 600, border: 0, background: "#050505"}}
                />
            </Modal>
        </>
    );
};

const SettingsTab: React.FC = () => {
    const [api, contextHolder] = notification.useNotification();
    const [settings, setSettings] = React.useState<MarketingSettings | null>(null);
    const [quota, setQuota] = React.useState<{sent: number; limit: number; remaining: number; reset_at: string} | null>(null);
    const [dailyLimit, setDailyLimit] = React.useState<number>(250);
    const [saving, setSaving] = React.useState(false);

    const loadData = async () => {
        try {
            const [s, q] = await Promise.all([adminProvider.getMarketingSettings(), adminProvider.getMarketingQuota()]);
            setSettings(s);
            setDailyLimit(s.daily_limit);
            setQuota(q);
        } catch (e) { console.error(e); }
    };

    React.useEffect(() => { loadData(); }, []);

    const handleSave = async () => {
        if (dailyLimit < 50 || dailyLimit > 250) {
            api.error({message: "Daily limit must be between 50 and 250", placement: "bottomRight"});
            return;
        }
        setSaving(true);
        try {
            const updated = await adminProvider.updateMarketingSettings(dailyLimit);
            setSettings(updated);
            api.success({message: "Settings saved", placement: "bottomRight"});
            loadData();
        } catch (e: any) {
            api.error({message: e?.response?.data?.error || "Failed", placement: "bottomRight"});
        } finally {
            setSaving(false);
        }
    };

    return (
        <>
            {contextHolder}
            <Card title="Daily Email Budget" style={{marginBottom: 16}}>
                <Typography.Paragraph type="secondary">
                    Brevo plan ceiling: <strong>300/day</strong>. Reservation for transactional sends (invoices, signup, password reset): <strong>50/day</strong>. Marketing cap is therefore clamped to 250.
                </Typography.Paragraph>
                <Row align="middle" gutter={16}>
                    <Col>
                        <Input
                            type="number"
                            min={50}
                            max={250}
                            value={dailyLimit}
                            onChange={(e) => setDailyLimit(parseInt(e.target.value || "0", 10))}
                            style={{width: 120}}
                            addonAfter="/ day"
                        />
                    </Col>
                    <Col>
                        <Button type="primary" loading={saving} onClick={handleSave}>Save</Button>
                    </Col>
                    {settings && <Col><Typography.Text type="secondary">Last updated: {new Date(settings.updated_at).toLocaleString()}</Typography.Text></Col>}
                </Row>
            </Card>

            <Card title="Today's Quota">
                {quota ? (
                    <Row justify="space-around">
                        <Statistic title="Sent today" value={quota.sent} suffix={`/ ${quota.limit}`}
                            valueStyle={{color: quota.remaining > 0 ? "#10b981" : "#ef4444"}} />
                        <Statistic title="Remaining" value={quota.remaining}
                            valueStyle={{color: quota.remaining > 50 ? "#10b981" : quota.remaining > 0 ? "#f59e0b" : "#ef4444"}} />
                        <Statistic title="Resets" value={new Date(quota.reset_at).toLocaleString()}
                            valueStyle={{fontSize: 14, color: "#94a3b8"}} />
                    </Row>
                ) : (
                    <Typography.Text type="secondary">Loading...</Typography.Text>
                )}
            </Card>
        </>
    );
};

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

    const campaignsTab = (
        <>
            {quota && (
                <Card size="small" style={{marginBottom: 16}}>
                    <Row justify="space-around">
                        <Statistic title="Emails Sent Today" value={quota.sent} suffix={`/ ${quota.limit}`} valueStyle={{color: quota.remaining > 0 ? "#10b981" : "#ef4444"}} />
                        <Statistic title="Remaining" value={quota.remaining} valueStyle={{color: quota.remaining > 50 ? "#10b981" : quota.remaining > 0 ? "#f59e0b" : "#ef4444"}} />
                        <Statistic title="Quota Resets" value={new Date(quota.reset_at).toLocaleString()} valueStyle={{fontSize: 14, color: "#94a3b8"}} />
                    </Row>
                </Card>
            )}
            <Row align="middle" justify="end" style={{marginBottom: 12}}>
                <Space>
                    <Button icon={<ReloadOutlined />} onClick={loadData}>Refresh</Button>
                    <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>New Campaign</Button>
                </Space>
            </Row>
            <Table
                columns={columns}
                dataSource={campaigns}
                rowKey="uuid"
                loading={loading}
                pagination={{total, pageSize: 20}}
                size="middle"
            />
        </>
    );

    return (
        <PageContainer header={{title: "", breadcrumb: {}}}>
            {contextHolder}
            <Row align="middle" justify="space-between" style={{marginBottom: 16}}>
                <Typography.Title level={3} style={{margin: 0}}>Marketing</Typography.Title>
            </Row>

            <Tabs
                defaultActiveKey="campaigns"
                items={[
                    {key: "campaigns", label: <span><MailOutlined /> Campaigns</span>, children: campaignsTab},
                    {key: "sequences", label: <span><SendOutlined /> Sequences</span>, children: <SequencesTab />},
                    {key: "templates", label: <span><FileTextOutlined /> Templates</span>, children: <TemplatesTab templates={templates as unknown as MarketingTemplate[]} />},
                    {key: "settings", label: <span><SettingOutlined /> Settings</span>, children: <SettingsTab />}
                ]}
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
