import {
    benefitBanxicoDisplay,
    benefitContext,
    benefitMarkup,
    savedBenefitFromInput,
    type SavedBenefit
} from './benefits';
import { clearAllInputs } from './clear-form';
import {
    initializeComparisonMode,
    setComparisonMode,
    setupComparisonResizeHandler
} from './comparison-controller';
import { attachCommaFormatting, formatNumber, stripCommas } from './money-formatting';
import { hoursDisplay, isSalaryBlockedFrequency, salaryLabelText } from './package-rules';

interface RegimeControls {
    benefitsSection: HTMLElement | null;
    currencySelection: HTMLElement | null;
    paymentFreqSelect: HTMLSelectElement | null;
    unpaidVacationDiv: HTMLElement | null;
    currencySelect: HTMLSelectElement | null;
    exchangeRateDiv: HTMLElement | null;
    exchangeRateDisplay: HTMLElement | null;
}

interface BenefitInputControls {
    typeSelect: HTMLSelectElement;
    amountInput: HTMLInputElement;
    percentageLabel: HTMLElement | null;
    currencySelect: HTMLSelectElement | null;
    cadenceSelect: HTMLSelectElement | null;
    cadenceHidden: HTMLInputElement | null;
    percentageCadenceLabel: HTMLElement | null;
    banxicoNotice: HTMLElement | null;
}

interface BenefitNoticeControls {
    currencySelect: HTMLSelectElement | null;
    typeSelect: HTMLSelectElement;
    banxicoNotice: HTMLElement | null;
}

const benefitCounters: number[] = [0, 0];

export function toggleRegime(select: HTMLSelectElement, index: number): void {
    const controls = regimeControls(index);
    const paymentFreqSelect = controls.paymentFreqSelect;
    if (!paymentFreqSelect) return;

    const currentFreq = paymentFreqSelect.value;
    if (select.value === 'resico') {
        applyResicoRegime(controls, paymentFreqSelect);
    } else {
        applySalaryRegime(controls, paymentFreqSelect, currentFreq);
    }

    restorePaymentFrequency(paymentFreqSelect, currentFreq);
    toggleSalaryLabel(paymentFreqSelect, index);
}

export function regimeControls(index: number): RegimeControls {
    return {
        benefitsSection: document.querySelector<HTMLElement>(`.benefits-section-${index}`),
        currencySelection: document.querySelector<HTMLElement>(`.currency-selection-${index}`),
        paymentFreqSelect: document.querySelector<HTMLSelectElement>(`.payment-frequency-select-${index}`),
        unpaidVacationDiv: document.querySelector<HTMLElement>(`.unpaid-vacation-${index}`),
        currencySelect: document.querySelectorAll<HTMLSelectElement>('select[name="Currency[]"]')[index] ?? null,
        exchangeRateDiv: document.querySelector<HTMLElement>(`.exchange-rate-input-${index}`),
        exchangeRateDisplay: document.querySelector<HTMLElement>(`.exchange-rate-display-${index}`)
    };
}

export function applyResicoRegime(controls: RegimeControls, paymentFreqSelect: HTMLSelectElement): void {
    setDisplay(controls.benefitsSection, 'none');
    setDisplay(controls.currencySelection, 'block');
    setDisplay(controls.unpaidVacationDiv, 'block');
    setBenefitCheckboxes(controls.benefitsSection, false);
    Array.from(paymentFreqSelect.options).forEach(enablePaymentOption);
}

export function applySalaryRegime(
    controls: RegimeControls,
    paymentFreqSelect: HTMLSelectElement,
    currentFreq: string
): void {
    setDisplay(controls.benefitsSection, 'block');
    setDisplay(controls.currencySelection, 'none');
    setDisplay(controls.unpaidVacationDiv, 'none');
    setBenefitCheckboxes(controls.benefitsSection, true);
    setCurrencyToMXN(controls.currencySelect);
    setDisplay(controls.exchangeRateDiv, 'none');
    setDisplay(controls.exchangeRateDisplay, 'none');
    Array.from(paymentFreqSelect.options).forEach(setSalaryPaymentOption);
    resetInvalidSalaryFrequency(paymentFreqSelect, currentFreq);
}

