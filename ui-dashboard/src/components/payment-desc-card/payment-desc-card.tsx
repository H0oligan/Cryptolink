import "./payment-desc-card.scss";

import * as React from "react";
import {Descriptions, Tag, Button, Tooltip, Space} from "antd";
import {CopyOutlined, LinkOutlined} from "@ant-design/icons";
import bevis from "src/utils/bevis";
import {Payment, CURRENCY_SYMBOL} from "src/types";
import PaymentStatusLabel from "src/components/payment-status/payment-status";
import SpinWithMask from "src/components/spin-with-mask/spin-with-mask";
import copyToClipboard from "src/utils/copy-to-clipboard";
import TimeLabel from "src/components/time-label/time-label";
import renderStrippedStr from "src/utils/render-stripped-str";

interface Props {
    data?: Payment;
    openNotificationFunc: (title: string, description: string) => void;
}

const emptyState: Payment = {
    id: "loading",
    orderId: "loading",
    price: "",
    type: "payment",
    createdAt: "1997-05-01 15:00",
    status: "failed",
    currency: "USD",
    redirectUrl: "loading",
    paymentUrl: "loading",
    description: "loading",
    isTest: false
};

const b = bevis("payment-desc-card");

const displayPrice = (record: Payment) => {
    let ticker = record.currency + " ";
    if (record.currency in CURRENCY_SYMBOL && CURRENCY_SYMBOL[record.currency] !== "") {
        ticker = CURRENCY_SYMBOL[record.currency];
    }

    return ticker + record.price;
};

const truncateHash = (hash: string) => {
    if (hash.length <= 20) return hash;
    return hash.slice(0, 12) + "..." + hash.slice(-10);
};

const HashWithCopy = ({
    value,
    onCopy
}: {
    value: string;
    onCopy: () => void;
}) => (
    <Space>
        <span style={{fontFamily: "monospace", fontSize: 12}}>{truncateHash(value)}</span>
        <Tooltip title="Copy">
            <CopyOutlined style={{cursor: "pointer", color: "#6366f1"}} onClick={onCopy} />
        </Tooltip>
    </Space>
);

const PaymentDescCard: React.FC<Props> = ({data, openNotificationFunc}) => {
    React.useEffect(() => {
        if (!data) {
            data = emptyState;
        }
    }, [data]);

    const paymentInfo = data?.additionalInfo?.payment;

    return (
        <>
            <SpinWithMask isLoading={!data} />
            {data && (
                <>
                    <Descriptions column={1} size="small" bordered>
                        <Descriptions.Item label={<span className={b("item-title")}>ID</span>}>
                            <span className={data.isTest ? b("test-label") : ""}>{data.id}</span>{" "}
                            {data.isTest && <Tag color="yellow">test payment</Tag>}
                        </Descriptions.Item>
                        <Descriptions.Item label={<span className={b("item-title")}>Status</span>}>
                            <PaymentStatusLabel status={data.status} />
                        </Descriptions.Item>
                        <Descriptions.Item label={<span className={b("item-title")}>Created at</span>}>
                            <TimeLabel time={data.createdAt} />
                        </Descriptions.Item>
                        <Descriptions.Item label={<span className={b("item-title")}>Order ID</span>}>
                            {data.orderId ?? "Not provided"}
                        </Descriptions.Item>
                        <Descriptions.Item label={<span className={b("item-title")}>Price</span>}>
                            {displayPrice(data)}
                        </Descriptions.Item>
                        <Descriptions.Item label={<span className={b("item-title")}>Description</span>}>
                            {data.description ?? "Not provided"}
                        </Descriptions.Item>
                        <Descriptions.Item label={<span className={b("item-title")}>Payment URL</span>}>
                            <Space>
                                <span className={b("link__text")}>{renderStrippedStr(data?.paymentUrl ?? "")}</span>
                                <CopyOutlined
                                    className={b("link")}
                                    onClick={() =>
                                        copyToClipboard(data?.paymentUrl ? data.paymentUrl : "", openNotificationFunc)
                                    }
                                />
                            </Space>
                        </Descriptions.Item>

                        {paymentInfo?.customerEmail && (
                            <Descriptions.Item label={<span className={b("item-title")}>Customer</span>}>
                                {paymentInfo.customerEmail}
                            </Descriptions.Item>
                        )}

                        {paymentInfo?.selectedCurrency && (
                            <Descriptions.Item label={<span className={b("item-title")}>Crypto Method</span>}>
                                <Tag color="blue">{paymentInfo.selectedCurrency}</Tag>
                            </Descriptions.Item>
                        )}

                        {paymentInfo?.serviceFee && (
                            <Descriptions.Item label={<span className={b("item-title")}>Service Fee</span>}>
                                {paymentInfo.serviceFee}
                            </Descriptions.Item>
                        )}

                        {paymentInfo?.networkFee && (
                            <Descriptions.Item label={<span className={b("item-title")}>Network Fee</span>}>
                                {paymentInfo.networkFee}
                            </Descriptions.Item>
                        )}

                        {paymentInfo?.transactionHash && (
                            <Descriptions.Item label={<span className={b("item-title")}>Tx Hash</span>}>
                                <HashWithCopy
                                    value={paymentInfo.transactionHash}
                                    onCopy={() =>
                                        copyToClipboard(paymentInfo!.transactionHash!, openNotificationFunc)
                                    }
                                />
                                {paymentInfo.explorerLink && (
                                    <Tooltip title="View on explorer">
                                        <a
                                            href={paymentInfo.explorerLink}
                                            target="_blank"
                                            rel="noopener noreferrer"
                                            style={{marginLeft: 8}}
                                        >
                                            <LinkOutlined /> Explorer
                                        </a>
                                    </Tooltip>
                                )}
                            </Descriptions.Item>
                        )}

                        {paymentInfo?.senderAddress && (
                            <Descriptions.Item label={<span className={b("item-title")}>Sender</span>}>
                                <HashWithCopy
                                    value={paymentInfo.senderAddress}
                                    onCopy={() =>
                                        copyToClipboard(paymentInfo!.senderAddress!, openNotificationFunc)
                                    }
                                />
                            </Descriptions.Item>
                        )}
                    </Descriptions>
                </>
            )}
        </>
    );
};

export default PaymentDescCard;
