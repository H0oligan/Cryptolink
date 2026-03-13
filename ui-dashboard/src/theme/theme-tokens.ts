import {theme} from "antd";
import type {ThemeConfig} from "antd";

export const darkTokens: ThemeConfig = {
    algorithm: theme.darkAlgorithm,
    token: {
        // Brand — Matrix Neon (emerald on black)
        colorPrimary: "#10b981",
        colorSuccess: "#10b981",
        colorWarning: "#f59e0b",
        colorError: "#ef4444",
        colorInfo: "#10b981",

        // Backgrounds — near-black
        colorBgContainer: "#0a0a0a",
        colorBgElevated: "#111111",
        colorBgLayout: "#050505",
        colorBgSpotlight: "#141414",

        // Text — cool white with slight green tint
        colorText: "#e2e8f0",
        colorTextSecondary: "#94a3b8",
        colorTextTertiary: "#64748b",
        colorTextQuaternary: "#475569",

        // Borders — subtle dark
        colorBorder: "#1e1e1e",
        colorBorderSecondary: "#151515",
        colorSplit: "rgba(255, 255, 255, 0.04)",

        // Misc
        borderRadius: 8,
        wireframe: false,

        // Link — emerald
        colorLink: "#10b981",
        colorLinkHover: "#34d399",
        colorLinkActive: "#059669"
    },
    components: {
        Layout: {
            siderBg: "#070707",
            headerBg: "#070707",
            bodyBg: "#050505",
            triggerBg: "#111111"
        },
        Menu: {
            darkItemBg: "#070707",
            darkItemSelectedBg: "rgba(16, 185, 129, 0.12)",
            darkItemHoverBg: "rgba(16, 185, 129, 0.06)",
            darkItemColor: "#94a3b8",
            darkItemSelectedColor: "#10b981"
        },
        Table: {
            headerBg: "#0a0a0a",
            headerColor: "#94a3b8",
            rowHoverBg: "rgba(16, 185, 129, 0.04)",
            borderColor: "#1e1e1e"
        },
        Card: {
            colorBgContainer: "#0a0a0a",
            colorBorderSecondary: "#1e1e1e"
        },
        Button: {
            primaryShadow: "0 0 12px rgba(16, 185, 129, 0.4)"
        },
        Input: {
            colorBgContainer: "#0e0e0e",
            activeBorderColor: "#10b981",
            hoverBorderColor: "#059669"
        },
        Select: {
            colorBgContainer: "#0e0e0e",
            colorBgElevated: "#111111"
        },
        Modal: {
            contentBg: "#0a0a0a",
            headerBg: "#0a0a0a"
        },
        Drawer: {
            colorBgElevated: "#0a0a0a"
        },
        Notification: {
            colorBgElevated: "#111111"
        },
        Descriptions: {
            colorSplit: "#1e1e1e"
        },
        Progress: {
            remainingColor: "#1e1e1e"
        },
        Tag: {
            defaultBg: "rgba(16, 185, 129, 0.08)",
            defaultColor: "#10b981"
        },
        Tabs: {
            inkBarColor: "#10b981",
            itemSelectedColor: "#10b981"
        },
        Statistic: {
            colorTextDescription: "#64748b"
        }
    }
};

export const lightTokens: ThemeConfig = {
    algorithm: theme.defaultAlgorithm,
    token: {
        colorPrimary: "#059669",
        colorSuccess: "#10b981",
        colorWarning: "#f59e0b",
        colorError: "#ef4444",
        colorInfo: "#059669",

        colorBgContainer: "#ffffff",
        colorBgElevated: "#ffffff",
        colorBgLayout: "#f8fafc",

        colorText: "rgba(0, 0, 0, 0.88)",
        colorTextSecondary: "rgba(0, 0, 0, 0.65)",
        colorTextTertiary: "rgba(0, 0, 0, 0.45)",

        colorBorder: "#e2e8f0",
        colorBorderSecondary: "#f1f5f9",

        borderRadius: 8,
        wireframe: false,

        colorLink: "#059669",
        colorLinkHover: "#10b981",
        colorLinkActive: "#047857"
    },
    components: {
        Layout: {
            siderBg: "#ffffff",
            headerBg: "#ffffff",
            bodyBg: "#f8fafc"
        },
        Button: {
            primaryShadow: "0 2px 4px rgba(5, 150, 105, 0.2)"
        }
    }
};

// CSS custom properties for SCSS files
export const darkCSSVars: Record<string, string> = {
    "--cl-bg-deepest": "#050505",
    "--cl-bg-container": "#0a0a0a",
    "--cl-bg-elevated": "#111111",
    "--cl-bg-spotlight": "#141414",
    "--cl-bg-hover": "rgba(16, 185, 129, 0.04)",
    "--cl-text-primary": "#e2e8f0",
    "--cl-text-secondary": "#94a3b8",
    "--cl-text-tertiary": "#64748b",
    "--cl-border": "#1e1e1e",
    "--cl-border-secondary": "#151515",
    "--cl-accent-primary": "#10b981",
    "--cl-accent-secondary": "#6366f1",
    "--cl-accent-success": "#10b981",
    "--cl-accent-warning": "#f59e0b",
    "--cl-accent-error": "#ef4444",
    "--cl-logo-bg": "#111111",
    "--cl-logo-text": "#e2e8f0",
    "--cl-mask-bg": "rgba(5, 5, 5, 0.8)",
    "--cl-code-bg": "#0e0e0e",
    "--cl-shadow-glow": "0 0 20px rgba(16, 185, 129, 0.08)",
    "--cl-card-border-glow": "0 0 0 1px rgba(16, 185, 129, 0.08)",
    "--cl-monospace": "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
    "--cl-neon-glow": "0 0 20px rgba(16, 185, 129, 0.3), 0 0 40px rgba(16, 185, 129, 0.1)",
    "--cl-neon-glow-subtle": "0 0 10px rgba(16, 185, 129, 0.15)"
};

export const lightCSSVars: Record<string, string> = {
    "--cl-bg-deepest": "#f8fafc",
    "--cl-bg-container": "#ffffff",
    "--cl-bg-elevated": "#ffffff",
    "--cl-bg-spotlight": "#f1f5f9",
    "--cl-bg-hover": "rgba(5, 150, 105, 0.04)",
    "--cl-text-primary": "rgba(0, 0, 0, 0.88)",
    "--cl-text-secondary": "rgba(0, 0, 0, 0.65)",
    "--cl-text-tertiary": "rgba(0, 0, 0, 0.45)",
    "--cl-border": "#e2e8f0",
    "--cl-border-secondary": "#f1f5f9",
    "--cl-accent-primary": "#059669",
    "--cl-accent-secondary": "#6366f1",
    "--cl-accent-success": "#10b981",
    "--cl-accent-warning": "#f59e0b",
    "--cl-accent-error": "#ef4444",
    "--cl-logo-bg": "#333333",
    "--cl-logo-text": "#333333",
    "--cl-mask-bg": "rgba(0, 0, 0, 0.05)",
    "--cl-code-bg": "#f1f5f9",
    "--cl-shadow-glow": "none",
    "--cl-card-border-glow": "none",
    "--cl-monospace": "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
    "--cl-neon-glow": "none",
    "--cl-neon-glow-subtle": "none"
};
