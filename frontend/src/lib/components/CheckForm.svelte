<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { Search, Settings, X, Loader2, AlertCircle } from 'lucide-svelte';
	import ResultsCard from './ResultsCard.svelte';
	import type { CheckType, HistoryItem, IpResponse, DomainResponse } from '$lib/types';
	import { checkIp, checkDomain, postCheckIp, postCheckDomain } from '$lib/api';
	import { ipSchema, domainSchema } from '$lib/validators';

	interface Props {
		selectedItem: HistoryItem | null;
		onCheckComplete: (item: HistoryItem) => void;
	}

	let { selectedItem, onCheckComplete }: Props = $props();

	let checkType = $state<CheckType>('ip');
	let inputValue = $state('');
	let customBlacklists = $state('');
	let customNameservers = $state('');
	let showAdvanced = $state(false);
	let loading = $state(false);
	let result = $state<IpResponse | DomainResponse | null>(null);
	let validationError = $state<string | null>(null);

	// Update form when history item is selected
	$effect(() => {
		if (selectedItem) {
			checkType = selectedItem.type;
			inputValue = selectedItem.value;
			result = selectedItem.result;
			customBlacklists = '';
			customNameservers = '';
			showAdvanced = false;
		}
	});

	function validateInput(): boolean {
		try {
			if (checkType === 'ip') {
				ipSchema.parse(inputValue);
			} else {
				domainSchema.parse(inputValue);
			}
			validationError = null;
			return true;
		} catch (error: any) {
			validationError = error.errors[0]?.message || 'Invalid input';
			return false;
		}
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		
		if (!validateInput()) {
			toast.error(validationError || 'Invalid input');
			return;
		}

		loading = true;
		result = null;

		try {
			let response: IpResponse | DomainResponse;

			// Parse custom inputs if provided
			const blacklists = customBlacklists
				.split('\n')
				.map((bl) => bl.trim())
				.filter((bl) => bl.length > 0);
			const nameservers = customNameservers
				.split('\n')
				.map((ns) => ns.trim())
				.filter((ns) => ns.length > 0);

			// Use POST endpoint if custom blacklists are provided
			if (blacklists.length > 0) {
				if (checkType === 'ip') {
					response = await postCheckIp({
						ip: inputValue,
						blacklists,
						nameservers: nameservers.length > 0 ? nameservers : undefined
					});
				} else {
					response = await postCheckDomain({
						domain: inputValue,
						blacklists,
						nameservers: nameservers.length > 0 ? nameservers : undefined
					});
				}
			} else {
				// Use GET endpoint for default blacklists
				if (checkType === 'ip') {
					response = await checkIp(inputValue);
				} else {
					response = await checkDomain(inputValue);
				}
			}

			result = response;

			// Add to history
			const historyItem: HistoryItem = {
				type: checkType,
				value: inputValue,
				timestamp: Date.now(),
				result: response
			};
			onCheckComplete(historyItem);

			if (response.BlackListed) {
				toast.error('Found in blacklists!');
			} else {
				toast.success('Clean - Not blacklisted');
			}
		} catch (error) {
			const message = error instanceof Error ? error.message : 'Check failed';
			toast.error(message);
			console.error('Check error:', error);
		} finally {
			loading = false;
		}
	}

	function handleInputChange() {
		if (validationError) {
			validateInput();
		}
	}

	function clearForm() {
		inputValue = '';
		customBlacklists = '';
		customNameservers = '';
		result = null;
		validationError = null;
	}
</script>

<div class="space-y-6">
	<div class="rounded-lg border border-border bg-card p-6">
		<h2 class="text-2xl font-bold mb-6">Check Reputation</h2>

		<form onsubmit={handleSubmit} class="space-y-4">
			<!-- Type Selection -->
			<div class="flex gap-2">
				<button
					type="button"
					onclick={() => {
						checkType = 'ip';
						clearForm();
					}}
					class={`flex-1 rounded-lg px-4 py-2 font-medium transition-colors ${
						checkType === 'ip'
							? 'bg-primary text-primary-foreground'
							: 'bg-secondary text-secondary-foreground hover:bg-accent'
					}`}
				>
					Check IP
				</button>
				<button
					type="button"
					onclick={() => {
						checkType = 'domain';
						clearForm();
					}}
					class={`flex-1 rounded-lg px-4 py-2 font-medium transition-colors ${
						checkType === 'domain'
							? 'bg-primary text-primary-foreground'
							: 'bg-secondary text-secondary-foreground hover:bg-accent'
					}`}
				>
					Check Domain
				</button>
			</div>

			<!-- Input Field -->
			<div>
				<label for="input" class="block text-sm font-medium mb-2">
					{checkType === 'ip' ? 'IP Address' : 'Domain Name'}
				</label>
				<div class="relative">
					<input
						id="input"
						type="text"
						bind:value={inputValue}
						oninput={handleInputChange}
						placeholder={checkType === 'ip' ? '8.8.8.8' : 'example.com'}
						class={`w-full rounded-lg border ${validationError ? 'border-destructive' : 'border-input'} bg-background px-4 py-2 pr-10 focus:outline-none focus:ring-2 focus:ring-ring`}
						disabled={loading}
					/>
					{#if inputValue}
						<button
							type="button"
							onclick={clearForm}
							class="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
						>
							<X class="h-5 w-5" />
						</button>
					{/if}
				</div>
				{#if validationError}
					<div class="mt-2 flex items-center gap-2 text-sm text-destructive">
						<AlertCircle class="h-4 w-4" />
						<span>{validationError}</span>
					</div>
				{/if}
			</div>

			<!-- Advanced Options -->
			<div>
				<button
					type="button"
					onclick={() => (showAdvanced = !showAdvanced)}
					class="flex items-center gap-2 text-sm font-medium text-muted-foreground hover:text-foreground transition-colors"
				>
					<Settings class="h-4 w-4" />
					Advanced Options
				</button>

				{#if showAdvanced}
					<div class="mt-4 space-y-4 rounded-lg border border-border bg-muted/30 p-4">
						<div>
							<label for="blacklists" class="block text-sm font-medium mb-2">
								Custom Blacklists (one per line, max 20)
							</label>
							<textarea
								id="blacklists"
								bind:value={customBlacklists}
								placeholder="zen.spamhaus.org&#10;bl.spamcop.net"
								rows="4"
								class="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
								disabled={loading}
							></textarea>
						</div>

						<div>
							<label for="nameservers" class="block text-sm font-medium mb-2">
								Custom Nameservers (one per line, max 3)
							</label>
							<textarea
								id="nameservers"
								bind:value={customNameservers}
								placeholder="8.8.8.8&#10;1.1.1.1"
								rows="3"
								class="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
								disabled={loading}
							></textarea>
						</div>
					</div>
				{/if}
			</div>

			<!-- Submit Button -->
			<button
				type="submit"
				disabled={loading || !inputValue}
				class="w-full rounded-lg bg-primary px-4 py-3 font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex items-center justify-center gap-2"
			>
				{#if loading}
					<Loader2 class="h-5 w-5 animate-spin" />
					Checking...
				{:else}
					<Search class="h-5 w-5" />
					Check Now
				{/if}
			</button>
		</form>
	</div>

	<!-- Results -->
	{#if result}
		<ResultsCard {result} {checkType} />
	{/if}
</div>
