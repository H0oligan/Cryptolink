import apiRequest from "src/utils/api-request";
import withApiPath from "src/utils/with-api-path";
import {SubscriptionPlan} from "./subscription-provider";

export interface AdminPlan extends SubscriptionPlan {
    is_active: boolean;
}

export interface CreatePlanParams {
    id: string;
    name: string;
    description: string;
    price_usd: number;
    billing_period: string;
    max_payments_monthly: number | null;
    max_merchants: number;
    max_api_calls_monthly: number | null;
    max_volume_monthly_usd: number | null;
    features: Record<string, any>;
    is_active: boolean;
}

export interface AdminMerchant {
    id: number;
    uuid: string;
    name: string;
    website: string;
    creator_email: string;
    creator_name: string;
    active_plan_id: string | null;
    active_plan_name: string | null;
    monthly_volume_usd: string;
    payment_count: number;
    created_at: string;
}

export interface AdminUser {
    id: number;
    uuid: string;
    email: string;
    name: string;
    is_super_admin: boolean;
    merchant_count: number;
    created_at: string;
}

export interface PaginatedResponse<T> {
    results: T[];
    total: number;
    limit: number;
    offset: number;
}

export interface EmailSettings {
    smtp_host: string;
    smtp_port: number;
    smtp_user: string;
    from_name: string;
    from_email: string;
    is_active: boolean;
}

export interface EmailSettingsUpdate extends EmailSettings {
    smtp_pass: string;
}

export interface EmailLogEntry {
    id: number;
    to_email: string;
    subject: string;
    template: string;
    status: string;
    error_message: string;
    created_at: string;
}

export interface SystemStats {
    total_merchants: number;
    total_users: number;
    paying_merchants: number;
    free_tier_count: number;
    basic_tier_count: number;
    pro_tier_count: number;
    enterprise_tier_count: number;
    no_subscription_count: number;
    monthly_revenue: string;
}

export interface AdminContact {
    id: number;
    uuid: string;
    email: string;
    marketing_consent: boolean;
    terms_accepted_at: string | null;
    source_merchant_name: string;
    created_at: string;
}

export interface CollectorFactoryConfig {
    blockchain: string;
    implementationAddress: string;
    factoryAddress: string;
    createdAt: string;
    updatedAt: string;
}

const adminProvider = {
    // Plans
    async listPlans(): Promise<AdminPlan[]> {
        const response = await apiRequest.get(withApiPath("/admin/plans"));
        return response.data;
    },

    async getPlan(planId: string): Promise<AdminPlan> {
        const response = await apiRequest.get(withApiPath(`/admin/plans/${planId}`));
        return response.data;
    },

    async createPlan(params: CreatePlanParams): Promise<AdminPlan> {
        const response = await apiRequest.post(withApiPath("/admin/plans"), params);
        return response.data;
    },

    async updatePlan(planId: string, params: CreatePlanParams): Promise<AdminPlan> {
        const response = await apiRequest.put(withApiPath(`/admin/plans/${planId}`), params);
        return response.data;
    },

    // Merchants
    async listMerchants(limit?: number, offset?: number): Promise<PaginatedResponse<AdminMerchant>> {
        const response = await apiRequest.get(withApiPath("/admin/merchants"), {
            params: {limit: limit || 50, offset: offset || 0}
        });
        return response.data;
    },

    async assignMerchantPlan(merchantId: number, planId: string): Promise<void> {
        await apiRequest.put(withApiPath(`/admin/merchants/${merchantId}/plan`), {plan_id: planId});
    },

    async deleteMerchant(merchantId: number): Promise<void> {
        await apiRequest.delete(withApiPath(`/admin/merchants/${merchantId}`));
    },

    // Users
    async listUsers(limit?: number, offset?: number): Promise<PaginatedResponse<AdminUser>> {
        const response = await apiRequest.get(withApiPath("/admin/users"), {
            params: {limit: limit || 50, offset: offset || 0}
        });
        return response.data;
    },

    async deleteUser(userId: number): Promise<void> {
        await apiRequest.delete(withApiPath(`/admin/users/${userId}`));
    },

    // Stats
    async getStats(): Promise<SystemStats> {
        const response = await apiRequest.get(withApiPath("/admin/subscription/stats"));
        return response.data;
    },

    // Subscriptions
    async listSubscriptions(): Promise<any[]> {
        const response = await apiRequest.get(withApiPath("/admin/subscription/list"));
        return response.data;
    },

    // Email
    async getEmailSettings(): Promise<EmailSettings> {
        const response = await apiRequest.get(withApiPath("/admin/email/settings"));
        return response.data;
    },

    async updateEmailSettings(params: EmailSettingsUpdate): Promise<EmailSettings> {
        const response = await apiRequest.put(withApiPath("/admin/email/settings"), params);
        return response.data;
    },

    async sendEmail(params: {to: string; subject: string; body: string}): Promise<void> {
        await apiRequest.post(withApiPath("/admin/email/send"), params);
    },

    async testEmail(): Promise<{message: string}> {
        const response = await apiRequest.post(withApiPath("/admin/email/test"));
        return response.data;
    },

    async getEmailLogs(limit?: number, offset?: number): Promise<PaginatedResponse<EmailLogEntry>> {
        const response = await apiRequest.get(withApiPath("/admin/email/log"), {
            params: {limit: limit || 50, offset: offset || 0}
        });
        return response.data;
    },

    // Collector Factories
    async listCollectorFactories(): Promise<CollectorFactoryConfig[]> {
        const response = await apiRequest.get(withApiPath("/admin/collector-factories"));
        return response.data;
    },

    async getCollectorFactory(blockchain: string): Promise<CollectorFactoryConfig> {
        const response = await apiRequest.get(withApiPath(`/admin/collector-factories/${blockchain}`));
        return response.data;
    },

    async upsertCollectorFactory(data: {
        blockchain: string;
        implementationAddress: string;
        factoryAddress: string;
    }): Promise<CollectorFactoryConfig> {
        const response = await apiRequest.post(withApiPath("/admin/collector-factories"), data);
        return response.data;
    },

    // Contacts
    async listContacts(limit?: number, offset?: number, search?: string): Promise<PaginatedResponse<AdminContact>> {
        const response = await apiRequest.get(withApiPath("/admin/contacts"), {
            params: {limit: limit || 20, offset: offset || 0, ...(search ? {search} : {})}
        });
        return response.data;
    },

    async exportContacts(marketingOnly?: boolean): Promise<Blob> {
        const response = await apiRequest.get(withApiPath("/admin/contacts/export"), {
            params: marketingOnly ? {marketing_only: true} : {},
            responseType: "blob"
        });
        return response.data;
    }
};

export default adminProvider;
