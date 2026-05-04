// SPDX-License-Identifier: AGPL-3.0-or-later
import { redirect } from '@sveltejs/kit';

export const ssr = false;

export const load = ({ url }) => {
	const params = new URLSearchParams(url.searchParams);
	params.set('new', '1');
	throw redirect(307, `/notifications?${params.toString()}`);
};
