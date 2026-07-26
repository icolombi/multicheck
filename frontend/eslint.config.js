import js from '@eslint/js';
import ts from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';
import prettier from 'eslint-config-prettier/flat';
import globals from 'globals';
import svelteConfig from './svelte.config.js';

// ESLint 9 flat config. The order of the entries matters: the two prettier
// entries come last so they switch off the stylistic rules that would otherwise
// fight with `npm run format`.
export default ts.config(
	js.configs.recommended,
	...ts.configs.recommended,
	...svelte.configs.recommended,
	prettier,
	...svelte.configs.prettier,

	{
		languageOptions: {
			globals: {
				...globals.browser,
				// vite.config.ts and svelte.config.js run in Node.
				...globals.node
			}
		}
	},

	{
		files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
		languageOptions: {
			parserOptions: {
				// Lets the Svelte parser hand <script lang="ts"> blocks to the
				// TypeScript parser, and resolve $lib aliases through svelte.config.js.
				parser: ts.parser,
				svelteConfig
			}
		}
	},

	{
		// Generated or vendored output: never our code to fix.
		ignores: ['build/', '.svelte-kit/', 'dist/', 'node_modules/', 'static/']
	}
);
