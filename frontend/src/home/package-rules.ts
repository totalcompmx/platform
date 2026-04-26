const salaryLabels: Record<string, string> = {
    hourly: '💰 Tarifa Por Hora',
    daily: '💰 Salario Diario',
    weekly: '💰 Salario Semanal',
    biweekly: '💰 Salario Quincenal',
    monthly: '💰 Salario Bruto'
};

export function salaryLabelText(value: string): string {
    return salaryLabels[value] ?? salaryLabels.monthly;
}

export function hoursDisplay(value: string): string {
    if (value === 'hourly') {
        return 'block';
    }

    return 'none';
}

export function isSalaryBlockedFrequency(value: string): boolean {
    return value === 'daily' || value === 'hourly';
}

export function selectedAttr(selected: boolean): string {
    if (selected) return 'selected';
    return '';
}

export function checkedAttr(checked: boolean): string {
    if (checked) return 'checked';
    return '';
}

export function displayValue(show: boolean, visibleValue: string, hiddenValue: string): string {
    if (show) return visibleValue;
    return hiddenValue;
}
