import "./header-actions.scss";

import React from "react";
import {Avatar, Dropdown, MenuProps} from "antd";
import {BulbOutlined, DownOutlined} from "@ant-design/icons";
import {RouteContext, RouteContextType} from "@ant-design/pro-components";
import ThemeToggle from "src/theme/theme-toggle";
import {useTheme} from "src/theme/theme-context";
import ConnectWalletButton from "src/wallet/connect-wallet-button";

interface AccountUser {
    email?: string;
    name?: string;
    profileImageUrl?: string;
}

// Account chip: avatar (initial when there is no photo) + email, opens the menu.
// The email is repeated as the menu header so it stays visible on phones.
const AccountChip: React.FC<{user?: AccountUser | null; menu: MenuProps["items"]; compact: boolean}> = ({user, menu, compact}) => {
    const {isDark, toggleTheme} = useTheme();
    const label = user?.email || user?.name || "";
    const items: MenuProps["items"] = [
        {key: "account", label: <span className="header-actions__menu-email">{label}</span>, disabled: true},
        {type: "divider"},
        // Phones hide the header theme toggle; keep it reachable here.
        ...(compact
            ? [{key: "theme", icon: <BulbOutlined />, label: isDark ? "Light mode" : "Dark mode", onClick: toggleTheme}]
            : []),
        ...(menu ?? [])
    ];

    return (
        <Dropdown menu={{items}} trigger={["click"]} placement="bottomRight" getPopupContainer={() => document.body}>
            <button type="button" className="header-actions__account" aria-label="Account menu">
                <Avatar size={28} src={user?.profileImageUrl || undefined} className="header-actions__avatar">
                    {label.charAt(0).toUpperCase()}
                </Avatar>
                {!compact && <span className="header-actions__email">{label}</span>}
                {!compact && <DownOutlined className="header-actions__caret" />}
            </button>
        </Dropdown>
    );
};

// Right side of the dashboard header, the dApp convention:
// [context switcher] [theme] | [account] [Connect Wallet]  ← wallet in the corner
const HeaderActions: React.FC<{
    user?: AccountUser | null;
    menu: MenuProps["items"];
    switcher?: React.ReactNode;
}> = ({user, menu, switcher}) => (
    <RouteContext.Consumer>
        {(routeCtx: RouteContextType) => {
            const compact = Boolean(routeCtx.isMobile);
            return (
                <div className="header-actions">
                    {switcher}
                    {!compact && <ThemeToggle />}
                    {!compact && <span className="header-actions__divider" />}
                    <AccountChip user={user} menu={menu} compact={compact} />
                    <ConnectWalletButton compact={compact} />
                </div>
            );
        }}
    </RouteContext.Consumer>
);

export default HeaderActions;
