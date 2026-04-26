const totalCompHomeConfig = window.TotalCompHome || {};

function homeCSRFToken() {
    return totalCompHomeConfig.csrfToken || '';
}

function homeUSDMXNRate() {
    return totalCompHomeConfig.usdMxnRate || '20.00';
}

function homeUSDMXNLabel() {
    return `$${homeUSDMXNRate()}`;
}

let benefitCounters = [0, 0]; // Counters for each package

function toggleRegime(select, index) {
    const controls = regimeControls(index);
    if (!controls.paymentFreqSelect) return;

    const currentFreq = controls.paymentFreqSelect.value;
    if (select.value === 'resico') {
        applyResicoRegime(controls);
    } else {
        applySalaryRegime(controls, currentFreq);
    }

    restorePaymentFrequency(controls.paymentFreqSelect, currentFreq);
    toggleSalaryLabel(controls.paymentFreqSelect, index);
}

function regimeControls(index) {
    return {
        benefitsSection: document.querySelector(`.benefits-section-${index}`),
        currencySelection: document.querySelector(`.currency-selection-${index}`),
        paymentFreqSelect: document.querySelector(`.payment-frequency-select-${index}`),
        unpaidVacationDiv: document.querySelector(`.unpaid-vacation-${index}`),
        currencySelect: document.querySelectorAll('select[name="Currency[]"]')[index],
        exchangeRateDiv: document.querySelector(`.exchange-rate-input-${index}`),
        exchangeRateDisplay: document.querySelector(`.exchange-rate-display-${index}`)
    };
}

function applyResicoRegime(controls) {
    setDisplay(controls.benefitsSection, 'none');
    setDisplay(controls.currencySelection, 'block');
    setDisplay(controls.unpaidVacationDiv, 'block');
    setBenefitCheckboxes(controls.benefitsSection, false);
    Array.from(controls.paymentFreqSelect.options).forEach(enablePaymentOption);
}

function applySalaryRegime(controls, currentFreq) {
    setDisplay(controls.benefitsSection, 'block');
    setDisplay(controls.currencySelection, 'none');
    setDisplay(controls.unpaidVacationDiv, 'none');
    setBenefitCheckboxes(controls.benefitsSection, true);
    setCurrencyToMXN(controls.currencySelect);
    setDisplay(controls.exchangeRateDiv, 'none');
    setDisplay(controls.exchangeRateDisplay, 'none');
    Array.from(controls.paymentFreqSelect.options).forEach(setSalaryPaymentOption);
    resetInvalidSalaryFrequency(controls.paymentFreqSelect, currentFreq);
}

function setDisplay(element, displayValue) {
    if (element) {
        element.style.display = displayValue;
    }
}

function setBenefitCheckboxes(container, checked) {
    if (!container) return;
    container.querySelectorAll('input[type="checkbox"]').forEach(cb => {
        cb.checked = checked;
    });
}

function setCurrencyToMXN(currencySelect) {
    if (currencySelect) {
        currencySelect.value = 'MXN';
    }
}

function enablePaymentOption(option) {
    option.style.display = '';
    option.disabled = false;
}

function setSalaryPaymentOption(option) {
    if (isSalaryBlockedFrequency(option.value)) {
        option.style.display = 'none';
        option.disabled = true;
        return;
    }

    enablePaymentOption(option);
}

function isSalaryBlockedFrequency(value) {
    return value === 'daily' || value === 'hourly';
}

function resetInvalidSalaryFrequency(paymentFreqSelect, currentFreq) {
    if (isSalaryBlockedFrequency(currentFreq)) {
        paymentFreqSelect.value = 'monthly';
    }
}

function restorePaymentFrequency(paymentFreqSelect, currentFreq) {
    if (paymentFreqSelect.value === currentFreq) return;
    if (paymentOptionEnabled(paymentFreqSelect, currentFreq)) {
        paymentFreqSelect.value = currentFreq;
    }
}

