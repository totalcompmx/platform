import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import {
    benefitAmountPlaceholder,
    benefitBanxicoDisplay,
    benefitContext,
    benefitFixedControlStyle,
    benefitMarkup,
    savedBenefitFromInput
} from '../home/benefits';
import { clearAllInputs, type ClearForm, type ClearFormDocument, type ClearFormInput } from '../home/clear-form';
import { homeCSRFToken, homeUSDMXNLabel, getHomeConfig } from '../home/config';
import {
    initializeComparisonMode,
    package2Visible,
    setComparisonMode,
    setupComparisonResizeHandler
} from '../home/comparison-controller';
import { attachCommaFormatting, formatNumber, stripCommas } from '../home/money-formatting';
import {
    checkedAttr,
    displayValue,
    hoursDisplay,
    isSalaryBlockedFrequency,
    salaryLabelText,
    selectedAttr
} from '../home/package-rules';

interface TestForm extends ClearForm {
    children: ClearFormInput[];
    submitted: boolean;
}

function testDocument(forms: TestForm[]): ClearFormDocument {
    function createElement(tagName: 'form'): ClearForm;
    function createElement(tagName: 'input'): ClearFormInput;
    function createElement(tagName: 'form' | 'input'): ClearForm | ClearFormInput {
        if (tagName === 'form') {
            const form: TestForm = {
                method: '',
                action: '',
                children: [],
                submitted: false,
                appendChild(child: ClearFormInput) {
                    form.children.push(child);
                },
                submit() {
                    form.submitted = true;
                }
            };

            return form;
        }

        return {
            type: '',
            name: '',
            value: ''
        };
    }

    return {
        body: {
            appendChild: element => {
                forms.push(element as TestForm);
            }
        },
        createElement
    };
}

async function loadHomeModule(): Promise<typeof import('../home/home')> {
    vi.resetModules();
    return import('../home/home');
}

function setViewport(width: number): void {
    Object.defineProperty(window, 'innerWidth', {
        configurable: true,
        value: width
    });
}

function expectElement<T extends Element>(selector: string): T {
    const element = document.querySelector<T>(selector);
    if (!element) {
        throw new Error(`Missing fixture element: ${selector}`);
    }
    return element;
}

function hiddenInput(id: string, value: string): string {
    return `<input id="${id}" type="hidden" value="${value}">`;
}

function setHomeConfig(config: unknown): void {
    setHomeConfigContent(JSON.stringify(config));
}

function setHomeConfigContent(content: string): void {
    document.body.insertAdjacentHTML(
        'beforeend',
        `<script type="application/json" id="totalcomp-home-config">${content}</script>`
    );
}

function packageFixture(index: number): string {
    return `
        <section id="package-${index + 1}" data-package-index="${index}">
            <input class="package-name-input">
            <select name="Regime[]" class="regime-select">
                <option value="sueldos_salarios">Salary</option>
                <option value="resico">RESICO</option>
            </select>
            <div class="benefits-section-${index}">
                <input type="checkbox" name="HasAguinaldo[]" value="${index}">
                <input type="checkbox" name="HasValesDespensa[]" value="${index}">
                <input type="checkbox" name="HasPrimaVacacional[]" value="${index}">
                <input type="checkbox" name="HasFondoAhorro[]" value="${index}">
            </div>
            <div class="currency-selection-${index}"></div>
            <select name="Currency[]" class="currency-select">
                <option value="MXN">MXN</option>
                <option value="USD">USD</option>
            </select>
            <div class="exchange-rate-input-${index}"></div>
            <div class="exchange-rate-display-${index}"></div>
            <select name="PaymentFrequency[]" class="payment-frequency-select-${index}">
                <option value="daily">Daily</option>
                <option value="hourly">Hourly</option>
                <option value="weekly">Weekly</option>
                <option value="biweekly">Biweekly</option>
                <option value="monthly">Monthly</option>
            </select>
            <label class="salary-label-${index}"></label>
            <div class="hours-per-week-${index}"></div>
            <div class="unpaid-vacation-${index}"></div>
            <input name="HoursPerWeek[]">
            <input name="UnpaidVacationDays[]">
            <input name="ExchangeRate[]">
            <input name="InitialEquityUSD[]" class="money-input">
            <input name="RefresherMinUSD[]" class="money-input">
            <input name="RefresherMaxUSD[]" class="money-input">
            <input name="AguinaldoDays[]">
            <input name="ValesDespensaAmount[]" class="money-input">
            <input name="VacationDays[]">
            <input name="PrimaVacacionalPercent[]">
            <input name="FondoAhorroPercent[]">
            <input type="text" class="salary-input">
            <input class="equity-toggle-checkbox" data-package-index="${index}" type="checkbox">
            <div class="equity-section-${index}"></div>
            <input class="refresher-checkbox" type="checkbox">
            <div class="refresher-fields-${index}"></div>
            <button type="button" data-action="add-benefit" data-package-index="${index}">Add</button>
            <div id="otherBenefits-${index}"></div>
            <input class="saved-other-benefit-${index}"
                data-name="Internet ${index}"
                data-amount="${1000 + index}"
                data-taxfree="true"
                data-currency="USD"
                data-cadence="annual"
                data-ispercentage="false">
        </section>
    `;
}

