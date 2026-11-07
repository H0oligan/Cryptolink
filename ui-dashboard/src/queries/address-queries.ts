import {useMutation, useQuery, useQueryClient, UseQueryResult} from "@tanstack/react-query";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import addressProvider from "src/providers/address-provider";
import {MerchantAddress, MerchantAddressParams} from "src/types";
import {sleep} from "src/utils";

const addressQueries = {
    listAddresses: (): UseQueryResult<MerchantAddress[]> => {
        const {merchantId} = useSharedMerchantId();

        return useQuery({
            queryKey: ["listAddresses"],
            queryFn: () => {
                return addressProvider.listAddresses(merchantId!);
            },
            staleTime: Infinity,
            enabled: Boolean(merchantId),
            retry: 2
        });
    },

    createAddress: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation({
            mutationFn: (params: MerchantAddressParams) => {
                return addressProvider.createAddress(merchantId!, params);
            },
            onSuccess: async () => {
                await sleep(1000);
                queryClient.invalidateQueries({queryKey: ["listAddresses"]});
            }
        });
    },

    updateAddress: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation({
            mutationFn: (params: MerchantAddress) => {
                return addressProvider.updateAddress(merchantId!, params.id, params.name);
            },
            onSuccess: async () => {
                await sleep(1000);
                queryClient.invalidateQueries({queryKey: ["listAddresses"]});
            }
        });
    },

    deleteAddress: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation({
            mutationFn: (addressId: string) => {
                return addressProvider.deleteAddress(merchantId!, addressId);
            },
            onSuccess: async () => {
                await sleep(1000);
                queryClient.invalidateQueries({queryKey: ["listAddresses"]});
            }
        });
    }
};

export default addressQueries;
