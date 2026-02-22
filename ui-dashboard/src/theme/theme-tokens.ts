import {theme} from "antd";
import type {ThemeConfig} from "antd";

export const darkTokens: ThemeConfig = {
    algorithm: theme.darkAlgorithm,
    token: {
        // Brand
        colorPrimary: "#818cf8",
        colorSuccess: "#10b981",
        colorWarning: "#f59e0b",
        colorError: "#ef4444",
        colorInfo: "#818cf8",

        // Backgrounds
        colorBgContainer: "#13131a",
        colorBgElevated: "#1a1a2e",
        colorBgLayout: "#0a0a0f",
        colorBgSpotlight: "#1e1e30",

        // Text
        colorText: "rgba(255, 255, 255, 0.92)",
        colorTextSecondary: "rgba(255, 255, 255, 0.65)",
        colorTextTertiary: "rgba(255, 255, 255, 0.45)",
        colorTextQuaternary: "rgba(255, 255, 255, 0.25)",

        // Borders
        colorBorder: "#2a2a3e",
        colorBorderSecondary: "#1f1f33",
        colorSplit: "rgba(255, 255, 255, 0.06)",

        // Misc
        borderRadius: 8,
        wireframe: false,

        // Link
        colorLink: "#818cf8",
        colorLinkHover: "#a5b4fc",
        colorLinkActive: "#6366f1"
    },
    components: {
        Layout: {
            siderBg: "#0f0f1a",
            headerBg: "#0f0f1a",
            bodyBg: "#0a0a0f",
            triggerBg: "#1a1a2e"
        },
        Menu: {
            darkItemBg: "#0f0f1a",
            darkItemSelectedBg: "rgba(129, 140, 248, 0.15)",
            darkItemHoverBg: "rgba(129, 140, 248, 0.08)",
            darkItemColor: "rgba(255, 255, 255, 0.65)",
            darkItemSelectedColor: "#818cf8"
        },
        Table: {
            headerBg: "#13131a",
            headerColor: "rgba(255, 255, 255, 0.65)",
            rowHoverBg: "rgba(129, 140, 248, 0.06)",
            borderColor: "#2a2a3e"
        },
        Card: {
            colorBgContainer: "#13131a",
            colorBorderSecondary: "#2a2a3e"
        },
        Button: {
            primaryShadow: "0 2px 8px rgba(129, 140, 248, 0.35)"
        },
        Input: {
            colorBgContainer: "#1a1a2e",
            activeBorderColor: "#818cf8",
            hoverBorderColor: "#6366f1"
        },
        Select: {
            colorBgContainer: "#1a1a2e",
            colorBgElevated: "#1a1a2e"
        },
        Modal: {
            contentBg: "#13131a",
            headerBg: "#13131a"
        },
        Drawer: {
            colorBgElevated: "#13131a"
        },
        Notification: {
            colorBgElevated: "#1a1a2e"
        },
        Descriptions: {
            colorSplit: "#2a2a3e"
        },
        Progress: {
            remainingColor: "#2a2a3e"
        },
        Tag: {
            defaultBg: "rgba(129, 140, 248, 0.1)",
            defaultColor: "#818cf8"
        },
        Tabs: {
            inkBarColor: "#818cf8",
            itemSelectedColor: "#818cf8"
        },
        Statistic: {
            colorTextDescription: "rgba(255, 255, 255, 0.45)"
        }
    }
};

export const lightTokens: ThemeConfig = {
    algorithm: theme.defaultAlgorithm,
    token: {
        colorPrimary: "#6366f1",
        colorSuccess: "#10b981",
        colorWarning: "#f59e0b",
        colorError: "#ef4444",
        colorInfo: "#6366f1",

        colorBgContainer: "#ffffff",
        colorBgElevated: "#ffffff",
        colorBgLayout: "#f5f5f5",

        colorText: "rgba(0, 0, 0, 0.88)",
        colorTextSecondary: "rgba(0, 0, 0, 0.65)",
        colorTextTertiary: "rgba(0, 0, 0, 0.45)",

        colorBorder: "#d9d9d9",
        colorBorderSecondary: "#f0f0f0",

        borderRadius: 8,
        wireframe: false,

        colorLink: "#6366f1",
        colorLinkHover: "#818cf8",
        colorLinkActive: "#4f46e5"
    },
    components: {
        Layout: {
            siderBg: "#ffffff",
            headerBg: "#ffffff",
            bodyBg: "#f5f5f5"
        },
        Button: {
            primaryShadow: "0 2px 4px rgba(99, 102, 241, 0.2)"
        }
    }
};

// CSS custom properties for SCSS files
export const darkCSSVars: Record<string, string> = {
    "--cl-bg-deepest": "#0a0a0f",
    "--cl-bg-container": "#13131a",
    "--cl-bg-elevated": "#1a1a2e",
    "--cl-bg-spotlight": "#1e1e30",
    "--cl-bg-hover": "rgba(129, 140, 248, 0.06)",
    "--cl-text-primary": "rgba(255, 255, 255, 0.92)",
    "--cl-text-secondary": "rgba(255, 255, 255, 0.65)",
    "--cl-text-tertiary": "rgba(255, 255, 255, 0.45)",
    "--cl-border": "#2a2a3e",
    "--cl-border-secondary": "#1f1f33",
    "--cl-accent-primary": "#818cf8",
    "--cl-accent-secondary": "#10b981",
    "--cl-accent-success": "#49D1AC",
    "--cl-accent-warning": "#f59e0b",
    "--cl-accent-error": "#ef4444",
    "--cl-logo-bg": "#1a1a2e",
    "--cl-logo-text": "rgba(255, 255, 255, 0.92)",
    "--cl-mask-bg": "rgba(10, 10, 15, 0.7)",
    "--cl-code-bg": "#1a1a2e",
    "--cl-shadow-glow": "0 0 20px rgba(129, 140, 248, 0.1)",
    "--cl-card-border-glow": "0 0 0 1px rgba(129, 140, 248, 0.12)",
    "--cl-monospace": "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace"
};

export const lightCSSVars: Record<string, string> = {
    "--cl-bg-deepest": "#f5f5f5",
    "--cl-bg-container": "#ffffff",
    "--cl-bg-elevated": "#ffffff",
    "--cl-bg-spotlight": "#fafafa",
    "--cl-bg-hover": "rgba(99, 102, 241, 0.04)",
    "--cl-text-primary": "rgba(0, 0, 0, 0.88)",
    "--cl-text-secondary": "rgba(0, 0, 0, 0.65)",
    "--cl-text-tertiary": "rgba(0, 0, 0, 0.45)",
    "--cl-border": "#d9d9d9",
    "--cl-border-secondary": "#f0f0f0",
    "--cl-accent-primary": "#6366f1",
    "--cl-accent-secondary": "#10b981",
    "--cl-accent-success": "#49D1AC",
    "--cl-accent-warning": "#f59e0b",
    "--cl-accent-error": "#ef4444",
    "--cl-logo-bg": "#333333",
    "--cl-logo-text": "#333333",
    "--cl-mask-bg": "rgba(0, 0, 0, 0.05)",
    "--cl-code-bg": "#f5f5f5",
    "--cl-shadow-glow": "none",
    "--cl-card-border-glow": "none",
    "--cl-monospace": "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace"
};