function savedFixture(): string {
    const values: Record<string, string> = {
        'saved-pkg-0-name': 'Paquete Uno',
        'saved-pkg-0-regime': 'resico',
        'saved-pkg-0-currency': 'USD',
        'saved-pkg-0-payment-freq': 'hourly',
        'saved-pkg-0-hours': '40',
        'saved-pkg-0-unpaid-vacation': '5',
        'saved-pkg-0-exchange-rate': '18.50',
        'saved-pkg-0-has-equity': 'true',
        'saved-pkg-0-initial-equity': '123456',
        'saved-pkg-0-has-refreshers': 'true',
        'saved-pkg-0-refresher-min': '1000',
        'saved-pkg-0-refresher-max': '2000',
        'saved-pkg-0-salary': '987654',
        'saved-pkg-0-has-aguinaldo': 'true',
        'saved-pkg-0-aguinaldo-days': '20',
        'saved-pkg-0-has-vales': 'true',
        'saved-pkg-0-vales-amount': '3000',
        'saved-pkg-0-has-prima': 'true',
        'saved-pkg-0-vacation-days': '12',
        'saved-pkg-0-prima-percent': '25',
        'saved-pkg-0-has-fondo': 'true',
        'saved-pkg-0-fondo-percent': '10',
        'saved-pkg-1-name': 'Paquete Dos',
        'saved-pkg-1-regime': 'sueldos_salarios',
        'saved-pkg-1-currency': 'MXN',
        'saved-pkg-1-payment-freq': 'monthly',
        'saved-pkg-1-hours': '',
        'saved-pkg-1-unpaid-vacation': '',
        'saved-pkg-1-exchange-rate': '',
        'saved-pkg-1-has-equity': 'false',
        'saved-pkg-1-initial-equity': '',
        'saved-pkg-1-has-refreshers': 'false',
        'saved-pkg-1-refresher-min': '',
        'saved-pkg-1-refresher-max': '',
        'saved-pkg-1-salary': '12345',
        'saved-pkg-1-has-aguinaldo': 'false',
        'saved-pkg-1-aguinaldo-days': '',
        'saved-pkg-1-has-vales': 'false',
        'saved-pkg-1-vales-amount': '',
        'saved-pkg-1-has-prima': 'false',
        'saved-pkg-1-vacation-days': '',
        'saved-pkg-1-prima-percent': '',
        'saved-pkg-1-has-fondo': 'false',
        'saved-pkg-1-fondo-percent': ''
    };

    return Object.entries(values).map(([id, value]) => hiddenInput(id, value)).join('');
}

function homeFixture(): void {
    document.body.innerHTML = `
        <form>
            <input type="text" class="money-input" value="1,234">
            <input type="number" value="5,678">
        </form>
        <div id="packagesWrapper">
            <div id="packagesGrid">
                ${packageFixture(0)}
                ${packageFixture(1)}
            </div>
            <div id="addComparisonButton">
                <button type="button">
                    <span class="btn-text-desktop"></span>
                    <span class="btn-text-mobile"></span>
                </button>
            </div>
        </div>
        <button id="submitButton" type="submit"></button>
        <button id="clearInputsButton" type="button"></button>
        <button type="button" data-action="add-comparison"></button>
        <button type="button" data-action="remove-comparison"></button>
        ${savedFixture()}
    `;
}

beforeEach(() => {
    document.body.innerHTML = '';
    setViewport(1280);
    vi.restoreAllMocks();
    vi.useRealTimers();
});

afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
});

