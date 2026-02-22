import apiRequest from "src/utils/api-request";
import withApiPath from "src/utils/with-api-path";

export interface SubscriptionPlan {
    id: string;
    name: string;
    description: string;
    price_usd: string;
    billing_period: string;
    max_payments_monthly: number | null;
    max_merchants: number;
    max_api_calls_monthly: number | null;
    max_volume_monthly_usd: string | null;
    features: Record<string, any>;
}

export interface SubscriptionInfo {
    uuid: string;
    plan_id: string;
    status: string;
    current_period_start: string;
    current_period_end: string;
    auto_renew: boolean;
    plan?: SubscriptionPlan;
}

export interface UsageInfo {
    payment_count: number;
    payment_volume_usd: string;
    api_calls_count: number;
    period_start: string;
    period_end: string;
}

export interface CurrentSubscription {
    subscription: SubscriptionInfo | null;
    usage: UsageInfo | null;
}

export interface UpgradeResponse {
    subscription_uuid: string;
    payment_url: string;
    payment_uuid: string;
    amount_due: string;
    currency: string;
}

const subscriptionProvider = {
    async listPlans(): Promise<SubscriptionPlan[]> {
        const response = await apiRequest.get(withApiPath("/subscription/plans"));
        return response.data;
    },

    async getCurrentSubscription(merchantId: string): Promise<CurrentSubscription> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/subscription`));
        return response.data;
    },

    async upgradePlan(merchantId: string, planId: string, redirectUrl?: string): Promise<UpgradeResponse> {
        const response = await apiRequest.post(withApiPath(`/merchant/${merchantId}/subscription/upgrade`), {
            plan_id: planId,
            redirect_url: redirectUrl
        });
        return response.data;
    },

    async cancelSubscription(merchantId: string): Promise<void> {
        await apiRequest.post(withApiPath(`/merchant/${merchantId}/subscription/cancel`));
    },

    async getUsageHistory(merchantId: string): Promise<UsageInfo[]> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/subscription/usage`));
        return response.data;
    }
};

export default subscriptionProvider;
