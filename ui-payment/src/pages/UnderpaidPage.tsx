import * as React from "react";
import {useLocation, useNavigate} from "react-router-dom";
import Icon from "src/components/Icon";
import {Payment} from "src/types";
import renderCurrency from "src/utils/renderCurrency";

const UnderpaidPage: React.FC = () => {
    const location = useLocation();
    const navigate = useNavigate();
    const [payment, setPayment] = React.useState<Payment>();

    React.useEffect(() => {
        if (location.state?.payment) {
            setPayment(location.state.payment);
        } else {
            navigate("/not-found");
        }
    }, []);

    if (!payment || !payment.paymentInfo || !payment.paymentMethod) {
        return null;
    }

    const required = payment.paymentInfo.amountFormatted;
    const received = payment.paymentInfo.factAmountFormatted || "0";
    const ticker = payment.paymentMethod.displayName;
    const fiatPrice = renderCurrency(payment.currency, payment.price);

    return (
        <>
            <div className="mx-auto h-16 w-16 flex items-center justify-center mb-3.5 sm:mb-2">
                <div className="shrink-0 justify-self-center">
                    <Icon name="error" className="h-16 w-16" />
                </div>
            </div>

            <span className="block mx-auto text-2xl font-medium text-center mb-1 text-[#faad14]">
                Underpaid
            </span>

            <p className="block mx-auto text-sm text-center text-card-desc mb-6 max-w-[280px]">
                Your payment of <strong className="text-white">{fiatPrice}</strong> was not fully covered.
                The merchant has been notified and will review your payment.
            </p>

            <div className="bg-[#1a1a2e] border border-[#2a2a3e] rounded-xl p-4 mb-6 space-y-3">
                <div className="flex justify-between text-sm">
                    <span className="text-card-desc">Required</span>
                    <span className="text-white font-medium">{required} {ticker}</span>
                </div>
                <div className="flex justify-between text-sm">
                    <span className="text-card-desc">Received</span>
                    <span className="text-[#faad14] font-medium">{received} {ticker}</span>
                </div>
                <div className="border-t border-[#2a2a3e] pt-2 flex justify-between text-sm">
                    <span className="text-card-desc">Status</span>
                    <span className="text-[#faad14] font-medium">Awaiting merchant review</span>
                </div>
            </div>

            <p className="text-center text-xs text-card-desc opacity-70">
                The funds you sent are safe. The merchant will accept or request the remaining amount.
                If you have questions, please contact the merchant directly.
            </p>
        </>
    );
};

export default UnderpaidPage;