test('uses template-provided frontend config with safe fallbacks', () => {
    expect(getHomeConfig()).toEqual({});

    setHomeConfig({
        csrfToken: 'csrf-from-template',
        usdMxnRate: '19.1234'
    });

    expect(getHomeConfig()).toEqual({
        csrfToken: 'csrf-from-template',
        usdMxnRate: '19.1234'
    });
    expect(homeCSRFToken()).toBe('csrf-from-template');
    expect(homeUSDMXNLabel()).toBe('$19.1234');
    expect(homeCSRFToken({})).toBe('');
    expect(homeUSDMXNLabel({})).toBe('$20.00');

    document.body.innerHTML = '';
    setHomeConfigContent('{');
    expect(getHomeConfig()).toEqual({});

    document.body.innerHTML = '';
    setHomeConfigContent('');
    expect(getHomeConfig()).toEqual({});

    document.body.innerHTML = '';
    setHomeConfig([]);
    expect(getHomeConfig()).toEqual({});

    document.body.innerHTML = '';
    setHomeConfigContent('null');
    expect(getHomeConfig()).toEqual({});

    document.body.innerHTML = '';
    setHomeConfig('plain-text');
    expect(getHomeConfig()).toEqual({});

    document.body.innerHTML = '';
    setHomeConfig({ csrfToken: 123, usdMxnRate: false });
    expect(getHomeConfig()).toEqual({});

    const actualDocument = globalThis.document;
    vi.stubGlobal('document', undefined);
    expect(getHomeConfig()).toEqual({});
    vi.stubGlobal('document', actualDocument);
});

test('renders benefit contexts, markup, and saved attributes', () => {
    setHomeConfig({ usdMxnRate: '18.7654' });

    const fixed = benefitContext(0, 1, {
        name: 'Internet',
        amount: '1000',
        taxFree: true,
        currency: 'USD',
        cadence: 'annual'
    });
    const percentage = benefitContext(1, 2, {
        amount: 10,
        currency: 'MXN',
        isPercentage: true
    });
    const fallback = benefitContext(0, 3);
    const markup = benefitMarkup(fixed);

    expect(markup).toMatch(/benefit-0-1/);
    expect(markup).toMatch(/Internet/);
    expect(markup).toMatch(/\$18\.7654 MXN\/USD/);
    expect(fixed.taxFreeChecked).toBe('checked');
    expect(fixed.usdSelected).toBe('selected');
    expect(fixed.annualSelected).toBe('selected');
    expect(percentage.amount).toBe('10');
    expect(percentage.percentageSelected).toBe('selected');
    expect(percentage.fixedControlStyle).toBe('display: none;');
    expect(fallback.name).toBe('');
    expect(fallback.amount).toBe('');
    expect(fallback.mxnSelected).toBe('selected');
    expect(benefitFixedControlStyle(false)).toBe('');
    expect(benefitAmountPlaceholder(true)).toBe('10');
    expect(benefitAmountPlaceholder(false)).toBe('$1,500');
    expect(benefitBanxicoDisplay('USD', false)).toBe('block');
    expect(benefitBanxicoDisplay('USD', true)).toBe('none');
    expect(benefitBanxicoDisplay('MXN', false)).toBe('none');

    const element = document.createElement('input');
    element.dataset.name = 'Food';
    element.dataset.amount = '2500';
    element.dataset.taxfree = 'true';
    element.dataset.currency = 'USD';
    element.dataset.cadence = 'annual';
    element.dataset.ispercentage = 'true';
    expect(savedBenefitFromInput(element)).toEqual({
        name: 'Food',
        amount: '2500',
        taxFree: true,
        currency: 'USD',
        cadence: 'annual',
        isPercentage: true
    });

    const blankElement = document.createElement('input');
    blankElement.dataset.currency = '';
    blankElement.dataset.cadence = '';
    expect(savedBenefitFromInput(blankElement).currency).toBe('MXN');
    expect(savedBenefitFromInput(blankElement).cadence).toBe('monthly');

    const missingElement = document.createElement('input');
    expect(savedBenefitFromInput(missingElement).currency).toBe('MXN');
    expect(savedBenefitFromInput(missingElement).cadence).toBe('monthly');
});

test('posts clear requests through injected and browser documents', () => {
    const forms: TestForm[] = [];

    clearAllInputs({
        config: { csrfToken: 'csrf-clear-token' },
        confirmFn: () => true,
        documentRef: testDocument(forms)
    });

    expect(forms.length).toBe(1);
    expect(forms[0].method).toBe('POST');
    expect(forms[0].action).toBe('/clear');
    expect(forms[0].submitted).toBe(true);
    expect(forms[0].children).toEqual([
        {
            type: 'hidden',
            name: 'csrf_token',
            value: 'csrf-clear-token'
        }
    ]);

    clearAllInputs({
        confirmFn: () => false,
        documentRef: testDocument(forms)
    });
    expect(forms.length).toBe(1);

    Object.defineProperty(window, 'confirm', {
        configurable: true,
        value: vi.fn(() => true)
    });
    setHomeConfig({ csrfToken: 'browser-token' });
    clearAllInputs();
    const browserForm = document.body.querySelector<HTMLFormElement>('form[action="/clear"]');
    expect(browserForm?.method).toBe('POST');
    expect(browserForm?.querySelector<HTMLInputElement>('input[name="csrf_token"]')?.value).toBe('browser-token');
});

