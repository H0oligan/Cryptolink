import "./login-page.scss";

import * as React from "react";
import {AxiosError} from "axios";
import {useNavigate, useLocation, useSearchParams} from "react-router-dom";
import {Button, Typography, Form, Input, notification} from "antd";
import {GoogleOutlined, CheckOutlined} from "@ant-design/icons";
const logoImg = "/logo.svg";
import bevis from "src/utils/bevis";
import {useMount} from "react-use";
import localStorage from "src/utils/local-storage";
import {AuthProvider, UserCreateForm} from "src/types";
import authProvider from "src/providers/auth-provider";
import {sleep} from "src/utils";
import SpinWithMask from "src/components/spin-with-mask/spin-with-mask";

const b = bevis("login-page");

interface LoginState {
    isNeedLogout: boolean;
}

interface RegisterForm extends UserCreateForm {
    confirmPassword?: string;
}

const LoginPage: React.FC = () => {
    const [form] = Form.useForm<RegisterForm>();
    const [api, contextHolder] = notification.useNotification();
    const [isFormSubmitting, setIsFormSubmitting] = React.useState<boolean>(false);
    const [providersList, setProvidersList] = React.useState<AuthProvider[]>([]);
    const [searchParams] = useSearchParams();
    const [isRegisterMode, setIsRegisterMode] = React.useState<boolean>(searchParams.get("mode") === "register");
    const navigate = useNavigate();
    const state: LoginState = useLocation().state;

    const openNotification = (title: string, description: string) => {
        api.info({
            message: title,
            description,
            placement: "bottomRight",
            icon: <CheckOutlined style={{color: "#49D1AC"}} />
        });
    };

    const onSubmit = async (values: RegisterForm) => {
        try {
            setIsFormSubmitting(true);

            if (isRegisterMode) {
                await authProvider.register(values);
                openNotification("Welcome!", "Your account has been created successfully");
            } else {
                await authProvider.login(values);
                openNotification("Welcome back!", "");
            }

            navigate("/", {
                state: {realoadUserInfo: true}
            });

            await sleep(1000);
            form.resetFields();
        } catch (error) {
            console.error("error occurred: ", error);
        } finally {
            setIsFormSubmitting(false);
        }
    };

    const toggleMode = () => {
        setIsRegisterMode(!isRegisterMode);
        form.resetFields();
    };

    const googleRedirectLink = (): string => {
        let host = import.meta.env.VITE_BACKEND_HOST;
        if (host == "//") {
            host = "";
        }

        return `${host}/api/dashboard/v1/auth/redirect`;
    };

    useMount(async () => {
        window.addEventListener("popstate", () => navigate("/login", {replace: true}));

        if (state?.isNeedLogout) {
            localStorage.remove("merchantId");
        } else {
            try {
                await authProvider.getCookie();
                await authProvider.getMe();
                navigate("/");
            } catch (e) {
                if (e instanceof AxiosError && e.response?.status === 401) {
                    localStorage.remove("merchantId");
                }
            }
        }

        const availProviders = await authProvider.getProviders();
        setProvidersList(availProviders ?? []);
    });

    const isLoading = providersList.length === 0;

    return (
        <div className={b()}>
            {contextHolder}
            <div className={b("container")}>
                <div className={b("card")}>
                    <div className={b("logo")}>
                        <img src={logoImg} alt="logo" className={b("logo-img")} />
                        <Typography.Title className={b("logo-text")} level={3}>CryptoLink</Typography.Title>
                    </div>

                    <Typography.Title level={2} style={{marginBottom: 24}}>
                        {isRegisterMode ? "Create Account" : "Sign In"}
                    </Typography.Title>

                    <SpinWithMask isLoading={isLoading} />

                    {!isLoading ? (
                        <>
                            {providersList.findIndex((item) => item.name === "email") !== -1 ? (
                                <Form<RegisterForm>
                                    form={form}
                                    onFinish={onSubmit}
                                    layout="vertical"
                                    className={b("form")}
                                >
                                    {isRegisterMode && (
                                        <Form.Item
                                            name="name"
                                            rules={[
                                                {
                                                    required: true,
                                                    message: "Please input your name"
                                                },
                                                {
                                                    min: 2,
                                                    message: "Name must be at least 2 characters"
                                                }
                                            ]}
                                        >
                                            <Input placeholder="Full Name" size="large" />
                                        </Form.Item>
                                    )}
                                    <Form.Item
                                        name="email"
                                        rules={[
                                            {
                                                type: "email",
                                                message: "The input is not valid email"
                                            },
                                            {
                                                required: true,
                                                message: "Please input your email"
                                            }
                                        ]}
                                    >
                                        <Input placeholder="Email" size="large" />
                                    </Form.Item>
                                    <Form.Item
                                        name="password"
                                        rules={[
                                            {
                                                required: true,
                                                message: "Please input your password"
                                            },
                                            {
                                                min: 8,
                                                message: "Password must be at least 8 characters"
                                            }
                                        ]}
                                    >
                                        <Input.Password placeholder="Password" size="large" />
                                    </Form.Item>
                                    {isRegisterMode && (
                                        <Form.Item
                                            name="confirmPassword"
                                            dependencies={["password"]}
                                            rules={[
                                                {
                                                    required: true,
                                                    message: "Please confirm your password"
                                                },
                                                ({getFieldValue}) => ({
                                                    validator(_, value) {
                                                        if (!value || getFieldValue("password") === value) {
                                                            return Promise.resolve();
                                                        }
                                                        return Promise.reject(
                                                            new Error("Passwords do not match")
                                                        );
                                                    }
                                                })
                                            ]}
                                        >
                                            <Input.Password placeholder="Confirm Password" size="large" />
                                        </Form.Item>
                                    )}
                                    <Button
                                        disabled={isFormSubmitting}
                                        loading={isFormSubmitting}
                                        type="primary"
                                        htmlType="submit"
                                        className={b("btn")}
                                        size="large"
                                    >
                                        {isRegisterMode ? "Create Account" : "Sign in"}
                                    </Button>
                                    <div style={{textAlign: "center", marginTop: "16px"}}>
                                        <Typography.Link onClick={toggleMode}>
                                            {isRegisterMode
                                                ? "Already have an account? Sign in"
                                                : "Don't have an account? Register"}
                                        </Typography.Link>
                                    </div>
                                </Form>
                            ) : null}

                            {providersList.length == 2 ? (
                                <Typography.Text className={b("text-or")}>OR</Typography.Text>
                            ) : null}

                            {providersList.findIndex((item) => item.name === "google") !== -1 ? (
                                <Button
                                    key="submit"
                                    type="primary"
                                    href={googleRedirectLink()}
                                    className={b("btn")}
                                    size="large"
                                >
                                    Sign in / Register with Google <GoogleOutlined />
                                </Button>
                            ) : null}
                        </>
                    ) : null}
                </div>
                <div className={b("footer")}>
                    Powered by CryptoLink — Self-hosted crypto payments
                </div>
            </div>
        </div>
    );
};

export default LoginPage;
