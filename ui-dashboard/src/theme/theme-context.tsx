import * as React from "react";
import {ConfigProvider} from "antd";
import {darkTokens, lightTokens, darkCSSVars, lightCSSVars} from "./theme-tokens";

type ThemeMode = "dark" | "light";

interface ThemeContextType {
    mode: ThemeMode;
    toggleTheme: () => void;
    setTheme: (mode: ThemeMode) => void;
    isDark: boolean;
}

const STORAGE_KEY = "cryptolink-theme";

const ThemeContext = React.createContext<ThemeContextType>({
    mode: "dark",
    toggleTheme: () => {},
    setTheme: () => {},
    isDark: true
});

export const useTheme = () => React.useContext(ThemeContext);

const applyCSS = (vars: Record<string, string>) => {
    const root = document.documentElement;
    Object.entries(vars).forEach(([key, value]) => {
        root.style.setProperty(key, value);
    });
};

export const ThemeProvider: React.FC<{children: React.ReactNode}> = ({children}) => {
    const [mode, setMode] = React.useState<ThemeMode>(() => {
        const stored = localStorage.getItem(STORAGE_KEY);
        return stored === "light" || stored === "dark" ? stored : "dark";
    });

    const isDark = mode === "dark";

    React.useEffect(() => {
        localStorage.setItem(STORAGE_KEY, mode);
        document.documentElement.setAttribute("data-theme", mode);
        applyCSS(isDark ? darkCSSVars : lightCSSVars);

        const metaThemeColor = document.querySelector('meta[name="theme-color"]');
        if (metaThemeColor) {
            metaThemeColor.setAttribute("content", isDark ? "#0a0a0f" : "#ffffff");
        }
    }, [mode, isDark]);

    const toggleTheme = () => setMode((prev) => (prev === "dark" ? "light" : "dark"));
    const setTheme = (newMode: ThemeMode) => setMode(newMode);

    const antdTheme = isDark ? darkTokens : lightTokens;

    return (
        <ThemeContext.Provider value={{mode, toggleTheme, setTheme, isDark}}>
            <ConfigProvider theme={antdTheme}>{children}</ConfigProvider>
        </ThemeContext.Provider>
    );
};