test('keeps pure package and money helpers stable', () => {
    expect(formatNumber('$1234567.891')).toBe('1,234,567.89');
    expect(formatNumber('1234')).toBe('1,234');
    expect(stripCommas('1,234,567')).toBe('1234567');
    expect(salaryLabelText('hourly')).toBe('💰 Tarifa Por Hora');
    expect(salaryLabelText('daily')).toBe('💰 Salario Diario');
    expect(salaryLabelText('weekly')).toBe('💰 Salario Semanal');
    expect(salaryLabelText('biweekly')).toBe('💰 Salario Quincenal');
    expect(salaryLabelText('annual')).toBe('💰 Salario Bruto');
    expect(hoursDisplay('hourly')).toBe('block');
    expect(hoursDisplay('annual')).toBe('none');
    expect(isSalaryBlockedFrequency('daily')).toBe(true);
    expect(isSalaryBlockedFrequency('hourly')).toBe(true);
    expect(isSalaryBlockedFrequency('monthly')).toBe(false);
    expect(selectedAttr(true)).toBe('selected');
    expect(selectedAttr(false)).toBe('');
    expect(checkedAttr(true)).toBe('checked');
    expect(checkedAttr(false)).toBe('');
    expect(displayValue(true, 'yes', 'no')).toBe('yes');
    expect(displayValue(false, 'yes', 'no')).toBe('no');
});

test('formats money inputs and enforces max values', () => {
    vi.useFakeTimers();
    document.body.innerHTML = `
        <input id="plain" type="text">
        <input id="bounded" type="text" data-max="1000">
        <input id="belowMax" type="text" data-max="1000">
        <input id="notNumeric" type="text" data-max="1000">
        <input id="numberish" type="text">
    `;
    const plain = expectElement<HTMLInputElement>('#plain');
    const bounded = expectElement<HTMLInputElement>('#bounded');
    const belowMax = expectElement<HTMLInputElement>('#belowMax');
    const notNumeric = expectElement<HTMLInputElement>('#notNumeric');
    const numberish = expectElement<HTMLInputElement>('#numberish');
    Object.defineProperty(numberish, 'selectionStart', {
        configurable: true,
        value: null
    });
    const setSelectionRange = vi.fn();
    numberish.setSelectionRange = setSelectionRange;

    attachCommaFormatting([plain, bounded, belowMax, notNumeric, numberish]);

    plain.value = '1234567.891';
    plain.setSelectionRange(7, 7);
    plain.dispatchEvent(new Event('input'));
    expect(plain.value).toBe('1,234,567.89');

    bounded.value = '2000';
    bounded.dispatchEvent(new Event('input'));
    expect(bounded.value).toBe('1,000');
    expect(bounded.style.borderColor).toBe('#ef4444');
    vi.runOnlyPendingTimers();
    expect(bounded.style.borderColor).toBe('#e2e8f0');

    belowMax.value = '500';
    belowMax.dispatchEvent(new Event('input'));
    expect(belowMax.value).toBe('500');

    notNumeric.value = 'abc';
    notNumeric.dispatchEvent(new Event('input'));
    expect(notNumeric.value).toBe('');

    numberish.value = '12';
    numberish.dispatchEvent(new Event('input'));
    expect(numberish.value).toBe('12');
    expect(setSelectionRange).toHaveBeenCalledWith(2, 2);
});

