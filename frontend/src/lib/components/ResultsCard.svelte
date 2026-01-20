<script lang="ts">
	import { CheckCircle, XCircle, AlertTriangle, Clock, Database, Copy, Trash2 } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import type { IpResponse, DomainResponse, CheckType } from '$lib/types';
	import { clearCache } from '$lib/api';

	interface Props {
		result: IpResponse | DomainResponse;
		checkType: CheckType;
	}

	let { result, checkType }: Props = $props();

	let clearingCache = $state(false);

	// Derived icon components
	const StatusIcon = $derived(result.BlackListed ? XCircle : CheckCircle);
	const ValidIcon = $derived(result.Status ? CheckCircle : AlertTriangle);

	function copyToClipboard(text: string) {
		navigator.clipboard.writeText(text);
		toast.success('Copied to clipboard');
	}

	function copyJSON() {
		const json = JSON.stringify(result, null, 2);
		copyToClipboard(json);
	}

	async function handleClearCache() {
		// Use CacheKey from API response instead of constructing manually
		const cacheKey = result.CacheKey;
		const displayKey = checkType === 'ip' ? (result as IpResponse).IP : (result as DomainResponse).Domain;
		
		if (!cacheKey) {
			toast.error('No cache key available');
			return;
		}
		
		if (!confirm(`Clear cache for ${displayKey}?`)) {
			return;
		}

		clearingCache = true;
		try {
			await clearCache(cacheKey);
			toast.success('Cache cleared successfully');
		} catch (error) {
			toast.error('Failed to clear cache');
			console.error('Cache clear error:', error);
		} finally {
			clearingCache = false;
		}
	}

	function getStatusColor(blacklisted: boolean): string {
		return blacklisted ? 'text-destructive' : 'text-success';
	}

	function getStatusIcon(blacklisted: boolean) {
		return blacklisted ? XCircle : CheckCircle;
	}

	function getStatusText(blacklisted: boolean): string {
		return blacklisted ? 'Blacklisted' : 'Clean';
	}
</script>

<div class="rounded-lg border border-border bg-card p-6 space-y-6">
	<!-- Header -->
	<div class="flex items-start justify-between">
		<div class="flex-1">
			<div class="flex items-center gap-3 mb-2">
				<StatusIcon
					class={`h-8 w-8 ${getStatusColor(result.BlackListed)}`}
				/>
				<div>
					<h3 class="text-2xl font-bold">{getStatusText(result.BlackListed)}</h3>
					<p class="text-sm text-muted-foreground">
						{checkType === 'ip' ? (result as IpResponse).IP : (result as DomainResponse).Domain}
					</p>
				</div>
			</div>
		</div>

		<div class="flex gap-2">
			<button
				onclick={copyJSON}
				class="rounded-lg p-2 hover:bg-accent transition-colors"
				title="Copy JSON"
			>
				<Copy class="h-4 w-4" />
			</button>
			<button
				onclick={handleClearCache}
				disabled={clearingCache}
				class="rounded-lg p-2 hover:bg-accent transition-colors disabled:opacity-50"
				title="Clear cache"
			>
				<Trash2 class="h-4 w-4" />
			</button>
		</div>
	</div>

	<!-- Metadata -->
	<div class="flex flex-wrap gap-4 text-sm">
		<div class="flex items-center gap-2">
			<Clock class="h-4 w-4 text-muted-foreground" />
			<span class="text-muted-foreground">Time:</span>
			<span class="font-medium">{result.TimeTaken.toFixed(3)}s</span>
		</div>
		<div class="flex items-center gap-2">
			<Database class="h-4 w-4 text-muted-foreground" />
			<span class="text-muted-foreground">Cached:</span>
			<span class="font-medium">{result.Cached ? 'Yes' : 'No'}</span>
		</div>
		<div class="flex items-center gap-2">
			<ValidIcon
				class={`h-4 w-4 ${result.Status ? 'text-success' : 'text-destructive'}`}
			/>
			<span class="text-muted-foreground">Valid:</span>
			<span class="font-medium">
				{checkType === 'ip'
					? (result as IpResponse).ValidIP
						? 'Yes'
						: 'No'
					: (result as DomainResponse).ValidDomain
						? 'Yes'
						: 'No'}
			</span>
		</div>
	</div>

	<!-- Cache Key (collapsible detail for advanced users) -->
	{#if result.CacheKey}
		<details class="text-xs">
			<summary class="cursor-pointer text-muted-foreground hover:text-foreground transition-colors">
				Cache Key
			</summary>
			<div class="mt-2 rounded border border-border bg-muted/50 p-2">
				<div class="flex items-center justify-between gap-2">
					<code class="font-mono text-xs break-all">{result.CacheKey}</code>
					<button
						onclick={() => copyToClipboard(result.CacheKey)}
						class="shrink-0 rounded p-1 hover:bg-accent transition-colors"
						title="Copy cache key"
					>
						<Copy class="h-3 w-3" />
					</button>
				</div>
			</div>
		</details>
	{/if}

	<!-- Blacklist Results -->
	{#if result.BlackListed && Object.keys(result.BlackList).length > 0}
		<div>
			<h4 class="text-lg font-semibold mb-3">Blacklist Detections</h4>
			<div class="space-y-2">
				{#each Object.entries(result.BlackList) as [blacklist, ips]}
					<div class="rounded-lg border border-destructive/50 bg-destructive/10 p-3">
						<div class="flex items-start justify-between gap-2">
							<div class="flex-1">
								<p class="font-medium text-destructive">{blacklist}</p>
								<div class="mt-1 flex flex-wrap gap-2">
									{#each ips as ip}
										<span
											class="inline-flex items-center rounded bg-destructive/20 px-2 py-1 text-xs font-mono text-destructive"
										>
											{ip}
										</span>
									{/each}
								</div>
							</div>
							<button
								onclick={() => copyToClipboard(blacklist)}
								class="rounded p-1 hover:bg-destructive/20 transition-colors"
								title="Copy blacklist name"
							>
								<Copy class="h-3 w-3" />
							</button>
						</div>
					</div>
				{/each}
			</div>
		</div>
	{:else if !result.BlackListed}
		<div class="rounded-lg border border-success/50 bg-success/10 p-4 text-center">
			<CheckCircle class="h-8 w-8 text-success mx-auto mb-2" />
			<p class="font-medium text-success">Not found in any blacklists</p>
			<p class="text-sm text-muted-foreground mt-1">This {checkType} has a clean reputation</p>
		</div>
	{/if}

	<!-- Errors -->
	{#if result.Errors && result.Errors.length > 0}
		<div>
			<h4 class="text-lg font-semibold mb-3 flex items-center gap-2">
				<AlertTriangle class="h-5 w-5 text-destructive" />
				Errors
			</h4>
			<div class="space-y-2">
				{#each result.Errors as error}
					<div class="rounded-lg border border-destructive bg-destructive/10 p-3">
						<p class="text-sm text-destructive">{error}</p>
					</div>
				{/each}
			</div>
		</div>
	{/if}
</div>