export function setDisplay(element: HTMLElement | null, displayValue: string): void {
    if (element) {
        element.style.display = displayValue;
    }
}

export function setBenefitCheckboxes(container: HTMLElement | null, checked: boolean): void {
    if (!container) return;

    container.querySelectorAll<HTMLInputElement>('input[type="checkbox"]').forEach(checkbox => {
        checkbox.checked = checked;
    });
}

export function setCurrencyToMXN(currencySelect: HTMLSelectElement | null): void {
    if (currencySelect) {
        currencySelect.value = 'MXN';
    }
}

export function enablePaymentOption(option: HTMLOptionElement): void {
    option.style.display = '';
    option.disabled = false;
}

export function setSalaryPaymentOption(option: HTMLOptionElement): void {
    if (isSalaryBlockedFrequency(option.value)) {
        option.style.display = 'none';
        option.disabled = true;
        return;
    }

    enablePaymentOption(option);
}

export function resetInvalidSalaryFrequency(paymentFreqSelect: HTMLSelectElement, currentFreq: string): void {
    if (isSalaryBlockedFrequency(currentFreq)) {
        paymentFreqSelect.value = 'monthly';
    }
}

export function restorePaymentFrequency(paymentFreqSelect: HTMLSelectElement, currentFreq: string): void {
    if (paymentFreqSelect.value === currentFreq) return;
    if (paymentOptionEnabled(paymentFreqSelect, currentFreq)) {
        paymentFreqSelect.value = currentFreq;
    }
}

export function paymentOptionEnabled(paymentFreqSelect: HTMLSelectElement, currentFreq: string): boolean {
    return Array.from(paymentFreqSelect.options).some(option => {
        return option.value === currentFreq && !option.disabled;
    });
}

export function toggleExchangeRate(select: HTMLSelectElement, index: number): void {
    const exchangeRateInput = document.querySelector<HTMLElement>(`.exchange-rate-input-${index}`);
    setDisplay(exchangeRateInput, select.value === 'USD' ? 'block' : 'none');
}

export function toggleSalaryLabel(select: HTMLSelectElement, index: number): void {
    const salaryLabel = document.querySelector<HTMLElement>(`.salary-label-${index}`);
    const hoursPerWeek = document.querySelector<HTMLElement>(`.hours-per-week-${index}`);
    if (!salaryLabel || !hoursPerWeek) return;

    salaryLabel.textContent = salaryLabelText(select.value);
    hoursPerWeek.style.display = hoursDisplay(select.value);
}

export function addBenefit(packageIndex: number, savedBenefit: SavedBenefit | null = null): void {
    benefitCounters[packageIndex] = (benefitCounters[packageIndex] ?? 0) + 1;

    const context = benefitContext(packageIndex, benefitCounters[packageIndex], savedBenefit);
    const benefitDiv = newBenefitDiv(context.benefitId);
    const container = document.getElementById(`otherBenefits-${packageIndex}`);
    if (!container) return;

    benefitDiv.innerHTML = benefitMarkup(context);
    container.appendChild(benefitDiv);
    attachCommaFormatting(benefitDiv.querySelectorAll<HTMLInputElement>('.money-input'));
}

export function newBenefitDiv(benefitId: string): HTMLDivElement {
    const benefitDiv = document.createElement('div');
    benefitDiv.id = benefitId;
    benefitDiv.style.cssText = 'display: flex; flex-direction: column; gap: 0; background: white; padding: 0.5rem; border-radius: 6px; border: 1px solid #e2e8f0;';
    return benefitDiv;
}

export function removeBenefit(benefitId: string | null): void {
    if (!benefitId) return;

    const element = document.getElementById(benefitId);
    if (element) {
        element.remove();
    }
}

export function toggleBenefitInputType(benefitId: string | null): void {
    const controls = benefitInputControls(benefitId);
    if (!controls) return;

    controls.amountInput.value = '';
    const isPercentage = controls.typeSelect.value === 'percentage';
    if (isPercentage) {
        applyPercentageBenefitInput(controls);
        return;
    }

    applyFixedBenefitInput(controls);
}

