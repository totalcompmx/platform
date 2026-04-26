export interface TotalCompHomeConfig {
    csrfToken?: string;
    usdMxnRate?: string;
}

export function getHomeConfig(): TotalCompHomeConfig {
    if (typeof window === 'undefined') {
        return {};
    }

    return window.TotalCompHome ?? {};
}

export function homeCSRFToken(config: TotalCompHomeConfig = getHomeConfig()): string {
    return config.csrfToken ?? '';
}

export function homeUSDMXNRate(config: TotalCompHomeConfig = getHomeConfig()): string {
    return config.usdMxnRate ?? '20.00';
}

export function homeUSDMXNLabel(config: TotalCompHomeConfig = getHomeConfig()): string {
    return `$${homeUSDMXNRate(config)}`;
}
