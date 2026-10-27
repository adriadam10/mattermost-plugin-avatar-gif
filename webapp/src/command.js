// "/avatar-gif" with nothing after it opens the file picker in the webapp; anything
// else (a URL, "off") goes to the server as a normal command.
export function wantsPicker(message) {
    return /^\/avatar-gif\s*$/i.test(message.trim());
}

function cookie(name) {
    const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`));
    return match ? decodeURIComponent(match[1]) : '';
}

// Resolves with the chosen file, or null if the picker was closed.
export function pickFile() {
    return new Promise((resolve) => {
        const input = document.createElement('input');
        input.type = 'file';
        input.accept = 'image/gif';
        input.addEventListener('change', () => resolve(input.files[0] || null));
        input.addEventListener('cancel', () => resolve(null));
        input.click();
    });
}

// The server answers in the channel with an ephemeral post, success or not.
export async function upload(pluginId, file, args) {
    const body = new FormData();
    body.append('image', file);
    body.append('channel_id', args.channel_id || '');
    body.append('root_id', args.root_id || '');
    const base = window.basename || '';
    await fetch(`${base}/plugins/${pluginId}/api/v1/avatar`, {
        method: 'POST',
        body,
        credentials: 'same-origin',
        headers: {'X-CSRF-Token': cookie('MMCSRF'), 'X-Requested-With': 'XMLHttpRequest'},
    });
}
