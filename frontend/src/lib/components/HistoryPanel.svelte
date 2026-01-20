<script lang="ts">
	import { History, Trash2, Globe, Network } from 'lucide-svelte';
	import type { HistoryItem } from '$lib/types';

	interface Props {
		items: HistoryItem[];
		onSelect: (item: HistoryItem) => void;
	}

	let { items, onSelect }: Props = $props();

	function formatTime(timestamp: number): string {
		const date = new Date(timestamp);
		const now = new Date();
		const diff = now.getTime() - date.getTime();
		const minutes = Math.floor(diff / 60000);
		const hours = Math.floor(diff / 3600000);
		const days = Math.floor(diff / 86400000);

		if (minutes < 1) return 'Just now';
		if (minutes < 60) return `${minutes}m ago`;
		if (hours < 24) return `${hours}h ago`;
		return `${days}d ago`;
	}

	function clearHistory() {
		if (confirm('Clear all history?')) {
			items = [];
		}
	}

	function getStatusClass(item: HistoryItem): string {
		return item.result.BlackListed ? 'border-destructive/50' : 'border-success/50';
	}

	function getStatusIcon(item: HistoryItem) {
		return item.type === 'ip' ? Network : Globe;
	}
</script>

<div class="rounded-lg border border-border bg-card p-6">
	<div class="flex items-center justify-between mb-4">
		<h3 class="text-lg font-semibold flex items-center gap-2">
			<History class="h-5 w-5" />
			Recent Checks
		</h3>
		{#if items.length > 0}
			<button
				onclick={clearHistory}
				class="text-sm text-muted-foreground hover:text-foreground transition-colors"
				title="Clear history"
			>
				<Trash2 class="h-4 w-4" />
			</button>
		{/if}
	</div>

	{#if items.length === 0}
		<div class="text-center py-8 text-muted-foreground">
			<History class="h-12 w-12 mx-auto mb-3 opacity-50" />
			<p class="text-sm">No checks yet</p>
		</div>
	{:else}
		<div class="space-y-2 max-h-[600px] overflow-y-auto">
			{#each items as item (item.timestamp)}
				{@const Icon = getStatusIcon(item)}
				<button
					onclick={() => onSelect(item)}
					class={`w-full text-left rounded-lg border ${getStatusClass(item)} bg-card p-3 hover:bg-accent transition-colors`}
				>
					<div class="flex items-start gap-3">
						<Icon
							class={`h-5 w-5 mt-0.5 ${item.result.BlackListed ? 'text-destructive' : 'text-success'}`}
						/>
						<div class="flex-1 min-w-0">
							<p class="font-medium truncate">{item.value}</p>
							<div class="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
								<span>{formatTime(item.timestamp)}</span>
								<span>•</span>
								<span>{item.result.TimeTaken.toFixed(2)}s</span>
								{#if item.result.Cached}
									<span>•</span>
									<span>Cached</span>
								{/if}
							</div>
						</div>
						<div
							class={`text-xs font-medium px-2 py-1 rounded ${item.result.BlackListed ? 'bg-destructive/20 text-destructive' : 'bg-success/20 text-success'}`}
						>
							{item.result.BlackListed ? 'Listed' : 'Clean'}
						</div>
					</div>
				</button>
			{/each}
		</div>
	{/if}
</div>
