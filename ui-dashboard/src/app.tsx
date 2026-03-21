import "./app.scss";

import * as React from "react";
import {AxiosError} from "axios";
import {Routes, Route, useLocation, useNavigate} from "react-router-dom";
import {useMount} from "react-use";
import {ProLayout, RouteContext, RouteContextType} from "@ant-design/pro-components";
import {
    EditOutlined, LogoutOutlined, LinkOutlined, CheckOutlined, UserOutlined,
    DashboardOutlined, CreditCardOutlined, WalletOutlined,
    TeamOutlined, CrownOutlined, SettingOutlined, BookOutlined, ShopOutlined,
    WarningOutlined, DollarOutlined
} from "@ant-design/icons";
import {Select, Divider, Button, Avatar, Space, Dropdown, MenuProps, notification, FormInstance, Alert} from "antd";
import {usePostHog} from "posthog-js/react";
import bevis from "src/utils/bevis";
const logoImg = "/logo.svg";
import SettingsPage from "src/pages/settings-page/settings-page";
import {SupportMessage, User} from "src/types";
import authProvider from "src/providers/auth-provider";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import useSharedMerchants from "src/hooks/use-merchants";
import useSharedMerchant from "src/hooks/use-merchant";
import ManageMerchantsPage from "src/pages/manage-merchants-page/manage-merchants-page";
import BalancePage from "src/pages/balance-page/balance-page";
import LoginPage from "src/pages/login-page/login-page";
import PaymentsPage from "src/pages/payments-page/payments-page";
import DrawerForm from "src/components/drawer-form/drawer-form";
import SupportForm from "src/components/support-form/support-form";
import merchantProvider from "src/providers/merchant-provider";
import CustomersPage from "src/pages/customers-page/customers-page";
import {sleep} from "src/utils";
import PaymentLinksPage from "src/pages/payment-links-page/payments-links-page";
import WalletSetupPage from "src/pages/wallet-setup-page/wallet-setup-page";
import ProfilePage from "src/pages/profile-page/profile-page";
import SubscriptionPage from "src/pages/subscription-page/subscription-page";
import CurrenciesPage from "src/pages/currencies-page/currencies-page";
import useSharedPosthogStatus from "src/hooks/use-posthog-status";
import {toggled} from "./providers/toggles";
import ThemeToggle from "src/theme/theme-toggle";
import {useTheme} from "src/theme/theme-context";

interface MenuItem {
    path: string;
    name: string;
    icon?: React.ReactNode;
    onClick?: () => void;
}

const defaultMenus: MenuItem[] = [
    {path: "/payments", name: "Payments", icon: <CreditCardOutlined />},
    {path: "/payment-links", name: "Payment Links", icon: <LinkOutlined />},
    {path: "/balance", name: "Balance", icon: <WalletOutlined />},
    {path: "/customers", name: "Customers", icon: <TeamOutlined />},
    {path: "/subscription", name: "Subscription", icon: <CrownOutlined />},
    {path: "/currencies", name: "Currencies & Fees", icon: <DollarOutlined />},
    {path: "/wallet-setup", name: "Wallet Setup", icon: <WalletOutlined />},
    {path: "/settings", name: "Settings", icon: <SettingOutlined />},
    {path: "https://cryptolink.cc/doc/", name: "Documentation", icon: <BookOutlined />}
];


if (toggled("feedback")) {
    defaultMenus.push({
        path: "/support",
        name: "Support / Feedback"
    });
}

const manageMerchantsMenus: MenuItem[] = [
    {
        path: "/manage-merchants",
        name: "Manage Merchants",
        icon: <ShopOutlined />
    }
];

const menus = defaultMenus.concat(manageMerchantsMenus);

const b = bevis("app");

interface AppLoadState {
    realoadUserInfo: boolean;
}

