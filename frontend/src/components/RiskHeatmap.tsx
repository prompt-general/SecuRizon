import React from 'react';
import { RiskHeatmapCell } from '../types/dashboard';

interface RiskHeatmapProps {
  data: RiskHeatmapCell[];
  onCellClick: (provider: string, severity: string) => void;
}

const RiskHeatmap: React.FC<RiskHeatmapProps> = ({ data, onCellClick }) => {
  const providers = ['AWS', 'Azure', 'GCP'];
  const severities = ['critical', 'high', 'medium', 'low'];

  const getSeverityColor = (severity: string, count: number) => {
    const baseColors = {
      critical: 'bg-red-600',
      high: 'bg-orange-500',
      medium: 'bg-yellow-500',
      low: 'bg-green-500',
    };
    return count === 0 ? 'bg-gray-100' : baseColors[severity as keyof typeof baseColors];
  };

  return (
    <div className="overflow-x-auto">
      <table className="min-w-full">
        <thead>
          <tr>
            <th className="p-2 text-left text-sm font-semibold">Provider</th>
            {severities.map((severity) => (
              <th key={severity} className="p-2 text-center text-sm font-semibold capitalize">
                {severity}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {providers.map((provider) => (
            <tr key={provider}>
              <td className="p-2 font-medium">{provider}</td>
              {severities.map((severity) => {
                const cell = data.find((c) => c.provider === provider && c.severity === severity);
                return (
                  <td
                    key={`${provider}-${severity}`}
                    className={`p-2 cursor-pointer ${getSeverityColor(
                      severity,
                      cell?.count || 0
                    )} text-white text-center font-bold rounded`}
                    onClick={() => onCellClick(provider, severity)}
                  >
                    {cell?.count || 0}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};

export default RiskHeatmap;