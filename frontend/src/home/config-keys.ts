export const homeConfigKeys = ['csrfToken', 'usdMxnRate'] as const;

export type HomeConfigKey = (typeof homeConfigKeys)[number];
