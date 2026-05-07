import axios, {AxiosError} from "axios";
import {RenderErrorAlert} from "src/components/ErrorAlert";

const apiRequest = axios.create({
    baseURL: import.meta.env.VITE_BACKEND_HOST,
    headers: {
        "Content-Type": "application/json",
        "Cache-Control": "no-cache",
        Accept: "application/json"
    },
    withCredentials: true
});

apiRequest.interceptors.response.use(undefined, function (error: Error | AxiosError) {
    if (axios.isAxiosError(error) && error.response) {
        return Promise.reject(error);
    }
    RenderErrorAlert(error.message);
    return Promise.reject(error);
});

export default apiRequest;
