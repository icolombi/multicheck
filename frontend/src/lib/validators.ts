import { z } from 'zod';

export const ipSchema = z
	.string()
	.min(1, 'IP address is required')
	.refine(
		(ip) => {
			const parts = ip.split('.');
			if (parts.length !== 4) return false;
			return parts.every((part) => {
				// The octet must be digits only. parseInt alone accepts trailing
				// garbage ("1x" -> 1), which let "1x.2.3.4" through to the backend.
				if (!/^\d{1,3}$/.test(part)) return false;
				const num = Number(part);
				return num >= 0 && num <= 255;
			});
		},
		// IPv6 is intentionally rejected: the configured DNSBL zones are IPv4-only,
		// and the backend rejects it too.
		{ message: 'Invalid IPv4 address format' }
	);

export const domainSchema = z
	.string()
	.min(1, 'Domain is required')
	.refine(
		(domain) => {
			// Basic domain validation
			const domainRegex = /^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$/;
			return domainRegex.test(domain);
		},
		{ message: 'Invalid domain format' }
	);

export const blacklistSchema = z
	.array(z.string().min(1))
	.min(1, 'At least one blacklist is required')
	.max(20, 'Maximum 20 blacklists allowed');

export const nameserverSchema = z
	.array(ipSchema)
	.max(3, 'Maximum 3 nameservers allowed')
	.optional();