test('comparison controller handles desktop, mobile, resize, and missing controls', () => {
    expect(package2Visible()).toBe(false);
    setComparisonMode(true);

    document.body.innerHTML = `
        <div id="packagesWrapper"></div>
        <div id="packagesGrid"></div>
        <div id="package-2"></div>
        <div id="addComparisonButton"><button><span class="btn-text-desktop"></span><span class="btn-text-mobile"></span></button></div>
        <button id="submitButton"></button>
    `;

    setViewport(1280);
    setComparisonMode(true);
    expect(expectElement<HTMLElement>('#package-2').style.display).toBe('block');
    expect(expectElement<HTMLElement>('#packagesWrapper').style.display).toBe('flex');
    expect(expectElement<HTMLElement>('#submitButton').textContent).toBe('💰 Comparar Paquetes');

    setComparisonMode(false);
    expect(expectElement<HTMLElement>('#addComparisonButton').style.position).toBe('absolute');
    expect(expectElement<HTMLElement>('.btn-text-desktop').style.display).toBe('inline');
    expect(expectElement<HTMLElement>('.btn-text-mobile').style.display).toBe('none');

    setViewport(390);
    setComparisonMode(true);
    expect(expectElement<HTMLElement>('#packagesWrapper').style.display).toBe('block');
    setComparisonMode(false);
    expect(expectElement<HTMLElement>('#addComparisonButton').style.position).toBe('static');
    expect(expectElement<HTMLElement>('.btn-text-desktop').style.display).toBe('none');
    expect(expectElement<HTMLElement>('.btn-text-mobile').style.display).toBe('inline');

    initializeComparisonMode(() => true);
    expect(package2Visible()).toBe(true);

    vi.useFakeTimers();
    setupComparisonResizeHandler();
    expectElement<HTMLElement>('#package-2').style.display = 'block';
    window.dispatchEvent(new Event('resize'));
    vi.advanceTimersByTime(250);
    expect(expectElement<HTMLElement>('#submitButton').textContent).toBe('💰 Comparar Paquetes');

    document.body.innerHTML = `
        <div id="packagesWrapper"></div>
        <div id="packagesGrid"></div>
        <div id="package-2"></div>
        <div id="addComparisonButton"></div>
        <button id="submitButton"></button>
    `;
    setComparisonMode(false);
    expect(expectElement<HTMLElement>('#addComparisonButton').style.display).toBe('flex');

    document.body.innerHTML = `
        <div id="packagesWrapper"></div>
        <div id="packagesGrid"></div>
        <div id="package-2"></div>
        <div id="addComparisonButton"><button></button></div>
        <button id="submitButton"></button>
    `;
    setComparisonMode(false);
    expect(expectElement<HTMLElement>('#addComparisonButton button').textContent).toBe('');
});

