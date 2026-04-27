import type { TotalCompHomeConfig } from './config';
import { setStringConfigValue } from './config-field';
import { homeConfigKeys } from './config-keys';
import { isRecord } from './config-types';

export function configFromUnknown(value: unknown): TotalCompHomeConfig {
	if (!isRecord(value)) {
		return {};
	}

	const config: TotalCompHomeConfig = {};
	for (const key of homeConfigKeys) {
		setStringConfigValue(config, value, key);
	}
	return config;
}
