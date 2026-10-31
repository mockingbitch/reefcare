// Thin typed client for the ReefCare API (api/openapi.yaml). Types come from
// `npm run gen:api`; never hand-edit schema.d.ts.
import type { components } from './schema';

type Schemas = components['schemas'];
export type User = Schemas['User'];
export type Tank = Schemas['Tank'];
export type TankCreate = Schemas['TankCreate'];
export type TankPatch = Schemas['TankPatch'];
export type ProfilePatch = Schemas['ProfilePatch'];
export type Share = Schemas['Share'];
export type WaterChange = Schemas['WaterChange'];
export type WaterChangeSchedule = Schemas['WaterChangeSchedule'];
export type WaterChangeScheduleCreate = Schemas['WaterChangeScheduleCreate'];
export type WaterChangeSchedulePatch = Schemas['WaterChangeSchedulePatch'];
export type WaterChangeRecord = Schemas['WaterChangeRecord'];
export type WaterChangeEstimate = Schemas['WaterChangeEstimate'];
export type RecurrencePreview = { rrule: string; next_occurrences: string[] };
export type DosingProduct = Schemas['DosingProduct'];
export type DosingPlan = Schemas['DosingPlan'];
export type DosingPlanCreate = Schemas['DosingPlanCreate'];
export type DosingPlanPatch = Schemas['DosingPlanPatch'];
export type DoseEvent = Schemas['DoseEvent'];
export type DoseUnit = Schemas['DoseUnit'];
export type DosingForm = Schemas['DosingForm'];
export type Equipment = Schemas['Equipment'];
export type EquipmentCreate = Schemas['EquipmentCreate'];
export type EquipmentPatch = Schemas['EquipmentPatch'];
export type EquipmentConfiguration = Schemas['EquipmentConfiguration'];
export type MaintenancePlan = Schemas['MaintenancePlan'];
export type MaintenancePlanCreate = Schemas['MaintenancePlanCreate'];
export type MaintenancePlanPatch = Schemas['MaintenancePlanPatch'];
export type MaintenanceEvent = Schemas['MaintenanceEvent'];
export type Species = Schemas['Species'];
export type LivestockEntry = Schemas['LivestockEntry'];
export type LivestockEntryCreate = Schemas['LivestockEntryCreate'];
export type LivestockEntryPatch = Schemas['LivestockEntryPatch'];
export type LivestockEvent = Schemas['LivestockEvent'];
export type Notification = Schemas['Notification'];
export type TimelineEvent = Schemas['TimelineEvent'];
export type ParameterType = Schemas['ParameterType'];
export type WaterTest = Schemas['WaterTest'];
export type WaterTestCreate = Schemas['WaterTestCreate'];
export type Reading = Schemas['Reading'];
export type Target = Schemas['Target'];
export type TargetInput = Schemas['TargetInput'];
export type LatestReading = Schemas['LatestReading'];
export type Trends = Schemas['Trends'];
export type KnowledgeVersion = Schemas['KnowledgeVersion'];
export type KnowledgeVersionInput = Schemas['KnowledgeVersionInput'];
export type KnowledgeStructured = Schemas['KnowledgeStructured'];
export type KnowledgeArticle = Schemas['KnowledgeArticle'];
export type KnowledgeSource = Schemas['KnowledgeSource'];
export type ClaimClass = Schemas['ClaimClass'];
export type KnowledgeCategory = Schemas['KnowledgeCategory'];
export type Assessment = Schemas['Assessment'];
export type Finding = Schemas['Finding'];
export type I18nText = Schemas['I18nText'];
export type AssessmentFeedback = Schemas['AssessmentFeedback'];
export type Page<T> = { items: T[]; next_cursor: string | null };

export type ErrorDetail = { field?: string; code: string; params?: Record<string, string> };

/** A failed call. `code` is the API error code, or `network_error` when no response came back. */
export type ApiError = {
	status: number;
	code: string;
	message: string;
	details: ErrorDetail[];
	retryAfter?: number;
};

export type Result<T> = { ok: true; status: number; data: T } | { ok: false; error: ApiError };

/** The CSRF token the API set at sign-in (`__Host-` prefixed over https). */
export function csrfToken(cookies: string = globalThis.document?.cookie ?? ''): string {
	for (const part of cookies.split(';')) {
		const [name, ...rest] = part.trim().split('=');
		if (name === '__Host-rc_csrf' || name === 'rc_csrf') return decodeURIComponent(rest.join('='));
	}
	return '';
}

const unsafe = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

/** Listeners called when any request answers 401, e.g. to clear the session. */
const unauthenticatedListeners = new Set<() => void>();
export function onUnauthenticated(fn: () => void): () => void {
	unauthenticatedListeners.add(fn);
	return () => unauthenticatedListeners.delete(fn);
}

export async function api<T>(
	method: Method,
	path: string,
	body?: unknown,
	fetchFn: typeof fetch = fetch,
	extraHeaders: Record<string, string> = {}
): Promise<Result<T>> {
	const headers: Record<string, string> = { Accept: 'application/json', ...extraHeaders };
	if (body !== undefined) headers['Content-Type'] = 'application/json';
	if (unsafe.has(method)) {
		const token = csrfToken();
		if (token) headers['X-CSRF-Token'] = token;
	}
	let res: Response;
	try {
		res = await fetchFn(`/api/v1${path}`, {
			method,
			headers,
			body: body === undefined ? undefined : JSON.stringify(body),
			credentials: 'same-origin'
		});
	} catch {
		return {
			ok: false,
			error: { status: 0, code: 'network_error', message: 'network error', details: [] }
		};
	}
	if (res.status === 204) return { ok: true, status: 204, data: undefined as T };
	let json: unknown = null;
	try {
		json = await res.json();
	} catch {
		// Non-JSON body (proxy error page): handled below.
	}
	if (res.ok) return { ok: true, status: res.status, data: json as T };

	const env = (json as { error?: Partial<ApiError> & { details?: ErrorDetail[] } } | null)?.error;
	const retry = Number(res.headers.get('Retry-After'));
	const error: ApiError = {
		status: res.status,
		code: env?.code ?? (res.status >= 500 ? 'internal_error' : 'unknown'),
		message: env?.message ?? res.statusText,
		details: env?.details ?? [],
		retryAfter: Number.isFinite(retry) && retry > 0 ? retry : undefined
	};
	if (res.status === 401) unauthenticatedListeners.forEach((fn) => fn());
	return { ok: false, error };
}
