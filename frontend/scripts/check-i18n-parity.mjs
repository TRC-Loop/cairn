#!/usr/bin/env node
// SPDX-License-Identifier: AGPL-3.0-or-later
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const localesDir = resolve(here, '..', 'src', 'lib', 'i18n', 'locales');

function load(name) {
	const path = resolve(localesDir, `${name}.json`);
	return JSON.parse(readFileSync(path, 'utf8'));
}

const en = load('en');
const de = load('de');

const enKeys = new Set(Object.keys(en));
const deKeys = new Set(Object.keys(de));

const onlyEn = [...enKeys].filter((k) => !deKeys.has(k)).sort();
const onlyDe = [...deKeys].filter((k) => !enKeys.has(k)).sort();

if (onlyEn.length === 0 && onlyDe.length === 0) {
	console.log(`i18n parity OK (${enKeys.size} keys)`);
	process.exit(0);
}

if (onlyEn.length) {
	console.error(`Keys in en.json missing from de.json (${onlyEn.length}):`);
	for (const k of onlyEn) console.error(`  - ${k}`);
}
if (onlyDe.length) {
	console.error(`Keys in de.json missing from en.json (${onlyDe.length}):`);
	for (const k of onlyDe) console.error(`  - ${k}`);
}
process.exit(1);
