export interface DashboardData {
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
    total_risk_trend: 'up' | 'down' | 'stable';
    findings_trend: 'up' | 'down' | 'stable';
  };
  risk_heatmap: RiskHeatmapCell[];
  risk_trends: RiskTrendPoint[];
  compliance_scores: ComplianceScore[];
  critical_paths: AttackPath[];
  recent_findings: Finding[];
  high_risk_assets: Asset[];
}

export interface RiskHeatmapCell {
  provider: string;
  severity: 'critical' | 'high' | 'medium' | 'low';
  count: number;
  riskScore: number;
}

export interface RiskTrendPoint {
  timestamp: string;
  totalRisk: number;
  assetsCount: number;
  criticalFindings: number;
}

export interface ComplianceScore {
  framework: string;
  score: number;
  requirements: ComplianceRequirement[];
}

export interface ComplianceRequirement {
  id: string;
  name: string;
  status: 'compliant' | 'non_compliant' | 'unknown';
}