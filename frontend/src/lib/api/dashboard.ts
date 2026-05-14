// SPDX-License-Identifier: AGPL-3.0-or-later
import { apiRequest } from './client';

export type DashboardSummary = {
	monitors: {
		total: number;
		up: number;
		down: number;
		degraded: number;
		unknown: number;
		paused: number;
	};
	active_incidents: {
		count: number;
		latest_title?: string;
		latest_id?: number;
	};
	uptime_24h: {
		percentage: number;
		previous_24h_percentage: number;
		has_current: boolean;
		has_previous: boolean;
	};
	maintenance: {
		in_progress_count: number;
		next_window?: { id: number; title: string; starts_at: string };
	};
	recent_activity: Array<{
		type: string;
		id: number;
		label: string;
		timestamp: string;
	}>;
};

export async function getDashboard(): Promise<DashboardSummary> {
	return apiRequest<DashboardSummary>('/api/dashboard');
}