test('home module wires DOMContentLoaded behavior and interactive actions', async () => {
    vi.useFakeTimers();
    setViewport(1280);
    homeFixture();
    setHomeConfig({
        csrfToken: 'token',
        usdMxnRate: '17.90'
    });

    const home = await loadHomeModule();
    document.dispatchEvent(new Event('DOMContentLoaded'));

    expect(expectElement<HTMLInputElement>('.package-name-input').value).toBe('Paquete Uno');
    expect(expectElement<HTMLSelectElement>('.regime-select').value).toBe('resico');
    expect(expectElement<HTMLElement>('.currency-selection-0').style.display).toBe('block');
    expect(expectElement<HTMLElement>('.salary-label-0').textContent).toBe('💰 Tarifa Por Hora');
    expect(expectElement<HTMLElement>('.hours-per-week-0').style.display).toBe('block');
    expect(expectElement<HTMLInputElement>('input[name="InitialEquityUSD[]"]').value).toBe('123,456');
    expect(expectElement<HTMLElement>('.equity-section-0').style.display).toBe('block');
    expect(expectElement<HTMLElement>('.refresher-fields-0').style.display).toBe('block');
    expect(expectElement<HTMLElement>('#otherBenefits-0').children.length).toBe(1);
    expect(expectElement<HTMLElement>('#package-2').style.display).toBe('block');

    expectElement<HTMLButtonElement>('[data-action="remove-comparison"]').click();
    expect(expectElement<HTMLElement>('#package-2').style.display).toBe('none');
    expectElement<HTMLButtonElement>('[data-action="add-comparison"]').click();
    expect(expectElement<HTMLElement>('#package-2').style.display).toBe('block');

    const addButton = expectElement<HTMLButtonElement>('[data-action="add-benefit"][data-package-index="0"]');
    addButton.click();
    expect(expectElement<HTMLElement>('#otherBenefits-0').children.length).toBe(2);

    const latestBenefit = expectElement<HTMLElement>('#otherBenefits-0').lastElementChild as HTMLElement;
    const typeSelect = latestBenefit.querySelector<HTMLSelectElement>('.benefit-type-select');
    const amountInput = latestBenefit.querySelector<HTMLInputElement>('.benefit-amount-input');
    const currencySelect = latestBenefit.querySelector<HTMLSelectElement>('.benefit-currency-select');
    if (!typeSelect || !amountInput || !currencySelect) {
        throw new Error('Missing dynamic benefit controls');
    }

    typeSelect.value = 'percentage';
    typeSelect.dispatchEvent(new Event('change', { bubbles: true }));
    expect(amountInput.placeholder).toBe('10');
    expect(currencySelect.style.display).toBe('none');

    typeSelect.value = 'fixed';
    typeSelect.dispatchEvent(new Event('change', { bubbles: true }));
    expect(amountInput.placeholder).toBe('$1,500');
    currencySelect.value = 'USD';
    currencySelect.dispatchEvent(new Event('change', { bubbles: true }));
    expect(expectElement<HTMLElement>(`#banxico-notice-${typeSelect.dataset.benefitId}`).style.display).toBe('block');

    latestBenefit.querySelector<HTMLButtonElement>('[data-action="remove-benefit"]')?.click();
    expect(document.body.contains(latestBenefit)).toBe(false);

    const currencyPackage0 = document.querySelectorAll<HTMLSelectElement>('select[name="Currency[]"]')[0];
    currencyPackage0.value = 'USD';
    currencyPackage0.dispatchEvent(new Event('change'));
    expect(expectElement<HTMLElement>('.exchange-rate-input-0').style.display).toBe('block');
    currencyPackage0.value = 'MXN';
    currencyPackage0.dispatchEvent(new Event('change'));
    expect(expectElement<HTMLElement>('.exchange-rate-input-0').style.display).toBe('none');

    const payment = expectElement<HTMLSelectElement>('.payment-frequency-select-0');
    payment.value = 'monthly';
    payment.dispatchEvent(new Event('change'));
    expect(expectElement<HTMLElement>('.salary-label-0').textContent).toBe('💰 Salario Bruto');

    const regime = expectElement<HTMLSelectElement>('.regime-select');
    regime.value = 'sueldos_salarios';
    regime.dispatchEvent(new Event('change'));
    expect(expectElement<HTMLElement>('.benefits-section-0').style.display).toBe('block');
    expect(currencyPackage0.value).toBe('MXN');

    const equity = expectElement<HTMLInputElement>('.equity-toggle-checkbox');
    equity.checked = false;
    equity.dispatchEvent(new Event('change'));
    expect(expectElement<HTMLElement>('.equity-section-0').style.display).toBe('none');

    const refresher = expectElement<HTMLInputElement>('.refresher-checkbox');
    refresher.checked = false;
    refresher.dispatchEvent(new Event('change'));
    expect(expectElement<HTMLElement>('.refresher-fields-0').style.display).toBe('none');

    const salary = expectElement<HTMLInputElement>('.salary-input');
    salary.value = '123456';
    salary.dispatchEvent(new Event('input'));
    expect(salary.value).toBe('123,456');

    const form = expectElement<HTMLFormElement>('form');
    form.dispatchEvent(new Event('submit'));
    expect(salary.value).toBe('123456');
    expect(currencyPackage0.value).toBe('MXN');

    Object.defineProperty(window, 'confirm', {
        configurable: true,
        value: vi.fn(() => true)
    });
    expectElement<HTMLButtonElement>('#clearInputsButton').click();
    expect(document.body.querySelector<HTMLFormElement>('form[action="/clear"] input[name="csrf_token"]')?.value).toBe('token');

    expect(home.savedValue('saved-pkg-0-name')).toBe('Paquete Uno');
    expect(home.savedTrue('saved-pkg-0-has-equity')).toBe(true);
    expect(home.hasSavedPackage2Data()).toBe(true);
    expect(home.hasSavedInputValue('saved-pkg-1-salary')).toBe(true);
    expect(home.hasSavedPackage2Name()).toBe(true);

    expectElement<HTMLInputElement>('#saved-pkg-1-salary').value = '';
    expectElement<HTMLInputElement>('#saved-pkg-1-name').value = 'Custom Package';
    expect(home.hasSavedPackage2Data()).toBe(true);
    expectElement<HTMLInputElement>('#saved-pkg-1-name').value = 'Paquete 2';
    expect(home.hasSavedPackage2Data()).toBe(false);
    expectElement<HTMLInputElement>('#saved-pkg-1-name').value = '';
    expect(home.hasSavedPackage2Name()).toBe(false);
});

