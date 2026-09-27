export function runWindowAction(element, actionName, event, includeElement = false) {
    const action = window[actionName];
    if (typeof action !== 'function') {
        console.error(`Azione UI non disponibile: ${actionName}`);
        return;
    }

    const args = [];
    if (includeElement) args.push(element);
    if (element.dataset.actionArg !== undefined) args.push(element.dataset.actionArg);
    if (event.type === 'change') args.push(event);
    action(...args);
}

export function initializeUIActions() {
    document.addEventListener('click', event => {
        const element = event.target.closest('[data-action]');
        if (!element) return;

        if (element.matches('a[href="#"]')) event.preventDefault();

        if (element.dataset.action === 'openImportFilePicker') {
            document.getElementById('importFile')?.click();
            return;
        }

        runWindowAction(element, element.dataset.action, event, element.hasAttribute('data-action-self'));
    });

    document.addEventListener('change', event => {
        const element = event.target.closest('[data-change-actions]');
        if (!element) return;

        element.dataset.changeActions.split(/\s+/).filter(Boolean).forEach(actionName => {
            runWindowAction(element, actionName, event);
        });
    });
}
