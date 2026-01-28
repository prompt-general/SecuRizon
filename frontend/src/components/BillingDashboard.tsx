import React, { useState, useEffect } from 'react';
import {
    Card,
    Button,
    Table,
    Progress,
    Badge,
    Modal,
    Alert,
} from './ui';
import { api } from '../lib/api';
import { BillingData, UsageData, Invoice } from '../types/billing';

interface BillingDashboardProps {
    tenantId: string;
}

const PLANS = [
    {
        id: 'starter',
        name: 'Starter',
        price: 29,
        features: ['100 Assets', 'Basic Reporting', 'Email Support'],
    },
    {
        id: 'pro',
        name: 'Pro',
        price: 99,
        features: ['1000 Assets', 'Advanced Reporting', 'Priority Support', 'API Access'],
    },
    {
        id: 'enterprise',
        name: 'Enterprise',
        price: 499,
        features: ['Unlimited Assets', 'Custom Reporting', 'Dedicated Support', 'SSO', 'Audit Logs'],
    },
];

const BillingDashboard: React.FC<BillingDashboardProps> = ({ tenantId }) => {
    const [billingData, setBillingData] = useState<BillingData | null>(null);
    const [usage, setUsage] = useState<UsageData | null>(null);
    const [invoices, setInvoices] = useState<Invoice[]>([]);
    const [loading, setLoading] = useState(true);
    const [showUpgradeModal, setShowUpgradeModal] = useState(false);

    useEffect(() => {
        fetchBillingData();
    }, [tenantId]);

    const fetchBillingData = async () => {
        setLoading(true);
        try {
            const [billingRes, usageRes, invoicesRes] = await Promise.all([
                api.getBillingInfo(tenantId),
                api.getUsage(tenantId),
                api.getInvoices(tenantId),
            ]);
            setBillingData(billingRes.data);
            setUsage(usageRes.data);
            setInvoices(invoicesRes.data);
        } catch (error) {
            console.error('Failed to fetch billing data:', error);
        } finally {
            setLoading(false);
        }
    };

    const handleUpgrade = async (planId: string) => {
        try {
            const { data } = await api.upgradePlan(tenantId, planId);
            window.location.href = data.checkout_url;
        } catch (error) {
            console.error('Failed to upgrade plan:', error);
        }
    };

    if (loading) {
        return <div>Loading billing information...</div>;
    }

    return (
        <div className="space-y-6">
            {/* Current Plan */}
            <Card>
                <div className="flex justify-between items-center mb-4">
                    <div>
                        <h3 className="text-lg font-semibold">Current Plan</h3>
                        <p className="text-gray-600">{billingData?.plan.name}</p>
                    </div>
                    <Badge variant={billingData?.subscription.status === 'active' ? 'success' : 'warning'}>
                        {billingData?.subscription.status}
                    </Badge>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
                    <div>
                        <div className="text-sm text-gray-500">Next Billing Date</div>
                        <div className="font-semibold">
                            {new Date(billingData?.subscription.current_period_end || '').toLocaleDateString()}
                        </div>
                    </div>
                    <div>
                        <div className="text-sm text-gray-500">Monthly Cost</div>
                        <div className="font-semibold">
                            ${(billingData?.plan.monthly_price || 0) / 100}
                        </div>
                    </div>
                    <div>
                        <Button onClick={() => setShowUpgradeModal(true)}>
                            Upgrade Plan
                        </Button>
                    </div>
                </div>

                {/* Usage Progress Bars */}
                <div className="space-y-4">
                    <h4 className="font-medium">Usage This Month</h4>
                    {usage?.features.map((feature) => (
                        <div key={feature.name} className="space-y-1">
                            <div className="flex justify-between text-sm">
                                <span>{feature.name}</span>
                                <span>
                                    {feature.used} / {feature.limit || 'Unlimited'}
                                </span>
                            </div>
                            <Progress
                                value={(feature.used / (feature.limit || 1)) * 100}
                                color={feature.used > feature.limit * 0.9 ? 'red' : 'blue'}
                            />
                            {feature.limit && feature.used > feature.limit && (
                                <Alert variant="warning" className="mt-2">
                                    You have exceeded your {feature.name} limit. Additional charges may apply.
                                </Alert>
                            )}
                        </div>
                    ))}
                </div>
            </Card>

            {/* Estimated Cost */}
            <Card>
                <h3 className="text-lg font-semibold mb-4">Estimated Cost This Month</h3>
                <div className="space-y-2">
                    <div className="flex justify-between">
                        <span>Base Plan</span>
                        <span>${(billingData?.plan.monthly_price || 0) / 100}</span>
                    </div>
                    {usage?.overages.map((overage) => (
                        <div key={overage.feature} className="flex justify-between">
                            <span>{overage.feature} overage ({overage.units} units)</span>
                            <span>${overage.cost / 100}</span>
                        </div>
                    ))}
                    <div className="flex justify-between border-t pt-2 font-semibold">
                        <span>Total Estimated</span>
                        <span>${(billingData?.estimated_total || 0) / 100}</span>
                    </div>
                </div>
            </Card>

            {/* Recent Invoices */}
            <Card>
                <h3 className="text-lg font-semibold mb-4">Recent Invoices</h3>
                <Table>
                    <thead>
                        <tr>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Date</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Amount</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Download</th>
                        </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                        {invoices.map((invoice) => (
                            <tr key={invoice.id}>
                                <td className="px-6 py-4 whitespace-nowrap">{new Date(invoice.date).toLocaleDateString()}</td>
                                <td className="px-6 py-4 whitespace-nowrap">${invoice.amount / 100}</td>
                                <td className="px-6 py-4 whitespace-nowrap">
                                    <Badge variant={invoice.paid ? 'success' : 'warning'}>
                                        {invoice.paid ? 'Paid' : 'Pending'}
                                    </Badge>
                                </td>
                                <td className="px-6 py-4 whitespace-nowrap">
                                    {invoice.invoice_pdf && (
                                        <a
                                            href={invoice.invoice_pdf}
                                            target="_blank"
                                            rel="noopener noreferrer"
                                            className="text-blue-600 hover:underline"
                                        >
                                            PDF
                                        </a>
                                    )}
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </Table>
            </Card>

            {/* Payment Methods */}
            <Card>
                <div className="flex justify-between items-center mb-4">
                    <h3 className="text-lg font-semibold">Payment Methods</h3>
                    <Button variant="outline">Add Payment Method</Button>
                </div>
                {billingData?.payment_methods.map((method) => (
                    <div key={method.id} className="flex items-center justify-between p-3 border rounded mb-2">
                        <div className="flex items-center">
                            <div className="w-8 h-8 bg-gray-200 rounded flex items-center justify-center mr-3">
                                {method.brand === 'visa' ? 'VISA' : method.brand === 'mastercard' ? 'MC' : method.brand}
                            </div>
                            <div>
                                <div className="font-medium">
                                    {method.brand.toUpperCase()} •••• {method.last4}
                                </div>
                                <div className="text-sm text-gray-500">
                                    Expires {method.exp_month}/{method.exp_year}
                                </div>
                            </div>
                        </div>
                        {method.default && (
                            <Badge variant="success">Default</Badge>
                        )}
                    </div>
                ))}
            </Card>

            {/* Upgrade Modal */}
            <Modal
                isOpen={showUpgradeModal}
                onClose={() => setShowUpgradeModal(false)}
                title="Upgrade Your Plan"
            >
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    {PLANS.map((plan) => (
                        <Card
                            key={plan.id}
                            className={`border-2 ${plan.id === billingData?.plan.id
                                    ? 'border-blue-500'
                                    : 'border-gray-200'
                                }`}
                        >
                            <div className="text-center">
                                <h4 className="font-bold text-lg">{plan.name}</h4>
                                <div className="my-4">
                                    <span className="text-3xl font-bold">${plan.price}</span>
                                    <span className="text-gray-500">/month</span>
                                </div>
                                <ul className="text-left space-y-2 mb-6">
                                    {plan.features.map((feature) => (
                                        <li key={feature} className="flex items-center text-sm">
                                            <svg className="w-4 h-4 text-green-500 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                                <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                                            </svg>
                                            {feature}
                                        </li>
                                    ))}
                                </ul>
                                <Button
                                    variant={plan.id === billingData?.plan.id ? 'outline' : 'primary'}
                                    disabled={plan.id === billingData?.plan.id}
                                    onClick={() => handleUpgrade(plan.id)}
                                    fullWidth
                                >
                                    {plan.id === billingData?.plan.id ? 'Current Plan' : 'Upgrade'}
                                </Button>
                            </div>
                        </Card>
                    ))}
                </div>
            </Modal>
        </div>
    );
};

export default BillingDashboard;