export function benefitInputControls(benefitId: string | null): BenefitInputControls | null {
    if (!benefitId) return null;

    const benefitDiv = document.getElementById(benefitId);
    if (!benefitDiv) return null;

    const typeSelect = benefitDiv.querySelector<HTMLSelectElement>('.benefit-type-select');
    const amountInput = benefitDiv.querySelector<HTMLInputElement>('.benefit-amount-input');
    if (!typeSelect || !amountInput) return null;

    return {
        typeSelect,
        amountInput,
        percentageLabel: benefitDiv.querySelector<HTMLElement>('.percentage-label'),
        currencySelect: benefitDiv.querySelector<HTMLSelectElement>('.benefit-currency-select'),
        cadenceSelect: benefitDiv.querySelector<HTMLSelectElement>('.benefit-cadence-select'),
        cadenceHidden: benefitDiv.querySelector<HTMLInputElement>('.benefit-cadence-hidden'),
        percentageCadenceLabel: benefitDiv.querySelector<HTMLElement>('.percentage-cadence-label'),
        banxicoNotice: document.getElementById(`banxico-notice-${benefitId}`)
    };
}

export function applyPercentageBenefitInput(controls: BenefitInputControls): void {
    controls.amountInput.placeholder = '10';
    setDisplay(controls.percentageLabel, 'inline');
    setDisplay(controls.currencySelect, 'none');
    setDisplay(controls.cadenceSelect, 'none');
    setDisplay(controls.cadenceHidden, 'inline');
    setDisplay(controls.percentageCadenceLabel, 'inline');
    setDisplay(controls.banxicoNotice, 'none');
    controls.amountInput.classList.remove('money-input');
}

export function applyFixedBenefitInput(controls: BenefitInputControls): void {
    controls.amountInput.placeholder = '$1,500';
    setDisplay(controls.percentageLabel, 'none');
    setDisplay(controls.currencySelect, 'block');
    setDisplay(controls.cadenceSelect, 'block');
    setDisplay(controls.cadenceHidden, 'none');
    setDisplay(controls.percentageCadenceLabel, 'none');
    controls.amountInput.classList.add('money-input');
    attachCommaFormatting([controls.amountInput]);
    updateBanxicoNotice(controls);
}

export function toggleBenefitBanxicoNotice(benefitId: string | null): void {
    const controls = benefitBanxicoControls(benefitId);
    if (!controls) return;
    updateBanxicoNotice(controls);
}

export function benefitBanxicoControls(benefitId: string | null): BenefitNoticeControls | null {
    if (!benefitId) return null;

    const benefitDiv = document.getElementById(benefitId);
    if (!benefitDiv) return null;

    const currencySelect = benefitDiv.querySelector<HTMLSelectElement>('.benefit-currency-select');
    const typeSelect = benefitDiv.querySelector<HTMLSelectElement>('.benefit-type-select');
    if (!currencySelect || !typeSelect) return null;

    return {
        currencySelect,
        typeSelect,
        banxicoNotice: document.getElementById(`banxico-notice-${benefitId}`)
    };
}

export function updateBanxicoNotice(controls: BenefitNoticeControls): void {
    setDisplay(controls.banxicoNotice, benefitBanxicoNoticeDisplay(controls));
}

export function benefitBanxicoNoticeDisplay(controls: BenefitNoticeControls): string {
    if (!controls.currencySelect) return 'none';
    return benefitBanxicoDisplay(controls.currencySelect.value, controls.typeSelect.value !== 'fixed');
}

export function loadSavedValues(): void {
    [0, 1].forEach(loadSavedPackageValues);
}

export function loadSavedPackageValues(idx: number): void {
    const packageDiv = document.querySelector<HTMLElement>(`[data-package-index="${idx}"]`);
    if (!packageDiv) return;

    loadSavedPackageName(packageDiv, idx);
    loadSavedBasicValues(packageDiv, idx);
    loadSavedEquity(packageDiv, idx);
    loadSavedSalary(packageDiv, idx);
    loadSavedBenefitCheckboxes(idx);
    loadSavedOtherBenefits(idx);
}

