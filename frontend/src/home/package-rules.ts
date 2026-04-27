const salaryLabels: Record<string, string> = {
	hourly: '💰 Tarifa Por Hora',
	daily: '💰 Salario Diario',
	weekly: '💰 Salario Semanal',
	biweekly: '💰 Salario Quincenal',
	monthly: '💰 Salario Bruto'
};

const salaryHoursDisplays: Record<string, string> = {
	hourly: 'block'
};

const salaryBlockedFrequencies = new Set(['daily', 'hourly']);

const selectedAttributes: Record<string, string> = {
	true: 'selected',
	false: ''
};

const checkedAttributes: Record<string, string> = {
	true: 'checked',
	false: ''
};

export function salaryLabelText(value: string): string {
	return salaryLabels[value] ?? salaryLabels.monthly;
}

export function hoursDisplay(value: string): string {
	return salaryHoursDisplays[value] ?? 'none';
}

export function isSalaryBlockedFrequency(value: string): boolean {
	return salaryBlockedFrequencies.has(value);
}

export function selectedAttr(selected: boolean): string {
	return selectedAttributes[String(selected)];
}

export function checkedAttr(checked: boolean): string {
	return checkedAttributes[String(checked)];
}

export function displayValue(show: boolean, visibleValue: string, hiddenValue: string): string {
	return [hiddenValue, visibleValue][Number(show)];
}
