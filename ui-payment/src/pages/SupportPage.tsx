import * as React from "react";
import {useLocation, useNavigate} from "react-router-dom";
import Icon from "src/components/Icon";

const SupportPage: React.FC = () => {
    const location = useLocation();
    const navigate = useNavigate();
    const paymentId = location.state?.paymentId || "";
    const supportEmail = import.meta.env.VITE_SUPPORT_EMAIL || "support@cryptolink.cc";

    const [name, setName] = React.useState("");
    const [email, setEmail] = React.useState("");
    const [message, setMessage] = React.useState(
        paymentId ? `Payment ID: ${paymentId}\n\nDescribe your issue:\n` : ""
    );
    const [submitted, setSubmitted] = React.useState(false);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        const subject = paymentId ? `Support: Payment ${paymentId}` : "Support Request";
        const body = `Name: ${name}\nEmail: ${email}\n\n${message}`;
        window.location.href = `mailto:${supportEmail}?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(body)}`;
        setSubmitted(true);
    };

    return (
        <div className="flex flex-col">
            <div className="mx-auto h-16 w-16 flex items-center justify-center mb-3 sm:mb-2">
                <div className="shrink-0">
                    <Icon name="store" className="h-16 w-16" />
                </div>
            </div>
            <span className="block mx-auto text-2xl font-medium text-center mb-6">
                Contact Support
            </span>

            {!submitted ? (
                <>
                    <p className="text-sm text-card-desc text-center mb-6">
                        Need help with a payment? Fill out the form below or email us directly at{" "}
                        <a href={`mailto:${supportEmail}`} className="text-main-green-1 underline">
                            {supportEmail}
                        </a>
                    </p>

                    {paymentId && (
                        <div className="bg-[#1a1a2e] rounded-lg p-3 mb-4">
                            <span className="text-xs text-card-desc">Payment ID</span>
                            <p className="text-sm font-medium break-all">{paymentId}</p>
                        </div>
                    )}

                    <form onSubmit={handleSubmit} className="flex flex-col gap-3">
                        <div>
                            <label className="block text-sm font-medium mb-1">Name</label>
                            <input
                                type="text"
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                required
                                className="w-full border border-[#2a2a3e] bg-[#1a1a2e] text-white rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-main-green-1"
                                placeholder="Your name"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Email</label>
                            <input
                                type="email"
                                value={email}
                                onChange={(e) => setEmail(e.target.value)}
                                required
                                className="w-full border border-[#2a2a3e] bg-[#1a1a2e] text-white rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-main-green-1"
                                placeholder="your@email.com"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Message</label>
                            <textarea
                                value={message}
                                onChange={(e) => setMessage(e.target.value)}
                                required
                                rows={4}
                                className="w-full border border-[#2a2a3e] bg-[#1a1a2e] text-white rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-main-green-1 resize-none"
                                placeholder="Describe your issue..."
                            />
                        </div>
                        <button
                            type="submit"
                            className="relative border rounded-3xl bg-main-green-1 border-main-green-1 w-full h-14 font-medium text-xl text-white flex items-center justify-center mt-2"
                        >
                            Send Message
                            <Icon
                                name="arrow_right_white"
                                className="h-5 w-5 absolute right-10 xs:right-4 md:right-14"
                            />
                        </button>
                    </form>
                </>
            ) : (
                <div className="text-center">
                    <div className="mx-auto h-20 w-20 flex items-center justify-center mb-4">
                        <div className="shrink-0">
                            <Icon name="ok" className="h-20 w-20" />
                        </div>
                    </div>
                    <p className="text-lg font-medium mb-2">Email client opened!</p>
                    <p className="text-sm text-card-desc mb-6">
                        If your email client didn't open, you can email us directly at{" "}
                        <a href={`mailto:${supportEmail}`} className="text-main-green-1 underline">
                            {supportEmail}
                        </a>
                    </p>
                    <button
                        onClick={() => setSubmitted(false)}
                        className="text-main-green-1 underline text-sm"
                    >
                        Try again
                    </button>
                </div>
            )}
        </div>
    );
};

export default SupportPage;
