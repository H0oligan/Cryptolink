/** @type {import("tailwindcss").Config} */
module.exports = {
    content: ["./src/**/*.{html,js,ts,tsx}", "./node_modules/tw-elements/dist/js/**/*.js"],
    theme: {
        extend: {
            colors: {
                primary: "#818cf8",
                "primary-darker": "#6366f1",
                card: {
                    desc: "#94a3b8",
                    error: "#ef4444"
                },
                main: {
                    "green-1": "#818cf8",
                    "green-2": "#10b981",
                    "green-3": "#0a0a0f",
                    "error": "#ef4444",
                    "red-1": "rgba(239, 68, 68, 0.08)",
                    "red-2": "rgba(239, 68, 68, 0.15)"
                }
            },
            maxWidth: {
                "xl-desc-size": "300px",
                "sm-desc-size": "209px"
            },
            minHeight: {
                "mobile-card": "600px"
            },
            height: {
                "mobile-card-height": "calc(100vh - 3.75rem)"
            }
        },
        screens: {
            "xs": {"max": "390px"},
            "sm": {"max": "639px"},
            "md": {"min": "390px", "max": "639px"},
            "lg": "640px"
        }
    },
    plugins: [require("tw-elements/dist/plugin")]
};
