import { homeUSDMXNLabel } from './config';
import { formatNumber } from './money-formatting';
import { checkedAttr, displayValue, selectedAttr } from './package-rules';

export type SavedBenefit = Partial<{
	name: string | null;
	amount: string | number | null;
	taxFree: boolean;
	currency: string | null;
	cadence: string | null;
	isPercentage: boolean;
}>;

export interface BenefitContext {
    packageIndex: number;
    counter: number;
    benefitId: string;
    name: string;
    amount: string;
    taxFreeChecked: string;
    fixedSelected: string;
    percentageSelected: string;
    amountPlaceholder: string;
    percentageDisplay: string;
    fixedControlStyle: string;
    hiddenCadenceDisplay: string;
    mxnSelected: string;
    usdSelected: string;
    monthlySelected: string;
    annualSelected: string;
    banxicoDisplay: string;
}

export function benefitContext(
    packageIndex: number,
    counter: number,
    savedBenefit: SavedBenefit | null = null
): BenefitContext {
    const isPercentage = benefitIsPercentage(savedBenefit);
    const currency = savedBenefitCurrency(savedBenefit);
    const cadence = savedBenefitCadence(savedBenefit);

    return {
        packageIndex,
        counter,
        benefitId: `benefit-${packageIndex}-${counter}`,
        name: savedBenefitName(savedBenefit),
        amount: savedBenefitAmount(savedBenefit),
        taxFreeChecked: checkedAttr(savedBenefitTaxFree(savedBenefit)),
        fixedSelected: selectedAttr(!isPercentage),
        percentageSelected: selectedAttr(isPercentage),
        amountPlaceholder: benefitAmountPlaceholder(isPercentage),
        percentageDisplay: displayValue(isPercentage, 'inline', 'none'),
        fixedControlStyle: benefitFixedControlStyle(isPercentage),
        hiddenCadenceDisplay: displayValue(isPercentage, 'inline', 'none'),
        mxnSelected: selectedAttr(currency === 'MXN'),
        usdSelected: selectedAttr(currency === 'USD'),
        monthlySelected: selectedAttr(cadence === 'monthly'),
        annualSelected: selectedAttr(cadence === 'annual'),
        banxicoDisplay: benefitBanxicoDisplay(currency, isPercentage)
    };
}