test('home module initializes after DOMContentLoaded and preserves typed values across UI toggles', async () => {
    const originalReadyState = document.readyState;
    Object.defineProperty(document, 'readyState', {
        configurable: true,
        value: 'complete'
    });

    homeFixture();
    const salary = expectElement<HTMLInputElement>('.salary-input');
    const packageName = expectElement<HTMLInputElement>('.package-name-input');
    const valesAmount = expectElement<HTMLInputElement>('input[name="ValesDespensaAmount[]"]');

    salary.value = '98765';
    packageName.value = 'Typed offer';
    valesAmount.value = '1234';

    await loadHomeModule();

    const benefitCountBefore = expectElement<HTMLElement>('#otherBenefits-0').children.length;
    expectElement<HTMLButtonElement>('[data-action="add-benefit"][data-package-index="0"]').click();
    expect(expectElement<HTMLElement>('#otherBenefits-0').children.length).toBe(benefitCountBefore + 1);

    expectElement<HTMLButtonElement>('[data-action="add-comparison"]').click();
    expectElement<HTMLButtonElement>('[data-action="remove-comparison"]').click();

    const regime = expectElement<HTMLSelectElement>('.regime-select');
    regime.value = 'resico';
    regime.dispatchEvent(new Event('change'));

    expect(expectElement<HTMLElement>('.currency-selection-0').style.display).toBe('block');
    expect(salary.value).toBe('98765');
    expect(packageName.value).toBe('Typed offer');
    expect(valesAmount.value).toBe('1234');

    Object.defineProperty(document, 'readyState', {
        configurable: true,
        value: originalReadyState
    });
});

test('home module tolerates partial markup during initialization', async () => {
    document.body.innerHTML = '<main></main>';

    await loadHomeModule();

    expect(document.body.textContent).toBe('');
});

