const COOKIE_ACCEPTED_KEY = 'cookies_accepted';

export interface CookieConsentOptions {
    documentRef?: Document;
    storage?: Storage;
}

export function initializeCookieConsent(options: CookieConsentOptions = {}): void {
    const documentRef = options.documentRef ?? document;
    const storage = options.storage ?? window.localStorage;
    const banner = documentRef.querySelector<HTMLElement>('[data-cookie-banner]');
    const acceptButton = documentRef.querySelector<HTMLButtonElement>('[data-action="accept-cookies"]');

    if (!banner || !acceptButton) {
        return;
    }

    if (storage.getItem(COOKIE_ACCEPTED_KEY) === null) {
        banner.hidden = false;
    }

    acceptButton.addEventListener('click', () => {
        storage.setItem(COOKIE_ACCEPTED_KEY, 'true');
        banner.hidden = true;
    });
}
