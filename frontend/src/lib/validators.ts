import { z } from 'zod';

export const ipSchema = z
	.string()
	.min(1, 'IP address is required')
	.refine(
		(ip) => {
			const parts = ip.split('.');
			if (parts.length !== 4) return false;
			return parts.every((part) => {
				const num = parseInt(part, 10);
				return !isNaN(num) && num >= 0 && num <= 255;
			});
		},
		{ message: 'Invalid IP address format' }
	);

export const domainSchema = z
	.string()
	.min(1, 'Domain is required')
	.refine(
		(domain) => {
			// Basic domain validation
			const domainRegex =
				/^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$/;
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