function paymentOptionEnabled(paymentFreqSelect, currentFreq) {
    return Array.from(paymentFreqSelect.options).some(option => {
        return option.value === currentFreq && !option.disabled;
    });
}

function toggleExchangeRate(select, index) {
    const exchangeRateInput = document.querySelector(`.exchange-rate-input-${index}`);
    if (select.value === 'USD') {
        exchangeRateInput.style.display = 'block';
    } else {
        exchangeRateInput.style.display = 'none';
    }
}

function toggleSalaryLabel(select, index) {
    const salaryLabel = document.querySelector(`.salary-label-${index}`);
    const hoursPerWeek = document.querySelector(`.hours-per-week-${index}`);
    if (!salaryLabel) return;
    if (!hoursPerWeek) return;

    salaryLabel.textContent = salaryLabelText(select.value);
    hoursPerWeek.style.display = hoursDisplay(select.value);
}

function salaryLabelText(value) {
    const labels = {
        hourly: '💰 Tarifa Por Hora',
        daily: '💰 Salario Diario',
        weekly: '💰 Salario Semanal',
        biweekly: '💰 Salario Quincenal',
        monthly: '💰 Salario Bruto'
    };
    return labels[value] || labels.monthly;
}

function hoursDisplay(value) {
    if (value === 'hourly') {
        return 'block';
    }

    return 'none';
}

function addBenefit(packageIndex, savedBenefit = null) {
    benefitCounters[packageIndex]++;
    const context = benefitContext(packageIndex, benefitCounters[packageIndex], savedBenefit);
    const benefitDiv = newBenefitDiv(context.benefitId);
    benefitDiv.innerHTML = benefitMarkup(context);
    document.getElementById(`otherBenefits-${packageIndex}`).appendChild(benefitDiv);
    attachCommaFormatting(benefitDiv.querySelectorAll('.money-input'));
}

