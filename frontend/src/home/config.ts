import { parseHomeConfig } from './config-parser';

export type TotalCompHomeConfig = Partial<{
	csrfToken: string;
	usdMxnRate: string;
}>;

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
