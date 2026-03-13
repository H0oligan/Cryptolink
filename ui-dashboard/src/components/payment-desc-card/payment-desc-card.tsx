import "./payment-desc-card.scss";

import * as React from "react";
import {Descriptions, Tag, Button, Tooltip, Space, Modal, Input, notification} from "antd";
import {CopyOutlined, LinkOutlined, CheckCircleOutlined, CloseCircleOutlined} from "@ant-design/icons";
import bevis from "src/utils/bevis";
import {Payment, CURRENCY_SYMBOL} from "src/types";
import PaymentStatusLabel from "src/components/payment-status/payment-status";
import SpinWithMask from "src/components/spin-with-mask/spin-with-mask";
import copyToClipboard from "src/utils/copy-to-clipboard";
import TimeLabel from "src/components/time-label/time-label";
import renderStrippedStr from "src/utils/render-stripped-str";
import merchantProvider from "src/providers/merchant-provider";
import useSharedMerchantId from "src/hooks/use-merchant-id";

interface Props {
    data?: Payment;
    openNotificationFunc: (title: string, description: string) => void;
    onResolved?: () => void;
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
            <CopyOutlined style={{cursor: "pointer", color: "#10b981"}} onClick={onCopy} />
        </Tooltip>
    </Space>
);

const PaymentDescCard: React.FC<Props> = ({data, openNotificationFunc, onResolved}) => {
    const {merchantId} = useSharedMerchantId();
    const [resolveOpen, setResolveOpen] = React.useState(false);
    const [resolveNotes, setResolveNotes] = React.useState("");
    const [resolveTxHash, setResolveTxHash] = React.useState("");
    const [resolving, setResolving] = React.useState(false);
    const [declining, setDeclining] = React.useState(false);
    const [declineOpen, setDeclineOpen] = React.useState(false);
    const [declineNotes, setDeclineNotes] = React.useState("");

    React.useEffect(() => {
        if (!data) {
            data = emptyState;
        }
    }, [data]);

    const paymentInfo = data?.additionalInfo?.payment;

    const handleResolve = async () => {
        if (!data || !merchantId) return;
        setResolving(true);
        try {
            await merchantProvider.resolvePayment(merchantId, data.id, {
                notes: resolveNotes || undefined,
                txHash: resolveTxHash || undefined,
            });
            notification.success({
                message: "Payment resolved",
                description: "Payment has been marked as successful. Webhook will be delivered.",
                placement: "bottomRight",
            });
            setResolveOpen(false);
            setResolveNotes("");
            setResolveTxHash("");
            onResolved?.();
        } catch (err: any) {
            notification.error({
                message: "Failed to resolve payment",
                description: err?.response?.data?.message || err?.message || "Unknown error",
                placement: "bottomRight",
            });
        } finally {
            setResolving(false);
        }
    };

    const handleDecline = async () => {
        if (!data || !merchantId) return;
        setDeclining(true);
        try {
            await merchantProvider.declinePayment(merchantId, data.id, {
                notes: declineNotes || undefined,
            });
            notification.success({
                message: "Payment declined",
                description: "Underpaid payment has been marked as failed.",
                placement: "bottomRight",
            });
            setDeclineOpen(false);
            setDeclineNotes("");
            onResolved?.();
        } catch (err: any) {
            notification.error({
                message: "Failed to decline payment",
                description: err?.response?.data?.message || err?.message || "Unknown error",
                placement: "bottomRight",
            });
        } finally {
            setDeclining(false);
        }
    };

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

                    {/* Underpaid: Accept or Decline */}
                    {data.status === "underpaid" && data.type === "payment" && (
                        <div style={{marginTop: 16, textAlign: "center"}}>
                            <Space size="middle">
                                <Button
                                    type="primary"
                                    icon={<CheckCircleOutlined />}
                                    onClick={() => setResolveOpen(true)}
                                >
                                    Accept Payment
                                </Button>
                                <Button
                                    danger
                                    icon={<CloseCircleOutlined />}
                                    onClick={() => setDeclineOpen(true)}
                                >
                                    Decline Payment
                                </Button>
                            </Space>
                            <div style={{marginTop: 8, fontSize: 12, opacity: 0.7}}>
                                Customer sent less than the required amount. Funds are already in your wallet.
                            </div>
                        </div>
                    )}

                    {/* Failed: Resolve */}
                    {data.status === "failed" && data.type === "payment" && (
                        <div style={{marginTop: 16, textAlign: "center"}}>
                            <Button
                                type="primary"
                                icon={<CheckCircleOutlined />}
                                onClick={() => setResolveOpen(true)}
                            >
                                Resolve Payment
                            </Button>
                        </div>
                    )}

                    {/* Resolve / Accept modal */}
                    <Modal
                        title={data.status === "underpaid" ? "Accept Underpaid Payment" : "Resolve Payment"}
                        open={resolveOpen}
                        destroyOnClose
                        onCancel={() => setResolveOpen(false)}
                        onOk={handleResolve}
                        okText={data.status === "underpaid" ? "Accept Payment" : "Confirm Resolution"}
                        confirmLoading={resolving}
                    >
                        <p style={{marginBottom: 16}}>
                            {data.status === "underpaid"
                                ? <>This will mark the underpaid payment as <strong>successful</strong>. The partial amount is already in your wallet.</>
                                : <>This will mark the payment as <strong>successful</strong> and trigger the webhook notification. Use this when you have manually verified the customer's payment.</>
                            }
                        </p>
                        <div style={{marginBottom: 12}}>
                            <label style={{display: "block", marginBottom: 4, fontWeight: 500}}>
                                Transaction Hash (optional)
                            </label>
                            <Input
                                placeholder="0x..."
                                value={resolveTxHash}
                                onChange={(e) => setResolveTxHash(e.target.value)}
                            />
                        </div>
                        <div>
                            <label style={{display: "block", marginBottom: 4, fontWeight: 500}}>
                                Notes (optional)
                            </label>
                            <Input.TextArea
                                placeholder="e.g. Customer confirmed partial payment is acceptable"
                                value={resolveNotes}
                                onChange={(e) => setResolveNotes(e.target.value)}
                                rows={3}
                            />
                        </div>
                    </Modal>

                    {/* Decline modal */}
                    <Modal
                        title="Decline Underpaid Payment"
                        open={declineOpen}
                        destroyOnClose
                        onCancel={() => setDeclineOpen(false)}
                        onOk={handleDecline}
                        okText="Decline Payment"
                        okButtonProps={{danger: true}}
                        confirmLoading={declining}
                    >
                        <p style={{marginBottom: 16}}>
                            This will mark the payment as <strong>failed</strong>. The partial amount
                            already received remains in your wallet — the system does not handle refunds
                            for underpayments.
                        </p>
                        <div>
                            <label style={{display: "block", marginBottom: 4, fontWeight: 500}}>
                                Notes (optional)
                            </label>
                            <Input.TextArea
                                placeholder="e.g. Customer will be refunded manually"
                                value={declineNotes}
                                onChange={(e) => setDeclineNotes(e.target.value)}
                                rows={3}
                            />
                        </div>
                    </Modal>
                </>
            )}
        </>
    );
};

export default PaymentDescCard;
