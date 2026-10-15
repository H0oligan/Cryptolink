// Buffer polyfill for browser
import {Buffer} from "buffer";
(window as any).Buffer = Buffer;

import ReactDOM from "react-dom/client";
import {BrowserRouter} from "react-router-dom";
import {QueryClient, QueryClientProvider} from "@tanstack/react-query";
import {App as AntdApp} from "antd";
import AdminApp from "./admin-app";
import {ThemeProvider} from "./theme/theme-context";
import {WalletProvider} from "./wallet/wallet-context";

const queryClient = new QueryClient();

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
    <ThemeProvider>
        <AntdApp>
            <BrowserRouter basename="/admin">
                <QueryClientProvider client={queryClient}>
                    <WalletProvider handoffEnabled={false}>
                        <AdminApp />
                    </WalletProvider>
                </QueryClientProvider>
            </BrowserRouter>
        </AntdApp>
    </ThemeProvider>
);
