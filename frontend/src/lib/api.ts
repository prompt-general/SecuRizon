import { BillingData, UsageData, Invoice } from '../types/billing';

// Mock API implementation
export const api = {
    getBillingInfo: async (tenantId: string) => {
        // Simulate API call
        return {
            data: {
                plan: {
                    id: 'pro',
                    name: 'Pro Plan',
                    monthly_price: 9900,
                    features: ['Unlimited Assets', 'Advanced Reporting'],
                },
                subscription: {
                    status: 'active',
                    current_period_end: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString(),
                },
                payment_methods: [
                    {
                        id: 'pm_1',
                        brand: 'visa',
                        last4: '4242',
                        exp_month: 12,
                        exp_year: 2025,
                        default: true,
                    },
                ],
                estimated_total: 10500,
            } as BillingData,
        };
    },

    getUsage: async (tenantId: string) => {
        return {
            data: {
                features: [
                    { name: 'Assets', used: 850, limit: 1000 },
                    { name: 'Users', used: 5, limit: 10 },
                ],
                overages: [
                    { feature: 'Storage', units: 10, cost: 500 },
                ],
            } as UsageData,
        };
    },

    getInvoices: async (tenantId: string) => {
        return {
            data: [
                {
                    id: 'inv_1',
                    date: new Date().toISOString(),
                    amount: 9900,
                    paid: true,
                    invoice_pdf: '#',
                },
            ] as Invoice[],
        };
    },

    upgradePlan: async (tenantId: string, planId: string) => {
        return {
            data: {
                checkout_url: '#',
            },
        };
    },
};
