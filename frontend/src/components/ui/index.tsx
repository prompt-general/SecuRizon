import React from 'react';

export const Card: React.FC<{ children: React.ReactNode; className?: string }> = ({ children, className }) => (
    <div className={`bg-white shadow rounded-lg p-6 ${className || ''}`}>{children}</div>
);

export const Button: React.FC<{
    children: React.ReactNode;
    onClick?: () => void;
    variant?: 'primary' | 'outline';
    disabled?: boolean;
    fullWidth?: boolean;
}> = ({ children, onClick, variant = 'primary', disabled, fullWidth }) => {
    const baseClass = "px-4 py-2 rounded font-medium focus:outline-none";
    const variantClass = variant === 'primary'
        ? "bg-blue-600 text-white hover:bg-blue-700"
        : "border border-gray-300 text-gray-700 hover:bg-gray-50";
    const widthClass = fullWidth ? "w-full" : "";
    const disabledClass = disabled ? "opacity-50 cursor-not-allowed" : "";

    return (
        <button
            className={`${baseClass} ${variantClass} ${widthClass} ${disabledClass}`}
            onClick={onClick}
            disabled={disabled}
        >
            {children}
        </button>
    );
};

export const Table: React.FC<{ children: React.ReactNode }> = ({ children }) => (
    <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-200">{children}</table>
    </div>
);

export const Progress: React.FC<{ value: number; color?: string }> = ({ value, color = 'blue' }) => (
    <div className="w-full bg-gray-200 rounded-full h-2.5">
        <div
            className={`bg-${color}-600 h-2.5 rounded-full`}
            style={{ width: `${Math.min(value, 100)}%` }}
        ></div>
    </div>
);

export const Badge: React.FC<{ children: React.ReactNode; variant?: 'success' | 'warning' }> = ({ children, variant = 'success' }) => {
    const colorClass = variant === 'success' ? "bg-green-100 text-green-800" : "bg-yellow-100 text-yellow-800";
    return (
        <span className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${colorClass}`}>
            {children}
        </span>
    );
};

export const Modal: React.FC<{
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode
}> = ({ isOpen, onClose, title, children }) => {
    if (!isOpen) return null;
    return (
        <div className="fixed inset-0 z-50 overflow-y-auto">
            <div className="flex items-center justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:block sm:p-0">
                <div className="fixed inset-0 transition-opacity" aria-hidden="true">
                    <div className="absolute inset-0 bg-gray-500 opacity-75" onClick={onClose}></div>
                </div>
                <div className="inline-block align-bottom bg-white rounded-lg text-left overflow-hidden shadow-xl transform transition-all sm:my-8 sm:align-middle sm:max-w-lg sm:w-full">
                    <div className="bg-white px-4 pt-5 pb-4 sm:p-6 sm:pb-4">
                        <h3 className="text-lg leading-6 font-medium text-gray-900 mb-4">{title}</h3>
                        {children}
                    </div>
                </div>
            </div>
        </div>
    );
};

export const Alert: React.FC<{ variant?: 'warning'; className?: string; children: React.ReactNode }> = ({ variant, className, children }) => {
    const colorClass = variant === 'warning' ? "bg-yellow-50 border-yellow-400 text-yellow-700" : "bg-blue-50 border-blue-400 text-blue-700";
    return (
        <div className={`border-l-4 p-4 ${colorClass} ${className || ''}`}>
            <div className="flex">
                <div className="ml-3">
                    <p className="text-sm">{children}</p>
                </div>
            </div>
        </div>
    );
};
