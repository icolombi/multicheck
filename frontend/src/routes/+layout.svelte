<script lang="ts">
	import '../app.css';
	import { Toaster } from 'svelte-sonner';
	import { Moon, Sun } from 'lucide-svelte';
	import { onMount } from 'svelte';
	import type { Snippet } from 'svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();
	let darkMode = $state(false);

	onMount(() => {
		// Check system preference or localStorage
		const stored = localStorage.getItem('darkMode');
		if (stored !== null) {
			darkMode = stored === 'true';
		} else {
			darkMode = window.matchMedia('(prefers-color-scheme: dark)').matches;
		}
		updateTheme();
	});

	function toggleDarkMode() {
		darkMode = !darkMode;
		localStorage.setItem('darkMode', darkMode.toString());
		updateTheme();
	}

	function updateTheme() {
		if (darkMode) {
			document.documentElement.classList.add('dark');
		} else {
			document.documentElement.classList.remove('dark');
		}
	}
</script>

<div class="min-h-screen bg-background">
	<Toaster position="top-right" richColors />

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
					<a
						href="/"
						class="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
					>
						Check
					</a>
					<a
						href="/health"
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
