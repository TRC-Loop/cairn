// SPDX-License-Identifier: AGPL-3.0-or-later
import { writable } from 'svelte/store';

type VersionInfo = { version: string; revision: string };

const _store = writable<VersionInfo>({ version: 'dev', revision: 'unknown' });
let _loaded = false;

export const versionInfo = {
	subscribe: _store.subscribe,
	async load() {
		if (_loaded) return;
		_loaded = true;
		try {
			const res = await fetch('/api/version');
			if (res.ok) {
				_store.set(await res.json());
			}
		} catch {
			// keep defaults
		}
	}
};
