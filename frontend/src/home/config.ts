export interface TotalCompHomeConfig {
    csrfToken?: string;
    usdMxnRate?: string;
}

const CONFIG_SCRIPT_ID = 'totalcomp-home-config';

export function getHomeConfig(): TotalCompHomeConfig {
    if (typeof document === 'undefined') {
        return {};
    }

    const script = document.getElementById(CONFIG_SCRIPT_ID);
    if (!script) {
        return {};
    }

    return parseHomeConfig(script.textContent);
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

function parseHomeConfig(rawConfig: string | null): TotalCompHomeConfig {
    if (!rawConfig) {
        return {};
    }

    try {
        const parsedConfig: unknown = JSON.parse(rawConfig);

        if (!isRecord(parsedConfig)) {
            return {};
        }

        const config: TotalCompHomeConfig = {};

        if (typeof parsedConfig.csrfToken === 'string') {
            config.csrfToken = parsedConfig.csrfToken;
        }
        if (typeof parsedConfig.usdMxnRate === 'string') {
            config.usdMxnRate = parsedConfig.usdMxnRate;
        }

        return config;
    } catch {
        return {};
    }
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null && !Array.isArray(value);
}
