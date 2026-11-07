import {InfiniteData, useInfiniteQuery, UseInfiniteQueryResult, useMutation, useQueryClient} from "@tanstack/react-query";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import merchantProvider from "src/providers/merchant-provider";
import {PaymentParams, PaymentsPagination} from "src/types";
import {sleep} from "src/utils";

const PAGE_SIZE = 50;

const paymentsQueries = {
    listPayments: (): UseInfiniteQueryResult<InfiniteData<PaymentsPagination>> => {
        const {merchantId} = useSharedMerchantId();

        return useInfiniteQuery({
            queryKey: ["listPayments"],
            queryFn: ({pageParam}) => {
                return merchantProvider.listPayments(merchantId!, {
                    limit: PAGE_SIZE,
                    cursor: pageParam,
                    type: "payment",
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

    createPayment: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation({
            mutationFn: (params: PaymentParams) => {
                return merchantProvider.createPayment(merchantId!, params);
            },
            onSuccess: async () => {
                await sleep(1000);
                queryClient.invalidateQueries({queryKey: ["listPayments"]});
            }
        });
    },

    getPayment: () => {
        const {merchantId} = useSharedMerchantId();

        return useMutation({
            mutationFn: (paymentId: string) => {
                return merchantProvider.getPayment(merchantId!, paymentId);
            }
        });
    }
};

export default paymentsQueries;
