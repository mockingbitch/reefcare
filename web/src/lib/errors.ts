// Maps API error codes to localized text. The API's `message` is an English fallback
// only; users always see these translations (docs/api-guidelines.md §2).
import { m } from '#lib/paraglide/messages.js';
import type { ApiError, ErrorDetail } from '#lib/api/client.js';

export function errorMessage(e: ApiError): string {
	switch (e.code) {
		case 'network_error':
			return m.err_network();
		case 'internal_error':
			return m.err_internal();
		case 'unauthenticated':
		case 'session_expired':
			return m.err_unauthenticated();
		case 'invalid_credentials':
			return m.err_invalid_credentials();
		case 'csrf_failed':
			return m.err_csrf_failed();
		case 'account_suspended':
			return m.err_account_suspended();
		case 'email_not_verified':
			return m.err_email_not_verified();
		case 'forbidden':
			return m.err_forbidden();
		case 'not_found':
			return m.err_not_found();
		case 'rate_limited':
			return m.err_rate_limited({ seconds: String(e.retryAfter ?? 60) });
		case 'server_busy':
			return m.err_server_busy();
		case 'version_conflict':
			return m.err_version_conflict();
		case 'username_taken':
			return m.err_username_taken();
		case 'tank_archived':
			return m.err_tank_archived();
		case 'limit_reached':
			return m.err_limit_reached();
		case 'duplicate':
			return m.err_duplicate();
		case 'invalid_token':
			return m.err_invalid_token();
		case 'already_resolved':
			return m.err_already_resolved();
		case 'schedule_ended':
			return m.err_schedule_ended();
		case 'already_verified':
			return m.err_already_verified();
		case 'validation_failed':
		case 'malformed_json':
		case 'unknown_field':
			return m.err_validation_failed();
		case 'body_too_large':
			return m.err_body_too_large();
		default:
			return m.err_unknown({ code: e.code });
	}
}

export function fieldMessage(d: ErrorDetail): string {
	const p = d.params ?? {};
	switch (d.code) {
		case 'required':
			return m.f_required();
		case 'length':
			return p.min
				? m.f_length({ min: p.min, max: p.max ?? '' })
				: m.f_too_long_max({ max: p.max ?? '' });
		case 'invalid_characters':
			return m.f_invalid_characters();
		case 'invalid_email':
			return m.f_invalid_email();
		case 'invalid_format':
			return m.f_invalid_format();
		case 'invalid_timezone':
			return m.f_invalid_timezone();
		case 'unsupported':
			return m.f_unsupported();
		case 'too_short':
			return m.f_too_short({ min: p.min ?? '10' });
		case 'too_long':
			return m.f_too_long({ max: p.max ?? '128' });
		case 'too_common':
			return m.f_too_common();
		case 'matches_email':
			return m.f_matches_email();
		case 'incorrect':
			return m.f_incorrect();
		case 'invalid_decimal':
			return m.f_invalid_decimal();
		case 'must_be_positive':
			return m.f_must_be_positive();
		case 'must_not_be_negative':
			return m.f_must_not_be_negative();
		case 'out_of_range':
			return p.max_integer_digits
				? m.f_out_of_range_decimal({
						max_integer_digits: p.max_integer_digits,
						max_decimals: p.max_decimals ?? '0'
					})
				: m.f_out_of_range();
		case 'invalid_date':
			return m.f_invalid_date();
		case 'grantee_not_found':
			return m.f_grantee_not_found();
		case 'cannot_share_with_self':
			return m.f_cannot_share_with_self();
		case 'exactly_one_of':
			return m.f_exactly_one_of();
		case 'unsupported_unit':
			return m.f_unsupported_unit();
		case 'invalid_rrule':
			return m.f_invalid_rrule();
		case 'unsupported_rrule_part':
			return m.f_unsupported_rrule_part();
		case 'before_start':
			return m.f_before_start();
		case 'in_future':
			return m.f_in_future();
		default:
			return m.f_invalid();
	}
}

/** Field-level messages keyed by API field name (e.g. "dimensions_cm.width"). */
export function fieldErrors(e: ApiError | null): Record<string, string> {
	const out: Record<string, string> = {};
	for (const d of e?.details ?? []) {
		if (d.field && !(d.field in out)) out[d.field] = fieldMessage(d);
	}
	return out;
}
