# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

```bash
# Start development server
npm run dev

# Build for production
npm run build

# Lint code
npm run lint

# Format code (check)
npm run format:check

# Format code (write)
npm run format

# Preview production build
npm run preview

# Check unused dependencies
npm run knip
```

## Project Architecture

This is a **shadcn-admin** dashboard built with Vite, React, and TypeScript. The project uses TanStack Router for routing and TanStack Query for data fetching.

### Key Technologies
- **Build Tool**: Vite with React SWC plugin
- **UI Framework**: ShadcnUI (TailwindCSS + RadixUI)
- **Routing**: TanStack Router with file-based routing
- **State Management**: Zustand for auth store
- **Data Fetching**: TanStack Query with Axios
- **Authentication**: Clerk (partial implementation)
- **Styling**: TailwindCSS v4 with custom RTL support

### Directory Structure
- `src/features/` - Feature-based modules (auth, dashboard, tasks, users, etc.)
- `src/components/` - Shared UI components and layout components
- `src/routes/` - TanStack Router file-based routing
- `src/stores/` - Zustand state management
- `src/context/` - React context providers (theme, direction, font)
- `src/lib/` - Utility functions and helpers
- `src/hooks/` - Custom React hooks

### Routing System
Uses TanStack Router with file-based routing. Route tree is auto-generated in `src/routeTree.gen.ts`. Two main layout patterns:
- `(auth)/` - Authentication pages (sign-in, sign-up, etc.)
- `_authenticated/` - Protected pages requiring authentication

### Component Architecture
- **Layout Components**: `src/components/layout/` contains sidebar, header, and main layout
- **UI Components**: `src/components/ui/` contains ShadcnUI components (some customized for RTL)
- **Feature Components**: Each feature has its own components directory

### Customized ShadcnUI Components
Some components have been modified for RTL support and other improvements:
- **Modified**: scroll-area, sonner, separator
- **RTL Updated**: alert-dialog, calendar, command, dialog, dropdown-menu, select, table, sheet, sidebar, switch

### Path Alias
- `@/*` resolves to `./src/*`

### Development Notes
- Uses React 19 with StrictMode
- TanStack Router devtools enabled in development
- React Query devtools enabled in development
- Error handling with global error boundaries and toast notifications
- Theme system supports light/dark mode with system preference
- RTL language support with DirectionProvider