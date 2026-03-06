import "./payments-page.scss";

import * as React from "react";
import {flatten} from "lodash-es";

import {PageContainer} from "@ant-design/pro-components";
import {Button, Result, Table, Typography, Row, notification, FormInstance, Tooltip, Tag} from "antd";
import {CheckOutlined, LinkOutlined, CopyOutlined} from "@ant-design/icons";
import {ColumnsType} from "antd/es/table";
import {useMount} from "react-use";
import {CURRENCY_SYMBOL, Payment, PaymentParams} from "src/types";
import PaymentForm from "src/components/payment-add-form/payment-add-form";
import CollapseString from "src/components/collapse-string/collapse-string";
import paymentsQueries from "src/queries/payments-queries";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import DrawerForm from "src/components/drawer-form/drawer-form";
import PaymentDescCard from "src/components/payment-desc-card/payment-desc-card";
import PaymentStatusLabel from "src/components/payment-status/payment-status";
import TimeLabel from "src/components/time-label/time-label";
import {sleep} from "src/utils";
import copyToClipboard from "src/utils/copy-to-clipboard";

const displayPrice = (record: Payment) => {
    let ticker = record.currency + " ";
    if (record.currency in CURRENCY_SYMBOL && CURRENCY_SYMBOL[record.currency] !== "") {
        ticker = CURRENCY_SYMBOL[record.currency];
    }

    return ticker + record.price;
};

const truncateHash = (hash: string) => {
    if (hash.length <= 18) return hash;
    return hash.slice(0, 10) + "..." + hash.slice(-8);
};

const columns: ColumnsType<Payment> = [
    {
        title: "Created At",
        dataIndex: "createdAt",
        key: "createdAt",
        render: (_, record) => <TimeLabel time={record.createdAt} />
    },
    {
        title: "Status",
        dataIndex: "curStatus",
        key: "curStatus",
        render: (_, record: Payment) => <PaymentStatusLabel status={record.status} />
    },
    {
        title: "Price",
        dataIndex: "price",
        key: "price",
        width: "min-content",
        render: (_, record) => <span style={{whiteSpace: "nowrap"}}>{displayPrice(record)}</span>
    },
    {
        title: "Crypto",
        dataIndex: "selectedCurrency",
        key: "selectedCurrency",
        render: (_, record) =>
            record.additionalInfo?.payment?.selectedCurrency ? (
                <Tag color="blue" style={{fontSize: 11}}>
                    {record.additionalInfo.payment.selectedCurrency}
                </Tag>
            ) : null
    },
    {
        title: "Tx Hash",
        dataIndex: "txHash",
        key: "txHash",
        render: (_, record) => {
            const hash = record.additionalInfo?.payment?.transactionHash;
            const link = record.additionalInfo?.payment?.explorerLink;
            if (!hash) return null;
            return (
                <span style={{whiteSpace: "nowrap", fontFamily: "monospace", fontSize: 12}}>
                    {truncateHash(hash)}
                    {link && (
                        <Tooltip title="View on explorer">
                            <a href={link} target="_blank" rel="noopener noreferrer" style={{marginLeft: 4}}>
                                <LinkOutlined />
                            </a>
                        </Tooltip>
                    )}
                </span>
            );
        }
    },
    {
        title: "Order ID",
        dataIndex: "orderId",
        key: "orderId",
        render: (_, record) => (
            <CollapseString text={!record.orderId ? "Not provided" : record.orderId} collapseAt={12} withPopover />
        )
    },
    {
        title: "Description",
        dataIndex: "description",
        key: "description",
        render: (_, record) =>
            record.description ? <CollapseString text={record.description} collapseAt={32} withPopover /> : null
    }
];

