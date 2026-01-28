export interface Plan {
    id: string;
    name: string;
    monthly_price: number;
    features: string[];
}

export interface Subscription {
    status: 'active' | 'past_due' | 'canceled' | 'incomplete';
    current_period_end: string;
}

export interface PaymentMethod {
    id: string;
    brand: string;
    last4: string;
    exp_month: number;
    exp_year: number;
    default: boolean;
}

export interface BillingData {
    plan: Plan;
    subscription: Subscription;
    payment_methods: PaymentMethod[];
    estimated_total: number;
}

export interface FeatureUsage {
    name: string;
    used: number;
    limit: number;
}

export interface Overage {
    feature: string;
    units: number;
    cost: number;
}

export interface UsageData {
    features: FeatureUsage[];
    overages: Overage[];
}

export interface Invoice {
    id: string;
    date: string;
    amount: number;
    paid: boolean;
    invoice_pdf: string;
}
