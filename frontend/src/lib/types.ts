export type CheckType = 'ip' | 'domain';

export interface IpResponse {
	IP: string;
	ValidIP: boolean;
	BlackListed: boolean;
	Status: boolean;
	BlackList: Record<string, string[]>;
	Errors: string[];
	TimeTaken: number;
	Cached: boolean;
	CacheKey: string;
}

export interface DomainResponse {
	Domain: string;
	ValidDomain: boolean;
	BlackListed: boolean;
	Status: boolean;
	BlackList: Record<string, string[]>;
	Errors: string[];
	TimeTaken: number;
	Cached: boolean;
	CacheKey: string;
}

export interface CheckIpRequest {
	ip: string;
	blacklists: string[];
	nameservers?: string[];
}

export interface CheckDomainRequest {
	domain: string;
	blacklists: string[];
	nameservers?: string[];
}

export interface HealthResponse {
	Alive: boolean;
	Redis: boolean;
	RedisConnections: number;
	CachedItems: number;
	Uptime: number;
	GoVersion: string;
	Version: string;
	MemoryAlloc: number;
}

export interface HistoryItem {
	type: CheckType;
	value: string;
	timestamp: number;
	result: IpResponse | DomainResponse;
}
