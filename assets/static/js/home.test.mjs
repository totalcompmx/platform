import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const source = readFileSync(new URL('./home.js', import.meta.url), 'utf8');

function loadHomeScript(config = {}) {
    const forms = [];
    const listeners = {};

    const context = {
        console,
        confirm: () => true,
        clearTimeout: () => {},
        setTimeout: callback => callback(),
        window: {
            TotalCompHome: config,
            addEventListener: (event, callback) => {
                listeners[`window:${event}`] = callback;
            },
            innerWidth: 1280
        },
        document: {
            addEventListener: (event, callback) => {
                listeners[`document:${event}`] = callback;
            },
            body: {
                appendChild: element => {
                    forms.push(element);
                }
            },
            createElement: createElement,
            getElementById: () => null,
            querySelector: () => null,
            querySelectorAll: () => []
        }
    };

    vm.createContext(context);
    vm.runInContext(source, context, { filename: 'home.js' });

    return { context, forms, listeners };
}

function createElement(tagName) {
    return {
        tagName: tagName.toUpperCase(),
        children: [],
        style: {},
        submitted: false,
        appendChild(child) {
            this.children.push(child);
        },
        submit() {
            this.submitted = true;
        }
    };
}

test('loads without executing DOMContentLoaded work immediately', () => {
    const { listeners } = loadHomeScript();

    assert.equal(typeof listeners['document:DOMContentLoaded'], 'function');
    assert.equal(typeof listeners['window:resize'], 'function');
});

test('uses template-provided frontend config with safe fallbacks', () => {
    const configured = loadHomeScript({
        csrfToken: 'csrf-from-template',
        usdMxnRate: '19.1234'
    }).context;
    const fallback = loadHomeScript().context;

    assert.equal(configured.homeCSRFToken(), 'csrf-from-template');
    assert.equal(configured.homeUSDMXNLabel(), '$19.1234');
    assert.equal(fallback.homeCSRFToken(), '');
    assert.equal(fallback.homeUSDMXNLabel(), '$20.00');
});

test('renders Banxico notice from frontend config', () => {
    const { context } = loadHomeScript({ usdMxnRate: '18.7654' });
    const markup = context.benefitMarkup({
        packageIndex: 0,
        counter: 1,
        benefitId: 'benefit-0-1',
        name: 'Internet',
        amount: '100',
        taxFreeChecked: 'checked',
        mxnSelected: '',
        usdSelected: 'selected',
        monthlySelected: '',
        annualSelected: 'selected',
        fixedSelected: 'selected',
        percentageSelected: '',
        percentageDisplay: 'none',
        fixedControlStyle: '',
        hiddenCadenceDisplay: 'none',
        amountPlaceholder: '$1,500',
        banxicoDisplay: 'block'
    });

    assert.match(markup, /benefit-0-1/);
    assert.match(markup, /Internet/);
    assert.match(markup, /\$18\.7654 MXN\/USD/);
});

test('posts clear request with configured CSRF token', () => {
    const { context, forms } = loadHomeScript({ csrfToken: 'csrf-clear-token' });

    context.clearAllInputs();

    assert.equal(forms.length, 1);
    assert.equal(forms[0].method, 'POST');
    assert.equal(forms[0].action, '/clear');
    assert.equal(forms[0].submitted, true);
    assert.equal(forms[0].children.length, 1);
    assert.equal(forms[0].children[0].name, 'csrf_token');
    assert.equal(forms[0].children[0].value, 'csrf-clear-token');
});

test('keeps pure formatting helpers stable', () => {
    const { context } = loadHomeScript();

    assert.equal(context.formatNumber('$1234567.891'), '1,234,567.89');
    assert.equal(context.salaryLabelText('hourly'), '💰 Tarifa Por Hora');
    assert.equal(context.salaryLabelText('annual'), '💰 Salario Bruto');
    assert.equal(context.hoursDisplay('hourly'), 'block');
    assert.equal(context.hoursDisplay('annual'), 'none');
    assert.equal(context.benefitAmountPlaceholder(true), '10');
    assert.equal(context.benefitAmountPlaceholder(false), '$1,500');
    assert.equal(context.benefitBanxicoDisplay('USD', false), 'block');
    assert.equal(context.benefitBanxicoDisplay('USD', true), 'none');
});