export function savedValue(id: string): string {
    const element = document.getElementById(id);
    if (element instanceof HTMLInputElement || element instanceof HTMLSelectElement) {
        return element.value;
    }

    return '';
}

export function savedTrue(id: string): boolean {
    return savedValue(id) === 'true';
}

export function setPackageValue(packageDiv: ParentNode, selector: string, value: string): void {
    if (!value) return;

    const input = packageDiv.querySelector<HTMLInputElement | HTMLSelectElement>(selector);
    if (input) input.value = value;
}

export function setPackageFormattedValue(packageDiv: ParentNode, selector: string, value: string): void {
    if (!value) return;
    setPackageValue(packageDiv, selector, formatNumber(value));
}

export function setIndexedValue(name: string, index: number, value: string): void {
    if (!value) return;

    const input = document.querySelectorAll<HTMLInputElement>(`input[name="${name}"]`)[index];
    if (input) input.value = value;
}

export function setIndexedFormattedValue(name: string, index: number, value: string): void {
    if (!value) return;
    setIndexedValue(name, index, formatNumber(value));
}

export function loadSavedPackageName(packageDiv: ParentNode, idx: number): void {
    setPackageValue(packageDiv, '.package-name-input', savedValue(`saved-pkg-${idx}-name`));
}

export function loadSavedBasicValues(packageDiv: ParentNode, idx: number): void {
    setPackageValue(packageDiv, '.regime-select', savedValue(`saved-pkg-${idx}-regime`));
    loadSavedCurrency(packageDiv, idx);
    loadSavedPaymentFrequency(packageDiv, idx);
    setPackageValue(packageDiv, `input[name="HoursPerWeek[]"]`, savedValue(`saved-pkg-${idx}-hours`));
    setPackageValue(packageDiv, `input[name="UnpaidVacationDays[]"]`, savedValue(`saved-pkg-${idx}-unpaid-vacation`));
}

export function loadSavedCurrency(packageDiv: ParentNode, idx: number): void {
    const currency = savedValue(`saved-pkg-${idx}-currency`);
    if (!currency) return;

    setPackageValue(packageDiv, `select[name="Currency[]"]`, currency);
    if (currency === 'USD') {
        loadSavedExchangeRate(packageDiv, idx);
    }
}

export function loadSavedExchangeRate(packageDiv: ParentNode, idx: number): void {
    setDisplay(document.querySelector<HTMLElement>(`.exchange-rate-input-${idx}`), 'block');
    setPackageValue(packageDiv, `input[name="ExchangeRate[]"]`, savedValue(`saved-pkg-${idx}-exchange-rate`));
}

export function loadSavedPaymentFrequency(packageDiv: ParentNode, idx: number): void {
    const paymentFrequency = savedValue(`saved-pkg-${idx}-payment-freq`);
    if (!paymentFrequency) return;

    const freqSelect = packageDiv.querySelector<HTMLSelectElement>(`.payment-frequency-select-${idx}`);
    if (!freqSelect) return;

    freqSelect.value = paymentFrequency;
    toggleSalaryLabel(freqSelect, idx);
}

export function loadSavedEquity(packageDiv: ParentNode, idx: number): void {
    if (!savedTrue(`saved-pkg-${idx}-has-equity`)) return;

    loadSavedEquityToggle(packageDiv, idx);
    setPackageFormattedValue(packageDiv, `input[name="InitialEquityUSD[]"]`, savedValue(`saved-pkg-${idx}-initial-equity`));
    loadSavedRefreshers(packageDiv, idx);
}

export function loadSavedEquityToggle(packageDiv: ParentNode, idx: number): void {
    const equityCheckbox = packageDiv.querySelector<HTMLInputElement>(`.equity-toggle-checkbox[data-package-index="${idx}"]`);
    if (!equityCheckbox) return;

    equityCheckbox.checked = true;
    toggleEquitySection(equityCheckbox);
}