const App: React.FC = () => {
    const state: AppLoadState = useLocation().state;
    const posthog = usePostHog();
    const location = useLocation();
    const navigate = useNavigate();
    const [notificationApi, notificationElement] = notification.useNotification();
    const {isDark} = useTheme();

    const {merchants, getMerchants} = useSharedMerchants();
    const {getMerchant} = useSharedMerchant();
    const {merchantId, setMerchantId} = useSharedMerchantId();
    const {isPosthogActive} = useSharedPosthogStatus();
    const [user, setUser] = React.useState<User>();
    const [isSupportFormOpen, setIsSupportFormOpen] = React.useState<boolean>(false);
    const [isFormSubmitting, setIsFormSubmitting] = React.useState<boolean>(false);
    const [isLoading, setIsLoading] = React.useState<boolean>(true);
    const [resendingVerification, setResendingVerification] = React.useState(false);

    const handleResendVerification = async () => {
        try {
            setResendingVerification(true);
            await authProvider.resendVerification();
            notificationApi.success({
                message: "Verification email sent",
                description: "Please check your inbox",
                placement: "bottomRight",
                icon: <CheckOutlined style={{color: "#10b981"}} />
            });
        } catch (e) {
            notificationApi.error({
                message: "Failed to send verification email",
                description: "Please try again later",
                placement: "bottomRight"
            });
        } finally {
            setResendingVerification(false);
        }
    };

    const loadUserInfo = async () => {
        let newMerchantId = merchantId;
        let user: User;

        const getCookie = async () => {
            try {
                await authProvider.getCookie();
            } catch (e) {
                if (e instanceof AxiosError && e.response?.status === 401) {
                    navigate("/login", {
                        state: {
                            isNeedLogout: true
                        }
                    });
                }
            }
        };

        const getMe = async () => {
            try {
                user = await authProvider.getMe();
                setUser(user);
            } catch (e) {
                if (e instanceof AxiosError && e.response?.status === 401) {
                    navigate("/login", {
                        state: {
                            isNeedLogout: true
                        }
                    });
                }
            }
        };

        const listMerchants = async () => {
            if (!user) {
                return;
            }

            const merchants = await getMerchants();

            // Reset merchantId if it doesn't belong to the current user's merchants
            // (prevents cross-user leakage via stale localStorage)
            const storedIdValid = merchantId && merchants.some((m) => m.id === merchantId);
            if (!storedIdValid) {
                newMerchantId = merchants[0]?.id;
                setMerchantId(newMerchantId);
            }
            return merchants;
        };

        const listMerchant = async () => {
            if (user && newMerchantId) {
                await getMerchant(newMerchantId);
            }
        };

        await getCookie();
        await getMe();
        await listMerchants();
        await listMerchant();
        setIsLoading(false);
    };

    useMount(async () => {
        await loadUserInfo();
    });

    React.useEffect(() => {
        if (state?.realoadUserInfo) {
            loadUserInfo();
        }
    }, [state]);

    React.useEffect(() => {
        if (user && isPosthogActive) {
            posthog?.identify(user.email, {
                email: user.email,
                uuid: user.uuid
            });
        }
    }, [posthog, user]);

    const isManageMerchantsActive = location.pathname === "/manage-merchants";

    React.useEffect(() => {
        if (!isManageMerchantsActive && merchants && !merchants?.length && location.pathname !== "/login") {
            navigate("/manage-merchants");
            return;
        }

        if (location.pathname === "/") {
            navigate("/payments");
            return;
        }

        if (location.pathname === "/login") {
            document.title = "Login | Dashboard | CryptoLink";
            return;
        }

        const pageName = menus.find((item) => item.path === location.pathname)?.name;
        document.title = `${pageName} | Dashboard | CryptoLink`;
    }, [location, merchants]);

    const logout = async () => {
        if (isPosthogActive) {
            posthog?.reset(true);
        }

        // Clear stale merchant selection to prevent cross-user leakage
        setMerchantId(null);

        await authProvider.logout();
        navigate("/login", {
            state: {
                isNeedLogout: true
            }
        });
    };

    const userMenu: MenuProps["items"] = [
        {
            label: (
                <Space align="center" className={b("user-container")} onClick={() => navigate("/profile")}>
                    <span className={b("user-text")}>Profile</span>
                    <UserOutlined className={b("user-avatar")} />
                </Space>
            ),
            key: "profile"
        },
        ...(user?.isSuperAdmin ? [{
            label: (
                <Space align="center" className={b("user-container")} onClick={() => { window.location.href = "/admin/dashboard"; }}>
                    <span className={b("user-text")}>Admin Panel</span>
                    <DashboardOutlined className={b("user-avatar")} />
                </Space>
            ),
            key: "admin"
        }] : []),
        {type: "divider" as const},
        {
            label: (
                <Space align="center" className={b("user-container")} onClick={logout}>
                    <span className={b("user-text")}>Log out</span>
                    <LogoutOutlined className={b("user-avatar")} />
                </Space>
            ),
            key: "0"
        }
    ];

    const sendSupportForm = async (value: SupportMessage, form: FormInstance<SupportMessage>) => {
        try {
            setIsFormSubmitting(true);
            await merchantProvider.sendSupportMessage(merchantId!, value);
            setIsSupportFormOpen(false);
            notificationApi.info({
                message: "You have submitted a form",
                description: `Thank you for your ${value.topic.toLowerCase()}`,
                placement: "bottomRight",
                icon: <CheckOutlined style={{color: "#10b981"}} />
            });

            await sleep(1000);
            form.resetFields();
        } catch (error) {
            console.error("error occurred: ", error);
        } finally {
            setIsFormSubmitting(false);
        }
    };

    const loadingMerchantSwitcherMenu = [{label: "Loading...", value: "loading..."}];

    return (
        <Routes>
            <Route path="login" element={<LoginPage />} />
            <Route
                path="*"
                element={
                    <>
                        <ProLayout
                            className={b("layout")}
                            fixSiderbar
                            location={{
                                pathname: location.pathname
                            }}
                            breakpoint="xl"
                            route={{
                                routes: isManageMerchantsActive ? manageMerchantsMenus : defaultMenus
                            }}
                            logo={
                                <RouteContext.Consumer>
                                    {(routeCtx: RouteContextType) => (
                                        <div className={b("logo")}>
                                            <img src={logoImg} alt="logo" className={b("logo-img")} />
                                            {routeCtx.isMobile ? null : (
                                                <span className={b("logo-text")}>CryptoLink</span>
                                            )}
                                        </div>
                                    )}
                                </RouteContext.Consumer>
                            }
                            loading={isLoading}
                            navTheme={isDark ? "realDark" : "light"}
                            actionsRender={() => {
                                return [
                                    <ThemeToggle key="theme-toggle" />,
                                    !isManageMerchantsActive ? (
                                        <Select
                                            key="merchant-select"
                                            className={b("select")}
                                            getPopupContainer={() => document.body}
                                            dropdownRender={(menu) => (
                                                <>
                                                    {menu}
                                                    <Divider className={b("select-divider")} />
                                                    <Button
                                                        type="text"
                                                        icon={<EditOutlined />}
                                                        onClick={() => {
                                                            navigate("/manage-merchants");
                                                        }}
                                                        className={b("select-btn")}
                                                    >
                                                        Manage merchants
                                                    </Button>
                                                </>
                                            )}
                                            value={merchants ? merchantId : loadingMerchantSwitcherMenu[0].value}
                                            options={
                                                merchants
                                                    ? merchants.map((merchant) => ({
                                                          label: merchant.name,
                                                          value: merchant.id
                                                      }))
                                                    : loadingMerchantSwitcherMenu
                                            }
                                            onChange={async (value) => {
                                                if (value === "loading...") {
                                                    return;
                                                }

                                                setMerchantId(value);
                                                await getMerchant(value);
                                            }}
                                        />
                                    ) : null,
                                    <Dropdown key="user-dropdown" menu={{items: userMenu}} trigger={["click"]} className={b("user-wrap")} getPopupContainer={() => document.body}>
                                        <Space
                                            className={b("user-container", {
                                                "user-container_selected": true
                                            })}
                                        >
                                            <RouteContext.Consumer>
                                                {(routeCtx: RouteContextType) => (
                                                    <Space align="center">
                                                        <Avatar
                                                            src={user?.profileImageUrl}
                                                            size={"default"}
                                                            className={b("user-avatar")}
                                                        />
                                                        {routeCtx.isMobile ? null : (
                                                            <div className={b("user-text")}>{user?.email || user?.name}</div>
                                                        )}
                                                    </Space>
                                                )}
                                            </RouteContext.Consumer>
                                        </Space>
                                    </Dropdown>
                                ];
                            }}
                            menuItemRender={(item: MenuItem, dom: React.ReactNode) => {
                                const isLink = item.path?.includes("http", 0);

                                return (
                                    <RouteContext.Consumer>
                                        {(routeCtx: RouteContextType) => (
                                            <div onClick={routeCtx.isMobile ? item.onClick : undefined}>
                                                {isLink && (
                                                    <a href={item.path} target="_blank">
                                                        {item.name} <LinkOutlined />
                                                    </a>
                                                )}

                                                {item.path === "/support" && (
                                                    <div onClick={() => setIsSupportFormOpen(true)}>{dom}</div>
                                                )}

                                                {!isLink && item.path !== "/support" && (
                                                    <div
                                                        onClick={() => {
                                                            navigate(item.path || "/");
                                                        }}
                                                    >
                                                        {dom}
                                                    </div>
                                                )}
                                            </div>
                                        )}
                                    </RouteContext.Consumer>
                                );
                            }}
                            title={false}
                            defaultCollapsed
                            collapsedButtonRender={false}
                            layout="mix"
                            splitMenus={false}
                            headerTitleRender={() => null}
                        >
                            {notificationElement}
                            {user && user.emailVerified === false && (
                                <Alert
                                    type="warning"
                                    showIcon
                                    icon={<WarningOutlined />}
                                    style={{marginBottom: 16}}
                                    message={
                                        <span>
                                            Please verify your email address. Check your inbox or{" "}
                                            <Button
                                                type="link"
                                                size="small"
                                                loading={resendingVerification}
                                                onClick={handleResendVerification}
                                                style={{padding: 0, height: "auto"}}
                                            >
                                                Resend verification email
                                            </Button>
                                        </span>
                                    }
                                />
                            )}
                            <Routes>
                                <Route path="settings" element={<SettingsPage />} />
                                <Route path="payments" element={<PaymentsPage />} />
                                <Route path="payment-links" element={<PaymentLinksPage />} />
                                <Route path="manage-merchants" element={<ManageMerchantsPage />} />
                                <Route path="balance" element={<BalancePage />} />
                                <Route path="customers" element={<CustomersPage />} />
                                <Route path="wallet-setup" element={<WalletSetupPage />} />
                                <Route path="subscription" element={<SubscriptionPage />} />
                                <Route path="currencies" element={<CurrenciesPage />} />
                                <Route path="profile" element={<ProfilePage />} />
                                <Route path="*" element={"not found"} />
                            </Routes>
                        </ProLayout>
                        <DrawerForm
                            title="Contact us"
                            isFormOpen={isSupportFormOpen}
                            changeIsFormOpen={setIsSupportFormOpen}
                            formBody={
                                <SupportForm
                                    onCancel={() => setIsSupportFormOpen(false)}
                                    onFinish={sendSupportForm}
                                    isFormSubmitting={isFormSubmitting}
                                />
                            }
                            width={400}
                        />
                    </>
                }
            />
        </Routes>
    );
};

export default App;
