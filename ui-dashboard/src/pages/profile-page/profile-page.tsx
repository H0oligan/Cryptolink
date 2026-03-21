import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {Row, Typography, Card, Form, Input, Button, notification, Tag, Divider} from "antd";
import {CheckOutlined, CheckCircleOutlined, ExclamationCircleOutlined} from "@ant-design/icons";
import authProvider from "src/providers/auth-provider";

const ProfilePage: React.FC = () => {
    const [notificationApi, contextHolder] = notification.useNotification();
    const [profileForm] = Form.useForm();
    const [passwordForm] = Form.useForm();
    const [profileLoading, setProfileLoading] = React.useState(false);
    const [passwordLoading, setPasswordLoading] = React.useState(false);
    const [emailVerified, setEmailVerified] = React.useState<boolean | undefined>(undefined);

    React.useEffect(() => {
        const loadProfile = async () => {
            try {
                const user = await authProvider.getMe();
                profileForm.setFieldsValue({
                    name: user.name,
                    email: user.email,
                    companyName: user.companyName || "",
                    address: user.address || "",
                    website: user.website || "",
                    phone: user.phone || ""
                });
                setEmailVerified(user.emailVerified);
            } catch (e) {
                console.error("Failed to load profile", e);
            }
        };
        loadProfile();
    }, []);

    const handleProfileUpdate = async (values: {name: string; email: string; companyName: string; address: string; website: string; phone: string}) => {
        try {
            setProfileLoading(true);
            const updated = await authProvider.updateProfile(values);
            profileForm.setFieldsValue({
                name: updated.name,
                email: updated.email,
                companyName: updated.companyName || "",
                address: updated.address || "",
                website: updated.website || "",
                phone: updated.phone || ""
            });
            setEmailVerified(updated.emailVerified);
            notificationApi.success({
                message: "Profile updated",
                description: "Your profile has been updated successfully.",
                placement: "bottomRight",
                icon: <CheckOutlined style={{color: "#10b981"}} />
            });
        } catch (e: any) {
            notificationApi.error({
                message: "Update failed",
                description: e?.response?.data?.message || "Failed to update profile.",
                placement: "bottomRight"
            });
        } finally {
            setProfileLoading(false);
        }
    };

    const handlePasswordChange = async (values: {currentPassword: string; newPassword: string; confirmPassword: string}) => {
        if (values.newPassword !== values.confirmPassword) {
            notificationApi.error({
                message: "Passwords don't match",
                description: "New password and confirmation must match.",
                placement: "bottomRight"
            });
            return;
        }

        try {
            setPasswordLoading(true);
            await authProvider.updatePassword({
                currentPassword: values.currentPassword,
                newPassword: values.newPassword
            });
            passwordForm.resetFields();
            notificationApi.success({
                message: "Password changed",
                description: "Your password has been changed successfully.",
                placement: "bottomRight",
                icon: <CheckOutlined style={{color: "#10b981"}} />
            });
        } catch (e: any) {
            notificationApi.error({
                message: "Password change failed",
                description: e?.response?.data?.message || "Failed to change password.",
                placement: "bottomRight"
            });
        } finally {
            setPasswordLoading(false);
        }
    };

    return (
        <PageContainer header={{title: ""}}>
            {contextHolder}
            <Row align="middle" justify="space-between">
                <Typography.Title>Profile</Typography.Title>
            </Row>

            <Card title="Account Information" style={{marginBottom: 24}}
                extra={emailVerified !== undefined && (
                    emailVerified
                        ? <Tag icon={<CheckCircleOutlined />} color="success">Email Verified</Tag>
                        : <Tag icon={<ExclamationCircleOutlined />} color="warning">Email Not Verified</Tag>
                )}
            >
                <Form form={profileForm} layout="vertical" onFinish={handleProfileUpdate} style={{maxWidth: 480}}>
                    <Form.Item name="name" label="Display Name" rules={[{required: true, message: "Name is required"}]}>
                        <Input placeholder="Your display name" />
                    </Form.Item>
                    <Form.Item name="email" label="Email" rules={[{required: true, type: "email", message: "Valid email is required"}]}>
                        <Input placeholder="your@email.com" />
                    </Form.Item>
                    <Divider orientation="left" orientationMargin={0}>
                        <Typography.Text type="secondary" style={{fontSize: 13}}>Business Information</Typography.Text>
                    </Divider>
                    <Form.Item name="companyName" label="Company / Organization">
                        <Input placeholder="Your company name" />
                    </Form.Item>
                    <Form.Item name="address" label="Business Address">
                        <Input placeholder="Business address" />
                    </Form.Item>
                    <Form.Item name="website" label="Website">
                        <Input placeholder="https://example.com" />
                    </Form.Item>
                    <Form.Item name="phone" label="Phone">
                        <Input placeholder="+1 234 567 890" />
                    </Form.Item>
                    <Form.Item>
                        <Button type="primary" htmlType="submit" loading={profileLoading}>
                            Save Changes
                        </Button>
                    </Form.Item>
                </Form>
            </Card>

            <Card title="Change Password">
                <Form form={passwordForm} layout="vertical" onFinish={handlePasswordChange} style={{maxWidth: 480}}>
                    <Form.Item
                        name="currentPassword"
                        label="Current Password"
                        rules={[{required: true, message: "Current password is required"}]}
                    >
                        <Input.Password placeholder="Enter current password" />
                    </Form.Item>
                    <Form.Item
                        name="newPassword"
                        label="New Password"
                        rules={[
                            {required: true, message: "New password is required"},
                            {min: 8, message: "Password must be at least 8 characters"}
                        ]}
                    >
                        <Input.Password placeholder="Enter new password" />
                    </Form.Item>
                    <Form.Item
                        name="confirmPassword"
                        label="Confirm New Password"
                        rules={[
                            {required: true, message: "Please confirm your password"},
                            {min: 8, message: "Password must be at least 8 characters"}
                        ]}
                    >
                        <Input.Password placeholder="Confirm new password" />
                    </Form.Item>
                    <Form.Item>
                        <Button type="primary" htmlType="submit" loading={passwordLoading}>
                            Change Password
                        </Button>
                    </Form.Item>
                </Form>
            </Card>
        </PageContainer>
    );
};

export default ProfilePage;
