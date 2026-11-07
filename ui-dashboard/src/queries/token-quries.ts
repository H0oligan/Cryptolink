import {useMutation, useQuery, useQueryClient, UseQueryResult} from "@tanstack/react-query";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import tokenProvider from "src/providers/token-provider";
import {MerchantToken} from "src/types";
import {sleep} from "src/utils";

const tokenQueries = {
    listTokens: (): UseQueryResult<MerchantToken[]> => {
        const {merchantId} = useSharedMerchantId();

        return useQuery({
            queryKey: ["listTokens"],
            queryFn: () => {
                return tokenProvider.listTokens(merchantId!);
            },
            staleTime: Infinity,
            enabled: Boolean(merchantId),
            retry: 2
        });
    },

    createToken: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation({
            mutationFn: (name: string) => {
                return tokenProvider.createToken(merchantId!, name);
            },
            onSuccess: async () => {
                await sleep(1000);
                queryClient.invalidateQueries({queryKey: ["listTokens"]});
            }
        });
    },

    deleteToken: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation({
            mutationFn: (tokenId: string) => {
                return tokenProvider.deleteToken(merchantId!, tokenId);
            },
            onSuccess: async () => {
                await sleep(1000);
                queryClient.invalidateQueries({queryKey: ["listTokens"]});
            }
        });
    }
};

export default tokenQueries;
