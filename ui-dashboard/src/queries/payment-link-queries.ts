import {useMutation, useQuery, useQueryClient, UseQueryResult} from "@tanstack/react-query";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import paymentLinkProvider from "src/providers/payment-link-provider";
import {PaymentLink, PaymentLinkParams} from "src/types";
import {sleep} from "src/utils";

const paymentLinkQueriers = {
    listPaymentLinks: (): UseQueryResult<PaymentLink[]> => {
        const {merchantId} = useSharedMerchantId();

        return useQuery({
            queryKey: ["listPaymentLinks"],
            queryFn: () => {
                return paymentLinkProvider.listPaymentLinks(merchantId!);
            },
            staleTime: Infinity,
            enabled: Boolean(merchantId),
            retry: 2
        });
    },

    createPaymentLink: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation({
            mutationFn: (params: PaymentLinkParams) => {
                return paymentLinkProvider.createPaymentLink(merchantId!, params);
            },
            onSuccess: async () => {
                await sleep(1000);
                queryClient.invalidateQueries({queryKey: ["listPaymentLinks"]});
            }
        });
    },

    deletePaymentLink: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation({
            mutationFn: (paymentLinkId: string) => {
                return paymentLinkProvider.deletePaymentLink(merchantId!, paymentLinkId);
            },
            onSuccess: async () => {
                await sleep(1000);
                queryClient.invalidateQueries({queryKey: ["listPaymentLinks"]});
            }
        });
    }
};

export default paymentLinkQueriers;
