import type { TotalCompHomeConfig } from './config';
import type { HomeConfigKey } from './config-keys';

export function setStringConfigValue(
	config: TotalCompHomeConfig,
	value: Record<string, unknown>,
	key: HomeConfigKey
): void {
	if (typeof value[key] === 'string') {
		config[key] = value[key];
	}
}
