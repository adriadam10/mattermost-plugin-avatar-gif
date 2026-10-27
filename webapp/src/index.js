import manifest from '../../plugin.json';

import {pickFile, upload, wantsPicker} from './command.js';

class Plugin {
    initialize(registry) {
        registry.registerSlashCommandWillBePostedHook(async (message, args) => {
            if (!wantsPicker(message)) {
                return {message, args};
            }
            const file = await pickFile();
            if (file) {
                try {
                    await upload(manifest.id, file, args);
                } catch (err) {
                    return {error: {message: `/avatar-gif: ${err.message}`}};
                }
            }
            return {};
        });
    }
}

window.registerPlugin(manifest.id, new Plugin());
