import * as React from "react";
import {PageContainer} from "@ant-design/pro-components";
import {Row, Typography, notification, Divider} from "antd";
import {CheckOutlined} from "@ant-design/icons";
import PaymentMethodsSelect from "src/components/payment-methods-select/payment-methods-select";
import ApiKeysSection from "src/components/api-keys-section/api-keys-section";
import DevelopersSection from "src/components/developers-section/developers-section";

const SettingsPage: React.FC = () => {
    const [notificationApi, contextHolder] = notification.useNotification();

    const openNotification = (title: string, description: string) => {
        notificationApi.info({
            message: title,
            description,
            placement: "bottomRight",
            icon: <CheckOutlined style={{color: "#49D1AC"}} />
        });
    };

    return (
        <PageContainer header={{title: ""}}>
            {contextHolder}
            <Row align="middle" justify="space-between">
                <Typography.Title>Settings</Typography.Title>
            </Row>
            <PaymentMethodsSelect />
            <Divider />
            <DevelopersSection openPopupFunc={openNotification} />
            <ApiKeysSection openPopupFunc={openNotification} />
        </PageContainer>
    );
};

export default SettingsPage;