export function benefitMarkup(
    context: BenefitContext,
    usdMxnLabel: string = homeUSDMXNLabel()
): string {
    return `
        <div style="display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap; width: 100%;">
            <input type="text" name="OtherBenefitName-${context.packageIndex}[]" placeholder="Ej: Bono anual" value="${context.name}" style="flex: 1; min-width: 120px; padding: 0.5rem; border: 1px solid #e2e8f0; border-radius: 4px; font-size: 0.75rem;">
            
            <select name="OtherBenefitType-${context.packageIndex}[]" class="benefit-type-select" data-benefit-id="${context.benefitId}" style="width: 110px; padding: 0.5rem; border: 1px solid #e2e8f0; border-radius: 4px; font-size: 0.7rem; background: #f8fafc;">
                <option value="fixed" ${context.fixedSelected}>💵 Monto fijo</option>
                <option value="percentage" ${context.percentageSelected}>📊 % Salario</option>
            </select>
            
            <div class="benefit-amount-container" style="display: flex; gap: 0.25rem; align-items: center;">
                <input type="text" name="OtherBenefitAmount-${context.packageIndex}[]" placeholder="${context.amountPlaceholder}" value="${context.amount}" class="money-input benefit-amount-input" data-benefit-id="${context.benefitId}" style="width: 90px; padding: 0.5rem; border: 1px solid #e2e8f0; border-radius: 4px; font-size: 0.75rem;">
                <span class="percentage-label" style="display: ${context.percentageDisplay}; font-size: 0.75rem; color: #64748b; font-weight: 600;">%</span>
            </div>
            
            <select name="OtherBenefitCurrency-${context.packageIndex}[]" class="benefit-currency-select" data-benefit-id="${context.benefitId}" style="width: 70px; padding: 0.5rem; border: 1px solid #e2e8f0; border-radius: 4px; font-size: 0.7rem; ${context.fixedControlStyle}">
                <option value="MXN" ${context.mxnSelected}>MXN</option>
                <option value="USD" ${context.usdSelected}>USD</option>
            </select>
            
            <select name="OtherBenefitCadence-${context.packageIndex}[]" class="benefit-cadence-select" style="width: 90px; padding: 0.5rem; border: 1px solid #e2e8f0; border-radius: 4px; font-size: 0.7rem; ${context.fixedControlStyle}">
                <option value="monthly" ${context.monthlySelected}>Mensual</option>
                <option value="annual" ${context.annualSelected}>Anual</option>
            </select>
            
            <!-- Hidden input for percentage bonuses (always annual) -->
            <input type="hidden" name="OtherBenefitCadence-${context.packageIndex}[]" class="benefit-cadence-hidden" value="annual" style="display: ${context.hiddenCadenceDisplay};">
            <span class="percentage-cadence-label" style="display: ${context.hiddenCadenceDisplay}; font-size: 0.7rem; color: #64748b; font-weight: 500; padding: 0.5rem; background: #f8fafc; border-radius: 4px; border: 1px solid #e2e8f0;">📅 Anual</span>
            
            <label style="display: flex; align-items: center; white-space: nowrap; font-size: 0.7rem; cursor: pointer;">
                <input type="checkbox" name="OtherBenefitTaxFree-${context.packageIndex}[]" value="${context.counter}" ${context.taxFreeChecked} style="margin-right: 0.25rem;">
                Libre ISR
            </label>
            
            <button type="button" data-action="remove-benefit" data-benefit-id="${context.benefitId}" style="background: #ef4444; color: white; padding: 0.35rem 0.5rem; border: none; border-radius: 4px; cursor: pointer; font-size: 0.7rem;">🗑️</button>
        </div>
        
        <div id="banxico-notice-${context.benefitId}" class="banxico-notice" style="display: ${context.banxicoDisplay}; width: 100%; margin-top: 0.5rem; padding: 0.5rem; background: #dbeafe; border-left: 3px solid #3b82f6; border-radius: 4px;">
            <div style="font-size: 0.65rem; color: #1e40af; line-height: 1.4;">
                💡 <strong>Tipo de cambio oficial Banxico:</strong> ${usdMxnLabel} MXN/USD
                <div style="margin-top: 0.25rem; font-size: 0.6rem; color: #64748b;">Actualizado automáticamente a las 14:00 CST</div>
            </div>
        </div>
    `;
}

export function benefitFixedControlStyle(isPercentage: boolean): string {
	return displayValue(isPercentage, 'display: none;', '');
}

export function benefitAmountPlaceholder(isPercentage: boolean): string {
	return displayValue(isPercentage, '10', '$1,500');
}

export function benefitBanxicoDisplay(currency: string, isPercentage: boolean): string {
	return benefitBanxicoDisplays[`${currency}:${String(isPercentage)}`] ?? 'none';
}

const benefitBanxicoDisplays: Record<string, string> = {
	'USD:false': 'block'
};

export function savedBenefitFromInput(benefitInput: Element): SavedBenefit {
    return {
        name: benefitInput.getAttribute('data-name'),
        amount: benefitInput.getAttribute('data-amount'),
        taxFree: benefitInput.getAttribute('data-taxfree') === 'true',
        currency: savedBenefitAttribute(benefitInput, 'data-currency', 'MXN'),
        cadence: savedBenefitAttribute(benefitInput, 'data-cadence', 'monthly'),
        isPercentage: benefitInput.getAttribute('data-ispercentage') === 'true'
    };
}

function savedBenefitName(savedBenefit: SavedBenefit | null): string {
    return savedBenefit?.name ?? '';
}

function savedBenefitAmount(savedBenefit: SavedBenefit | null): string {
	return formatNumber((savedBenefit?.amount ?? '').toString());
}

function savedBenefitTaxFree(savedBenefit: SavedBenefit | null): boolean {
    return savedBenefit?.taxFree === true;
}

function savedBenefitCurrency(savedBenefit: SavedBenefit | null): string {
    return savedBenefit?.currency ?? 'MXN';
}

function savedBenefitCadence(savedBenefit: SavedBenefit | null): string {
    return savedBenefit?.cadence ?? 'monthly';
}

function benefitIsPercentage(savedBenefit: SavedBenefit | null): boolean {
    return savedBenefit?.isPercentage === true;
}

function savedBenefitAttribute(benefitInput: Element, name: string, fallback: string): string {
	return benefitInput.getAttribute(name) || fallback;
}
