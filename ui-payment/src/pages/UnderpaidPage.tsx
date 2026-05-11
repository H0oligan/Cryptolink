import * as React from "react";
import {useLocation, useNavigate} from "react-router-dom";
import {QRCodeSVG} from "qrcode.react";
import Icon from "src/components/Icon";
import CopyButton from "src/components/CopyButton";
import {Payment} from "src/types";
import renderConvertedResult from "src/utils/renderConvertedResult";
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

    const ticker = payment.paymentMethod.displayName;
    const fiatPrice = renderCurrency(payment.currency, payment.price);

    // Backend now truncates these to MaxDisplayDecimals; renderConvertedResult
    // is a second safety net for older API responses still in flight.
    const requiredAmount = payment.paymentInfo.amountFormatted;
    const receivedAmount =
        payment.paymentInfo.receivedAmount || payment.paymentInfo.factAmountFormatted || "0";
    const remainingAmount = payment.paymentInfo.remainingAmount || "";
    const remainingLink =
        payment.paymentInfo.remainingPaymentLink || payment.paymentInfo.paymentLink;

    const requiredDisplay = renderConvertedResult(requiredAmount, ticker) || `${requiredAmount} ${ticker}`;
    const receivedDisplay = renderConvertedResult(receivedAmount, ticker) || `${receivedAmount} ${ticker}`;
    const remainingDisplay = remainingAmount
        ? renderConvertedResult(remainingAmount, ticker) || `${remainingAmount} ${ticker}`
        : "";

    const getCryptoIconName = (name: string) => {
        const lowered = name.toLowerCase();
        return lowered.includes("_") ? lowered.split("_")[1] : lowered;
    };

    return (
        <>
            <div className="mx-auto h-16 w-16 flex items-center justify-center mb-3.5 sm:mb-2">
                <div className="shrink-0 justify-self-center">
                    <Icon name="error" className="h-16 w-16" />
                </div>
            </div>

            <span className="block mx-auto text-2xl font-medium text-center mb-1 text-[#faad14]">
                Payment Window Closed
            </span>

            <p className="block mx-auto text-sm text-center text-card-desc mb-4 max-w-[300px]">
                Your invoice of <strong className="text-white">{fiatPrice}</strong> is short by{" "}
                {remainingDisplay && (
                    <strong className="text-[#faad14]">{remainingDisplay}</strong>
                )}
                . The original 24-hour payment window has expired.
            </p>

            <div className="bg-[#1a1a2e] border border-[#2a2a3e] rounded-xl p-4 mb-5 space-y-2">
                <div className="flex justify-between text-sm">
                    <span className="text-card-desc">Required</span>
                    <span className="text-white font-medium">{requiredDisplay}</span>
                </div>
                <div className="flex justify-between text-sm">
                    <span className="text-card-desc">Received</span>
                    <span className="text-[#10b981] font-medium">{receivedDisplay}</span>
                </div>
                {remainingDisplay && (
                    <div className="border-t border-[#2a2a3e] pt-2 flex justify-between text-sm">
                        <span className="text-card-desc">Remaining</span>
                        <span className="text-[#faad14] font-medium">{remainingDisplay}</span>
                    </div>
                )}
            </div>

            {remainingAmount && (
                <>
                    <div className="mx-auto mb-4 max-w-sm rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-center">
                        <p className="text-sm font-semibold text-amber-300">
                            Want to complete this invoice?
                        </p>
                        <p className="mt-1 text-xs text-amber-100/90">
                            Send the remaining{" "}
                            <strong>{remainingDisplay}</strong> to the address below. Because the
                            original window has closed, the merchant must <strong>manually approve</strong>{" "}
                            your top-up — contact them before sending to confirm the rate is still honored.
                        </p>
                    </div>

                    <div className="flex relative justify-center mb-7 sm:hidden">
                        <QRCodeSVG size={180} level={"H"} value={remainingLink} />
                        <Icon
                            name={getCryptoIconName(payment.paymentMethod.ticker)}
                            dir="crypto"
                            className="absolute p-1 w-12 h-12 bg-[#13131a] border border-[#2a2a3e] rounded-full left-1/2 -translate-y-1/2 top-1/2 -translate-x-1/2"
                        />
                    </div>
                    <span className="block mx-auto text-sm mb-5 font-medium text-center text-card-desc sm:hidden">
                        or
                    </span>

                    <div className="mx-auto h-16 w-16 flex items-center justify-center mb-3.5 lg:hidden">
                        <div className="shrink-0">
                            <Icon
                                name={getCryptoIconName(payment.paymentMethod.ticker)}
                                dir="crypto"
                                className="h-16 w-16"
                            />
                        </div>
                    </div>

                    <CopyButton
                        textToCopy={payment.paymentInfo.recipientAddress}
                        displayText={payment.paymentInfo.recipientAddress}
                    />
                    <CopyButton
                        textToCopy={remainingAmount}
                        displayText={remainingDisplay}
                    />
                </>
            )}

            <p className="text-center text-xs text-card-desc opacity-70 mt-4">
                Funds you have already sent are safe at the recipient address. If you need help,
                contact the merchant directly with this invoice URL.
            </p>
        </>
    );
};

export default UnderpaidPage;
