# Multicheck Frontend

Modern, responsive frontend for the Multicheck DNSBL Reputation API built with SvelteKit.

## Features

- ✅ Check IP addresses and domains against DNS blacklists
- ⚡ Real-time validation and feedback
- 🎨 Beautiful dark/light mode support
- 📊 Detailed results with blacklist information
- 📜 Check history with quick re-checking
- 🏥 System health dashboard
- 🔧 Advanced options for custom blacklists and nameservers
- 📱 Fully responsive mobile-first design
- 🎯 TypeScript for type safety

## Tech Stack

- **SvelteKit** - Fast, modern framework with excellent DX
- **Tailwind CSS** - Utility-first styling with dark mode
- **Lucide Svelte** - Beautiful, consistent icons
- **Zod** - Schema validation
- **Svelte Sonner** - Toast notifications
- **TypeScript** - Type safety throughout

## Prerequisites

- Node.js 18+ and npm
- Multicheck API running on `http://localhost:8080`

## Installation

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview
```

The app will be available at `http://localhost:5173`

## API Proxy

The Vite dev server is configured to proxy API requests from `/api/*` to `http://localhost:8080`. This avoids CORS issues during development.

To change the API endpoint, edit `vite.config.ts`.

## Project Structure

```
src/
├── lib/
│   ├── components/
│   │   ├── CheckForm.svelte       # Main check form with validation
│   │   ├── ResultsCard.svelte     # Display check results
│   │   └── HistoryPanel.svelte    # Recent checks sidebar
│   ├── api.ts                     # API client functions
│   ├── types.ts                   # TypeScript type definitions
│   └── validators.ts              # Zod schemas for validation
├── routes/
│   ├── +layout.svelte             # App layout with header/footer
│   ├── +page.svelte               # Home page (check interface)
│   └── health/
│       └── +page.svelte           # Health status dashboard
└── app.css                        # Global styles and Tailwind config
```

## Usage

### Check IP or Domain

1. Select "Check IP" or "Check Domain" tab
2. Enter the value to check
3. (Optional) Click "Advanced Options" to specify custom blacklists/nameservers
4. Click "Check Now"

### View Results

Results show:
- ✓/✗ Status (blacklisted or clean)
- Response time and cache status
- List of blacklists with detections
- Specific IP codes returned by each blacklist
- Any errors encountered

### History

- Recent checks appear in the right sidebar
- Click any history item to reload it in the form
- Clear history with the trash icon

### Health Dashboard

Visit `/health` to see:
- API online status
- Redis connection status
- System uptime
- Memory usage
- Version information

## Development

```bash
# Type checking
npm run check

# Format code
npm run format

# Lint code
npm run lint
```

## Building for Production

```bash
npm run build
```

The build output will be in the `build/` directory. You can deploy it to any static hosting service or Node.js server.

## Environment Variables

No environment variables needed by default. API endpoint is configured in `vite.config.ts`.

## License

Same as parent project (Multicheck API)
