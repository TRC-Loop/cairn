// SPDX-License-Identifier: AGPL-3.0-or-later
import { apiRequest } from './client';

export type DBStats = {
	path: string;
	size_bytes: number;
	rows: Record<string, number>;
	health: 'good' | 'caution' | 'warning';
};

export async function getDBStats(): Promise<DBStats> {
	return apiRequest<DBStats>('/api/system/db-stats');
}
