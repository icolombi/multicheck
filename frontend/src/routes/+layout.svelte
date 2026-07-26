<script lang="ts">
	import '../app.css';
	import { Toaster } from 'svelte-sonner';
	import { Moon, Sun } from 'lucide-svelte';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import type { Snippet } from 'svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();
	let darkMode = $state(false);

	onMount(() => {
		// The theme itself is already applied by the blocking script in app.html, which
		// runs before first paint and avoids the flash of light theme. Here we only
		// mirror the resulting state so the toggle icon starts out correct.
		darkMode = document.documentElement.classList.contains('dark');
	});

	function toggleDarkMode() {
		darkMode = !darkMode;
		localStorage.setItem('darkMode', darkMode.toString());
		document.documentElement.classList.toggle('dark', darkMode);
	}
</script>

<div class="min-h-screen bg-background">
	<!-- Without an explicit theme the toasts stay light-coloured in dark mode. -->
	<Toaster position="top-right" richColors theme={darkMode ? 'dark' : 'light'} />

	<header class="border-b border-border bg-card">
		<div class="container mx-auto px-4 py-4">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-3">
					<div class="flex h-10 w-10 items-center justify-center rounded-lg bg-primary">
						<span class="text-xl font-bold text-primary-foreground">MC</span>
					</div>
					<div>
						<h1 class="text-xl font-bold text-foreground">Multicheck</h1>
						<p class="text-xs text-muted-foreground">DNSBL Reputation Checker</p>
					</div>
				</div>

				<nav class="flex items-center gap-4">
					<!-- resolve() keeps the links correct if the app is ever served
					     under a base path; a bare href would silently break. -->
					<a
						href={resolve('/')}
						class="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
					>
						Check
					</a>
					<a
						href={resolve('/health')}
						class="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
					>
						Health
					</a>
					<button
						onclick={toggleDarkMode}
						class="rounded-lg p-2 hover:bg-accent transition-colors"
						aria-label="Toggle dark mode"
					>
						{#if darkMode}
							<Sun class="h-5 w-5" />
						{:else}
							<Moon class="h-5 w-5" />
						{/if}
					</button>
				</nav>
			</div>
		</div>
	</header>

	<main class="container mx-auto px-4 py-8">
		{@render children()}
	</main>

	<footer class="border-t border-border py-6 mt-12">
		<div class="container mx-auto px-4 text-center text-sm text-muted-foreground">
			<p>Multicheck API Frontend - Built with SvelteKit & Tailwind CSS</p>
		</div>
	</footer>
</div>
