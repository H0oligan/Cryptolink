import apiRequest from "src/utils/api-request";
import withApiPath from "src/utils/with-api-path";

export interface XpubWallet {
    uuid: string;
    blockchain: string;
    derivationPath: string;
    lastDerivedIndex: number;
    createdAt: string;
}

export interface DerivedAddress {
    uuid: string;
    address: string;
    derivationPath: string;
    derivationIndex: number;
    isUsed: boolean;
    createdAt: string;
}

export interface CreateXpubWalletRequest {
    blockchain: string;
    xpub: string;
    derivationPath: string;
}

const xpubProvider = {
    async createXpubWallet(merchantId: string, data: CreateXpubWalletRequest): Promise<XpubWallet> {
        const response = await apiRequest.post(
            withApiPath(`/merchant/${merchantId}/xpub-wallet`),
            data
        );
        return response.data;
    },

    async listXpubWallets(merchantId: string): Promise<XpubWallet[]> {
        const response = await apiRequest.get(
            withApiPath(`/merchant/${merchantId}/xpub-wallet`)
        );
        return response.data;
    },

    async getXpubWallet(merchantId: string, walletId: string): Promise<XpubWallet> {
        const response = await apiRequest.get(
            withApiPath(`/merchant/${merchantId}/xpub-wallet/${walletId}`)
        );
        return response.data;
    },

    async deriveAddress(merchantId: string, walletId: string): Promise<DerivedAddress> {
        const response = await apiRequest.post(
            withApiPath(`/merchant/${merchantId}/xpub-wallet/${walletId}/derive`)
        );
        return response.data;
    },

    async getNextAddress(merchantId: string, walletId: string): Promise<DerivedAddress> {
        const response = await apiRequest.get(
            withApiPath(`/merchant/${merchantId}/xpub-wallet/${walletId}/next-address`)
        );
        return response.data;
    },

    async listDerivedAddresses(merchantId: string, walletId: string): Promise<DerivedAddress[]> {
        const response = await apiRequest.get(
            withApiPath(`/merchant/${merchantId}/xpub-wallet/${walletId}/addresses`)
        );
        return response.data;
    }
};

export default xpubProvider;
