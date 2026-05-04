// SPDX-License-Identifier: AGPL-3.0-or-later
import { get } from 'svelte/store';
import { _ } from 'svelte-i18n';

export type FieldErrors = Record<string, string>;

export function fieldErrorText(code: string | undefined | null): string {
	if (!code) return '';
	const t = get(_);
	const key = `errors.fields.${code}`;
	const localized = t(key);
	return localized !== key ? localized : code;
}

export type Rule<T> = (value: T) => string | null;

export function required<T>(): Rule<T> {
	return (v) => {
		if (v === null || v === undefined) return 'required';
		if (typeof v === 'string' && v.trim().length === 0) return 'required';
		if (Array.isArray(v) && v.length === 0) return 'required';
		return null;
	};
}

export function minLength(n: number): Rule<string> {
	return (v) => (typeof v === 'string' && v.length < n ? 'too_short' : null);
}

export function maxLength(n: number): Rule<string> {
	return (v) => (typeof v === 'string' && v.length > n ? 'too_long' : null);
}

export function pattern(regex: RegExp, code = 'invalid_format'): Rule<string> {
	return (v) => (typeof v === 'string' && v.length > 0 && !regex.test(v) ? code : null);
}

export function range(min: number, max: number): Rule<number> {
	return (v) => {
		if (typeof v !== 'number' || Number.isNaN(v)) return 'invalid_value';
		if (v < min || v > max) return 'out_of_range';
		return null;
	};
}

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
export function email(): Rule<string> {
	return (v) => (typeof v === 'string' && v.length > 0 && !EMAIL_RE.test(v) ? 'invalid_format' : null);
}

const URL_RE = /^https?:\/\/[^\s]+$/i;
export function url(): Rule<string> {
	return (v) => (typeof v === 'string' && v.length > 0 && !URL_RE.test(v) ? 'invalid_format' : null);
}

export function validateField<T>(value: T, rules: Rule<T>[]): string | null {
	for (const rule of rules) {
		const err = rule(value);
		if (err) return err;
	}
	return null;
}

export function validateForm<T extends Record<string, unknown>>(
	values: T,
	schema: { [K in keyof T]?: Rule<T[K]>[] }
): FieldErrors {
	const errors: FieldErrors = {};
	for (const key in schema) {
		const rules = schema[key];
		if (!rules) continue;
		const err = validateField(values[key], rules);
		if (err) errors[key] = err;
	}
	return errors;
}
