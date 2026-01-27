import React from 'react';

interface StatCardProps {
  title: string;
  value: number;
  subValue?: string;
  trend?: 'up' | 'down' | 'stable';
  color: 'red' | 'blue' | 'orange' | 'purple';
  format?: 'number' | 'percentage';
}

const StatCard: React.FC<StatCardProps> = ({
  title,
  value,
  subValue,
  trend,
  color,
  format = 'number',
}) => {
  const colorClasses = {
    red: 'bg-red-50 border-red-200',
    blue: 'bg-blue-50 border-blue-200',
    orange: 'bg-orange-50 border-orange-200',
    purple: 'bg-purple-50 border-purple-200',
  };

  const textColorClasses = {
    red: 'text-red-700',
    blue: 'text-blue-700',
    orange: 'text-orange-700',
    purple: 'text-purple-700',
  };

  return (
    <div className={`${colorClasses[color]} border rounded-lg p-6`}>
      <p className="text-sm text-gray-600 mb-2">{title}</p>
      <div className="flex justify-between items-start">
        <div>
          <p className={`${textColorClasses[color]} text-3xl font-bold`}>
            {format === 'percentage' ? `${value}%` : value.toLocaleString()}
          </p>
          {subValue && <p className="text-xs text-gray-500 mt-1">{subValue}</p>}
        </div>
        {trend && (
          <div className={`text-lg ${trend === 'up' ? 'text-red-600' : 'text-green-600'}`}>
            {trend === 'up' ? '↑' : trend === 'down' ? '↓' : '→'}
          </div>
        )}
      </div>
    </div>
  );
};

export default StatCard;