import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: 'tests/e2e',
	timeout: 30_000,
	// Run serially (one worker). Every spec drives the SAME live stack
	// (`make up`: one shared Postgres/Redis/backend/frontend, no per-worker
	// isolation), so concurrency buys little correctness signal here but costs
	// a lot of reliability on a shared, resource-bound dev machine:
	//   1. The /auth/* endpoints share ONE IP-keyed fixed-window rate limit
	//      (30 req/min, backend/internal/modules/auth/handler.go publicLimit) —
	//      all specs' register/login calls come from the same localhost IP, so
	//      parallel workers burst past it and flake with transient RATE_LIMITED.
	//   2. Each parallel worker holds its own Chromium context (~hundreds of MB);
	//      this repo's dev box runs with NO swap (`sysctl vm.swapusage` → 0), so
	//      two browsers plus the stack tip peak memory past free RAM and the OS
	//      SIGKILLs a worker — observed flaking exactly the timing/state-sensitive
	//      specs (cart tax preview, domain-nameserver reflection) that pass every
	//      time when run alone.
	// One worker removes both failure modes; specs already use unique per-run
	// data so nothing depended on cross-spec parallelism for correctness. It's
	// slower (~3-4min) but deterministic. A machine with more free RAM / swap
	// can safely raise this (e.g. `--workers=2`).
	workers: 1,
	use: {
		baseURL: 'http://localhost:5173'
	},
	// Two projects, run sequentially via `dependencies`:
	//
	// - `shared-stack`: every normal spec — they drive the one live stack
	//   (`make up`: mockserver + api + worker + frontend on :5173) that the
	//   webServer block below reuses.
	// - `isolated-install`: ONLY install.spec.ts. A normal spec can't cover the
	//   installation wizard — the shared stack is always already "installed"
	//   once cmd/seed runs, so it needs its OWN fresh, not-yet-installed
	//   instance. To keep this feasible on a memory-bound, swap-less dev box,
	//   install.spec drives that instance at the HTTP-API level (Playwright's
	//   `request` fixture — no browser, no second frontend), spinning up only a
	//   throwaway DB + a second `cmd/api`. `dependencies: ['shared-stack']`
	//   still runs it LAST and alone so its brief `go build` spike doesn't
	//   coincide with another spec's live browser. (The frontend wizard PAGE +
	//   hooks-gate are covered manually + by reused, already-tested patterns;
	//   see install.spec.ts's header for the full rationale.)
	projects: [
		{
			name: 'shared-stack',
			testIgnore: '**/install.spec.ts'
		},
		{
			name: 'isolated-install',
			testMatch: '**/install.spec.ts',
			dependencies: ['shared-stack']
		}
	],
	webServer: {
		command: 'npm run dev',
		url: 'http://localhost:5173',
		reuseExistingServer: !process.env.CI,
		timeout: 120_000
	}
});
