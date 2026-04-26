interface ComparisonControls {
    packagesWrapper: HTMLElement;
    packagesGrid: HTMLElement;
    package2: HTMLElement;
    addButton: HTMLElement;
    submitButton: HTMLElement;
}

export function setComparisonMode(isComparing: boolean): void {
    const controls = comparisonControls();
    if (!controls) return;

    if (isComparing) {
        applyComparisonMode(controls, isMobileViewport());
        return;
    }

    applySingleMode(controls, isMobileViewport());
}

export function setupComparisonResizeHandler(): void {
    let resizeTimer: number | undefined;
    window.addEventListener('resize', () => {
        window.clearTimeout(resizeTimer);
        resizeTimer = window.setTimeout(() => {
            setComparisonMode(package2Visible());
        }, 250);
    });
}

export function initializeComparisonMode(hasSavedPackage2Data: () => boolean): void {
    setComparisonMode(hasSavedPackage2Data());
}

export function package2Visible(): boolean {
    const package2 = document.getElementById('package-2');
    if (!package2) return false;
    return package2.style.display === 'block';
}

function comparisonControls(): ComparisonControls | null {
    const packagesWrapper = document.getElementById('packagesWrapper');
    const packagesGrid = document.getElementById('packagesGrid');
    const package2 = document.getElementById('package-2');
    const addButton = document.getElementById('addComparisonButton');
    const submitButton = document.getElementById('submitButton');

    if (!packagesWrapper || !packagesGrid || !package2 || !addButton || !submitButton) {
        return null;
    }

    return {
        packagesWrapper,
        packagesGrid,
        package2,
        addButton,
        submitButton
    };
}

function isMobileViewport(): boolean {
    return window.innerWidth < 1024;
}

function responsiveValue<T>(isMobile: boolean, mobileValue: T, desktopValue: T): T {
    if (isMobile) return mobileValue;
    return desktopValue;
}

function applyComparisonMode(controls: ComparisonControls, isMobile: boolean): void {
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

function applySingleMode(controls: ComparisonControls, isMobile: boolean): void {
    if (isMobile) {
        applyMobileSingleMode(controls);
        return;
    }

    applyDesktopSingleMode(controls);
}

function applyMobileSingleMode(controls: ComparisonControls): void {
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

function applyMobileAddButton(addButton: HTMLElement): void {
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

function applyDesktopSingleMode(controls: ComparisonControls): void {
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

function applyDesktopAddButton(addButton: HTMLElement): void {
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

function applyAddButtonTextLayout(
    addButton: HTMLElement,
    styles: Partial<CSSStyleDeclaration>,
    desktopDisplay: string,
    mobileDisplay: string
): void {
    const addButtonEl = addButton.querySelector<HTMLElement>('button');
    if (!addButtonEl) return;

    Object.assign(addButtonEl.style, styles);
    setAddButtonTextVisibility(addButtonEl, desktopDisplay, mobileDisplay);
}

function setAddButtonTextVisibility(
    addButtonEl: HTMLElement,
    desktopDisplay: string,
    mobileDisplay: string
): void {
    const desktopText = addButtonEl.querySelector<HTMLElement>('.btn-text-desktop');
    const mobileText = addButtonEl.querySelector<HTMLElement>('.btn-text-mobile');

    if (desktopText) desktopText.style.display = desktopDisplay;
    if (mobileText) mobileText.style.display = mobileDisplay;
}