export function loadSavedRefreshers(packageDiv: ParentNode, idx: number): void {
    if (!savedTrue(`saved-pkg-${idx}-has-refreshers`)) return;

    loadSavedRefresherToggle(packageDiv, idx);
    setIndexedFormattedValue('RefresherMinUSD[]', idx, savedValue(`saved-pkg-${idx}-refresher-min`));
    setIndexedFormattedValue('RefresherMaxUSD[]', idx, savedValue(`saved-pkg-${idx}-refresher-max`));
}

export function loadSavedRefresherToggle(packageDiv: ParentNode, idx: number): void {
    const refresherCheckbox = packageDiv.querySelector<HTMLInputElement>('.refresher-checkbox');
    if (!refresherCheckbox) return;

    refresherCheckbox.checked = true;
    toggleRefresherFields(refresherCheckbox, idx);
}

export function loadSavedSalary(packageDiv: ParentNode, idx: number): void {
    setPackageFormattedValue(packageDiv, '.salary-input', savedValue(`saved-pkg-${idx}-salary`));
}

export function loadSavedBenefitCheckboxes(idx: number): void {
    loadSavedAguinaldo(idx);
    loadSavedVales(idx);
    loadSavedPrima(idx);
    loadSavedFondo(idx);
}

export function loadSavedAguinaldo(idx: number): void {
    if (!savedTrue(`saved-pkg-${idx}-has-aguinaldo`)) return;

    checkFirst(`input[name="HasAguinaldo[]"][value="${idx}"]`);
    setIndexedValue('AguinaldoDays[]', idx, savedValue(`saved-pkg-${idx}-aguinaldo-days`));
}

export function loadSavedVales(idx: number): void {
    if (!savedTrue(`saved-pkg-${idx}-has-vales`)) return;

    checkFirst(`input[name="HasValesDespensa[]"][value="${idx}"]`);
    setIndexedFormattedValue('ValesDespensaAmount[]', idx, savedValue(`saved-pkg-${idx}-vales-amount`));
}

export function loadSavedPrima(idx: number): void {
    if (!savedTrue(`saved-pkg-${idx}-has-prima`)) return;

    checkFirst(`input[name="HasPrimaVacacional[]"][value="${idx}"]`);
    setIndexedValue('VacationDays[]', idx, savedValue(`saved-pkg-${idx}-vacation-days`));
    setIndexedValue('PrimaVacacionalPercent[]', idx, savedValue(`saved-pkg-${idx}-prima-percent`));
}

export function loadSavedFondo(idx: number): void {
    if (!savedTrue(`saved-pkg-${idx}-has-fondo`)) return;

    checkFirst(`input[name="HasFondoAhorro[]"][value="${idx}"]`);
    setIndexedValue('FondoAhorroPercent[]', idx, savedValue(`saved-pkg-${idx}-fondo-percent`));
}

export function checkFirst(selector: string): void {
    const checkbox = document.querySelector<HTMLInputElement>(selector);
    if (checkbox) {
        checkbox.checked = true;
    }
}

export function loadSavedOtherBenefits(idx: number): void {
    document.querySelectorAll<HTMLElement>(`.saved-other-benefit-${idx}`).forEach(benefitInput => {
        addBenefit(idx, savedBenefitFromInput(benefitInput));
    });
}

export function toggleRefresherFields(checkbox: HTMLInputElement, index: number): void {
    const refresherFields = document.querySelector<HTMLElement>(`.refresher-fields-${index}`);
    if (refresherFields) {
        refresherFields.style.display = checkbox.checked ? 'block' : 'none';
    }
}

export function toggleEquitySection(checkbox: HTMLInputElement): void {
    const packageIndex = checkbox.getAttribute('data-package-index');
    if (packageIndex === null) return;

    const equitySection = document.querySelector<HTMLElement>(`.equity-section-${packageIndex}`);
    if (equitySection) {
        equitySection.style.display = checkbox.checked ? 'block' : 'none';
    }
}

