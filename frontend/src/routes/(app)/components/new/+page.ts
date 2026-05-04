// SPDX-License-Identifier: AGPL-3.0-or-later
import { redirect } from '@sveltejs/kit';

export const ssr = false;

export const load = () => {
	throw redirect(307, '/components?new=1');
};
