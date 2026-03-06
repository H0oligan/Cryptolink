import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {
    Tabs, Card, Form, Input, InputNumber, Switch, Button, Space,
    Table, Typography, Tag, notification, Modal
} from "antd";
import {CheckOutlined, SendOutlined, SettingOutlined} from "@ant-design/icons";
import type {ColumnsType} from "antd/es/table";
import adminProvider, {EmailSettings, EmailLogEntry} from "src/providers/admin-provider";

const AdminEmailPage: React.FC = () => {
    const [notificationApi, contextHolder] = notification.useNotification();
    const [settingsForm] = Form.useForm();
    const [composeForm] = Form.useForm();
    const [settings, setSettings] = React.useState<EmailSettings | null>(null);
    const [logs, setLogs] = React.useState<EmailLogEntry[]>([]);
    const [logsTotal, setLogsTotal] = React.useState(0);
    const [logsPage, setLogsPage] = React.useState(1);
    const [settingsLoading, setSettingsLoading] = React.useState(true);
    const [saving, setSaving] = React.useState(false);
    const [testing, setTesting] = React.useState(false);
    const [sending, setSending] = React.useState(false);
    const [logsLoading, setLogsLoading] = React.useState(true);

    const loadSettings = async () => {
        try {
            setSettingsLoading(true);
            const data = await adminProvider.getEmailSettings();
            setSettings(data);
            settingsForm.setFieldsValue({
                smtp_host: data.smtp_host,
                smtp_port: data.smtp_port,
                smtp_user: data.smtp_user,
                smtp_pass: "",
                from_name: data.from_name,
                from_email: data.from_email,
                is_active: data.is_active
            });
        } catch (e) {
            console.error("Failed to load email settings", e);
        } finally {
            setSettingsLoading(false);
        }
    };

    const loadLogs = async (page: number) => {
        try {
            setLogsLoading(true);
            const data = await adminProvider.getEmailLogs(20, (page - 1) * 20);
            setLogs(data.results || []);
            setLogsTotal(data.total);
        } catch (e) {
            console.error("Failed to load email logs", e);
        } finally {
            setLogsLoading(false);
        }
    };

    React.useEffect(() => {
        loadSettings();
        loadLogs(1);
    }, []);

    const handleSaveSettings = async (values: any) => {
        try {
            setSaving(true);
            await adminProvider.updateEmailSettings(values);
            notificationApi.success({
                message: "Settings saved",
                placement: "bottomRight",
                icon: <CheckOutlined style={{color: "#49D1AC"}} />
            });
            await loadSettings();
        } catch (e: any) {
            notificationApi.error({
                message: "Save failed",
                description: e?.response?.data?.message || "Failed to save settings.",
                placement: "bottomRight"
            });
        } finally {
            setSaving(false);
        }
    };

    const handleTest = async () => {
        try {
            setTesting(true);
            const result = await adminProvider.testEmail();
            notificationApi.success({
                message: "Test email sent",
                description: result.message,
                placement: "bottomRight",
                icon: <CheckOutlined style={{color: "#49D1AC"}} />
            });
            await loadLogs(1);
        } catch (e: any) {
            notificationApi.error({
                message: "Test failed",
                description: e?.response?.data?.message || "Failed to send test email.",
                placement: "bottomRight"
            });
        } finally {
            setTesting(false);
        }
    };

    const handleSendEmail = async (values: any) => {
        try {
            setSending(true);
            await adminProvider.sendEmail(values);
            notificationApi.success({
                message: "Email sent",
                description: `Email sent to ${values.to}`,
                placement: "bottomRight",
                icon: <CheckOutlined style={{color: "#49D1AC"}} />
            });
            composeForm.resetFields();
            await loadLogs(1);
        } catch (e: any) {
            notificationApi.error({
                message: "Send failed",
                description: e?.response?.data?.message || "Failed to send email.",
                placement: "bottomRight"
            });
        } finally {
            setSending(false);
        }
    };

    const logColumns: ColumnsType<EmailLogEntry> = [
        {
            title: "To",
            dataIndex: "to_email",
            key: "to_email",
            width: 200
        },
        {
            title: "Subject",
            dataIndex: "subject",
            key: "subject"
        },
        {
            title: "Template",
            dataIndex: "template",
            key: "template",
            render: (t: string) => t ? <Tag>{t}</Tag> : "-"
        },
        {
            title: "Status",
            dataIndex: "status",
            key: "status",
            render: (status: string) => (
                <Tag color={status === "sent" ? "green" : "red"}>{status}</Tag>
            )
        },
        {
            title: "Sent",
            dataIndex: "created_at",
            key: "created_at",
            render: (date: string) => new Date(date).toLocaleString()
        }
    ];

    const tabItems = [
        {
            key: "settings",
            label: "SMTP Settings",
            children: (
                <Card loading={settingsLoading}>
                    <Form
                        form={settingsForm}
                        layout="vertical"
                        onFinish={handleSaveSettings}
                        style={{maxWidth: 500}}
                    >
                        <Form.Item name="smtp_host" label="SMTP Host" rules={[{required: true}]}>
                            <Input placeholder="smtp-relay.brevo.com" />
                        </Form.Item>
                        <Form.Item name="smtp_port" label="SMTP Port" rules={[{required: true}]}>
                            <InputNumber min={1} max={65535} style={{width: "100%"}} />
                        </Form.Item>
                        <Form.Item name="smtp_user" label="SMTP Username" rules={[{required: true}]}>
                            <Input placeholder="user@smtp-provider.com" />
                        </Form.Item>
                        <Form.Item name="smtp_pass" label="SMTP Password" tooltip="Leave empty to keep existing password">
                            <Input.Password placeholder="Enter to change, leave empty to keep" />
                        </Form.Item>
                        <Form.Item name="from_name" label="From Name" rules={[{required: true}]}>
                            <Input placeholder="CryptoLink" />
                        </Form.Item>
                        <Form.Item name="from_email" label="From Email" rules={[{required: true, type: "email"}]}>
                            <Input placeholder="contact@cryptolink.cc" />
                        </Form.Item>
                        <Form.Item name="is_active" label="Active" valuePropName="checked">
                            <Switch />
                        </Form.Item>
                        <Form.Item>
                            <Space>
                                <Button type="primary" htmlType="submit" loading={saving} icon={<SettingOutlined />}>
                                    Save Settings
                                </Button>
                                <Button onClick={handleTest} loading={testing}>
                                    Send Test Email
                                </Button>
                            </Space>
                        </Form.Item>
                    </Form>
                </Card>
            )
        },
        {
            key: "compose",
            label: "Compose",
            children: (
                <Card>
                    <Form
                        form={composeForm}
                        layout="vertical"
                        onFinish={handleSendEmail}
                        style={{maxWidth: 600}}
                    >
                        <Form.Item name="to" label="To" rules={[{required: true, type: "email"}]}>
                            <Input placeholder="recipient@example.com" />
                        </Form.Item>
                        <Form.Item name="subject" label="Subject" rules={[{required: true}]}>
                            <Input placeholder="Email subject" />
                        </Form.Item>
                        <Form.Item name="body" label="Body (HTML)" rules={[{required: true}]}>
                            <Input.TextArea rows={10} placeholder="<h2>Hello</h2><p>Your message here...</p>" />
                        </Form.Item>
                        <Form.Item>
                            <Button type="primary" htmlType="submit" loading={sending} icon={<SendOutlined />}>
                                Send Email
                            </Button>
                        </Form.Item>
                    </Form>
                </Card>
            )
        },
        {
            key: "logs",
            label: "Sent Log",
            children: (
                <Table
                    columns={logColumns}
                    dataSource={logs}
                    rowKey="id"
                    loading={logsLoading}
                    pagination={{
                        current: logsPage,
                        pageSize: 20,
                        total: logsTotal,
                        onChange: (p) => { setLogsPage(p); loadLogs(p); },
                        showTotal: (t) => `${t} emails`
                    }}
                />
            )
        }
    ];

    return (
        <PageContainer header={{title: ""}}>
            {contextHolder}
            <Typography.Title level={3}>Email Management</Typography.Title>
            <Tabs items={tabItems} />
        </PageContainer>
    );
};

export default AdminEmailPage;
