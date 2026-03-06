import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {Table, Typography, Tag} from "antd";
import type {ColumnsType} from "antd/es/table";
import adminProvider, {AdminUser} from "src/providers/admin-provider";

const AdminUsersPage: React.FC = () => {
    const [users, setUsers] = React.useState<AdminUser[]>([]);
    const [total, setTotal] = React.useState(0);
    const [loading, setLoading] = React.useState(true);
    const [page, setPage] = React.useState(1);
    const pageSize = 20;

    const loadUsers = async (p: number) => {
        try {
            setLoading(true);
            const data = await adminProvider.listUsers(pageSize, (p - 1) * pageSize);
            setUsers(data.results || []);
            setTotal(data.total);
        } catch (e) {
            console.error("Failed to load users", e);
        } finally {
            setLoading(false);
        }
    };

    React.useEffect(() => {
        loadUsers(page);
    }, [page]);

    const columns: ColumnsType<AdminUser> = [
        {
            title: "Email",
            dataIndex: "email",
            key: "email",
            render: (email: string) => <strong>{email}</strong>
        },
        {
            title: "Name",
            dataIndex: "name",
            key: "name"
        },
        {
            title: "Role",
            dataIndex: "is_super_admin",
            key: "role",
            render: (isAdmin: boolean) => (
                <Tag color={isAdmin ? "gold" : "default"}>
                    {isAdmin ? "Super Admin" : "Merchant"}
                </Tag>
            )
        },
        {
            title: "Merchants",
            dataIndex: "merchant_count",
            key: "merchant_count"
        },
        {
            title: "Registered",
            dataIndex: "created_at",
            key: "created_at",
            render: (date: string) => new Date(date).toLocaleDateString()
        }
    ];

    return (
        <PageContainer header={{title: ""}}>
            <Typography.Title level={3}>All Users</Typography.Title>
            <Table
                columns={columns}
                dataSource={users}
                rowKey="id"
                loading={loading}
                pagination={{
                    current: page,
                    pageSize,
                    total,
                    onChange: setPage,
                    showTotal: (t) => `${t} users`
                }}
            />
        </PageContainer>
    );
};

export default AdminUsersPage;