test('home helpers cover defensive branches and direct state transitions', async () => {
    homeFixture();
    const home = await loadHomeModule();

    home.initializeHome();

    const originalReadyState = document.readyState;
    Object.defineProperty(document, 'readyState', {
        configurable: true,
        value: 'loading'
    });
    const addEventListener = vi.spyOn(document, 'addEventListener');
    home.initializeHomeWhenReady();
    expect(addEventListener).toHaveBeenCalledWith('DOMContentLoaded', home.initializeHome, { once: true });
    addEventListener.mockRestore();
    Object.defineProperty(document, 'readyState', {
        configurable: true,
        value: originalReadyState
    });

    home.setDisplay(null, 'block');
    home.setBenefitCheckboxes(null, true);
    home.setCurrencyToMXN(null);
    home.toggleRegime(document.createElement('select'), 9);
    home.toggleSalaryLabel(document.createElement('select'), 9);
    home.addBenefit(9);
    home.removeBenefit(null);
    home.removeBenefit('missing');
    home.toggleBenefitInputType(null);
    home.toggleBenefitInputType('missing');
    home.toggleBenefitBanxicoNotice(null);
    home.toggleBenefitBanxicoNotice('missing');
    home.loadSavedPackageValues(9);
    home.setPackageValue(document, '.missing', 'x');
    home.setPackageValue(document, '.missing', '');
    home.setPackageFormattedValue(document, '.missing', '');
    home.setIndexedValue('Missing[]', 0, 'x');
    home.setIndexedValue('Missing[]', 0, '');
    home.setIndexedFormattedValue('Missing[]', 0, '');
    home.checkFirst('.missing');
    home.toggleEquitySection(document.createElement('input'));

    expect(home.savedValue('missing')).toBe('');
    expect(home.savedTrue('missing')).toBe(false);
    expect(home.packageIndexForElement(document.createElement('div'))).toBe(0);
    home.addClickHandler('.missing-action', () => {});

    const checkboxInput = document.createElement('input');
    checkboxInput.type = 'checkbox';
    checkboxInput.defaultChecked = true;
    checkboxInput.checked = false;
    expect(home.controlHasUserValue(checkboxInput)).toBe(true);

    const radioInput = document.createElement('input');
    radioInput.type = 'radio';
    radioInput.checked = false;
    expect(home.controlHasUserValue(radioInput)).toBe(false);

    const select = document.createElement('select');
    select.innerHTML = '<option value="daily">Daily</option><option value="monthly">Monthly</option>';
    expect(home.defaultSelectValue(select)).toBe('daily');
    select.options[1].defaultSelected = true;
    expect(home.defaultSelectValue(select)).toBe('monthly');
    const emptySelect = document.createElement('select');
    expect(home.defaultSelectValue(emptySelect)).toBe('');
    const blocked = select.options[0];
    const allowed = select.options[1];
    select.value = 'daily';
    home.resetInvalidSalaryFrequency(select, 'daily');
    expect(select.value).toBe('monthly');
    select.value = 'monthly';
    home.restorePaymentFrequency(select, 'monthly');
    expect(select.value).toBe('monthly');
    blocked.disabled = true;
    home.restorePaymentFrequency(select, 'daily');
    expect(select.value).toBe('monthly');
    blocked.disabled = false;
    home.restorePaymentFrequency(select, 'daily');
    expect(select.value).toBe('daily');
    expect(home.paymentOptionEnabled(select, 'missing')).toBe(false);

    home.setSalaryPaymentOption(blocked);
    home.setSalaryPaymentOption(allowed);
    expect(blocked.disabled).toBe(true);
    expect(allowed.disabled).toBe(false);

    const packageDiv = expectElement<HTMLElement>('[data-package-index="1"]');
    home.loadSavedCurrency(packageDiv, 9);
    home.loadSavedCurrency(packageDiv, 1);
    const barePackage = document.createElement('div');
    document.body.appendChild(barePackage);
    document.body.insertAdjacentHTML('beforeend', hiddenInput('saved-pkg-2-payment-freq', 'monthly'));
    home.loadSavedPaymentFrequency(barePackage, 2);
    home.loadSavedPaymentFrequency(packageDiv, 9);
    home.loadSavedEquity(packageDiv, 1);
    home.loadSavedEquityToggle(barePackage, 2);
    home.loadSavedRefreshers(packageDiv, 1);
    home.loadSavedRefresherToggle(barePackage, 2);
    home.loadSavedAguinaldo(1);
    home.loadSavedVales(1);
    home.loadSavedPrima(1);
    home.loadSavedFondo(1);
    expectElement<HTMLElement>('#otherBenefits-1').innerHTML = '';
    home.loadSavedOtherBenefits(1);
    expect(expectElement<HTMLElement>('#otherBenefits-1').children.length).toBe(1);

    document.body.innerHTML = `
        <div id="benefit-without-input">
            <select class="benefit-type-select"></select>
        </div>
        <div id="benefit-without-select">
            <input class="benefit-amount-input">
        </div>
        <div id="benefit-without-notice">
            <select class="benefit-currency-select"><option value="USD">USD</option></select>
        </div>
    `;
    expect(home.benefitInputControls('benefit-without-input')).toBeNull();
    expect(home.benefitInputControls('benefit-without-select')).toBeNull();
    expect(home.benefitBanxicoControls('benefit-without-notice')).toBeNull();
    expect(home.benefitBanxicoNoticeDisplay({
        currencySelect: null,
        typeSelect: document.createElement('select'),
        banxicoNotice: null
    })).toBe('none');

    document.body.innerHTML = `
        <div id="minimal-benefit">
            <select class="benefit-type-select"><option value="fixed">fixed</option></select>
            <input class="benefit-amount-input">
        </div>
    `;
    const minimalControls = home.benefitInputControls('minimal-benefit');
    expect(minimalControls?.currencySelect).toBeNull();
    if (!minimalControls) {
        throw new Error('Missing minimal benefit controls');
    }
    home.applyPercentageBenefitInput(minimalControls);
    home.applyFixedBenefitInput(minimalControls);
    minimalControls.typeSelect.innerHTML = '<option value="percentage">percentage</option>';
    minimalControls.typeSelect.value = 'percentage';
    home.toggleBenefitInputType('minimal-benefit');

    const checkbox = document.createElement('input');
    checkbox.type = 'checkbox';
    checkbox.checked = true;
    home.toggleRefresherFields(checkbox, 99);
    checkbox.setAttribute('data-package-index', '99');
    home.toggleEquitySection(checkbox);

    document.body.innerHTML = `
        <input type="text" value="">
        <select class="regime-select"><option value="sueldos_salarios" selected>Salary</option></select>
    `;
    home.handleSubmit();
    expect(expectElement<HTMLInputElement>('input').value).toBe('');

    document.body.innerHTML = `
        <input type="text" value="1,000">
        <select class="regime-select"><option value="resico" selected>RESICO</option></select>
    `;
    home.handleSubmit();
    expect(expectElement<HTMLInputElement>('input').value).toBe('1000');

    document.body.innerHTML = '';
    document.dispatchEvent(new Event('DOMContentLoaded'));

    const eventWithoutElement = new Event('change');
    Object.defineProperty(eventWithoutElement, 'target', { value: null });
    home.handleDynamicBenefitChange(eventWithoutElement);

    const clickWithoutElement = new MouseEvent('click');
    Object.defineProperty(clickWithoutElement, 'target', { value: null });
    home.handleDynamicBenefitClick(clickWithoutElement);

    const unrelated = document.createElement('div');
    home.handleDynamicBenefitChange(new Event('change'));
    unrelated.dispatchEvent(new MouseEvent('click', { bubbles: true }));
});
