import { configFromUnknown } from './config-object';
import type { TotalCompHomeConfig } from './config';

export function parseHomeConfig(rawConfig: string | null): TotalCompHomeConfig {
	if (!rawConfig) {
		return {};
	}

	return parseHomeConfigJSON(rawConfig);
}

function parseHomeConfigJSON(rawConfig: string): TotalCompHomeConfig {
	try {
		return configFromUnknown(JSON.parse(rawConfig));
	} catch {
		return {};
	}
}
