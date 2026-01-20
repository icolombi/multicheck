<script lang="ts">
	import CheckForm from '$lib/components/CheckForm.svelte';
	import HistoryPanel from '$lib/components/HistoryPanel.svelte';
	import type { HistoryItem } from '$lib/types';

	let historyItems = $state<HistoryItem[]>([]);
	let selectedItem = $state<HistoryItem | null>(null);

	function handleHistorySelect(item: HistoryItem) {
		selectedItem = item;
	}

	function handleCheckComplete(item: HistoryItem) {
		historyItems = [item, ...historyItems.slice(0, 19)]; // Keep last 20
		selectedItem = null;
	}
</script>

<svelte:head>
	<title>Multicheck - DNSBL Reputation Checker</title>
	<meta name="description" content="Check IP and domain reputation against DNS blacklists" />
</svelte:head>

<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
	<div class="lg:col-span-2">
		<CheckForm {selectedItem} onCheckComplete={handleCheckComplete} />
	</div>
	<div>
		<HistoryPanel items={historyItems} onSelect={handleHistorySelect} />
	</div>
</div>