export function setupHomeActions(): void {
    document.querySelectorAll<HTMLSelectElement>('.regime-select').forEach(select => {
        select.addEventListener('change', () => {
            toggleRegime(select, packageIndexForElement(select));
        });
    });

    document.querySelectorAll<HTMLSelectElement>('.currency-select').forEach(select => {
        select.addEventListener('change', () => {
            toggleExchangeRate(select, packageIndexForElement(select));
        });
    });

    document.querySelectorAll<HTMLSelectElement>('select[name="PaymentFrequency[]"]').forEach(select => {
        select.addEventListener('change', () => {
            toggleSalaryLabel(select, packageIndexForElement(select));
        });
    });

    document.querySelectorAll<HTMLButtonElement>('[data-action="add-benefit"]').forEach(button => {
        button.addEventListener('click', () => {
            addBenefit(Number(button.getAttribute('data-package-index')));
        });
    });

    addClickHandler('[data-action="add-comparison"]', () => {
        setComparisonMode(true);
    });
    addClickHandler('[data-action="remove-comparison"]', () => {
        setComparisonMode(false);
    });
    addClickHandler('#clearInputsButton', () => {
        clearAllInputs();
    });

    document.addEventListener('change', handleDynamicBenefitChange);
    document.addEventListener('click', handleDynamicBenefitClick);
}

export function addClickHandler(selector: string, handler: (event: MouseEvent) => void): void {
    const element = document.querySelector<HTMLElement>(selector);
    if (!element) return;

    element.addEventListener('click', handler);
}

export function packageIndexForElement(element: Element): number {
    const packageDiv = element.closest<HTMLElement>('[data-package-index]');
    if (!packageDiv) return 0;

    return Number(packageDiv.getAttribute('data-package-index'));
}

export function handleDynamicBenefitChange(event: Event): void {
    const target = event.target;
    if (!(target instanceof Element)) return;

    if (target.matches('.benefit-type-select')) {
        toggleBenefitInputType(target.getAttribute('data-benefit-id'));
    }
    if (target.matches('.benefit-currency-select')) {
        toggleBenefitBanxicoNotice(target.getAttribute('data-benefit-id'));
    }
}

export function handleDynamicBenefitClick(event: MouseEvent): void {
    const target = event.target;
    if (!(target instanceof Element)) return;

    const button = target.closest<HTMLElement>('[data-action="remove-benefit"]');
    if (!button) return;

    removeBenefit(button.getAttribute('data-benefit-id'));
}

export function handleSubmit(): void {
    document.querySelectorAll<HTMLInputElement>('input[type="text"], input[type="number"]').forEach(input => {
        if (input.value) {
            input.value = stripCommas(input.value);
        }
    });

    document.querySelectorAll<HTMLSelectElement>('.regime-select').forEach((regimeSelect, idx) => {
        if (regimeSelect.value !== 'sueldos_salarios') return;

        const currencySelect = document.querySelectorAll<HTMLSelectElement>('select[name="Currency[]"]')[idx];
        if (currencySelect) {
            currencySelect.value = 'MXN';
        }
    });
}

export function hasSavedPackage2Data(): boolean {
    if (hasSavedInputValue('saved-pkg-1-salary')) return true;
    return hasSavedPackage2Name();
}

export function hasSavedInputValue(id: string): boolean {
    return savedValue(id) !== '';
}

export function hasSavedPackage2Name(): boolean {
    const value = savedValue('saved-pkg-1-name');
    if (value === '') return false;
    return value !== 'Paquete 2';
}

setupComparisonResizeHandler();

document.addEventListener('DOMContentLoaded', () => {
    setupHomeActions();

    document.querySelectorAll<HTMLInputElement>('.equity-toggle-checkbox').forEach(checkbox => {
        checkbox.addEventListener('change', () => {
            toggleEquitySection(checkbox);
        });
    });

    document.querySelectorAll<HTMLInputElement>('.refresher-checkbox').forEach((checkbox, index) => {
        checkbox.addEventListener('change', () => {
            toggleRefresherFields(checkbox, index);
        });
    });

    loadSavedValues();

    document.querySelectorAll<HTMLSelectElement>('.regime-select').forEach((select, index) => {
        toggleRegime(select, index);
    });

    attachCommaFormatting(document.querySelectorAll<HTMLInputElement>('.salary-input, .money-input'));

    const form = document.querySelector<HTMLFormElement>('form');
    if (form) {
        form.addEventListener('submit', handleSubmit);
    }

    initializeComparisonMode(hasSavedPackage2Data);
});