const PaymentsPage: React.FC = () => {
    const [api, contextHolder] = notification.useNotification();
    const listPayments = paymentsQueries.listPayments();
    const createPayment = paymentsQueries.createPayment();
    const [isFormOpen, setFormOpen] = React.useState<boolean>(false);
    const [openedCard, changeOpenedCard] = React.useState<Payment[]>([]);
    const [payments, setPayments] = React.useState<Payment[]>(
        flatten((listPayments.data?.pages || []).map((page) => page.results))
    );
    const [isFormSubmitting, setIsFormSubmitting] = React.useState<boolean>(false);
    const {merchantId} = useSharedMerchantId();

    const isLoading = listPayments.isLoading || createPayment.isLoading || listPayments.isFetching;

    useMount(async () => {
        if (!merchantId) {
            return;
        }

        await sleep(1000);
        listPayments.remove();

        await listPayments.refetch();
    });

    React.useEffect(() => {
        setPayments(flatten((listPayments.data?.pages || []).map((page) => page.results)));
    }, [listPayments.data]);

    React.useEffect(() => {
        if (merchantId) {
            listPayments.refetch();
        }
    }, [merchantId]);

    const openNotification = (title: string, description: string) => {
        api.info({
            message: title,
            description,
            placement: "bottomRight",
            icon: <CheckOutlined style={{color: "#49D1AC"}} />
        });
    };

    const uploadCreatedPayment = async (value: PaymentParams, form: FormInstance<PaymentParams>) => {
        try {
            setIsFormSubmitting(true);
            await createPayment.mutateAsync(value);
            setFormOpen(false);
            openNotification("Payment was created", "");

            await sleep(1000);
            form.resetFields();
        } catch (error) {
            console.error("error occurred: ", error);
        } finally {
            setIsFormSubmitting(false);
        }
    };

    const changeIsCardOpen = (value: boolean) => {
        if (!value) {
            changeOpenedCard([]);
        }
    };

    const exportCSV = () => {
        const headers = ["Date", "Status", "Price", "Currency", "Crypto", "TxHash", "Sender", "OrderID", "Description"];
        const rows = payments.map((p) => [
            p.createdAt,
            p.status,
            p.price,
            p.currency,
            p.additionalInfo?.payment?.selectedCurrency || "",
            p.additionalInfo?.payment?.transactionHash || "",
            p.additionalInfo?.payment?.senderAddress || "",
            p.orderId || "",
            p.description || ""
        ]);
        const csv = [headers, ...rows].map((r) => r.map((v) => `"${String(v).replace(/"/g, '""')}"`).join(",")).join("\n");
        const blob = new Blob([csv], {type: "text/csv"});
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `payments-${new Date().toISOString().slice(0, 10)}.csv`;
        a.click();
        URL.revokeObjectURL(url);
    };

    return (
        <PageContainer
            header={{
                title: "",
                breadcrumb: {}
            }}
        >
            {contextHolder}
            <Row align="middle" justify="space-between">
                <Typography.Title>Payments</Typography.Title>
                <Row gap={8} style={{gap: 8, marginTop: 20}}>
                    <Button onClick={exportCSV}>Export CSV</Button>
                    <Button type="primary" onClick={() => setFormOpen(true)}>
                        New Payment
                    </Button>
                </Row>
            </Row>
            <Table
                columns={columns}
                dataSource={payments}
                rowKey={(record) => record.id}
                rowClassName="payments-page__row"
                loading={isLoading}
                pagination={false}
                size="middle"
                scroll={{x: "max-content"}}
                footer={() => (
                    <Button
                        type="primary"
                        onClick={() => listPayments.fetchNextPage()}
                        disabled={!listPayments.hasNextPage}
                    >
                        Load more
                    </Button>
                )}
                locale={{
                    emptyText: (
                        <Result
                            icon={<></>}
                            title="Your orders will be here"
                            subTitle="To create an order, click to the button at the right top of the table"
                        ></Result>
                    )
                }}
                onRow={(record) => {
                    return {
                        onClick: () => {
                            changeOpenedCard([record]);
                        }
                    };
                }}
            />
            <DrawerForm
                title="Create payment"
                isFormOpen={isFormOpen}
                changeIsFormOpen={setFormOpen}
                formBody={
                    <PaymentForm
                        onCancel={() => {
                            setFormOpen(false);
                        }}
                        onFinish={uploadCreatedPayment}
                        isFormSubmitting={isFormSubmitting}
                    />
                }
            />
            <DrawerForm
                title="Payment details"
                isFormOpen={Boolean(openedCard.length)}
                changeIsFormOpen={changeIsCardOpen}
                formBody={<PaymentDescCard data={openedCard[0]} openNotificationFunc={openNotification} />}
                hasCloseBtn
                width={600}
            />
        </PageContainer>
    );
};

export default PaymentsPage;
