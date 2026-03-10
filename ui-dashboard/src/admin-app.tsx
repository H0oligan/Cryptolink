import "./app.scss";

import * as React from "react";
import {AxiosError} from "axios";
import {Routes, Route, useLocation, useNavigate, Navigate} from "react-router-dom";
import {useMount} from "react-use";
import {ProLayout, RouteContext, RouteContextType} from "@ant-design/pro-components";
import {
    LogoutOutlined, DashboardOutlined, ArrowLeftOutlined, UserOutlined,
    ShopOutlined, CrownOutlined, MailOutlined, AlertOutlined, ThunderboltOutlined
} from "@ant-design/icons";
import {Avatar, Space, Dropdown, MenuProps} from "antd";
import bevis from "src/utils/bevis";
const logoImg = "/logo.svg";
import {User} from "src/types";
import authProvider from "src/providers/auth-provider";
import LoginPage from "src/pages/login-page/login-page";
import AdminDashboardPage from "src/pages/admin/dashboard-page/dashboard-page";
import AdminPlansPage from "src/pages/admin/plans-page/plans-page";
import AdminMerchantsPage from "src/pages/admin/merchants-page/merchants-page";
import AdminUsersPage from "src/pages/admin/users-page/users-page";
import AdminEmailPage from "src/pages/admin/email-page/email-page";
import AdminPaymentsPage from "src/pages/admin/payments-page/payments-page";
import AdminContractsPage from "src/pages/admin/contracts-page/contracts-page";
import ThemeToggle from "src/theme/theme-toggle";
import {useTheme} from "src/theme/theme-context";

interface MenuItem {
    path: string;
    name: string;
    icon?: React.ReactNode;
}

// Routes are relative to /admin basename — /dashboard resolves to /admin/dashboard
const adminMenus: MenuItem[] = [
    {path: "/dashboard", name: "Admin Dashboard", icon: <DashboardOutlined />},
    {path: "/merchants", name: "Merchants", icon: <ShopOutlined />},
    {path: "/users", name: "Users", icon: <UserOutlined />},
    {path: "/plans", name: "Plans", icon: <CrownOutlined />},
    {path: "/email", name: "Email", icon: <MailOutlined />},
    {path: "/payments", name: "Payments Support", icon: <AlertOutlined />},
    {path: "/contracts", name: "Contracts", icon: <ThunderboltOutlined />},
];

const b = bevis("app");

interface AppLoadState {
    realoadUserInfo?: boolean;
}