function benefitContext(packageIndex, counter, savedBenefit) {
    const isPercentage = benefitIsPercentage(savedBenefit);
    const currency = savedBenefitCurrency(savedBenefit);
    const cadence = savedBenefitCadence(savedBenefit);
    return {
        packageIndex: packageIndex,
        counter: counter,
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

function newBenefitDiv(benefitId) {
    const benefitDiv = document.createElement('div');
    benefitDiv.id = benefitId;
    benefitDiv.style.cssText = 'display: flex; flex-direction: column; gap: 0; background: white; padding: 0.5rem; border-radius: 6px; border: 1px solid #e2e8f0;';
    return benefitDiv;
}

function savedBenefitName(savedBenefit) {
    if (!savedBenefit) return '';
    return savedBenefit.name;
}

function savedBenefitAmount(savedBenefit) {
    if (!savedBenefit) return '';
    if (!savedBenefit.amount) return '';
    return formatNumber(savedBenefit.amount.toString());
}

function savedBenefitTaxFree(savedBenefit) {
    if (!savedBenefit) return false;
    return savedBenefit.taxFree === true;
}

function savedBenefitCurrency(savedBenefit) {
    if (!savedBenefit) return 'MXN';
    return savedBenefit.currency;
}

function savedBenefitCadence(savedBenefit) {
    if (!savedBenefit) return 'monthly';
    return savedBenefit.cadence;
}

function benefitIsPercentage(savedBenefit) {
    if (!savedBenefit) return false;
    return savedBenefit.isPercentage === true;
}

function selectedAttr(selected) {
    if (selected) return 'selected';
    return '';
}

function checkedAttr(checked) {
    if (checked) return 'checked';
    return '';
}

function displayValue(show, visibleValue, hiddenValue) {
    if (show) return visibleValue;
    return hiddenValue;
}

function benefitFixedControlStyle(isPercentage) {
    if (isPercentage) return 'display: none;';
    return '';
}

function benefitAmountPlaceholder(isPercentage) {
    if (isPercentage) return '10';
    return '$1,500';
}

function benefitBanxicoDisplay(currency, isPercentage) {
    if (currency !== 'USD') return 'none';
    if (isPercentage) return 'none';
    return 'block';
}

function benefitMarkup(context) {
    return `
        <div style="display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap; width: 100%;">
            <input type="text" name="OtherBenefitName-${context.packageIndex}[]" placeholder="Ej: Bono anual" value="${context.name}" style="flex: 1; min-width: 120px; padding: 0.5rem; border: 1px solid #e2e8f0; border-radius: 4px; font-size: 0.75rem;">
            
            <select name="OtherBenefitType-${context.packageIndex}[]" class="benefit-type-select" onchange="toggleBenefitInputType('${context.benefitId}')" style="width: 110px; padding: 0.5rem; border: 1px solid #e2e8f0; border-radius: 4px; font-size: 0.7rem; background: #f8fafc;">
                <option value="fixed" ${context.fixedSelected}>💵 Monto fijo</option>
                <option value="percentage" ${context.percentageSelected}>📊 % Salario</option>
            </select>
            
            <div class="benefit-amount-container" style="display: flex; gap: 0.25rem; align-items: center;">
                <input type="text" name="OtherBenefitAmount-${context.packageIndex}[]" placeholder="${context.amountPlaceholder}" value="${context.amount}" class="money-input benefit-amount-input" data-benefit-id="${context.benefitId}" style="width: 90px; padding: 0.5rem; border: 1px solid #e2e8f0; border-radius: 4px; font-size: 0.75rem;">
                <span class="percentage-label" style="display: ${context.percentageDisplay}; font-size: 0.75rem; color: #64748b; font-weight: 600;">%</span>
            </div>
            
            <select name="OtherBenefitCurrency-${context.packageIndex}[]" class="benefit-currency-select" onchange="toggleBenefitBanxicoNotice('${context.benefitId}')" style="width: 70px; padding: 0.5rem; border: 1px solid #e2e8f0; border-radius: 4px; font-size: 0.7rem; ${context.fixedControlStyle}">
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
            
            <button type="button" onclick="removeBenefit('${context.benefitId}')" style="background: #ef4444; color: white; padding: 0.35rem 0.5rem; border: none; border-radius: 4px; cursor: pointer; font-size: 0.7rem;">🗑️</button>
        </div>
        
        <div id="banxico-notice-${context.benefitId}" class="banxico-notice" style="display: ${context.banxicoDisplay}; width: 100%; margin-top: 0.5rem; padding: 0.5rem; background: #dbeafe; border-left: 3px solid #3b82f6; border-radius: 4px;">
            <div style="font-size: 0.65rem; color: #1e40af; line-height: 1.4;">
                💡 <strong>Tipo de cambio oficial Banxico:</strong> ${homeUSDMXNLabel()} MXN/USD
                <div style="margin-top: 0.25rem; font-size: 0.6rem; color: #64748b;">Actualizado automáticamente a las 14:00 CST</div>
            </div>
        </div>
    `;
}

function removeBenefit(benefitId) {
    const element = document.getElementById(benefitId);
    if (element) {
        element.remove();
    }
}

function toggleBenefitInputType(benefitId) {
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

function benefitInputControls(benefitId) {
    const benefitDiv = document.getElementById(benefitId);
    if (!benefitDiv) return null;
    const controls = {
        typeSelect: benefitDiv.querySelector('.benefit-type-select'),
        amountInput: benefitDiv.querySelector('.benefit-amount-input'),
        percentageLabel: benefitDiv.querySelector('.percentage-label'),
        currencySelect: benefitDiv.querySelector('.benefit-currency-select'),
        cadenceSelect: benefitDiv.querySelector('.benefit-cadence-select'),
        cadenceHidden: benefitDiv.querySelector('.benefit-cadence-hidden'),
        percentageCadenceLabel: benefitDiv.querySelector('.percentage-cadence-label'),
        banxicoNotice: document.getElementById(`banxico-notice-${benefitId}`)
    };
    if (!controls.typeSelect) return null;
    if (!controls.amountInput) return null;
    return controls;
}

function applyPercentageBenefitInput(controls) {
    controls.amountInput.placeholder = '10';
    setDisplay(controls.percentageLabel, 'inline');
    setDisplay(controls.currencySelect, 'none');
    setDisplay(controls.cadenceSelect, 'none');
    setDisplay(controls.cadenceHidden, 'inline');
    setDisplay(controls.percentageCadenceLabel, 'inline');
    setDisplay(controls.banxicoNotice, 'none');
    controls.amountInput.classList.remove('money-input');
}

function applyFixedBenefitInput(controls) {
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

function toggleBenefitBanxicoNotice(benefitId) {
    const controls = benefitBanxicoControls(benefitId);
    if (!controls) return;
    updateBanxicoNotice(controls);
}

function benefitBanxicoControls(benefitId) {
    const benefitDiv = document.getElementById(benefitId);
    if (!benefitDiv) return null;
    const controls = {
        currencySelect: benefitDiv.querySelector('.benefit-currency-select'),
        typeSelect: benefitDiv.querySelector('.benefit-type-select'),
        banxicoNotice: document.getElementById(`banxico-notice-${benefitId}`)
    };
    if (Object.values(controls).every(Boolean)) {
        return controls;
    }

    return null;
}

function updateBanxicoNotice(controls) {
    setDisplay(controls.banxicoNotice, benefitBanxicoNoticeDisplay(controls));
}

function benefitBanxicoNoticeDisplay(controls) {
    if (controls.currencySelect.value !== 'USD') return 'none';
    if (controls.typeSelect.value !== 'fixed') return 'none';
    return 'block';
}

function formatNumber(value) {
    // Remove all non-digit characters except decimal point
    const cleanValue = value.replace(/[^\d.]/g, '');
    // Split on decimal point
    const parts = cleanValue.split('.');
    // Add commas to integer part
    parts[0] = parts[0].replace(/\B(?=(\d{3})+(?!\d))/g, ',');
    // Return formatted value (limit to 2 decimal places if there's a decimal)
    return parts.length > 1 ? parts[0] + '.' + parts[1].substring(0, 2) : parts[0];
}

function attachCommaFormatting(inputs) {
    inputs.forEach(input => {
        input.addEventListener('input', function(e) {
            const cursorPosition = e.target.selectionStart;
            const oldLength = e.target.value.length;
            let formatted = formatNumber(e.target.value);
            
            // Check for max value constraint from the active fiscal year
            const maxValue = e.target.getAttribute('data-max');
            if (maxValue) {
                const numericValue = parseFloat(formatted.replace(/,/g, ''));
                if (!isNaN(numericValue) && numericValue > parseFloat(maxValue)) {
                    formatted = formatNumber(maxValue);
                    // Visual feedback: briefly highlight red
                    e.target.style.borderColor = '#ef4444';
                    setTimeout(() => {
                        e.target.style.borderColor = '#e2e8f0';
                    }, 500);
                }
            }
            
            const newLength = formatted.length;
            e.target.value = formatted;
            
            // Adjust cursor position after formatting
            const diff = newLength - oldLength;
            e.target.setSelectionRange(cursorPosition + diff, cursorPosition + diff);
        });
    });
}

// Load saved values from hidden inputs
function loadSavedValues() {
    [0, 1].forEach(loadSavedPackageValues);
}

function loadSavedPackageValues(idx) {
    const packageDiv = document.querySelector(`[data-package-index="${idx}"]`);
    if (!packageDiv) return;

    loadSavedPackageName(packageDiv, idx);
    loadSavedBasicValues(packageDiv, idx);
    loadSavedEquity(packageDiv, idx);
    loadSavedSalary(packageDiv, idx);
    loadSavedBenefitCheckboxes(idx);
    loadSavedOtherBenefits(idx);
}

function savedValue(id) {
    const element = document.getElementById(id);
    if (!element) return '';
    return element.value;
}

function savedTrue(id) {
    return savedValue(id) === 'true';
}

function setPackageValue(packageDiv, selector, value) {
    if (!value) return;
    const input = packageDiv.querySelector(selector);
    if (input) input.value = value;
}

function setPackageFormattedValue(packageDiv, selector, value) {
    if (!value) return;
    setPackageValue(packageDiv, selector, formatNumber(value));
}

function setIndexedValue(name, index, value) {
    if (!value) return;
    const input = document.querySelectorAll(`input[name="${name}"]`)[index];
    if (input) input.value = value;
}

function setIndexedFormattedValue(name, index, value) {
    if (!value) return;
    setIndexedValue(name, index, formatNumber(value));
}

function loadSavedPackageName(packageDiv, idx) {
    setPackageValue(packageDiv, '.package-name-input', savedValue(`saved-pkg-${idx}-name`));
}

function loadSavedBasicValues(packageDiv, idx) {
    setPackageValue(packageDiv, '.regime-select', savedValue(`saved-pkg-${idx}-regime`));
    loadSavedCurrency(packageDiv, idx);
    loadSavedPaymentFrequency(packageDiv, idx);
    setPackageValue(packageDiv, `input[name="HoursPerWeek[]"]`, savedValue(`saved-pkg-${idx}-hours`));
    setPackageValue(packageDiv, `input[name="UnpaidVacationDays[]"]`, savedValue(`saved-pkg-${idx}-unpaid-vacation`));
}

function loadSavedCurrency(packageDiv, idx) {
    const currency = savedValue(`saved-pkg-${idx}-currency`);
    if (!currency) return;
    setPackageValue(packageDiv, `select[name="Currency[]"]`, currency);
    if (currency === 'USD') {
        loadSavedExchangeRate(packageDiv, idx);
    }
}

function loadSavedExchangeRate(packageDiv, idx) {
    setDisplay(document.querySelector(`.exchange-rate-input-${idx}`), 'block');
    setPackageValue(packageDiv, `input[name="ExchangeRate[]"]`, savedValue(`saved-pkg-${idx}-exchange-rate`));
}

function loadSavedPaymentFrequency(packageDiv, idx) {
    const paymentFrequency = savedValue(`saved-pkg-${idx}-payment-freq`);
    if (!paymentFrequency) return;

    const freqSelect = packageDiv.querySelector(`.payment-frequency-select-${idx}`);
    if (!freqSelect) return;

    freqSelect.value = paymentFrequency;
    toggleSalaryLabel(freqSelect, idx);
}

function loadSavedEquity(packageDiv, idx) {
    if (!savedTrue(`saved-pkg-${idx}-has-equity`)) return;
    loadSavedEquityToggle(packageDiv, idx);
    setPackageFormattedValue(packageDiv, `input[name="InitialEquityUSD[]"]`, savedValue(`saved-pkg-${idx}-initial-equity`));
    loadSavedRefreshers(packageDiv, idx);
}

function loadSavedEquityToggle(packageDiv, idx) {
    const equityCheckbox = packageDiv.querySelector(`.equity-toggle-checkbox[data-package-index="${idx}"]`);
    if (!equityCheckbox) return;
    equityCheckbox.checked = true;
    toggleEquitySection(equityCheckbox);
}

function loadSavedRefreshers(packageDiv, idx) {
    if (!savedTrue(`saved-pkg-${idx}-has-refreshers`)) return;
    loadSavedRefresherToggle(packageDiv, idx);
    setIndexedFormattedValue('RefresherMinUSD[]', idx, savedValue(`saved-pkg-${idx}-refresher-min`));
    setIndexedFormattedValue('RefresherMaxUSD[]', idx, savedValue(`saved-pkg-${idx}-refresher-max`));
}

function loadSavedRefresherToggle(packageDiv, idx) {
    const refresherCheckbox = packageDiv.querySelector('.refresher-checkbox');
    if (!refresherCheckbox) return;
    refresherCheckbox.checked = true;
    toggleRefresherFields(refresherCheckbox, idx);
}

function loadSavedSalary(packageDiv, idx) {
    setPackageFormattedValue(packageDiv, '.salary-input', savedValue(`saved-pkg-${idx}-salary`));
}

function loadSavedBenefitCheckboxes(idx) {
    loadSavedAguinaldo(idx);
    loadSavedVales(idx);
    loadSavedPrima(idx);
    loadSavedFondo(idx);
}

function loadSavedAguinaldo(idx) {
    if (!savedTrue(`saved-pkg-${idx}-has-aguinaldo`)) return;
    checkFirst(`input[name="HasAguinaldo[]"][value="${idx}"]`);
    setIndexedValue('AguinaldoDays[]', idx, savedValue(`saved-pkg-${idx}-aguinaldo-days`));
}

function loadSavedVales(idx) {
    if (!savedTrue(`saved-pkg-${idx}-has-vales`)) return;
    checkFirst(`input[name="HasValesDespensa[]"][value="${idx}"]`);
    setIndexedFormattedValue('ValesDespensaAmount[]', idx, savedValue(`saved-pkg-${idx}-vales-amount`));
}

function loadSavedPrima(idx) {
    if (!savedTrue(`saved-pkg-${idx}-has-prima`)) return;
    checkFirst(`input[name="HasPrimaVacacional[]"][value="${idx}"]`);
    setIndexedValue('VacationDays[]', idx, savedValue(`saved-pkg-${idx}-vacation-days`));
    setIndexedValue('PrimaVacacionalPercent[]', idx, savedValue(`saved-pkg-${idx}-prima-percent`));
}

function loadSavedFondo(idx) {
    if (!savedTrue(`saved-pkg-${idx}-has-fondo`)) return;
    checkFirst(`input[name="HasFondoAhorro[]"][value="${idx}"]`);
    setIndexedValue('FondoAhorroPercent[]', idx, savedValue(`saved-pkg-${idx}-fondo-percent`));
}

function checkFirst(selector) {
    const checkboxes = document.querySelectorAll(selector);
    if (checkboxes.length > 0) {
        checkboxes[0].checked = true;
    }
}

function loadSavedOtherBenefits(idx) {
    document.querySelectorAll(`.saved-other-benefit-${idx}`).forEach(benefitInput => {
        addBenefit(idx, savedBenefitFromInput(benefitInput));
    });
}

function savedBenefitFromInput(benefitInput) {
    return {
        name: benefitInput.getAttribute('data-name'),
        amount: benefitInput.getAttribute('data-amount'),
        taxFree: benefitInput.getAttribute('data-taxfree') === 'true',
        currency: savedBenefitAttribute(benefitInput, 'data-currency', 'MXN'),
        cadence: savedBenefitAttribute(benefitInput, 'data-cadence', 'monthly'),
        isPercentage: benefitInput.getAttribute('data-ispercentage') === 'true'
    };
}

function savedBenefitAttribute(benefitInput, name, fallback) {
    const value = benefitInput.getAttribute(name);
    if (value === null) return fallback;
    if (value === '') return fallback;
    return value;
}

// Strip commas before form submission
function clearAllInputs() {
    if (!confirm('¿Estás seguro de que quieres limpiar todos los campos?')) {
        return;
    }
    
    // Make a POST request to clear the session
    const form = document.createElement('form');
    form.method = 'POST';
    form.action = '/clear';
    
    // Add CSRF token
    const csrfInput = document.createElement('input');
    csrfInput.type = 'hidden';
    csrfInput.name = 'csrf_token';
    csrfInput.value = homeCSRFToken();
    form.appendChild(csrfInput);
    
    document.body.appendChild(form);
    form.submit();
}

// Toggle refresher fields
function toggleRefresherFields(checkbox, index) {
    const refresherFields = document.querySelector(`.refresher-fields-${index}`);
    if (refresherFields) {
        refresherFields.style.display = checkbox.checked ? 'block' : 'none';
    }
}

// Toggle entire equity section
function toggleEquitySection(checkbox) {
    const packageIndex = checkbox.getAttribute('data-package-index');
    const equitySection = document.querySelector(`.equity-section-${packageIndex}`);
    if (equitySection) {
        equitySection.style.display = checkbox.checked ? 'block' : 'none';
    }
}

document.addEventListener('DOMContentLoaded', function() {
    // Setup equity section toggle checkboxes
    document.querySelectorAll('.equity-toggle-checkbox').forEach(checkbox => {
        checkbox.addEventListener('change', function() {
            toggleEquitySection(this);
        });
    });
    
    // Setup refresher checkbox toggles
    document.querySelectorAll('.refresher-checkbox').forEach((checkbox, index) => {
        checkbox.addEventListener('change', function() {
            toggleRefresherFields(this, index);
        });
    });
    // Load saved values FIRST (this sets regime, frequency, and all inputs)
    loadSavedValues();
    
    // Then sync UI visibility/options based on loaded regime
    // Since toggleRegime no longer destroys the dropdown, values are preserved
    document.querySelectorAll('.regime-select').forEach((select, index) => {
        toggleRegime(select, index);
    });
    
    // Attach comma formatting to all money inputs
    attachCommaFormatting(document.querySelectorAll('.salary-input, .money-input'));
    
    // Handle form submission
    document.querySelector('form').addEventListener('submit', function(e) {
        // Remove commas from all inputs
        const inputs = document.querySelectorAll('input[type="text"], input[type="number"]');
        inputs.forEach(input => {
            if (input.value) {
                input.value = input.value.replace(/,/g, '');
            }
        });
        
        // Force MXN for Sueldos y Salarios (safety check)
        document.querySelectorAll('.regime-select').forEach((regimeSelect, idx) => {
            if (regimeSelect.value === 'sueldos_salarios') {
                const currencySelect = document.querySelectorAll('select[name="Currency[]"]')[idx];
                if (currencySelect) {
                    currencySelect.value = 'MXN';
                }
            }
        });
    });
    
    // Initialize comparison mode on page load
    initializeComparisonMode();
});

// ========== COMPARISON MODE TOGGLE FUNCTIONS ==========

function setComparisonMode(isComparing) {
    const controls = comparisonControls();
    if (!controls) return;

    if (isComparing) {
        applyComparisonMode(controls, isMobileViewport());
        return;
    }

    applySingleMode(controls, isMobileViewport());
}

function comparisonControls() {
    const controls = {
        packagesWrapper: document.getElementById('packagesWrapper'),
        packagesGrid: document.getElementById('packagesGrid'),
        package2: document.getElementById('package-2'),
        addButton: document.getElementById('addComparisonButton'),
        submitButton: document.getElementById('submitButton')
    };
    if (Object.values(controls).every(Boolean)) {
        return controls;
    }

    return null;
}

function isMobileViewport() {
    return window.innerWidth < 1024;
}

function responsiveValue(isMobile, mobileValue, desktopValue) {
    if (isMobile) return mobileValue;
    return desktopValue;
}

function applyComparisonMode(controls, isMobile) {
    Object.assign(controls.packagesWrapper.style, {
        position: 'relative',
        display: responsiveValue(isMobile, 'block', 'flex'),
        justifyContent: responsiveValue(isMobile, 'flex-start', 'center'),
        gap: '1rem',
        alignItems: 'stretch',
        padding: responsiveValue(isMobile, '0 10px', '0')
    });
    Object.assign(controls.packagesGrid.style, {
        width: responsiveValue(isMobile, '100%', 'auto'),
        maxWidth: '100%',
        margin: '0',
        gridTemplateColumns: responsiveValue(isMobile, '1fr', 'repeat(auto-fit, minmax(400px, 1fr))')
    });
    controls.package2.style.display = 'block';
    controls.addButton.style.display = 'none';
    controls.submitButton.textContent = '💰 Comparar Paquetes';
}

function applySingleMode(controls, isMobile) {
    if (isMobile) {
        applyMobileSingleMode(controls);
        return;
    }

    applyDesktopSingleMode(controls);
}

function applyMobileSingleMode(controls) {
    Object.assign(controls.packagesWrapper.style, {
        position: 'static',
        display: 'block',
        padding: '0 10px',
        marginBottom: '2rem'
    });
    Object.assign(controls.packagesGrid.style, {
        width: '100%',
        maxWidth: '100%',
        margin: '0',
        gridTemplateColumns: '1fr'
    });
    controls.package2.style.display = 'none';
    applyMobileAddButton(controls.addButton);
    controls.submitButton.textContent = '💰 Calcular Compensación';
}

function applyMobileAddButton(addButton) {
    Object.assign(addButton.style, {
        position: 'static',
        left: 'auto',
        top: 'auto',
        display: 'flex',
        width: '100%',
        marginTop: '1rem',
        justifyContent: 'center'
    });
    applyAddButtonTextLayout(addButton, {
        writingMode: 'horizontal-tb',
        width: 'auto',
        maxWidth: '280px',
        padding: '0.875rem 1.5rem',
        height: 'auto',
        fontSize: '0.85rem'
    }, 'none', 'inline');
}

function applyDesktopSingleMode(controls) {
    Object.assign(controls.packagesWrapper.style, {
        position: 'relative',
        display: 'block',
        padding: '0'
    });
    Object.assign(controls.packagesGrid.style, {
        width: '780px',
        maxWidth: '780px',
        margin: '0 auto',
        gridTemplateColumns: '1fr'
    });
    controls.package2.style.display = 'none';
    applyDesktopAddButton(controls.addButton);
    controls.submitButton.textContent = '💰 Calcular Compensación';
}

function applyDesktopAddButton(addButton) {
    Object.assign(addButton.style, {
        position: 'absolute',
        left: 'calc(50% + 390px + 1rem)',
        top: '0',
        display: 'flex'
    });
    applyAddButtonTextLayout(addButton, {
        writingMode: 'vertical-rl',
        width: 'auto',
        maxWidth: 'none',
        padding: '1.5rem 0.75rem',
        height: 'auto',
        maxHeight: '420px',
        fontSize: '0.7rem'
    }, 'inline', 'none');
}

function applyAddButtonTextLayout(addButton, styles, desktopDisplay, mobileDisplay) {
    const addButtonEl = addButton.querySelector('button');
    if (!addButtonEl) return;
    Object.assign(addButtonEl.style, styles);
    setAddButtonTextVisibility(addButtonEl, desktopDisplay, mobileDisplay);
}

function setAddButtonTextVisibility(addButtonEl, desktopDisplay, mobileDisplay) {
    const desktopText = addButtonEl.querySelector('.btn-text-desktop');
    const mobileText = addButtonEl.querySelector('.btn-text-mobile');
    if (desktopText) desktopText.style.display = desktopDisplay;
    if (mobileText) mobileText.style.display = mobileDisplay;
}

// Re-run on window resize to adjust layout
let resizeTimer;
window.addEventListener('resize', function() {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(function() {
        setComparisonMode(package2Visible());
    }, 250);
});

function package2Visible() {
    const package2 = document.getElementById('package-2');
    if (!package2) return false;
    return package2.style.display === 'block';
}

function initializeComparisonMode() {
    setComparisonMode(hasSavedPackage2Data());
}

function hasSavedPackage2Data() {
    if (hasSavedInputValue('saved-pkg-1-salary')) return true;
    return hasSavedPackage2Name();
}

function hasSavedInputValue(id) {
    return savedValue(id) !== '';
}

function hasSavedPackage2Name() {
    const value = savedValue('saved-pkg-1-name');
    if (value === '') return false;
    return value !== 'Paquete 2';
}