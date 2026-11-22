import type { IpResponse, DomainResponse } from './types.ts';

/**
 * What a check result actually tells the user.
 *
 * A result that is not blacklisted but carries errors (a DNS timeout, a blacklist
 * refusing the query) is not "clean": some lists were never answered. It used to
 * be shown as "Clean - Not blacklisted", a confident verdict the backend never
 * gave. A positive listing stays "listed" even with errors on other lists, since
 * the listing itself was confirmed.
 */
export type Verdict = 'listed' | 'clean' | 'incomplete';

export function verdictOf(result: IpResponse | DomainResponse): Verdict {
	if (result.BlackListed) return 'listed';
	if (result.Errors && result.Errors.length > 0) return 'incomplete';
	return 'clean';
}

export const verdictLabel: Record<Verdict, string> = {
	listed: 'Blacklisted',
	clean: 'Clean',
	incomplete: 'Incomplete'
};

// Tailwind text colour per verdict, shared by the results card and the history.
export const verdictTextClass: Record<Verdict, string> = {
	listed: 'text-destructive',
	clean: 'text-success',
	incomplete: 'text-warning'
};
