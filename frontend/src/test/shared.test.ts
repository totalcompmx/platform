import { beforeEach, expect, test, vi } from 'vitest';
import { initializeConfirmSubmit } from '../shared/confirm-submit';
import { initializeCookieConsent } from '../shared/cookie-consent';

beforeEach(() => {
    document.body.innerHTML = '';
    window.localStorage.clear();
    vi.restoreAllMocks();
});

function cookieFixture(): HTMLElement {
    document.body.innerHTML = `
        <div data-cookie-banner hidden>
            <button type="button" data-action="accept-cookies">Accept</button>
        </div>
    `;

    const banner = document.querySelector<HTMLElement>('[data-cookie-banner]');
    if (!banner) {
        throw new Error('Missing cookie banner fixture');
    }

    return banner;
}

function testDocument(markup: string): Document {
    const documentRef = document.implementation.createHTMLDocument('test');
    documentRef.body.innerHTML = markup;
    return documentRef;
}

function click(element: Element): boolean {
    return element.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
}

function submit(submitter?: HTMLElement): SubmitEvent {
    const event = new Event('submit', { bubbles: true, cancelable: true }) as SubmitEvent;
    Object.defineProperty(event, 'submitter', { value: submitter ?? null });
    return event;
}

test('cookie banner appears when localStorage is empty', () => {
    const banner = cookieFixture();

    initializeCookieConsent();

    expect(banner.hidden).toBe(false);
});

test('cookie banner stays hidden when cookies were already accepted', () => {
    window.localStorage.setItem('cookies_accepted', 'true');
    const banner = cookieFixture();

    initializeCookieConsent();

    expect(banner.hidden).toBe(true);
});

test('cookie accept stores the value and hides the banner', () => {
    const banner = cookieFixture();
    initializeCookieConsent();

    document.querySelector<HTMLButtonElement>('[data-action="accept-cookies"]')?.click();

    expect(window.localStorage.getItem('cookies_accepted')).toBe('true');
    expect(banner.hidden).toBe(true);
});

test('cookie consent safely ignores missing banner or button', () => {
    document.body.innerHTML = '<button type="button" data-action="accept-cookies">Accept</button>';
    expect(() => initializeCookieConsent()).not.toThrow();

    document.body.innerHTML = '<div data-cookie-banner hidden></div>';
    expect(() => initializeCookieConsent()).not.toThrow();
});

test('confirm accepts and rejects regular clicks', () => {
    const documentRef = testDocument('<button type="button" data-confirm="Continue?"><span>Go</span></button>');
    const confirmFn = vi.fn(() => true);
    initializeConfirmSubmit({ confirmFn, documentRef });

    expect(click(documentRef.querySelector('span') as Element)).toBe(true);
    expect(confirmFn).toHaveBeenCalledWith('Continue?');

    confirmFn.mockReturnValue(false);
    expect(click(documentRef.querySelector('span') as Element)).toBe(false);
});

test('confirm defaults to the browser document and window confirmation', () => {
    document.body.innerHTML = '<button type="button" data-confirm="Default?">Default</button>';
    const confirmFn = vi.fn(() => true);
    Object.defineProperty(window, 'confirm', {
        configurable: true,
        value: confirmFn
    });
    initializeConfirmSubmit();

    expect(click(document.querySelector('button') as HTMLButtonElement)).toBe(true);
    expect(confirmFn).toHaveBeenCalledWith('Default?');
    Reflect.deleteProperty(window, 'confirm');
});

test('confirm accepts and rejects clicks on non-control elements', () => {
    const documentRef = testDocument('<div data-confirm="Open?"><span>Open</span></div>');
    const confirmFn = vi.fn(() => false);
    initializeConfirmSubmit({ confirmFn, documentRef });

    expect(click(documentRef.querySelector('span') as Element)).toBe(false);
    expect(confirmFn).toHaveBeenCalledWith('Open?');
});

test('confirm accepts and rejects submit events', () => {
    const documentRef = testDocument(`
        <form>
            <button type="submit" data-confirm="Regenerate?">Regenerate</button>
        </form>
    `);
    const form = documentRef.querySelector('form') as HTMLFormElement;
    const button = documentRef.querySelector('button') as HTMLButtonElement;
    const confirmFn = vi.fn(() => true);
    initializeConfirmSubmit({ confirmFn, documentRef });

    expect(form.dispatchEvent(submit(button))).toBe(true);
    expect(confirmFn).toHaveBeenCalledWith('Regenerate?');

    confirmFn.mockReturnValue(false);
    expect(form.dispatchEvent(submit(button))).toBe(false);
});

test('confirm uses form-level data when no submitter is available', () => {
    const documentRef = testDocument('<form data-confirm="Submit form?"></form>');
    const form = documentRef.querySelector('form') as HTMLFormElement;
    const confirmFn = vi.fn(() => true);
    initializeConfirmSubmit({ confirmFn, documentRef });

    expect(form.dispatchEvent(submit())).toBe(true);
    expect(confirmFn).toHaveBeenCalledWith('Submit form?');
});

test('confirm ignores submit events without a confirmation target', () => {
    const documentRef = testDocument('<form></form>');
    const form = documentRef.querySelector('form') as HTMLFormElement;
    const confirmFn = vi.fn(() => false);
    initializeConfirmSubmit({ confirmFn, documentRef });

    expect(form.dispatchEvent(submit())).toBe(true);
    expect(confirmFn).not.toHaveBeenCalled();
});

test('confirm ignores submit-control clicks so submit handles them once', () => {
    const documentRef = testDocument(`
        <button type="submit" data-confirm="Button submit?">Submit</button>
        <input type="submit" data-confirm="Input submit?">
        <input type="image" data-confirm="Image submit?" alt="Submit">
    `);
    const confirmFn = vi.fn(() => true);
    initializeConfirmSubmit({ confirmFn, documentRef });

    for (const control of documentRef.querySelectorAll('[data-confirm]')) {
        expect(click(control)).toBe(true);
    }
    expect(confirmFn).not.toHaveBeenCalled();
});

test('confirm ignores missing and empty confirmation messages', () => {
    const documentRef = testDocument(`
        <button type="button">Plain</button>
        <input type="button" data-confirm="">
    `);
    const confirmFn = vi.fn(() => false);
    initializeConfirmSubmit({ confirmFn, documentRef });

    expect(click(documentRef.querySelector('button') as HTMLButtonElement)).toBe(true);
    expect(click(documentRef.querySelector('input') as HTMLInputElement)).toBe(true);
    documentRef.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
    expect(confirmFn).not.toHaveBeenCalled();
});
