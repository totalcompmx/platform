import type { TotalCompHomeConfig } from './home/config';

declare global {
    interface Window {
        TotalCompHome?: TotalCompHomeConfig;
    }
}

export {};
