export function formatNumber(value: string): string {
    const cleanValue = value.replace(/[^\d.]/g, '');
    const parts = cleanValue.split('.');

    parts[0] = parts[0].replace(/\B(?=(\d{3})+(?!\d))/g, ',');

    return parts.length > 1 ? `${parts[0]}.${parts[1].substring(0, 2)}` : parts[0];
}

export function stripCommas(value: string): string {
    return value.replace(/,/g, '');
}

export function attachCommaFormatting(inputs: Iterable<HTMLInputElement>): void {
    Array.from(inputs).forEach(input => {
        input.addEventListener('input', () => {
            const cursorPosition = input.selectionStart ?? input.value.length;
            const oldLength = input.value.length;
            let formatted = formatNumber(input.value);

            const maxValue = input.getAttribute('data-max');
            if (maxValue) {
                const numericValue = Number.parseFloat(stripCommas(formatted));
                if (!Number.isNaN(numericValue) && numericValue > Number.parseFloat(maxValue)) {
                    formatted = formatNumber(maxValue);
                    input.style.borderColor = '#ef4444';
                    window.setTimeout(() => {
                        input.style.borderColor = '#e2e8f0';
                    }, 500);
                }
            }

            const newLength = formatted.length;
            input.value = formatted;

            const diff = newLength - oldLength;
            input.setSelectionRange(cursorPosition + diff, cursorPosition + diff);
        });
    });
}
