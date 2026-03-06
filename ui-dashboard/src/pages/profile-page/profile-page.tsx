import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {Row, Typography, Card, Form, Input, Button, notification, Space, Divider} from "antd";
import {CheckOutlined} from "@ant-design/icons";
import authProvider from "src/providers/auth-provider";

const ProfilePage: React.FC = () => {
    const [notificationApi, contextHolder] = notification.useNotification();
    const [profileForm] = Form.useForm();
    const [passwordForm] = Form.useForm();
    const [profileLoading, setProfileLoading] = React.useState(false);
    const [passwordLoading, setPasswordLoading] = React.useState(false);

    React.useEffect(() => {
        const loadProfile = async () => {
            try {
                const user = await authProvider.getMe();
                profileForm.setFieldsValue({
                    name: user.name,
                    email: user.email
                });
            } catch (e) {
                console.error("Failed to load profile", e);
            }
        };
        loadProfile();
    }, []);

    const handleProfileUpdate = async (values: {name: string; email: string}) => {
        try {
            setProfileLoading(true);
            const updated = await authProvider.updateProfile(values);
            profileForm.setFieldsValue({name: updated.name, email: updated.email});
            notificationApi.success({
                message: "Profile updated",
                description: "Your profile has been updated successfully.",
                placement: "bottomRight",
                icon: <CheckOutlined style={{color: "#49D1AC"}} />
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
                icon: <CheckOutlined style={{color: "#49D1AC"}} />
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

            <Card title="Account Information" style={{marginBottom: 24}}>
                <Form form={profileForm} layout="vertical" onFinish={handleProfileUpdate} style={{maxWidth: 480}}>
                    <Form.Item name="name" label="Display Name" rules={[{required: true, message: "Name is required"}]}>
                        <Input placeholder="Your display name" />
                    </Form.Item>
                    <Form.Item name="email" label="Email" rules={[{required: true, type: "email", message: "Valid email is required"}]}>
                        <Input placeholder="your@email.com" />
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
