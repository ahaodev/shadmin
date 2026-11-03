// A session revision changes only on sign-in/sign-out, never on token refresh.
// Requests and asynchronous loaders retain the revision they started with.
class SessionLifecycle {
  version = 0
  private controller = new AbortController()
  private resetHandlers = new Set<() => void>()

  get signal(): AbortSignal {
    return this.controller.signal
  }

  isCurrent(version: number): boolean {
    return version === this.version
  }

  onReset(handler: () => void): () => void {
    this.resetHandlers.add(handler)
    return () => this.resetHandlers.delete(handler)
  }

  reset(): void {
    this.version++
    this.controller.abort()
    this.controller = new AbortController()
    this.resetHandlers.forEach((handler) => handler())
  }
}

export const session = new SessionLifecycle()
