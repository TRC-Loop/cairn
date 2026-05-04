// SPDX-License-Identifier: AGPL-3.0-or-later
import { redirect } from '@sveltejs/kit';

export const ssr = false;

export function load() {
	throw redirect(307, '/settings/profile');
}
