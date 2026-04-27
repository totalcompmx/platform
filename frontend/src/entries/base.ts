import 'vite/modulepreload-polyfill';
import { initializeConfirmSubmit } from '../shared/confirm-submit';
import { initializeCookieConsent } from '../shared/cookie-consent';

initializeCookieConsent();
initializeConfirmSubmit();
