import type {
	IpResponse,
	DomainResponse,
	CheckIpRequest,
	CheckDomainRequest,
	HealthResponse
} from './types';

const API_BASE = '/api';

export async function checkIp(ip: string): Promise<IpResponse> {
	const response = await fetch(`${API_BASE}/ip/${ip}`, { cache: 'no-store' });
	if (!response.ok) {
		throw new Error(`Failed to check IP: ${response.statusText}`);
	}
	return response.json();
}

export async function checkDomain(domain: string): Promise<DomainResponse> {
	const response = await fetch(`${API_BASE}/domain/${domain}`, { cache: 'no-store' });
	if (!response.ok) {
		throw new Error(`Failed to check domain: ${response.statusText}`);
	}
	return response.json();
}

export async function postCheckIp(data: CheckIpRequest): Promise<IpResponse> {
	const response = await fetch(`${API_BASE}/ip/check`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json'
		},
		body: JSON.stringify(data),
		cache: 'no-store'
	});
	if (!response.ok) {
		throw new Error(`Failed to check IP: ${response.statusText}`);
	}
	return response.json();
}

export async function postCheckDomain(data: CheckDomainRequest): Promise<DomainResponse> {
	const response = await fetch(`${API_BASE}/domain/check`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json'
		},
		body: JSON.stringify(data),
		cache: 'no-store'
	});
	if (!response.ok) {
		throw new Error(`Failed to check domain: ${response.statusText}`);
	}
	return response.json();
}

export async function getHealth(): Promise<HealthResponse> {
	const response = await fetch(`${API_BASE}/health`, { cache: 'no-store' });
	if (!response.ok) {
		throw new Error(`Failed to get health status: ${response.statusText}`);
	}
	return response.json();
}

export async function clearCache(key: string): Promise<{ Status: boolean; Errors: string[] }> {
	const response = await fetch(`${API_BASE}/clear-cache/${key}`, { cache: 'no-store' });
	if (!response.ok) {
		throw new Error(`Failed to clear cache: ${response.statusText}`);
	}
	return response.json();
}
