import React, { useState, useEffect } from 'react';
import {
    DashboardLayout,
    RiskHeatmap,
    AttackPathVisualizer,
    AssetTable,
    FindingsTable,
    RiskTrendChart,
    ComplianceScoreCard,
    QuickActions,
    StatCard,
} from './components';
import { useRouter } from 'next/router'; // Assuming Next.js router, adjust if using React Router
import api from '../../api'; // Adjust import path to your API client

// Types for dashboard data (you may need to adjust based on actual API response)
interface DashboardData {
    summary: {
        total_risk_score: number;
        total_assets: number;
        internet_exposed_assets: number;
        total_findings: number;
        critical_findings: number;
        total_attack_paths: number;
        exploitable_paths: number;
    };
    trends: {
        total_risk_trend: any; // replace with actual type
    };
    risk_heatmap: any[]; // replace with actual type
    risk_trends: any[]; // replace with actual type
    compliance_scores: any[]; // replace with actual type
    critical_paths?: any[]; // replace with actual type
    recent_findings?: any[]; // replace with actual type
    high_risk_assets?: any[]; // replace with actual type
}

const Dashboard: React.FC = () => {
    const router = useRouter();
    const [timeRange, setTimeRange] = useState('7d');
    const [selectedProvider, setSelectedProvider] = useState<string | null>(null);
    const [dashboardData, setDashboardData] = useState<DashboardData | null>(null);
    const [showPathModal, setShowPathModal] = useState(false);
    const [selectedPath, setSelectedPath] = useState<any>(null);
    const [showSimulationModal, setShowSimulationModal] = useState(false);
    const [showInviteModal, setShowInviteModal] = useState(false);

    useEffect(() => {
        fetchDashboardData();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [timeRange, selectedProvider]);

    const fetchDashboardData = async () => {
        try {
            const response = await api.getDashboardData({
                timeframe: timeRange,
                provider: selectedProvider,
            });
            setDashboardData(response.data);
        } catch (err) {
            console.error('Failed to fetch dashboard data', err);
        }
    };

    const generateComplianceReport = async () => {
        // Placeholder – implement report generation logic
        console.log('Generating compliance report');
    };

    return (
        <DashboardLayout>
            {/* Top Stats Row */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
                <StatCard
                    title="Total Risk Score"
                    value={dashboardData?.summary.total_risk_score || 0}
                    trend={dashboardData?.trends.total_risk_trend}
                    color="red"
                    format="number"
                />
                <StatCard
                    title="Assets"
                    value={dashboardData?.summary.total_assets || 0}
                    subValue={`${dashboardData?.summary.internet_exposed_assets || 0} exposed`}
                    color="blue"
                />
                <StatCard
                    title="Findings"
                    value={dashboardData?.summary.total_findings || 0}
                    subValue={`${dashboardData?.summary.critical_findings || 0} critical`}
                    color="orange"
                />
                <StatCard
                    title="Attack Paths"
                    value={dashboardData?.summary.total_attack_paths || 0}
                    subValue={`${dashboardData?.summary.exploitable_paths || 0} exploitable`}
                    color="purple"
                />
            </div>

            {/* Risk Heatmap */}
            <div className="mb-6">
                <RiskHeatmap
                    data={dashboardData?.risk_heatmap || []}
                    onCellClick={(provider, severity) => {
                        router.push(`/findings?provider=${provider}&severity=${severity}`);
                    }}
                />
            </div>

            {/* Charts Row */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
                <RiskTrendChart
                    data={dashboardData?.risk_trends || []}
                    timeframe={timeRange}
                    onTimeframeChange={setTimeRange}
                />
                <ComplianceScoreCard
                    scores={dashboardData?.compliance_scores || []}
                    frameworks={['CIS AWS', 'CIS Azure', 'NIST CSF']}
                />
            </div>

            {/* Critical Attack Paths */}
            <div className="mb-6">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">Critical Attack Paths</h2>
                    <button
                        className="btn btn-secondary"
                        onClick={() => router.push('/attack-paths')}
                    >
                        View All
                    </button>
                </div>
                <AttackPathVisualizer
                    paths={dashboardData?.critical_paths?.slice(0, 3) || []}
                    interactive={true}
                    onPathSelect={(path) => {
                        setSelectedPath(path);
                        setShowPathModal(true);
                    }}
                />
            </div>

            {/* Recent Findings & Assets */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                <div>
                    <div className="flex justify-between items-center mb-4">
                        <h3 className="text-lg font-semibold">Recent High-Risk Findings</h3>
                        <button
                            className="btn btn-text"
                            onClick={() => router.push('/findings')}
                        >
                            View All
                        </button>
                    </div>
                    <FindingsTable
                        findings={dashboardData?.recent_findings || []}
                        compact={true}
                        onFindingClick={(finding) => {
                            router.push(`/findings/${finding.id}`);
                        }}
                    />
                </div>

                <div>
                    <div className="flex justify-between items-center mb-4">
                        <h3 className="text-lg font-semibold">High-Risk Assets</h3>
                        <button
                            className="btn btn-text"
                            onClick={() => router.push('/assets')}
                        >
                            View All
                        </button>
                    </div>
                    <AssetTable
                        assets={dashboardData?.high_risk_assets || []}
                        compact={true}
                        onAssetClick={(asset) => {
                            router.push(`/assets/${asset.id}`);
                        }}
                    />
                </div>
            </div>

            {/* Quick Actions */}
            <div className="mt-8">
                <QuickActions
                    actions={[
                        {
                            title: 'Run Attack Simulation',
                            icon: 'ShieldAlert',
                            onClick: () => setShowSimulationModal(true),
                            description: 'Simulate attacks from internet-facing assets',
                        },
                        {
                            title: 'Generate Compliance Report',
                            icon: 'FileText',
                            onClick: () => generateComplianceReport(),
                            description: 'Generate PDF report for selected frameworks',
                        },
                        {
                            title: 'Bulk Remediate',
                            icon: 'Wrench',
                            onClick: () => router.push('/remediation/bulk'),
                            description: 'Remediate multiple findings at once',
                        },
                        {
                            title: 'Invite Team Members',
                            icon: 'Users',
                            onClick: () => setShowInviteModal(true),
                            description: 'Add team members to SecuRizon',
                        },
                    ]}
                />
            </div>
        </DashboardLayout>
    );
};

export default Dashboard;
