// SPDX-License-Identifier: AGPL-3.0-or-later
import { apiRequest } from './client';

export async function fetchSetupStatus(): Promise<{ setup_complete: boolean }> {
	return apiRequest('/api/setup/status');
}
