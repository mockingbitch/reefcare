// Timestamps arrive in UTC and are shown in the profile's time zone (frontend rules).

const fallbackZone = 'Asia/Ho_Chi_Minh';

function zoneOrFallback(timeZone: string | undefined): string {
	try {
		new Intl.DateTimeFormat('en', { timeZone });
		return timeZone ?? fallbackZone;
	} catch {
		return fallbackZone;
	}
}

export function formatDateTime(iso: string, locale: string, timeZone?: string): string {
	return new Intl.DateTimeFormat(locale, {
		dateStyle: 'medium',
		timeStyle: 'short',
		timeZone: zoneOrFallback(timeZone)
	}).format(new Date(iso));
}

/** Formats a calendar date (YYYY-MM-DD) without shifting it across time zones. */
export function formatDate(date: string, locale: string): string {
	const [y, mo, d] = date.split('-').map(Number);
	return new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeZone: 'UTC' }).format(
		new Date(Date.UTC(y, mo - 1, d))
	);
}

/** Displays a decimal string with the locale's decimal separator, keeping every digit. */
export function formatDecimal(value: string, locale: string): string {
	const sep = locale.startsWith('vi') ? ',' : '.';
	return value.replace('.', sep);
}

/** Today's date (YYYY-MM-DD) in the given zone, for date input limits. */
export function today(timeZone?: string): string {
	return new Intl.DateTimeFormat('en-CA', { timeZone: zoneOrFallback(timeZone) }).format(
		new Date()
	);
}
