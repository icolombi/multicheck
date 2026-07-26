import type {
	IpResponse,
	DomainResponse,
	CheckIpRequest,
	CheckDomainRequest,
	HealthResponse,
	ClearCacheResponse
} from './types';

const API_BASE = '/api';

// Slightly above the backend httpWriteTimeout (30s) so a server-side timeout is
// reported as such, rather than being masked by the client aborting first.
const REQUEST_TIMEOUT_MS = 35_000;

/**
 * Single entry point for every API call.
 *
 * It exists to fix three problems the per-function fetch calls shared: requests
 * could hang forever (leaving the UI spinner stuck), the JSON error body returned
 * by the backend was discarded in favour of an often-empty `statusText`, and a
 * non-JSON response (e.g. an nginx HTML error page) surfaced as an opaque
 * SyntaxError.
 */
async function request<T>(path: string, init?: RequestInit): Promise<T> {
	let response: Response;

	try {
		response = await fetch(`${API_BASE}${path}`, {
			...init,
			cache: 'no-store',
			signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS)
		});
	} catch (error) {
		if (error instanceof DOMException && error.name === 'TimeoutError') {
			throw new Error(`Request timed out after ${REQUEST_TIMEOUT_MS / 1000}s`);
		}
		throw new Error('Cannot reach the API. Is the backend running?');
	}

	const body = await parseJson(response);

	if (!response.ok) {
		throw new Error(errorMessage(response, body));
	}

	return body as T;
}

// Returns null instead of throwing when the response is not JSON, so the caller
// can still build a meaningful error message from the status code.
async function parseJson(response: Response): Promise<unknown> {
	try {
		return await response.json();
	} catch {
		return null;
	}
}

// The backend reports failures in an `Errors[]` array; prefer it over statusText,
// which is empty over HTTP/2.
function errorMessage(response: Response, body: unknown): string {
	const errors = (body as { Errors?: unknown })?.Errors;
	if (Array.isArray(errors) && errors.length > 0) {
		return errors.join('; ');
	}
	return response.statusText
		? `Request failed: ${response.status} ${response.statusText}`
		: `Request failed with status ${response.status}`;
}

function jsonPost(data: unknown): RequestInit {
	return {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(data)
	};
}

export async function checkIp(ip: string): Promise<IpResponse> {
	return request<IpResponse>(`/ip/${encodeURIComponent(ip)}`);
}

export async function checkDomain(domain: string): Promise<DomainResponse> {
	return request<DomainResponse>(`/domain/${encodeURIComponent(domain)}`);
}

export async function postCheckIp(data: CheckIpRequest): Promise<IpResponse> {
	return request<IpResponse>('/ip/check', jsonPost(data));
}

export async function postCheckDomain(data: CheckDomainRequest): Promise<DomainResponse> {
	return request<DomainResponse>('/domain/check', jsonPost(data));
}

export async function getHealth(): Promise<HealthResponse> {
	return request<HealthResponse>('/health');
}

export async function clearCache(key: string): Promise<ClearCacheResponse> {
	// DELETE, not GET: the endpoint is destructive and must not be reachable by
	// browser prefetch or a cross-site GET.
	return request<ClearCacheResponse>(`/clear-cache/${encodeURIComponent(key)}`, {
		method: 'DELETE'
	});
}
