import apiRequest from "src/utils/api-request";
import withApiPath from "src/utils/with-api-path";

export interface EvmCollector {
    uuid: string;
    blockchain: string;
    chainId: number;
    contractAddress: string;
    ownerAddress: string;
    factoryAddress: string;
    isActive: boolean;
    createdAt: string;
}

export interface SetupCollectorRequest {
    blockchain: string;
    ownerAddress: string;
    contractAddress: string;
    chainId: number;
    factoryAddress: string;
}

export interface TokenBalance {
    contract: string;
    ticker: string;
    amount: string;
    usdAmount: string;
}

export interface CollectorBalance {
    native: {
        amount: string;
        ticker: string;
        usdAmount: string;
    };
    tokens: TokenBalance[];
}

const evmCollectorProvider = {
    async listCollectors(merchantId: string): Promise<EvmCollector[]> {
        const response = await apiRequest.get(
            withApiPath(`/merchant/${merchantId}/evm-collector`)
        );
        return response.data || [];
    },

    async setupCollector(merchantId: string, data: SetupCollectorRequest): Promise<EvmCollector> {
        const response = await apiRequest.post(
            withApiPath(`/merchant/${merchantId}/evm-collector`),
            data
        );
        return response.data;
    },

    async getCollector(merchantId: string, blockchain: string): Promise<EvmCollector> {
        const response = await apiRequest.get(
            withApiPath(`/merchant/${merchantId}/evm-collector/${blockchain}`)
        );
        return response.data;
    },

    async deleteCollector(merchantId: string, blockchain: string): Promise<void> {
        await apiRequest.delete(
            withApiPath(`/merchant/${merchantId}/evm-collector/${blockchain}`)
        );
    },

    async getBalance(merchantId: string, blockchain: string): Promise<CollectorBalance> {
        const response = await apiRequest.get(
            withApiPath(`/merchant/${merchantId}/evm-collector/${blockchain}/balance`)
        );
        return response.data;
    }
};

export default evmCollectorProvider;
