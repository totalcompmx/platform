export interface ConfirmSubmitOptions {
    confirmFn?: (message: string) => boolean;
    documentRef?: Document;
}

export function initializeConfirmSubmit(options: ConfirmSubmitOptions = {}): void {
    const documentRef = options.documentRef ?? document;
    const confirmFn = options.confirmFn ?? ((message: string) => window.confirm(message));

    documentRef.addEventListener('click', event => {
        const trigger = confirmTrigger(event.target);

        if (!trigger || isSubmitControl(trigger)) {
            return;
        }

        confirmOrPrevent(trigger, event, confirmFn);
    });

    documentRef.addEventListener('submit', event => {
        const submitEvent = event as SubmitEvent;
        const trigger = confirmTrigger(submitEvent.submitter) ?? confirmTrigger(event.target);

        if (!trigger) {
            return;
        }

        confirmOrPrevent(trigger, event, confirmFn);
    });
}

function confirmTrigger(target: EventTarget | null): HTMLElement | null {
    if (!(target instanceof Element)) {
        return null;
    }

    return target.closest<HTMLElement>('[data-confirm]');
}

function isSubmitControl(element: HTMLElement): boolean {
    if (element instanceof HTMLButtonElement) {
        return element.type === 'submit';
    }

    if (element instanceof HTMLInputElement) {
        return element.type === 'submit' || element.type === 'image';
    }

    return false;
}

function confirmOrPrevent(
    trigger: HTMLElement,
    event: Event,
    confirmFn: (message: string) => boolean
): void {
    const message = trigger.dataset.confirm;

    if (!message || confirmFn(message)) {
        return;
    }

    event.preventDefault();
}
