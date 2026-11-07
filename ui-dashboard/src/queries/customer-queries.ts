import {InfiniteData, useInfiniteQuery, UseInfiniteQueryResult, useMutation} from "@tanstack/react-query";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import merchantProvider from "src/providers/merchant-provider";
import {CustomerPagination} from "src/types";

const PAGE_SIZE = 50;

const customersQueries = {
    listCustomers: (): UseInfiniteQueryResult<InfiniteData<CustomerPagination>> => {
        const {merchantId} = useSharedMerchantId();

        return useInfiniteQuery({
            queryKey: ["listCustomers"],
            queryFn: ({pageParam}) => {
                return merchantProvider.listCustomers(merchantId!, {
                    limit: PAGE_SIZE,
                    cursor: pageParam,
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

    getCustomer: () => {
        const {merchantId} = useSharedMerchantId();

        return useMutation({
            mutationFn: (customerId: string) => {
                return merchantProvider.getCustomerDetails(merchantId!, customerId);
            }
        });
    }
};

export default customersQueries;
