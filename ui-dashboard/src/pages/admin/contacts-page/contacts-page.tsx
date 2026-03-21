import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {Table, Typography, Tag, Input, Button, Checkbox, Space, message} from "antd";
import {DownloadOutlined} from "@ant-design/icons";
import type {ColumnsType} from "antd/es/table";
import adminProvider, {AdminContact} from "src/providers/admin-provider";

const AdminContactsPage: React.FC = () => {
    const [contacts, setContacts] = React.useState<AdminContact[]>([]);
    const [total, setTotal] = React.useState(0);
    const [loading, setLoading] = React.useState(true);
    const [page, setPage] = React.useState(1);
    const [search, setSearch] = React.useState("");
    const [marketingOnly, setMarketingOnly] = React.useState(false);
    const [exporting, setExporting] = React.useState(false);
    const pageSize = 20;

    const loadContacts = async (p: number, searchTerm?: string) => {
        try {
            setLoading(true);
            const data = await adminProvider.listContacts(pageSize, (p - 1) * pageSize, searchTerm || search);
            setContacts(data.results || []);
            setTotal(data.total);
        } catch (e) {
            console.error("Failed to load contacts", e);
        } finally {
            setLoading(false);
        }
    };

    React.useEffect(() => {
        loadContacts(page);
    }, [page]);

    const handleSearch = (value: string) => {
        setSearch(value);
        setPage(1);
        loadContacts(1, value);
    };

    const handleExport = async () => {
        try {
            setExporting(true);
            const blob = await adminProvider.exportContacts(marketingOnly);
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement("a");
            a.href = url;
            a.download = `contacts${marketingOnly ? "-marketing" : ""}-${new Date().toISOString().slice(0, 10)}.csv`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            window.URL.revokeObjectURL(url);
            message.success("Contacts exported");
        } catch (e) {
            message.error("Failed to export contacts");
        } finally {
            setExporting(false);
        }
    };

    const columns: ColumnsType<AdminContact> = [
        {
            title: "Email",
            dataIndex: "email",
            key: "email",
            render: (email: string) => <strong>{email}</strong>
        },
        {
            title: "Source Merchant",
            dataIndex: "source_merchant_name",
            key: "source_merchant_name",
            render: (name: string) => name || "-"
        },
        {
            title: "Marketing Consent",
            dataIndex: "marketing_consent",
            key: "marketing_consent",
            render: (consent: boolean) =>
                consent ? <Tag color="green">Yes</Tag> : <Tag color="default">No</Tag>
        },
        {
            title: "Terms Accepted",
            dataIndex: "terms_accepted_at",
            key: "terms_accepted_at",
            render: (date: string | null) => (date ? new Date(date).toLocaleDateString() : "-")
        },
        {
            title: "Created",
            dataIndex: "created_at",
            key: "created_at",
            render: (date: string) => new Date(date).toLocaleDateString()
        }
    ];

    return (
        <PageContainer header={{title: ""}}>
            <Typography.Title level={3}>Contacts</Typography.Title>
            <Typography.Text type="secondary" style={{display: "block", marginBottom: 16}}>
                Invoice payers across all merchants — global CryptoLink consent tracking
            </Typography.Text>
            <Space style={{marginBottom: 16}} wrap>
                <Input.Search
                    placeholder="Search by email"
                    allowClear
                    onSearch={handleSearch}
                    style={{width: 300}}
                />
                <Checkbox
                    checked={marketingOnly}
                    onChange={(e) => setMarketingOnly(e.target.checked)}
                >
                    Export marketing-consented only
                </Checkbox>
                <Button
                    icon={<DownloadOutlined />}
                    onClick={handleExport}
                    loading={exporting}
                >
                    Export CSV
                </Button>
            </Space>
            <Table
                columns={columns}
                dataSource={contacts}
                rowKey="id"
                loading={loading}
                pagination={{
                    current: page,
                    pageSize,
                    total,
                    onChange: setPage,
                    showTotal: (t) => `${t} contacts`
                }}
            />
        </PageContainer>
    );
};

export default AdminContactsPage;
