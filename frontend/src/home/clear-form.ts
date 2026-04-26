import { homeCSRFToken, type TotalCompHomeConfig } from './config';

export interface ClearFormInput {
    type: string;
    name: string;
    value: string;
}

export interface ClearForm {
    method: string;
    action: string;
    appendChild(child: ClearFormInput): void;
    submit(): void;
}

export interface ClearFormDocument {
    body: {
        appendChild(element: ClearForm): void;
    };
    createElement(tagName: 'form'): ClearForm;
    createElement(tagName: 'input'): ClearFormInput;
}

interface ClearAllInputsOptions {
    config?: TotalCompHomeConfig;
    confirmFn?: (message: string) => boolean;
    documentRef?: ClearFormDocument;
}

export function clearAllInputs(options: ClearAllInputsOptions = {}): void {
    const confirmFn = options.confirmFn ?? ((message: string) => window.confirm(message));
    if (!confirmFn('¿Estás seguro de que quieres limpiar todos los campos?')) {
        return;
    }

    const documentRef = options.documentRef ?? (document as unknown as ClearFormDocument);
    const form = documentRef.createElement('form');
    form.method = 'POST';
    form.action = '/clear';

    const csrfInput = documentRef.createElement('input');
    csrfInput.type = 'hidden';
    csrfInput.name = 'csrf_token';
    csrfInput.value = homeCSRFToken(options.config);
    form.appendChild(csrfInput);

    documentRef.body.appendChild(form);
    form.submit();
}
