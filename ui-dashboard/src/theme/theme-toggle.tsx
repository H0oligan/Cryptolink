import * as React from "react";
import {Button, Tooltip} from "antd";
import {BulbOutlined, BulbFilled} from "@ant-design/icons";
import {useTheme} from "./theme-context";

const ThemeToggle: React.FC = () => {
    const {isDark, toggleTheme} = useTheme();

    return (
        <Tooltip title={isDark ? "Switch to Light Mode" : "Switch to Dark Mode"}>
            <Button
                type="text"
                onClick={toggleTheme}
                icon={isDark ? <BulbOutlined /> : <BulbFilled />}
                style={{
                    fontSize: 18,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    color: "var(--cl-text-secondary)"
                }}
            />
        </Tooltip>
    );
};

export default ThemeToggle;