const AdminApp: React.FC = () => {
    const locationState: AppLoadState = useLocation().state;
    const location = useLocation();
    const navigate = useNavigate();
    const {isDark} = useTheme();

    const [user, setUser] = React.useState<User | undefined>();
    const [isLoading, setIsLoading] = React.useState<boolean>(true);

    const loadUserInfo = async () => {
        try {
            await authProvider.getCookie();
        } catch {
            // getCookie failure is non-fatal
        }

        try {
            const u = await authProvider.getMe();
            if (!u.isSuperAdmin) {
                // Regular merchant — redirect to merchant panel
                window.location.href = "/merchants/payments";
                return;
            }
            setUser(u);
        } catch (e) {
            // Not authenticated — user stays undefined, routes will redirect to login
            if (!(e instanceof AxiosError && e.response?.status === 401)) {
                console.error("Unexpected auth error", e);
            }
        }

        setIsLoading(false);
    };

    // Initial load on mount — always run regardless of current path
    useMount(async () => {
        await loadUserInfo();
    });

    // Re-run after LoginPage navigates to "/" with {realoadUserInfo: true}
    React.useEffect(() => {
        if (locationState?.realoadUserInfo) {
            setUser(undefined);
            setIsLoading(true);
            loadUserInfo();
        }
    }, [locationState]);

    React.useEffect(() => {
        const pageName = adminMenus.find((item) => item.path === location.pathname)?.name;
        document.title = pageName ? `${pageName} | Admin | CryptoLink` : "Admin | CryptoLink";
    }, [location]);

    const logout = async () => {
        await authProvider.logout();
        setUser(undefined);
        navigate("/login", {state: {isNeedLogout: true}});
    };

    const userMenu: MenuProps["items"] = [
        {
            label: (
                <Space align="center" className={b("user-container")}
                    onClick={() => { window.location.href = "/merchants/payments"; }}>
                    <span className={b("user-text")}>Merchant Panel</span>
                    <ArrowLeftOutlined className={b("user-avatar")} />
                </Space>
            ),
            key: "merchant"
        },
        {type: "divider" as const},
        {
            label: (
                <Space align="center" className={b("user-container")} onClick={logout}>
                    <span className={b("user-text")}>Log out</span>
                    <LogoutOutlined className={b("user-avatar")} />
                </Space>
            ),
            key: "logout"
        }
    ];

    return (
        <Routes>
            {/* Login route: redirect to dashboard if already authenticated */}
            <Route
                path="login"
                element={user && !isLoading ? <Navigate to="/dashboard" replace /> : <LoginPage />}
            />

            <Route
                path="*"
                element={
                    <ProLayout
                        className={b("layout")}
                        fixSiderbar
                        location={{pathname: location.pathname}}
                        breakpoint="xl"
                        route={{routes: adminMenus}}
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
                        actionsRender={() => [
                            <ThemeToggle key="theme-toggle" />,
                            <Dropdown key="user-dropdown" menu={{items: userMenu}} trigger={["click"]}
                                className={b("user-wrap")} getPopupContainer={() => document.body}>
                                <Space className={b("user-container", {"user-container_selected": true})}>
                                    <RouteContext.Consumer>
                                        {(routeCtx: RouteContextType) => (
                                            <Space align="center">
                                                <Avatar src={user?.profileImageUrl} size="default"
                                                    className={b("user-avatar")} />
                                                {routeCtx.isMobile ? null : (
                                                    <div className={b("user-text")}>{user?.email || user?.name}</div>
                                                )}
                                            </Space>
                                        )}
                                    </RouteContext.Consumer>
                                </Space>
                            </Dropdown>
                        ]}
                        menuItemRender={(item: MenuItem, dom: React.ReactNode) => (
                            <RouteContext.Consumer>
                                {(routeCtx: RouteContextType) => (
                                    <div onClick={routeCtx.isMobile ? undefined : () => navigate(item.path || "/")}>
                                        {dom}
                                    </div>
                                )}
                            </RouteContext.Consumer>
                        )}
                        title={false}
                        defaultCollapsed
                        collapsedButtonRender={false}
                        layout="mix"
                        splitMenus={false}
                        headerTitleRender={() => null}
                    >
                        {/* Don't render inner routes while loading — prevents premature redirects */}
                        {!isLoading && (
                            <Routes>
                                {/* Unauthenticated: redirect all to login */}
                                {!user && <Route path="*" element={<Navigate to="/login" replace />} />}

                                {/* Authenticated super admin routes */}
                                {user && (
                                    <>
                                        <Route path="/" element={<Navigate to="/dashboard" replace />} />
                                        <Route path="dashboard" element={<AdminDashboardPage />} />
                                        <Route path="merchants" element={<AdminMerchantsPage />} />
                                        <Route path="users" element={<AdminUsersPage />} />
                                        <Route path="plans" element={<AdminPlansPage />} />
                                        <Route path="email" element={<AdminEmailPage />} />
                                        <Route path="payments" element={<AdminPaymentsPage />} />
                                        <Route path="contracts" element={<AdminContractsPage />} />
                                        <Route path="*" element="not found" />
                                    </>
                                )}
                            </Routes>
                        )}
                    </ProLayout>
                }
            />
        </Routes>
    );
};

export default AdminApp;
