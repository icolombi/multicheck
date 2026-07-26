<script lang="ts">
	import { onMount } from 'svelte';
	import { Activity, Database, Clock, Cpu, Package } from 'lucide-svelte';
	import type { HealthResponse } from '$lib/types';
	import { getHealth } from '$lib/api';

	let health = $state<HealthResponse | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(() => {
		loadHealth();
		// Refresh every 5 seconds, but skip polling while the tab is hidden: there is
		// nobody to read the result and it keeps hitting the backend indefinitely.
		const interval = setInterval(() => {
			if (!document.hidden) loadHealth();
		}, 5000);

		// Refresh immediately when the tab becomes visible again, so the user is not
		// looking at data that went stale while the tab was in the background.
		const onVisibilityChange = () => {
			if (!document.hidden) loadHealth();
		};
		document.addEventListener('visibilitychange', onVisibilityChange);

		return () => {
			clearInterval(interval);
			document.removeEventListener('visibilitychange', onVisibilityChange);
		};
	});

	async function loadHealth() {
		try {
			health = await getHealth();
			error = null;
		} catch (e) {
			// Keep the last known-good `health` so a single failed poll does not wipe
			// the dashboard; the error is surfaced as a stale-data banner instead.
			error = e instanceof Error ? e.message : 'Failed to load health status';
		} finally {
			loading = false;
		}
	}

	function formatUptime(seconds: number): string {
		const hours = Math.floor(seconds / 3600);
		const minutes = Math.floor((seconds % 3600) / 60);
		const secs = Math.floor(seconds % 60);
		return `${hours}h ${minutes}m ${secs}s`;
	}

	function formatBytes(bytes: number): string {
		return `${(bytes / 1024).toFixed(2)} MB`;
	}
</script>

<svelte:head>
	<title>Health Status - Multicheck</title>
</svelte:head>

<div class="max-w-4xl mx-auto">
	<h1 class="text-3xl font-bold mb-6">System Health</h1>

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<div
				class="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent"
			></div>
		</div>
	{:else}
		{#if error}
			<div
				class="mb-6 rounded-lg border border-destructive bg-destructive/10 p-4"
				role="status"
				aria-live="polite"
			>
				<p class="text-destructive">
					{health ? `Showing last known data — refresh failed: ${error}` : error}
				</p>
			</div>
		{/if}

		{#if health}
			<div class="grid grid-cols-1 md:grid-cols-2 gap-6">
				<!-- API Status -->
				<div class="rounded-lg border border-border bg-card p-6">
					<div class="flex items-center gap-3 mb-4">
						<Activity class="h-5 w-5 text-primary" />
						<h2 class="text-lg font-semibold">API Status</h2>
					</div>
					<div class="flex items-center gap-2">
						<div
							class={`h-3 w-3 rounded-full ${health.Alive ? 'bg-success' : 'bg-destructive'}`}
						></div>
						<span class="text-sm font-medium">{health.Alive ? 'Online' : 'Offline'}</span>
					</div>
				</div>

				<!-- Redis Status -->
				<div class="rounded-lg border border-border bg-card p-6">
					<div class="flex items-center gap-3 mb-4">
						<Database class="h-5 w-5 text-primary" />
						<h2 class="text-lg font-semibold">Redis</h2>
					</div>
					<div class="space-y-2">
						<div class="flex items-center gap-2">
							<div
								class={`h-3 w-3 rounded-full ${health.Redis ? 'bg-success' : 'bg-destructive'}`}
							></div>
							<span class="text-sm font-medium">{health.Redis ? 'Connected' : 'Disconnected'}</span>
						</div>
						<div class="text-sm text-muted-foreground">
							Connections: {health.RedisConnections}
						</div>
						<div class="text-sm text-muted-foreground">
							Cached Items: {health.CachedItems}
						</div>
					</div>
				</div>

				<!-- Uptime -->
				<div class="rounded-lg border border-border bg-card p-6">
					<div class="flex items-center gap-3 mb-4">
						<Clock class="h-5 w-5 text-primary" />
						<h2 class="text-lg font-semibold">Uptime</h2>
					</div>
					<p class="text-2xl font-bold">{formatUptime(health.Uptime / 1_000_000_000)}</p>
				</div>

				<!-- Memory -->
				<div class="rounded-lg border border-border bg-card p-6">
					<div class="flex items-center gap-3 mb-4">
						<Cpu class="h-5 w-5 text-primary" />
						<h2 class="text-lg font-semibold">Memory</h2>
					</div>
					<p class="text-2xl font-bold">{formatBytes(health.MemoryAlloc)}</p>
				</div>

				<!-- Version Info -->
				<div class="rounded-lg border border-border bg-card p-6 md:col-span-2">
					<div class="flex items-center gap-3 mb-4">
						<Package class="h-5 w-5 text-primary" />
						<h2 class="text-lg font-semibold">Version Information</h2>
					</div>
					<div class="grid grid-cols-2 gap-4 text-sm">
						<div>
							<p class="text-muted-foreground">Go Version</p>
							<p class="font-medium">{health.GoVersion}</p>
						</div>
						<div>
							<p class="text-muted-foreground">App Version</p>
							<p class="font-medium">{health.Version}</p>
						</div>
					</div>
				</div>
			</div>
		{/if}
	{/if}
</div>
