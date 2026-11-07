import {InfiniteData, useInfiniteQuery, UseInfiniteQueryResult, useQueryClient, useMutation} from "@tanstack/react-query";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import balancesProvider from "src/providers/balance-provider";
import merchantProvider from "src/providers/merchant-provider";
import {MerchantBalance, PaymentsPagination, Withdrawal, ConvertParams} from "src/types";
import {sleep} from "src/utils";

const PAGE_SIZE = 50;

const balancesQueries = {
    listBalances: (): UseInfiniteQueryResult<InfiniteData<MerchantBalance[]>> => {
        const {merchantId} = useSharedMerchantId();

        return useInfiniteQuery({
            queryKey: ["listBalances"],
            queryFn: () => {
                return balancesProvider.listBalances(merchantId!);
            },
            initialPageParam: undefined,
            getNextPageParam: () => undefined,
            staleTime: Infinity,
            enabled: Boolean(merchantId),
            retry: 2
        });
    },

    listWithdrawal: (): UseInfiniteQueryResult<InfiniteData<PaymentsPagination>> => {
        const {merchantId} = useSharedMerchantId();

        return useInfiniteQuery({
            queryKey: ["listWithdrawal"],
            queryFn: ({pageParam}) => {
                return merchantProvider.listPayments(merchantId!, {
                    limit: PAGE_SIZE,
                    cursor: pageParam,
                    type: "withdrawal",
                    reverseOrder: true
                });
            },
            initialPageParam: "",
            getNextPageParam: (lastPage) => lastPage.cursor || undefined,
            staleTime: Infinity,
            enabled: Boolean(merchantId),
            retry: 2
        });
    },

    createWithdrawal: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation({
            mutationFn: (params: Withdrawal) => {
                return balancesProvider.createWithdrawal(merchantId!, params);
            },
            onSuccess: async () => {
                await sleep(1000);
                queryClient.invalidateQueries({queryKey: ["listWithdrawal"]});
                queryClient.invalidateQueries({queryKey: ["listBalances"]});
            }
        });
    },

    getServiceFee: () => {
        const {merchantId} = useSharedMerchantId();

        return useMutation({
            mutationFn: (balanceId: string) => {
                return balancesProvider.getServiceFee(merchantId!, balanceId);
            }
        });
    },

    getExchangeRate: () => {
        const {merchantId} = useSharedMerchantId();

        return useMutation({
            mutationFn: (params: ConvertParams) => {
                return balancesProvider.getCurrencyExchangeRate(merchantId!, params);
            }
        });
    }
};

export default balancesQueries;
